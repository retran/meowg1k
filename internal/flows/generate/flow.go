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

// Package generate provides the flow for generating content using a language model.
package generate

import (
	"context"
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/core/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// FlowFactory creates instances of the generate flow with injected dependencies.
type FlowFactory struct {
	taskConfigProvider               ports.TaskConfigProvider
	userPromptProvider               ports.UserPromptProvider
	systemPromptProvider             ports.SystemPromptProvider
	contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
	outputWriter                     ports.OutputWriter
}

// NewFlowFactory creates a new generate flow factory with injected services.
func NewFlowFactory(
	taskConfigProvider ports.TaskConfigProvider,
	userPromptProvider ports.UserPromptProvider,
	systemPromptProvider ports.SystemPromptProvider,
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

		future := executor.RunActivity(
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

		if err := f.outputWriter.PrintLine(invokeOutput.Content); err != nil {
			return fmt.Errorf("failed to print generated content: %w", err)
		}

		return nil
	}
}
