// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package draftcontent implements an activity that invokes an LLM to draft text responses.
package draftcontent

import (
	"context"
	"errors"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input represents the input for the InvokeLLM activity.
type Input struct {
	Preset       *preset.ResolvedPreset
	SystemPrompt string
	UserPrompt   string
	Messages     []gateway.Message
	Tools        []gateway.ToolDefinition
}

// Output represents the output from the InvokeLLM activity.
type Output struct {
	Metadata map[string]any
	Response *gateway.GenerateContentResponse
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

		executorCtx.SendRunning("")

		generationGateway, err := f.gatewayFactory.NewGenerationGateway(ctx, input.Preset)
		if err != nil {
			return nil, fmt.Errorf("failed to create generation gateway: %w", err)
		}

		response, err := f.executeWithRetry(ctx, generationGateway, input)
		if err != nil {
			return nil, err
		}

		executorCtx.SendCompleted("")

		return &Output{Response: response}, nil
	}
}

func (f *Factory) executeWithRetry(
	ctx context.Context,
	generationGateway ports.GenerationGateway,
	input *Input,
) (*gateway.GenerateContentResponse, error) {
	request := buildRequest(input)

	response, err := write(ctx, generationGateway, request)
	if err != nil {
		// If tools were requested but the gateway doesn't support native tool calling,
		// retry without tools so JSON fallback logic can still work upstream.
		if len(input.Tools) > 0 && errors.Is(err, gateway.ErrToolCallingNotSupported) {
			request = request.WithTools(nil)
			return write(ctx, generationGateway, request)
		}
		return nil, err
	}

	return response, nil
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
		input.Preset.Model,
		input.SystemPrompt,
		input.UserPrompt,
		input.Preset.MaxOutputTokens,
	).WithMessages(input.Messages).
		WithTools(input.Tools).
		WithTemperature(input.Preset.Temperature).
		WithTopP(input.Preset.TopP).
		WithTopK(input.Preset.TopK).
		WithFrequencyPenalty(input.Preset.FrequencyPenalty).
		WithPresencePenalty(input.Preset.PresencePenalty).
		WithSeed(input.Preset.Seed).
		WithStop(input.Preset.Stop).
		WithResponseFormat(input.Preset.ResponseFormat).
		WithResponseSchema(input.Preset.ResponseSchema).
		WithCandidateCount(input.Preset.CandidateCount).
		WithLogProbs(input.Preset.LogProbs).
		WithTopLogProbs(input.Preset.TopLogProbs).
		WithLogitBias(input.Preset.LogitBias).
		WithServiceTier(input.Preset.ServiceTier).
		WithUser(input.Preset.User).
		WithRepetitionPenalty(input.Preset.RepetitionPenalty).
		WithMinP(input.Preset.MinP).
		WithTopA(input.Preset.TopA).
		WithTypicalP(input.Preset.TypicalP).
		WithMirostat(input.Preset.Mirostat).
		WithMirostatTau(input.Preset.MirostatTau).
		WithMirostatEta(input.Preset.MirostatEta).
		WithGrammar(input.Preset.Grammar)
}

func write(
	ctx context.Context,
	generationGateway ports.GenerationGateway,
	request *gateway.GenerateContentRequest,
) (*gateway.GenerateContentResponse, error) {
	content, err := generationGateway.GenerateContent(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to write content: %w", err)
	}
	return content, nil
}
