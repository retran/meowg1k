// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package write implements the workflow for generating arbitrary content using LLMs based on prompts and tasks.
package write

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/retran/meowg1k/internal/activities/draftcontent"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/domain/task"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// TaskConfigProvider provides resolved task configuration.
type TaskConfigProvider interface {
	Get() (*task.ResolvedConfig, error)
}

// UserPromptProvider provides the user prompt for content generation.
type UserPromptProvider interface {
	GetUserPrompt() (string, error)
}

// SystemPromptProvider provides the system prompt for content generation.
type SystemPromptProvider interface {
	GetSystemPrompt() (string, error)
}

// FlowFactory creates instances of the write flow with injected dependencies.
type FlowFactory struct {
	taskConfigProvider               TaskConfigProvider
	userPromptProvider               UserPromptProvider
	systemPromptProvider             SystemPromptProvider
	contentGenerationActivityFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]
	outputWriter                     ports.OutputWriter
}

// NewFlowFactory creates a new write flow factory with injected adapters.
func NewFlowFactory(
	taskConfigProvider TaskConfigProvider,
	userPromptProvider UserPromptProvider,
	systemPromptProvider SystemPromptProvider,
	contentGenerationActivityFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output],
	outputWriter ports.OutputWriter,
) (*FlowFactory, error) {
	if taskConfigProvider == nil {
		return nil, fmt.Errorf("taskConfigProvider is nil")
	}

	if userPromptProvider == nil {
		return nil, fmt.Errorf("userPromptProvider is nil")
	}

	if systemPromptProvider == nil {
		return nil, fmt.Errorf("systemPromptProvider is nil")
	}

	if contentGenerationActivityFactory == nil {
		return nil, fmt.Errorf("contentGenerationActivityFactory is nil")
	}

	if outputWriter == nil {
		return nil, fmt.Errorf("outputWriter is nil")
	}

	return &FlowFactory{
		taskConfigProvider:               taskConfigProvider,
		userPromptProvider:               userPromptProvider,
		systemPromptProvider:             systemPromptProvider,
		contentGenerationActivityFactory: contentGenerationActivityFactory,
		outputWriter:                     outputWriter,
	}, nil
}

// NewFlow creates and returns the content generation flow function with improved, multi-step status reporting.
func (f *FlowFactory) NewFlow() func(context.Context, *executor.Context) error {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		exec, err := f.validateFlowContext(ctx, flowCtx)
		if err != nil {
			return err
		}

		taskConfig, err := f.taskConfigProvider.Get()
		if err != nil {
			return fmt.Errorf("failed to get task config: %w", err)
		}

		userPrompt, systemPrompt, err := f.getPrompts()
		if err != nil {
			return err
		}
		flowCtx.SendRunningWithDetails(
			"I'm drafting the response",
			fmt.Sprintf("prompt=%q", truncatePrompt(userPrompt)),
		)

		invokeOutput, err := f.runInvokeActivity(ctx, flowCtx, exec, taskConfig.Profile, userPrompt, systemPrompt)
		if err != nil {
			return err
		}

		if err := f.printOutput(invokeOutput); err != nil {
			return err
		}

		flowCtx.SendCompletedWithDetails(
			"I've drafted the response",
			fmt.Sprintf("prompt=%q", truncatePrompt(userPrompt)),
		)
		return nil
	}
}

func truncatePrompt(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "empty prompt"
	}
	if len(trimmed) > 80 {
		return trimmed[:80] + "..."
	}
	return trimmed
}

func (f *FlowFactory) validateFlowContext(ctx context.Context, flowCtx *executor.Context) (executor.Executor, error) {
	if f == nil {
		return nil, fmt.Errorf("factory is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if flowCtx == nil {
		return nil, fmt.Errorf("flow context is nil")
	}

	exec := flowCtx.GetExecutor()
	if exec == nil {
		return nil, fmt.Errorf("executor not available in context")
	}
	return exec, nil
}

func (f *FlowFactory) getPrompts() (userPrompt string, systemPrompt string, err error) {
	userPrompt, err = f.userPromptProvider.GetUserPrompt()
	if err != nil {
		return "", "", fmt.Errorf("failed to get user prompt: %w", err)
	}

	systemPrompt, err = f.systemPromptProvider.GetSystemPrompt()
	if err != nil {
		return "", "", fmt.Errorf("failed to get system prompt: %w", err)
	}

	return userPrompt, systemPrompt, nil
}

func (f *FlowFactory) runInvokeActivity(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	resolvedProfile *profile.ResolvedProfile,
	userPrompt string,
	systemPrompt string,
) (*draftcontent.Output, error) {
	activity := f.contentGenerationActivityFactory.NewActivity()
	input := &draftcontent.Input{
		Profile:      resolvedProfile,
		UserPrompt:   userPrompt,
		SystemPrompt: systemPrompt,
	}

	invokeOutput, err := executor.ExecuteActivity(
		ctx,
		exec,
		flowCtx,
		"InvokeLLM",
		activity,
		input,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute \"InvokeLLM\" activity: %w", err)
	}
	return invokeOutput, nil
}

func (f *FlowFactory) printOutput(invokeOutput *draftcontent.Output) error {
	time.Sleep(300 * time.Millisecond)

	if err := f.outputWriter.PrintLine(strings.TrimSpace(invokeOutput.Content)); err != nil {
		return fmt.Errorf("failed to print generated content: %w", err)
	}

	return nil
}
