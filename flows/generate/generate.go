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

package generate

import (
	"context"

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/flows"
	"github.com/retran/meowg1k/internal/services/config/loader"
	"github.com/retran/meowg1k/internal/services/config/registry"
	"github.com/retran/meowg1k/internal/services/config/resolver"
	"github.com/retran/meowg1k/internal/services/config/validator"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/prompt"
	"github.com/spf13/cobra"
)

// GenerateFlow creates and configures the complete generate workflow with dependency injection.
func GenerateFlow(feedbackHandler flows.FeedbackHandler, cfg *config.Config) *flows.Flow {
	flow := flows.NewFlow()

	// Create individual services with proper dependencies
	registryService := registry.NewService()
	validatorService := validator.NewService(registryService)
	loaderService := loader.NewService()
	resolverService := resolver.NewService(registryService, validatorService)
	promptBuilder := prompt.NewBuilder()

	// Create gateway factory for LLM connections
	gatewayFactory := gateway.NewGatewayFactory()

	// Build the complete generate workflow
	flows.AddTask(flow, "resolve-params", NewResolveParamsExecutor(loaderService, resolverService, promptBuilder)).
		LinkToID("create-gateway")

	flows.AddTask(flow, "create-gateway", NewCreateGatewayExecutor(gatewayFactory)).
		LinkToID("generate-content")

	flows.AddTask(flow, "generate-content", &GenerateContentExecutor{})

	// Configure feedback handler if provided
	if feedbackHandler != nil {
		flow.WithFeedbackHandler(feedbackHandler)
	}

	return flow.SetStart("resolve-params")
}

// ExecuteGenerate runs the generate flow with the provided command and configuration.
func ExecuteGenerate(ctx context.Context, cmd *cobra.Command, cfg *config.Config, feedbackHandler flows.FeedbackHandler) (string, error) {
	flow := GenerateFlow(feedbackHandler, cfg)

	input := Input{
		Cmd:    cmd,
		Config: cfg,
	}

	result, err := flow.Run(ctx, input)
	if err != nil {
		return "", err
	}

	// Extract the final content from the result
	if generatedContent, ok := result.(GeneratedContent); ok {
		return generatedContent.Content, nil
	}

	return "", flows.ErrInvalidInput
}
