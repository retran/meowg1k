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

	"github.com/retran/meowg1k/internal/services/prompt"
	"github.com/retran/meowg1k/internal/services/task"
	"github.com/retran/meowg1k/pkg/executor"
)

type GenerateFlowFactory struct {
	taskService             task.Service
	userPromptProvider      prompt.UserPromptProvider
	systemPromptProvider    prompt.SystemPromptProvider
	generateActivityFactory *GenerateActivityFactory
}

func NewGenerateFlowFactory(
	taskService task.Service,
	userPromptProvider prompt.UserPromptProvider,
	systemPromptProvider prompt.SystemPromptProvider,
	generateActivityFactory *GenerateActivityFactory,
) *GenerateFlowFactory {
	return &GenerateFlowFactory{
		taskService:             taskService,
		userPromptProvider:      userPromptProvider,
		systemPromptProvider:    systemPromptProvider,
		generateActivityFactory: generateActivityFactory,
	}
}

// NewFlow creates and returns the generate activity function.
func (f *GenerateFlowFactory) NewFlow() func(context.Context, *executor.ExecutorContext) error {
	return func(ctx context.Context, flowCtx *executor.ExecutorContext) error {
		task := f.taskService.Get()

		userPrompt, err := f.userPromptProvider.GetUserPrompt()
		if err != nil {
			return fmt.Errorf("failed to get user prompt: %w", err)
		}

		systemPrompt, err := f.systemPromptProvider.GetSystemPrompt()
		if err != nil {
			return fmt.Errorf("failed to get system prompt: %w", err)
		}

		activity := f.generateActivityFactory.NewActivity()
		input := &GenerateInput{
			Profile:      task.Profile,
			UserPrompt:   userPrompt,
			SystemPrompt: systemPrompt,
		}

		future := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "Generate", activity, input)

		output, err := future.Get(ctx)
		if err != nil {
			return err
		}

		generateOutput, ok := output.(*GenerateOutput)
		if !ok {
			return errors.New("invalid output type from generate activity")
		}

		fmt.Println(generateOutput.Content)

		return nil
	}
}
