// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package summarize

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type mockPresetResolver struct {
	preset *preset.ResolvedPreset
	err    error
}

func (m *mockPresetResolver) Get(_ preset.Preset) (*preset.ResolvedPreset, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.preset, nil
}

type mockGenerationGateway struct {
	lastRequest *gateway.GenerateContentRequest
	resp        *gateway.GenerateContentResponse
	err         error
}

func (m *mockGenerationGateway) GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (*gateway.GenerateContentResponse, error) {
	_ = ctx
	m.lastRequest = request
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return &gateway.GenerateContentResponse{
		Blocks: []gateway.ContentBlock{{Kind: gateway.ContentBlockText, Text: "summary"}},
	}, nil
}

type mockGenerationGatewayFactory struct {
	gateway ports.GenerationGateway
	err     error
}

func (m *mockGenerationGatewayFactory) NewGenerationGateway(ctx context.Context, _ *preset.ResolvedPreset) (ports.GenerationGateway, error) {
	_ = ctx
	if m.err != nil {
		return nil, m.err
	}
	return m.gateway, nil
}

func TestSummarizeActivity_Success(t *testing.T) {
	gw := &mockGenerationGateway{}
	gwFactory := &mockGenerationGatewayFactory{gateway: gw}
	resolver := &mockPresetResolver{preset: &preset.ResolvedPreset{Model: "model"}}

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory(gwFactory, resolver, "default").NewActivity()
	output, err := activity(context.Background(), flowCtx, &Input{Content: "diff", Type: "diff"})
	require.NoError(t, err)
	assert.Equal(t, "summary", output.Summary)

	require.NotNil(t, gw.lastRequest)
	assert.Contains(t, gw.lastRequest.SystemPrompt(), "expert code reviewer")
	assert.Contains(t, gw.lastRequest.UserPrompt(), "Please summarize")
}

func TestSummarizeActivity_EmptyPresetName(t *testing.T) {
	gwFactory := &mockGenerationGatewayFactory{gateway: &mockGenerationGateway{}}
	resolver := &mockPresetResolver{preset: &preset.ResolvedPreset{Model: "model"}}

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory(gwFactory, resolver, "").NewActivity()
	_, err := activity(context.Background(), flowCtx, &Input{Content: "text", Type: "text"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "preset is not set")
}

func TestSummarizeActivity_PresetError(t *testing.T) {
	gwFactory := &mockGenerationGatewayFactory{gateway: &mockGenerationGateway{}}
	resolver := &mockPresetResolver{err: errors.New("preset error")}

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory(gwFactory, resolver, "default").NewActivity()
	_, err := activity(context.Background(), flowCtx, &Input{Content: "text", Type: "text"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve preset")
}

func TestSummarizeActivity_GatewayFactoryError(t *testing.T) {
	gwFactory := &mockGenerationGatewayFactory{err: errors.New("factory error")}
	resolver := &mockPresetResolver{preset: &preset.ResolvedPreset{Model: "model"}}

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory(gwFactory, resolver, "default").NewActivity()
	_, err := activity(context.Background(), flowCtx, &Input{Content: "text", Type: "text"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create gateway")
}

func TestSummarizeActivity_GenerationError(t *testing.T) {
	gw := &mockGenerationGateway{err: errors.New("gen error")}
	gwFactory := &mockGenerationGatewayFactory{gateway: gw}
	resolver := &mockPresetResolver{preset: &preset.ResolvedPreset{Model: "model"}}

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory(gwFactory, resolver, "default").NewActivity()
	_, err := activity(context.Background(), flowCtx, &Input{Content: "text", Type: "text"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate content")
}

func TestSummarizeActivity_EmptyResponse(t *testing.T) {
	gw := &mockGenerationGateway{
		resp: &gateway.GenerateContentResponse{
			Blocks: []gateway.ContentBlock{{Kind: gateway.ContentBlockText, Text: "  "}},
		},
	}
	gwFactory := &mockGenerationGatewayFactory{gateway: gw}
	resolver := &mockPresetResolver{preset: &preset.ResolvedPreset{Model: "model"}}

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory(gwFactory, resolver, "default").NewActivity()
	_, err := activity(context.Background(), flowCtx, &Input{Content: "text", Type: "text"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no response")
}
