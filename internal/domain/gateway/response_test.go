// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateContentResponse_Text_Nil(t *testing.T) {
	var r *GenerateContentResponse
	assert.Equal(t, "", r.Text())
}

func TestGenerateContentResponse_Text_Empty(t *testing.T) {
	r := &GenerateContentResponse{}
	assert.Equal(t, "", r.Text())
}

func TestGenerateContentResponse_Text_SingleBlock(t *testing.T) {
	r := &GenerateContentResponse{
		Blocks: []ContentBlock{
			{Kind: ContentBlockText, Text: "hello world"},
		},
	}
	assert.Equal(t, "hello world", r.Text())
}

func TestGenerateContentResponse_Text_MultipleBlocks(t *testing.T) {
	r := &GenerateContentResponse{
		Blocks: []ContentBlock{
			{Kind: ContentBlockText, Text: "hello"},
			{Kind: ContentBlockReasoning, Text: "thinking..."},
			{Kind: ContentBlockText, Text: " world"},
		},
	}
	// Only text blocks concatenated (reasoning is skipped), then trimmed
	assert.Equal(t, "hello world", r.Text())
}

func TestGenerateContentResponse_Text_IgnoresNonTextBlocks(t *testing.T) {
	r := &GenerateContentResponse{
		Blocks: []ContentBlock{
			{Kind: ContentBlockReasoning, Text: "only reasoning"},
			{Kind: ContentBlockToolCall, ToolCall: &ToolCall{Name: "fn"}},
		},
	}
	assert.Equal(t, "", r.Text())
}

func TestGenerateContentResponse_Reasoning_Nil(t *testing.T) {
	var r *GenerateContentResponse
	assert.Equal(t, "", r.Reasoning())
}

func TestGenerateContentResponse_Reasoning_Empty(t *testing.T) {
	r := &GenerateContentResponse{}
	assert.Equal(t, "", r.Reasoning())
}

func TestGenerateContentResponse_Reasoning_WithBlock(t *testing.T) {
	r := &GenerateContentResponse{
		Blocks: []ContentBlock{
			{Kind: ContentBlockText, Text: "output"},
			{Kind: ContentBlockReasoning, Text: "because of X"},
		},
	}
	assert.Equal(t, "because of X", r.Reasoning())
}

func TestGenerateContentResponse_Reasoning_MultipleBlocks(t *testing.T) {
	r := &GenerateContentResponse{
		Blocks: []ContentBlock{
			{Kind: ContentBlockReasoning, Text: "step1"},
			{Kind: ContentBlockReasoning, Text: " step2"},
		},
	}
	assert.Equal(t, "step1 step2", r.Reasoning())
}

func TestGenerateContentResponse_ToolCalls_Nil(t *testing.T) {
	var r *GenerateContentResponse
	assert.Nil(t, r.ToolCalls())
}

func TestGenerateContentResponse_ToolCalls_Empty(t *testing.T) {
	r := &GenerateContentResponse{}
	assert.Empty(t, r.ToolCalls())
}

func TestGenerateContentResponse_ToolCalls_WithCalls(t *testing.T) {
	tc := &ToolCall{Name: "search", ID: "1", Arguments: map[string]any{"q": "foo"}}
	r := &GenerateContentResponse{
		Blocks: []ContentBlock{
			{Kind: ContentBlockText, Text: "result"},
			{Kind: ContentBlockToolCall, ToolCall: tc},
		},
	}
	calls := r.ToolCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "search", calls[0].Name)
}

func TestGenerateContentResponse_ToolCalls_SkipsNilToolCall(t *testing.T) {
	r := &GenerateContentResponse{
		Blocks: []ContentBlock{
			{Kind: ContentBlockToolCall, ToolCall: nil}, // nil ToolCall pointer
		},
	}
	assert.Empty(t, r.ToolCalls())
}

func TestGenerateContentRequest_WithMessages(t *testing.T) {
	req := NewGenerateContentRequest("m", "s", "u", 100)
	msgs := []Message{
		{Role: MessageRoleUser, Content: "hello"},
		{Role: MessageRoleAssistant, Content: "hi"},
	}
	result := req.WithMessages(msgs)
	assert.Equal(t, req, result)
	assert.Equal(t, msgs, req.Messages())
}

func TestGenerateContentRequest_Messages_Nil(t *testing.T) {
	var req *GenerateContentRequest
	assert.Nil(t, req.Messages())
}

func TestGenerateContentRequest_Messages_NotSet(t *testing.T) {
	req := NewGenerateContentRequest("m", "s", "u", 100)
	assert.Nil(t, req.Messages())
}

func TestGenerateContentRequest_WithTools(t *testing.T) {
	req := NewGenerateContentRequest("m", "s", "u", 100)
	tools := []ToolDefinition{
		{Name: "search", Description: "search the web", Parameters: map[string]any{"q": "string"}},
	}
	result := req.WithTools(tools)
	assert.Equal(t, req, result)
	assert.Equal(t, tools, req.Tools())
}

func TestGenerateContentRequest_Tools_Nil(t *testing.T) {
	var req *GenerateContentRequest
	assert.Nil(t, req.Tools())
}

func TestGenerateContentRequest_Tools_NotSet(t *testing.T) {
	req := NewGenerateContentRequest("m", "s", "u", 100)
	assert.Nil(t, req.Tools())
}

func TestGenerateContentRequest_ToolsJSON_Nil(t *testing.T) {
	var req *GenerateContentRequest
	s, err := req.ToolsJSON()
	assert.NoError(t, err)
	assert.Equal(t, "", s)
}

func TestGenerateContentRequest_ToolsJSON_Empty(t *testing.T) {
	req := NewGenerateContentRequest("m", "s", "u", 100)
	s, err := req.ToolsJSON()
	assert.NoError(t, err)
	assert.Equal(t, "", s)
}

func TestGenerateContentRequest_ToolsJSON_WithTools(t *testing.T) {
	req := NewGenerateContentRequest("m", "s", "u", 100)
	tools := []ToolDefinition{
		{Name: "calc", Description: "calculator"},
	}
	req.WithTools(tools)
	s, err := req.ToolsJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, s)

	// Validate it's valid JSON
	var decoded []ToolDefinition
	err = json.Unmarshal([]byte(s), &decoded)
	require.NoError(t, err)
	assert.Len(t, decoded, 1)
	assert.Equal(t, "calc", decoded[0].Name)
}
