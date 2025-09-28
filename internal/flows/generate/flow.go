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

	"github.com/retran/meowg1k/internal/services/prompt"
	"github.com/retran/meowg1k/internal/services/task"
	"github.com/retran/meowg1k/pkg/executor"
)

type FlowFactory struct {
	taskService          task.Service
	userPromptProvider   prompt.UserPromptProvider
	systemPromptProvider prompt.SystemPromptProvider
	activityFactory      *ActivityFactory
}

func NewFlowFactory(
	taskService task.Service,
	userPromptProvider prompt.UserPromptProvider,
	systemPromptProvider prompt.SystemPromptProvider,
	activityFactory *ActivityFactory,
) *FlowFactory {
	return &FlowFactory{
		taskService:          taskService,
		userPromptProvider:   userPromptProvider,
		systemPromptProvider: systemPromptProvider,
		activityFactory:      activityFactory,
	}
}

// NewFlow creates and returns the generate activity function with improved, multi-step status reporting.
func (f *FlowFactory) NewFlow() func(context.Context, *executor.ExecutorContext) error {
	return func(ctx context.Context, flowCtx *executor.ExecutorContext) error {
		task := f.taskService.Get()

		subject := "Content generation"
		if task.Name != "" {
			subject = fmt.Sprintf("Task \"%s\"", task.Name)
		}

		flowCtx.SendProgress(0.0, fmt.Sprintf("Starting %s...", subject))

		flowCtx.SendProgress(0.0, "Preparing prompts...")
		userPrompt, err := f.userPromptProvider.GetUserPrompt()
		if err != nil {
			return fmt.Errorf("failed to get user prompt: %w", err)
		}

		systemPrompt, err := f.systemPromptProvider.GetSystemPrompt()
		if err != nil {
			return fmt.Errorf("failed to get system prompt: %w", err)
		}

		status := "Generating content..."
		if task.Name != "" {
			status = fmt.Sprintf("Executing task \"%s\"...", task.Name)
		}
		flowCtx.SendProgress(0.0, status)

		activity := f.activityFactory.NewActivity()
		input := &ContentInput{
			Profile:      task.Profile,
			UserPrompt:   userPrompt,
			SystemPrompt: systemPrompt,
		}

		future := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "GenerateContent", activity, input)

		output, err := future.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to execute \"GenerateContent\" activity: %w", err)
		}

		flowCtx.SendProgress(0.0, "Processing result...")
		generateOutput, ok := output.(*ContentOutput)
		if !ok {
			return errors.New("invalid output type from \"GenerateContent\" activity")
		}

		flowCtx.SendCompleted(fmt.Sprintf("%s completed.", subject))

		time.Sleep(300 * time.Millisecond)

		fmt.Println(generateOutput.Content)

		return nil
	}
}
