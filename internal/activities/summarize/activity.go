// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package summarize implements an activity for summarizing content using an LLM.
package summarize

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input for summarization.
type Input struct {
	Content string
	Type    string // "text", "diff", "file"
}

// Output defines the output of the summarization operation.
type Output struct {
	Summary string
}

// Factory builds summarize activities.
type Factory struct {
	gatewayFactory ports.GenerationGatewayFactory
	presetResolver ports.PresetResolver
	presetName     string
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new summarize activity factory.
func NewFactory(
	gatewayFactory ports.GenerationGatewayFactory,
	presetResolver ports.PresetResolver,
	presetName string,
) *Factory {
	return &Factory{
		gatewayFactory: gatewayFactory,
		presetResolver: presetResolver,
		presetName:     presetName,
	}
}

// NewActivity creates the activity.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		if strings.TrimSpace(f.presetName) == "" {
			return nil, fmt.Errorf("summarize tool preset is not set (configure agent.personas.<name>.preset so the do flow can pick one, or pass a preset when constructing the summarize factory)")
		}

		resolvedPreset, err := f.presetResolver.Get(preset.Preset(f.presetName))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve preset %s: %w", f.presetName, err)
		}

		gw, err := f.gatewayFactory.NewGenerationGateway(ctx, resolvedPreset)
		if err != nil {
			return nil, fmt.Errorf("failed to create gateway: %w", err)
		}

		systemPrompt := "You are a helpful assistant that summarizes text concisely."
		if input.Type == "diff" {
			systemPrompt = "You are an expert code reviewer. Summarize the following git diff concisely, focusing on the intent and impact of the changes."
		}

		userPrompt := fmt.Sprintf("Please summarize the following %s:\n\n%s", input.Type, input.Content)

		flowCtx.SendRunningWithDetails("Summarizing content", fmt.Sprintf("type=%s len=%d", input.Type, len(input.Content)))

		req := gateway.NewGenerateContentRequest(
			resolvedPreset.Model,
			systemPrompt,
			userPrompt,
			0, // max tokens
		)

		resp, err := gw.GenerateContent(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to generate content: %w", err)
		}

		summary := strings.TrimSpace(resp.Text())
		if summary == "" {
			return nil, fmt.Errorf("no response from LLM")
		}

		return &Output{Summary: summary}, nil
	}
}
