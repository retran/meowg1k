// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package ask implements the workflow for answering questions about code using RAG.
package ask

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/draftcontent"
	"github.com/retran/meowg1k/internal/activities/fetchcontext"
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

// ConfigReader provides access to answer configuration.
type ConfigReader interface {
	Get() (*config.Config, error)
}

// Factory creates instances of the ask flow.
type Factory struct {
	retrieveContextFactory executor.ActivityFactory[*fetchcontext.Input, *fetchcontext.Output]
	invokeLLMFactory       executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]
	parametersReader       CommandParametersReader
	profileResolver        ports.ProfileResolver
	outputWriter           ports.OutputWriter
	configReader           ConfigReader
}

// NewFactory creates a new ask flow factory.
func NewFactory(
	retrieveContextFactory executor.ActivityFactory[*fetchcontext.Input, *fetchcontext.Output],
	invokeLLMFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output],
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
		return f.runAskFlow(ctx, flowCtx)
	}
}

const defaultAnswerSystemPrompt = `You are a helpful AI assistant that answers questions based on the provided context.

Instructions:
- Use ONLY the information from the context to answer the question
- If the context doesn't contain enough information to fully answer the question, say so
- Be concise but thorough
- Cite specific files/locations from the context when relevant
- If the question cannot be answered with the given context, clearly state that`

func (f *Factory) runAskFlow(ctx context.Context, flowCtx *executor.Context) error {
	cfg, err := f.loadAnswerConfig()
	if err != nil {
		return err
	}

	params, err := f.resolveAnswerParams(cfg)
	if err != nil {
		return err
	}
	flowCtx.SendRunningWithDetails(
		"I'm answering the question",
		fmt.Sprintf(
			"question=%q\nsnapshots=%s\ntop_k=%d\nmin_score=%.2f",
			params.question,
			strings.Join(params.snapshots, ","),
			params.topK,
			params.minScore,
		),
	)

	resolvedProfile, err := f.profileResolver.Get(profile.Profile(params.profileName))
	if err != nil {
		return fmt.Errorf("failed to resolve profile '%s': %w", params.profileName, err)
	}

	exec, err := f.getExecutor(flowCtx)
	if err != nil {
		return err
	}

	retrieveContextOutput, err := f.runRetrieveContext(ctx, flowCtx, exec, params)
	if err != nil {
		return err
	}

	if retrieveContextOutput.Context == "" {
		return f.handleEmptyContext(flowCtx, params.question)
	}

	userPrompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s", retrieveContextOutput.Context, params.question)

	invokeLLMOutput, err := f.runInvokeLLM(ctx, flowCtx, exec, resolvedProfile, defaultAnswerSystemPrompt, userPrompt)
	if err != nil {
		return err
	}

	if err := f.outputWriter.PrintLine(strings.TrimSpace(invokeLLMOutput.Content)); err != nil {
		return fmt.Errorf("failed to print generated content: %w", err)
	}

	flowCtx.SendCompletedWithDetails("I've answered the question", fmt.Sprintf("question=%q", params.question))

	return nil
}

func (f *Factory) handleEmptyContext(flowCtx *executor.Context, question string) error {
	if err := f.outputWriter.PrintLine("No relevant context found to answer the question."); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	flowCtx.SendCompletedWithDetails("I couldn't find relevant context", fmt.Sprintf("question=%q", question))
	return nil
}

type answerParams struct {
	question    string
	profileName string
	snapshots   []string
	topK        int
	minScore    float32
}

func (f *Factory) loadAnswerConfig() (*config.Config, error) {
	cfg, err := f.configReader.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	if cfg.Answer == nil {
		return nil, fmt.Errorf("answer configuration is missing")
	}
	return cfg, nil
}

func (f *Factory) resolveAnswerParams(cfg *config.Config) (*answerParams, error) {
	question, err := f.parametersReader.GetQuestionFlag()
	if err != nil {
		return nil, fmt.Errorf("failed to get question: %w", err)
	}
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}

	snapshots, err := f.parametersReader.GetSnapshotsFlag()
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots: %w", err)
	}
	if len(snapshots) == 0 {
		snapshots = []string{"_workdir_", "_stage_", "_head_"}
	}

	topK, err := f.parametersReader.GetTopKFlag()
	if err != nil {
		return nil, fmt.Errorf("failed to get topK: %w", err)
	}
	if topK <= 0 {
		topK = defaultTopK(cfg.Answer.TopK)
	}

	minScore, err := f.parametersReader.GetMinScoreFlag()
	if err != nil {
		return nil, fmt.Errorf("failed to get min score: %w", err)
	}
	if minScore <= 0 {
		minScore = defaultMinScore(cfg.Answer.MinScore)
	}

	profileName, err := f.parametersReader.GetProfileFlag()
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}
	if profileName == "" {
		profileName = cfg.Answer.Profile
		if profileName == "" {
			return nil, fmt.Errorf("profile is required (set in config answer.profile or via --profile flag)")
		}
	}

	return &answerParams{
		question:    question,
		profileName: profileName,
		snapshots:   snapshots,
		topK:        topK,
		minScore:    minScore,
	}, nil
}

func defaultTopK(configValue int) int {
	if configValue > 0 {
		return configValue
	}
	return 10
}

func defaultMinScore(configValue float32) float32 {
	if configValue > 0 {
		return configValue
	}
	if configValue < 0 {
		return 0.0
	}
	return configValue
}

func (f *Factory) getExecutor(flowCtx *executor.Context) (executor.Executor, error) {
	exec := flowCtx.GetExecutor()
	if exec == nil {
		return nil, fmt.Errorf("executor not available in context")
	}
	return exec, nil
}

func (f *Factory) runRetrieveContext(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	params *answerParams,
) (*fetchcontext.Output, error) {
	retrieveContextActivity := f.retrieveContextFactory.NewActivity()
	retrieveContextInput := &fetchcontext.Input{
		QueryText:        params.question,
		SnapshotPriority: params.snapshots,
		TopK:             params.topK,
		MinScore:         params.minScore,
	}

	retrieveContextOutput, err := executor.ExecuteActivity(ctx, exec, flowCtx, "RetrieveContext", retrieveContextActivity, retrieveContextInput)
	if err != nil {
		return nil, fmt.Errorf("context retrieval failed: %w", err)
	}
	return retrieveContextOutput, nil
}

func (f *Factory) runInvokeLLM(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	resolvedProfile *profile.ResolvedProfile,
	systemPrompt string,
	userPrompt string,
) (*draftcontent.Output, error) {
	invokeLLMActivity := f.invokeLLMFactory.NewActivity()
	invokeLLMInput := &draftcontent.Input{
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}

	invokeLLMOutput, err := executor.ExecuteActivity(ctx, exec, flowCtx, "InvokeLLM", invokeLLMActivity, invokeLLMInput)
	if err != nil {
		return nil, fmt.Errorf("answer generation failed: %w", err)
	}
	return invokeLLMOutput, nil
}
