// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package invokellm implements an activity that invokes an LLM to generate text responses.
package invokellm

import (
	"context"
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
}

// Output represents the output from the InvokeLLM activity.
type Output struct {
	Content  string
	Metadata map[string]any
}

// Factory creates instances of the InvokeLLM activity with injected dependencies.
type Factory struct {
	gatewayFactory ports.GenerationGatewayFactory
}

// Compile-time check to ensure Factory implements ActivityFactory interface
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
		if f == nil {
			return nil, fmt.Errorf("invoke LLM factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		executorCtx.SendRunning("Invoking LLM")

		generationGateway, err := f.gatewayFactory.NewGenerationGateway(ctx, input.Profile)
		if err != nil {
			return nil, fmt.Errorf("failed to create generation gateway: %w", err)
		}

		request := gateway.NewGenerateContentRequest(
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
