// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package summarize

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	Content string
	Type    string // "text", "diff", "file"
}

type Output struct {
	Summary string
}

type Factory struct {
	gatewayFactory  ports.GenerationGatewayFactory
	profileResolver ports.ProfileResolver
	profileName     string
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory(
	gatewayFactory ports.GenerationGatewayFactory,
	profileResolver ports.ProfileResolver,
	profileName string,
) *Factory {
	return &Factory{
		gatewayFactory:  gatewayFactory,
		profileResolver: profileResolver,
		profileName:     profileName,
	}
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		if strings.TrimSpace(f.profileName) == "" {
			return nil, fmt.Errorf("summarize tool profile is not set (configure agent.personas.<name>.profile so the do flow can pick one, or pass a profile when constructing the summarize factory)")
		}

		resolvedProfile, err := f.profileResolver.Get(profile.Profile(f.profileName))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve profile %s: %w", f.profileName, err)
		}

		gw, err := f.gatewayFactory.NewGenerationGateway(ctx, resolvedProfile)
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
			resolvedProfile.Model,
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
