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

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/flows"
	"github.com/retran/meowg1k/internal/services/config/loader"
	"github.com/retran/meowg1k/internal/services/config/resolver"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/prompt"
	"github.com/spf13/cobra"
)

const (
	stdinContextWrapper = "\n\n```\n%s\n```"
)

// Input holds the initial input for the generate flow.
type Input struct {
	Cmd    *cobra.Command
	Config *config.Config
}

// Params holds all the resolved parameters for a generation request.
type Params struct {
	Profile      *config.ResolvedProfile // Resolved configuration profile for the LLM
	SystemPrompt string                  // System-level instruction for the LLM
	UserPrompt   string                  // User's main prompt, potentially combined with stdin
}

// ResolvedParams represents the resolved parameters for generation.
type ResolvedParams struct {
	Params *Params
	Config *config.Config // Keep config for next task
}

// GenerationGateway represents the created gateway for content generation.
type GenerationGateway struct {
	Gateway gateway.GenerationGateway
	Params  *Params // Keep params for content generation
}

// GeneratedContent represents the final generated content.
type GeneratedContent struct {
	Content string
}

// ResolveParamsExecutor implements parameter resolution with dependency injection.
type ResolveParamsExecutor struct {
	LoaderService   loader.Service
	ResolverService resolver.Service
	PromptBuilder   prompt.Builder
}

// NewResolveParamsExecutor creates a new ResolveParamsExecutor with injected dependencies.
func NewResolveParamsExecutor(loaderService loader.Service, resolverService resolver.Service, promptBuilder prompt.Builder) *ResolveParamsExecutor {
	if loaderService == nil {
		panic("loaderService cannot be nil")
	}
	if resolverService == nil {
		panic("resolverService cannot be nil")
	}
	if promptBuilder == nil {
		panic("promptBuilder cannot be nil")
	}

	return &ResolveParamsExecutor{
		LoaderService:   loaderService,
		ResolverService: resolverService,
		PromptBuilder:   promptBuilder,
	}
}

func (e *ResolveParamsExecutor) Execute(ctx context.Context, input interface{}) (ResolvedParams, flows.Outcome[any], error) {
	inputData, ok := input.(Input)
	if !ok {
		return ResolvedParams{}, flows.Outcome[any]{}, flows.ErrInvalidInput
	}

	params, err := e.resolveParams(inputData.Cmd, inputData.Config)
	if err != nil {
		return ResolvedParams{}, flows.Outcome[any]{}, err
	}

	return ResolvedParams{
		Params: params,
		Config: inputData.Config,
	}, flows.Outcome[any]{Type: flows.OutcomeSuccess}, nil
}

// resolveParams resolves all parameters needed for content generation.
func (e *ResolveParamsExecutor) resolveParams(cmd *cobra.Command, cfg *config.Config) (*Params, error) {
	if cmd == nil {
		return nil, fmt.Errorf("command cannot be nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Use resolver service to resolve task configuration (without stdin handling)
	profileName, systemPrompt, baseUserPrompt, err := e.ResolverService.ResolveTaskConfiguration(cmd, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve task configuration: %w", err)
	}

	// Use resolver service to resolve profile
	profile, err := e.ResolverService.ResolveProfile(cfg, profileName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile: %w", err)
	}

	// Use prompt builder to combine base prompt with stdin content
	finalUserPrompt, err := e.PromptBuilder.BuildUserPrompt(baseUserPrompt, stdinContextWrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to build user prompt: %w", err)
	}

	if finalUserPrompt == "" {
		return nil, fmt.Errorf("user prompt is required")
	}

	return &Params{
		Profile:      profile,
		SystemPrompt: systemPrompt,
		UserPrompt:   finalUserPrompt,
	}, nil
}

// CreateGatewayExecutor implements gateway creation with dependency injection.
type CreateGatewayExecutor struct {
	GatewayFactory gateway.GatewayFactory
}

// NewCreateGatewayExecutor creates a new CreateGatewayExecutor with injected dependencies.
func NewCreateGatewayExecutor(gatewayFactory gateway.GatewayFactory) *CreateGatewayExecutor {
	if gatewayFactory == nil {
		panic("gatewayFactory cannot be nil")
	}

	return &CreateGatewayExecutor{
		GatewayFactory: gatewayFactory,
	}
}

func (e *CreateGatewayExecutor) Execute(ctx context.Context, input interface{}) (GenerationGateway, flows.Outcome[any], error) {
	inputData, ok := input.(ResolvedParams)
	if !ok {
		return GenerationGateway{}, flows.Outcome[any]{}, flows.ErrInvalidInput
	}

	// Use injected GatewayFactory instead of creating new one
	gw, err := e.GatewayFactory.CreateGenerationGateway(
		ctx,
		gateway.Provider(inputData.Params.Profile.Provider),
		inputData.Params.Profile.BaseURL,
		inputData.Params.Profile.APIKey,
	)
	if err != nil {
		return GenerationGateway{}, flows.Outcome[any]{}, err
	}

	return GenerationGateway{
		Gateway: gw,
		Params:  inputData.Params,
	}, flows.Outcome[any]{Type: flows.OutcomeSuccess}, nil
}

// GenerateContentExecutor implements content generation.
type GenerateContentExecutor struct{}

func (e *GenerateContentExecutor) Execute(ctx context.Context, input interface{}) (GeneratedContent, flows.Outcome[any], error) {
	inputData, ok := input.(GenerationGateway)
	if !ok {
		return GeneratedContent{}, flows.Outcome[any]{}, flows.ErrInvalidInput
	}

	content, err := e.generateContent(ctx, inputData.Gateway, inputData.Params)
	if err != nil {
		return GeneratedContent{}, flows.Outcome[any]{}, err
	}

	return GeneratedContent{Content: content}, flows.Outcome[any]{Type: flows.OutcomeSuccess}, nil
}

// generateContent generates content using the configured gateway.
func (e *GenerateContentExecutor) generateContent(ctx context.Context, gw gateway.GenerationGateway, params *Params) (string, error) {
	request := gateway.NewGenerateContentRequest(params.Profile.Model, params.SystemPrompt, params.UserPrompt, params.Profile.MaxOutputTokens)

	ctx, cancel := context.WithTimeout(ctx, params.Profile.Timeout)
	defer cancel()

	content, err := gw.GenerateContent(ctx, request)
	if err != nil {
		return "", err
	}

	return content, nil
}
