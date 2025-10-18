// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package generate implements the workflow for generating arbitrary content using LLMs based on prompts and tasks.
package generate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/retran/meowg1k/internal/activities/invokellm"
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

// FlowFactory creates instances of the generate flow with injected dependencies.
type FlowFactory struct {
	taskConfigProvider               TaskConfigProvider
	userPromptProvider               UserPromptProvider
	systemPromptProvider             SystemPromptProvider
	contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
	outputWriter                     ports.OutputWriter
}

// NewFlowFactory creates a new generate flow factory with injected adapters.
func NewFlowFactory(
	taskConfigProvider TaskConfigProvider,
	userPromptProvider UserPromptProvider,
	systemPromptProvider SystemPromptProvider,
	contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output],
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
		if f == nil {
			return fmt.Errorf("factory is nil")
		}
		if ctx == nil {
			return fmt.Errorf("context is nil")
		}
		if flowCtx == nil {
			return fmt.Errorf("flow context is nil")
		}

		task, err := f.taskConfigProvider.Get()
		if err != nil {
			return fmt.Errorf("failed to get task config: %w", err)
		}

		flowCtx.SendRunning("Generating content")

		userPrompt, err := f.userPromptProvider.GetUserPrompt()
		if err != nil {
			return fmt.Errorf("failed to get user prompt: %w", err)
		}

		systemPrompt, err := f.systemPromptProvider.GetSystemPrompt()
		if err != nil {
			return fmt.Errorf("failed to get system prompt: %w", err)
		}

		activity := f.contentGenerationActivityFactory.NewActivity()
		input := &invokellm.Input{
			Profile:      task.Profile,
			UserPrompt:   userPrompt,
			SystemPrompt: systemPrompt,
		}

		future := executor.ExecuteActivity(
			flowCtx.GetExecutor(),
			ctx,
			flowCtx,
			"InvokeLLM",
			activity,
			input,
		)

		invokeOutput, err := future.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to execute \"InvokeLLM\" activity: %w", err)
		}

		flowCtx.SendCompleted("")

		time.Sleep(300 * time.Millisecond)

		if err := f.outputWriter.PrintLine(strings.TrimSpace(invokeOutput.Content)); err != nil {
			return fmt.Errorf("failed to print generated content: %w", err)
		}

		return nil
	}
}
