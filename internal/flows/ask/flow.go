/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package ask implements the workflow for answering questions about code using RAG.
package ask

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/activities/retrievecontext"
	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// CommandParametersReader reads command-line parameters and flags.
type CommandParametersReader interface {
	GetQuestionFlag() (string, error)
	GetSnapshotsFlag() ([]string, error)
	GetTopKFlag() (int, error)
	GetMinScoreFlag() (float32, error)
	GetProfileFlag() (string, error)
}

// ConfigReader provides access to ask configuration.
type ConfigReader interface {
	Get() (*config.Config, error)
}

// Factory creates instances of the ask flow.
type Factory struct {
	retrieveContextFactory executor.ActivityFactory[*retrievecontext.Input, *retrievecontext.Output]
	invokeLLMFactory       executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
	parametersReader       CommandParametersReader
	profileResolver        ports.ProfileResolver
	outputWriter           ports.OutputWriter
	configReader           ConfigReader
}

// NewFactory creates a new ask flow factory.
func NewFactory(
	retrieveContextFactory executor.ActivityFactory[*retrievecontext.Input, *retrievecontext.Output],
	invokeLLMFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output],
	parametersReader CommandParametersReader,
	profileResolver ports.ProfileResolver,
	outputWriter ports.OutputWriter,
	configReader ConfigReader,
) (*Factory, error) {
	if retrieveContextFactory == nil {
		return nil, fmt.Errorf("ask.NewFactory: retrieveContextFactory cannot be nil")
	}
	if invokeLLMFactory == nil {
		return nil, fmt.Errorf("ask.NewFactory: invokeLLMFactory cannot be nil")
	}
	if parametersReader == nil {
		return nil, fmt.Errorf("ask.NewFactory: parametersReader cannot be nil")
	}
	if profileResolver == nil {
		return nil, fmt.Errorf("ask.NewFactory: profileResolver cannot be nil")
	}
	if outputWriter == nil {
		return nil, fmt.Errorf("ask.NewFactory: outputWriter cannot be nil")
	}
	if configReader == nil {
		return nil, fmt.Errorf("ask.NewFactory: configReader cannot be nil")
	}

	return &Factory{
		retrieveContextFactory: retrieveContextFactory,
		invokeLLMFactory:       invokeLLMFactory,
		parametersReader:       parametersReader,
		profileResolver:        profileResolver,
		outputWriter:           outputWriter,
		configReader:           configReader,
	}, nil
}

// NewFlow creates and returns the ask flow function.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		flowCtx.SendRunning("Starting ask flow")

		// Read configuration
		cfg, err := f.configReader.Get()
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		if cfg.Ask == nil {
			return fmt.Errorf("ask configuration is missing")
		}

		// Read command parameters
		question, err := f.parametersReader.GetQuestionFlag()
		if err != nil {
			return fmt.Errorf("failed to get question: %w", err)
		}

		if question == "" {
			return fmt.Errorf("question is required")
		}

		snapshots, err := f.parametersReader.GetSnapshotsFlag()
		if err != nil {
			return fmt.Errorf("failed to get snapshots: %w", err)
		}

		if len(snapshots) == 0 {
			// Default to searching workdir, stage, and head
			snapshots = []string{"_workdir_", "_stage_", "_head_"}
		}

		topK, err := f.parametersReader.GetTopKFlag()
		if err != nil {
			return fmt.Errorf("failed to get topK: %w", err)
		}

		if topK <= 0 {
			// Use config default if flag not set
			topK = cfg.Ask.TopK
			if topK <= 0 {
				topK = 10 // Fallback default
			}
		}

		minScore, err := f.parametersReader.GetMinScoreFlag()
		if err != nil {
			return fmt.Errorf("failed to get min score: %w", err)
		}

		if minScore <= 0 {
			// Use config default if flag not set
			minScore = cfg.Ask.MinScore
			if minScore < 0 {
				minScore = 0.0 // Fallback default
			}
		}

		profileName, err := f.parametersReader.GetProfileFlag()
		if err != nil {
			return fmt.Errorf("failed to get profile: %w", err)
		}

		if profileName == "" {
			// Use config default if flag not set
			profileName = cfg.Ask.Profile
			if profileName == "" {
				return fmt.Errorf("profile is required (set in config ask.profile or via --profile flag)")
			}
		}

		// Resolve profile
		resolvedProfile, err := f.profileResolver.Get(profile.Profile(profileName))
		if err != nil {
			return fmt.Errorf("failed to resolve profile '%s': %w", profileName, err)
		}

		exec := flowCtx.GetExecutor()
		if exec == nil {
			return fmt.Errorf("executor not available in context")
		}

		// Step 1: Retrieve formatted context using retrieval activity
		retrieveContextActivity := f.retrieveContextFactory.NewActivity()
		retrieveContextInput := &retrievecontext.Input{
			QueryText:        question,
			SnapshotPriority: snapshots,
			TopK:             topK,
			MinScore:         minScore,
		}

		retrieveContextFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "RetrieveContext", retrieveContextActivity, retrieveContextInput)
		retrieveContextOutput, err := retrieveContextFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("context retrieval failed: %w", err)
		}

		// Check if we found any results
		if retrieveContextOutput.Context == "" {
			if err := f.outputWriter.PrintLine("No relevant context found to answer the question."); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
			flowCtx.SendCompleted("No context found")
			return nil
		}

		// Step 2: Execute LLM invocation to generate answer
		invokeLLMActivity := f.invokeLLMFactory.NewActivity()

		// Use default system prompt for RAG
		systemPrompt := `You are a helpful AI assistant that answers questions based on the provided context.

Instructions:
- Use ONLY the information from the context to answer the question
- If the context doesn't contain enough information to fully answer the question, say so
- Be concise but thorough
- Cite specific files/locations from the context when relevant
- If the question cannot be answered with the given context, clearly state that`

		userPrompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s", retrieveContextOutput.Context, question)

		invokeLLMInput := &invokellm.Input{
			Profile:      resolvedProfile,
			SystemPrompt: systemPrompt,
			UserPrompt:   userPrompt,
		}

		invokeLLMFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "InvokeLLM", invokeLLMActivity, invokeLLMInput)
		invokeLLMOutput, err := invokeLLMFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("answer generation failed: %w", err)
		}

		if err := f.outputWriter.PrintLine(strings.TrimSpace(invokeLLMOutput.Content)); err != nil {
			return fmt.Errorf("failed to print generated content: %w", err)
		}

		flowCtx.SendCompleted("Ask flow completed")
		return nil
	}
}
