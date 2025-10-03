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
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/output"
	"github.com/retran/meowg1k/internal/services/prompt"
	"github.com/retran/meowg1k/internal/services/task"
	"github.com/retran/meowg1k/pkg/executor"
)

// ErrInvalidActivityOutputType is returned when an activity returns an unexpected output type.
var ErrInvalidActivityOutputType = errors.New("invalid output type from InvokeLLM activity")

// FlowFactory creates instances of the generate flow with injected dependencies.
type FlowFactory struct {
	taskService          task.Service
	userPromptProvider   prompt.UserPromptProvider
	systemPromptProvider prompt.SystemPromptProvider
	invokeLLMFactory     *invokellm.Factory
	outputService        output.Service
}

// NewFlowFactory creates a new generate flow factory with injected services.
func NewFlowFactory(
	taskService task.Service,
	userPromptProvider prompt.UserPromptProvider,
	systemPromptProvider prompt.SystemPromptProvider,
	gatewayFactory gateway.Factory,
	outputService output.Service,
) *FlowFactory {
	invokeLLMFactory := invokellm.NewFactory(gatewayFactory)
	return &FlowFactory{
		taskService:          taskService,
		userPromptProvider:   userPromptProvider,
		systemPromptProvider: systemPromptProvider,
		invokeLLMFactory:     invokeLLMFactory,
		outputService:        outputService,
	}
}

// NewFlow creates and returns the content generation flow function with improved, multi-step status reporting.
func (f *FlowFactory) NewFlow() func(context.Context, *executor.Context) error {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		task := f.taskService.Get()

		flowCtx.SendRunning("Generating content")

		userPrompt, err := f.userPromptProvider.GetUserPrompt()
		if err != nil {
			return fmt.Errorf("failed to get user prompt: %w", err)
		}

		systemPrompt, err := f.systemPromptProvider.GetSystemPrompt()
		if err != nil {
			return fmt.Errorf("failed to get system prompt: %w", err)
		}

		activity := f.invokeLLMFactory.NewActivity()
		input := &invokellm.Input{
			Profile:      task.Profile,
			UserPrompt:   userPrompt,
			SystemPrompt: systemPrompt,
		}

		future := flowCtx.GetExecutor().RunActivity(ctx, flowCtx, "InvokeLLM", activity, input)

		output, err := future.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to execute \"InvokeLLM\" activity: %w", err)
		}

		invokeOutput, ok := output.(*invokellm.Output)

		if !ok {
			return ErrInvalidActivityOutputType
		}

		flowCtx.SendCompleted("")

		time.Sleep(300 * time.Millisecond)

		f.outputService.PrintLine(invokeOutput.Content)

		return nil
	}
}
