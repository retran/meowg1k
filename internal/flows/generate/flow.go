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
	"errors"
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/services/task"
	"github.com/retran/meowg1k/pkg/executor"
)

// ErrInvalidActivityOutputType is returned when an unexpected activity output type is received.
var (
	ErrInvalidActivityOutputType = errors.New("invalid output type from InvokeLLM activity")
	// ErrFactoryIsNil indicates that the factory is nil.
	ErrFactoryIsNil = errors.New("factory is nil")
	// ErrContextIsNil indicates that the context is nil.
	ErrContextIsNil = errors.New("context is nil")
	// ErrFlowContextIsNil indicates that the flow context is nil.
	ErrFlowContextIsNil = errors.New("flow context is nil")
	// ErrTaskConfigProviderIsNil indicates that the taskConfigProvider is nil.
	ErrTaskConfigProviderIsNil = errors.New("taskConfigProvider is nil")
	// ErrUserPromptProviderIsNil indicates that the userPromptProvider is nil.
	ErrUserPromptProviderIsNil = errors.New("userPromptProvider is nil")
	// ErrSystemPromptProviderIsNil indicates that the systemPromptProvider is nil.
	ErrSystemPromptProviderIsNil = errors.New("systemPromptProvider is nil")
	// ErrContentGenerationActivityFactoryIsNil indicates that the contentGenerationActivityFactory is nil.
	ErrContentGenerationActivityFactoryIsNil = errors.New("contentGenerationActivityFactory is nil")
	// ErrOutputWriterIsNil indicates that the outputWriter is nil.
	ErrOutputWriterIsNil = errors.New("outputWriter is nil")
)

// TaskConfigProvider provides resolved task configuration.
type TaskConfigProvider interface {
	Get() (*task.Configuration, error)
}

// UserPromptProvider provides the user prompt for content generation.
type UserPromptProvider interface {
	GetUserPrompt() (string, error)
}

// SystemPromptProvider provides the system prompt for content generation.
type SystemPromptProvider interface {
	GetSystemPrompt() (string, error)
}

// ContentGenerationActivityFactory creates content generation activities.
type ContentGenerationActivityFactory interface {
	NewActivity() executor.Activity[*invokellm.Input, *invokellm.Output]
}

// OutputWriter writes output to the user.
type OutputWriter interface {
	PrintLine(line string) error
}

// FlowFactory creates instances of the generate flow with injected dependencies.
type FlowFactory struct {
	taskConfigProvider               TaskConfigProvider
	userPromptProvider               UserPromptProvider
	systemPromptProvider             SystemPromptProvider
	contentGenerationActivityFactory ContentGenerationActivityFactory
	outputWriter                     OutputWriter
}

// NewFlowFactory creates a new generate flow factory with injected services.
func NewFlowFactory(
	taskConfigProvider TaskConfigProvider,
	userPromptProvider UserPromptProvider,
	systemPromptProvider SystemPromptProvider,
	contentGenerationActivityFactory ContentGenerationActivityFactory,
	outputWriter OutputWriter,
) (*FlowFactory, error) {
	if taskConfigProvider == nil {
		return nil, ErrTaskConfigProviderIsNil
	}
	if userPromptProvider == nil {
		return nil, ErrUserPromptProviderIsNil
	}
	if systemPromptProvider == nil {
		return nil, ErrSystemPromptProviderIsNil
	}
	if contentGenerationActivityFactory == nil {
		return nil, ErrContentGenerationActivityFactoryIsNil
	}
	if outputWriter == nil {
		return nil, ErrOutputWriterIsNil
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
			return ErrFactoryIsNil
		}
		if ctx == nil {
			return ErrContextIsNil
		}
		if flowCtx == nil {
			return ErrFlowContextIsNil
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
