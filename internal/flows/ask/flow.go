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

// Package ask provides flows for RAG-based question answering.
package ask

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	queryactivity "github.com/retran/meowg1k/internal/activities/query"
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

// Factory creates instances of the ask flow.
type Factory struct {
	queryFactory     executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output]
	invokeLLMFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
	parametersReader CommandParametersReader
	profileResolver  ports.ProfileResolver
	outputWriter     ports.OutputWriter
}

// NewFactory creates a new ask flow factory.
func NewFactory(
	queryFactory executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output],
	invokeLLMFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output],
	parametersReader CommandParametersReader,
	profileResolver ports.ProfileResolver,
	outputWriter ports.OutputWriter,
) (*Factory, error) {
	if queryFactory == nil {
		return nil, fmt.Errorf("ask.NewFactory: queryFactory cannot be nil")
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

	return &Factory{
		queryFactory:     queryFactory,
		invokeLLMFactory: invokeLLMFactory,
		parametersReader: parametersReader,
		profileResolver:  profileResolver,
		outputWriter:     outputWriter,
	}, nil
}

// NewFlow creates and returns the ask flow function.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		flowCtx.SendRunning("Starting ask flow")

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
			topK = 10 // Default
		}

		minScore, err := f.parametersReader.GetMinScoreFlag()
		if err != nil {
			return fmt.Errorf("failed to get min score: %w", err)
		}

		if minScore < 0 {
			minScore = 0.0 // Default
		}

		profileName, err := f.parametersReader.GetProfileFlag()
		if err != nil {
			return fmt.Errorf("failed to get profile: %w", err)
		}

		if profileName == "" {
			return fmt.Errorf("profile is required")
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

		// Step 1: Execute query activity to retrieve context
		queryActivity := f.queryFactory.NewActivity()
		queryInput := &queryactivity.Input{
			QueryText:        question,
			SnapshotPriority: snapshots,
			TopK:             topK,
			MinScore:         minScore,
		}

		queryFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "Query", queryActivity, queryInput)
		queryOutput, err := queryFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("query failed: %w", err)
		}

		// Check if we found any results
		if len(queryOutput.Results) == 0 {
			if err := f.outputWriter.PrintLine("No relevant context found to answer the question."); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
			flowCtx.SendCompleted("No context found")
			return nil
		}

		// Step 2: Format context from search results
		var contextBuilder strings.Builder
		contextBuilder.WriteString("# Retrieved Context\n\n")
		for i, result := range queryOutput.Results {
			contextBuilder.WriteString(fmt.Sprintf("## Source %d (Score: %.4f)\n", i+1, result.Score))
			contextBuilder.WriteString(fmt.Sprintf("**File:** %s (Lines %d-%d)\n\n", result.FilePath, result.StartLine, result.EndLine))
			contextBuilder.WriteString("```\n")
			contextBuilder.WriteString(result.TextContent)
			contextBuilder.WriteString("\n```\n\n")
		}
		context := contextBuilder.String()

		// Step 3: Execute LLM invocation to generate answer
		invokeLLMActivity := f.invokeLLMFactory.NewActivity()

		// Use default system prompt for RAG
		systemPrompt := `You are a helpful AI assistant that answers questions based on the provided context.

Instructions:
- Use ONLY the information from the context to answer the question
- If the context doesn't contain enough information to fully answer the question, say so
- Be concise but thorough
- Cite specific files/locations from the context when relevant
- If the question cannot be answered with the given context, clearly state that`

		userPrompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s", context, question)

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

		// Step 4: Output the answer
		if err := f.outputWriter.PrintLine(fmt.Sprintf("Question: %s\n", question)); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}

		if err := f.outputWriter.PrintLine(fmt.Sprintf("\nAnswer:\n%s\n", invokeLLMOutput.Content)); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}

		if err := f.outputWriter.PrintLine(fmt.Sprintf("\nSources: %d documents", len(queryOutput.Results))); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}

		flowCtx.SendCompleted("Ask flow completed")
		return nil
	}
}
