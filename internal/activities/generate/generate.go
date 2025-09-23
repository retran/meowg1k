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
	"fmt"

	gatewaymodels "github.com/retran/meowg1k/internal/models/gateway"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/profiles"
	"github.com/retran/meowg1k/internal/services/prompt"
	"github.com/retran/meowg1k/internal/services/tasks"
	"github.com/retran/meowg1k/pkg/activity"
)

// GenerateActivityFactory creates instances of the generate activity with injected dependencies.
type GenerateActivityFactory struct {
	taskResolver    tasks.Service
	profileResolver profiles.Service
	promptBuilder   prompt.Builder
	gatewayFactory  gateway.GatewayFactory
}

// NewGenerateActivityFactory creates a new generate activity factory with injected services.
func NewGenerateActivityFactory(
	taskResolver tasks.Service,
	profileResolver profiles.Service,
	promptBuilder prompt.Builder,
	gatewayFactory gateway.GatewayFactory,
) *GenerateActivityFactory {
	return &GenerateActivityFactory{
		taskResolver:    taskResolver,
		profileResolver: profileResolver,
		promptBuilder:   promptBuilder,
		gatewayFactory:  gatewayFactory,
	}
}

// CreateActivity creates and returns the generate activity function.
func (f *GenerateActivityFactory) CreateActivity() func(context.Context, *activity.ActivityContext, *GenerateInput) (*GenerateOutput, error) {
	return func(ctx context.Context, activityCtx *activity.ActivityContext, input *GenerateInput) (*GenerateOutput, error) {
		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		if input.Config == nil {
			return nil, fmt.Errorf("config cannot be nil")
		}

		if input.Command == nil {
			return nil, fmt.Errorf("command cannot be nil")
		}

		activityCtx.SendProgress(0.1, "Resolving task configuration")

		// Step 1: Resolve task configuration using injected task resolver service
		profileName, systemPrompt, userPrompt, err := f.taskResolver.ResolveTaskConfiguration()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve task configuration: %w", err)
		}

		activityCtx.SendProgress(0.3, "Building prompts")

		// Step 2: Build prompts using injected prompt builder service
		// Start with the base user prompt
		finalUserPrompt := userPrompt

		// Add stdin content if present
		if input.StdinContent != "" {
			finalUserPrompt = f.promptBuilder.CombinePrompts(userPrompt, input.StdinContent)
		}

		// System prompt is used as-is
		finalSystemPrompt := systemPrompt

		activityCtx.SendProgress(0.5, "Creating gateway")

		// Step 3: Resolve profile configuration using injected profile resolver service
		resolvedProfile, err := f.profileResolver.ResolveProfile(profileName)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve profile '%s': %w", profileName, err)
		}

		// Create generation gateway using injected factory
		generationGateway, err := f.gatewayFactory.CreateGenerationGateway(ctx, resolvedProfile.Provider, resolvedProfile.BaseURL, resolvedProfile.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create generation gateway: %w", err)
		}

		activityCtx.SendProgress(0.7, "Generating content")

		// Step 4: Generate content using the gateway
		request := gatewaymodels.NewGenerateContentRequest(
			resolvedProfile.Model,
			finalSystemPrompt,
			finalUserPrompt,
			resolvedProfile.MaxOutputTokens,
		)

		content, err := generationGateway.GenerateContent(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to generate content: %w", err)
		}

		activityCtx.SendProgress(1.0, "Generation completed")

		// Build metadata
		metadata := map[string]interface{}{
			"profile_name":      profileName,
			"model":             resolvedProfile.Model,
			"provider":          string(resolvedProfile.Provider),
			"system_prompt":     finalSystemPrompt,
			"user_prompt":       finalUserPrompt,
			"max_output_tokens": resolvedProfile.MaxOutputTokens,
			"from_task":         input.TaskName != "",
			"task_name":         input.TaskName,
			"has_stdin":         input.StdinContent != "",
		}

		return &GenerateOutput{
			Content:  content,
			Metadata: metadata,
		}, nil
	}
}
