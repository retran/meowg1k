// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package invokellm implements an activity that invokes an LLM to generate text responses.
package invokellm

import (
	"context"
	"errors"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input represents the input for the InvokeLLM activity.
type Input struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	UserPrompt   string
	Tools        []gateway.ToolDefinition
}

// Output represents the output from the InvokeLLM activity.
type Output struct {
	Metadata  map[string]any
	Content   string
	ToolCalls []gateway.ToolCall
}

// Factory creates instances of the InvokeLLM activity with injected dependencies.
type Factory struct {
	gatewayFactory ports.GenerationGatewayFactory
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new InvokeLLM activity factory with the provided gateway factory.
func NewFactory(gatewayFactory ports.GenerationGatewayFactory) (*Factory, error) {
	if gatewayFactory == nil {
		return nil, fmt.Errorf("gateway factory cannot be nil")
	}

	return &Factory{
		gatewayFactory: gatewayFactory,
	}, nil
}

// NewActivity creates and returns the InvokeLLM activity function with progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if err := validateInput(f, input); err != nil {
			return nil, err
		}

		modelName := ""
		if input.Profile != nil {
			modelName = input.Profile.Model
		}
		if modelName == "" {
			modelName = "unknown-model"
		}

		// Simplified status message - the spinner will show "Running: Invoking LLM (model)"
		// No need to spam the user with every LLM call
		executorCtx.SendRunning(fmt.Sprintf("Invoking LLM (%s)", modelName))

		generationGateway, err := f.gatewayFactory.NewGenerationGateway(ctx, input.Profile)
		if err != nil {
			return nil, fmt.Errorf("failed to create generation gateway: %w", err)
		}

		request := buildRequest(input)
		content, toolCalls, err := generateWithOptionalTools(ctx, generationGateway, request, input.Tools)
		if err != nil {
			return nil, err
		}

		metadata := map[string]any{}

		// Simplified completion message
		executorCtx.SendCompleted(fmt.Sprintf("LLM response (%s)", modelName))

		return &Output{
			Content:   content,
			Metadata:  metadata,
			ToolCalls: toolCalls,
		}, nil
	}
}

func validateInput(factory *Factory, input *Input) error {
	if factory == nil {
		return fmt.Errorf("invoke LLM factory is nil")
	}
	if input == nil {
		return fmt.Errorf("input cannot be nil")
	}
	return nil
}

func buildRequest(input *Input) *gateway.GenerateContentRequest {
	return gateway.NewGenerateContentRequest(
		input.Profile.Model,
		input.SystemPrompt,
		input.UserPrompt,
		input.Profile.MaxOutputTokens,
	).WithTemperature(input.Profile.Temperature).
		WithTopP(input.Profile.TopP).
		WithTopK(input.Profile.TopK).
		WithFrequencyPenalty(input.Profile.FrequencyPenalty).
		WithPresencePenalty(input.Profile.PresencePenalty).
		WithSeed(input.Profile.Seed).
		WithStop(input.Profile.Stop).
		WithResponseFormat(input.Profile.ResponseFormat).
		WithResponseSchema(input.Profile.ResponseSchema).
		WithCandidateCount(input.Profile.CandidateCount).
		WithLogProbs(input.Profile.LogProbs).
		WithTopLogProbs(input.Profile.TopLogProbs).
		WithLogitBias(input.Profile.LogitBias).
		WithServiceTier(input.Profile.ServiceTier).
		WithUser(input.Profile.User).
		WithRepetitionPenalty(input.Profile.RepetitionPenalty).
		WithMinP(input.Profile.MinP).
		WithTopA(input.Profile.TopA).
		WithTypicalP(input.Profile.TypicalP).
		WithMirostat(input.Profile.Mirostat).
		WithMirostatTau(input.Profile.MirostatTau).
		WithMirostatEta(input.Profile.MirostatEta).
		WithGrammar(input.Profile.Grammar)
}

func generateWithOptionalTools(
	ctx context.Context,
	generationGateway ports.GenerationGateway,
	request *gateway.GenerateContentRequest,
	tools []gateway.ToolDefinition,
) (string, []gateway.ToolCall, error) {
	if len(tools) > 0 {
		response, err := tryGenerateWithTools(ctx, generationGateway, request, tools)
		if err != nil {
			return "", nil, err
		}
		if response != nil {
			return response.Content, response.ToolCalls, nil
		}
	}

	content, err := generationGateway.GenerateContent(ctx, request)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate content: %w", err)
	}
	return content, nil, nil
}

func tryGenerateWithTools(
	ctx context.Context,
	generationGateway ports.GenerationGateway,
	request *gateway.GenerateContentRequest,
	tools []gateway.ToolDefinition,
) (*gateway.GenerateContentResponse, error) {
	toolGateway, ok := generationGateway.(ports.ToolCallingGateway)
	if !ok {
		return nil, nil
	}

	response, err := toolGateway.GenerateContentWithTools(ctx, request, tools)
	if err != nil {
		if errors.Is(err, gateway.ErrToolCallingNotSupported) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to generate content with tools: %w", err)
	}
	return response, nil
}
