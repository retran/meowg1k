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

// Package generate provides the activity for generating content using a language model.
package generate

import (
	"context"
	"errors"
	"fmt"

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/pkg/executor"
)

var (
	// ErrInputCannotBeNil indicates that the input parameter is nil
	ErrInputCannotBeNil = errors.New("input cannot be nil")
	// ErrInvalidInputType indicates that the input type is not supported
	ErrInvalidInputType = errors.New("invalid input type")
)

// ActivityFactory creates instances of the generate activity with injected dependencies.
type ActivityFactory struct {
	gatewayFactory gateway.Factory
}

// NewActivityFactory creates a new generate activity factory with injected services.
func NewActivityFactory(
	gatewayFactory gateway.Factory,
) *ActivityFactory {
	return &ActivityFactory{
		gatewayFactory: gatewayFactory,
	}
}

// NewActivity creates and returns the generate activity function with added progress reporting.
func (f *ActivityFactory) NewActivity() func(
	context.Context, *executor.Context, any) (any, error) {
	return func(ctx context.Context, executorCtx *executor.Context, input any) (any, error) {
		executorCtx.SendProgress(0.0, "Preparing generation request...")

		if input == nil {
			return nil, ErrInputCannotBeNil
		}

		generateInput, ok := input.(*ContentInput)
		if !ok {
			return nil, fmt.Errorf("%w: %T", ErrInvalidInputType, input)
		}

		executorCtx.SendProgress(0.0, "Sending generation request to large language model...")

		generationGateway, err := f.gatewayFactory.NewGenerationGateway(ctx, generateInput.Profile)
		if err != nil {
			return nil, fmt.Errorf("failed to create generation gateway: %w", err)
		}

		request := mdGateway.NewGenerateContentRequest(
			generateInput.Profile.Model,
			generateInput.SystemPrompt,
			generateInput.UserPrompt,
			generateInput.Profile.MaxOutputTokens,
		)

		// This is the primary long-running operation within the activity.
		content, err := generationGateway.GenerateContent(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to generate content: %w", err)
		}

		// Report that the generation itself is complete.
		executorCtx.SendProgress(0.0, "Response received, preparing output...")

		metadata := map[string]any{}

		executorCtx.SendCompleted("Generation request completed.")

		return &ContentOutput{
			Content:  content,
			Metadata: metadata,
		}, nil
	}
}
