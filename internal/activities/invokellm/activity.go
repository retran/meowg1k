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

// Package invokellm provides a reusable activity for invoking LLM to generate content.
package invokellm

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input represents the input for the InvokeLLM activity.
type Input struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	UserPrompt   string
}

// Output represents the output from the InvokeLLM activity.
type Output struct {
	Content  string
	Metadata map[string]any
}

// Factory creates instances of the InvokeLLM activity with injected dependencies.
type Factory struct {
	gatewayFactory gateway.Factory
}

// NewFactory creates a new InvokeLLM activity factory with injected services.
func NewFactory(gatewayFactory gateway.Factory) *Factory {
	return &Factory{
		gatewayFactory: gatewayFactory,
	}
}

// NewActivity creates and returns the InvokeLLM activity function with progress reporting.
func (f *Factory) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		if activityInput == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		executorCtx.SendRunning("Invoking LLM")

		input, ok := activityInput.(*Input)
		if !ok {
			return nil, fmt.Errorf("%w: %T", executor.ErrInvalidInputType, activityInput)
		}

		generationGateway, err := f.gatewayFactory.NewGenerationGateway(ctx, input.Profile)
		if err != nil {
			return nil, fmt.Errorf("failed to create generation gateway: %w", err)
		}

		request := gateway.NewGenerateContentRequest(
			input.Profile.Model,
			input.SystemPrompt,
			input.UserPrompt,
			input.Profile.MaxOutputTokens,
		)

		content, err := generationGateway.GenerateContent(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to generate content: %w", err)
		}

		metadata := map[string]any{}

		executorCtx.SendCompleted("")

		return &Output{
			Content:  content,
			Metadata: metadata,
		}, nil
	}
}
