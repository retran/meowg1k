// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaingateway "github.com/retran/meowg1k/internal/domain/gateway"
)

// --- parseCopilotTokenExpiry ---

func TestParseCopilotTokenExpiry(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(30 * time.Minute).Unix()
	past := time.Now().Add(-5 * time.Minute).Unix()

	tests := []struct {
		name       string
		token      string
		wantAfter  time.Time
		wantBefore time.Time
	}{
		{
			name:       "valid exp field",
			token:      fmt.Sprintf("tid=abc123;exp=%d;sku=pro", future),
			wantAfter:  time.Unix(future-1, 0),
			wantBefore: time.Unix(future+1, 0),
		},
		{
			name:       "exp first",
			token:      fmt.Sprintf("exp=%d;tid=abc", future),
			wantAfter:  time.Unix(future-1, 0),
			wantBefore: time.Unix(future+1, 0),
		},
		{
			name:       "expired token still parsed",
			token:      fmt.Sprintf("tid=abc;exp=%d", past),
			wantAfter:  time.Unix(past-1, 0),
			wantBefore: time.Unix(past+1, 0),
		},
		{
			name:       "no exp falls back to ~25 min",
			token:      "tid=abc;sku=pro",
			wantAfter:  time.Now().Add(20 * time.Minute),
			wantBefore: time.Now().Add(30 * time.Minute),
		},
		{
			name:       "malformed exp falls back",
			token:      "exp=notanumber",
			wantAfter:  time.Now().Add(20 * time.Minute),
			wantBefore: time.Now().Add(30 * time.Minute),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := parseCopilotTokenExpiry(tc.token)
			assert.True(t, result.After(tc.wantAfter), "expiry %v should be after %v", result, tc.wantAfter)
			assert.True(t, result.Before(tc.wantBefore), "expiry %v should be before %v", result, tc.wantBefore)
		})
	}
}

// --- generateMachineID ---

func TestGenerateMachineID(t *testing.T) {
	t.Parallel()

	id, err := generateMachineID()
	require.NoError(t, err)
	// SHA256 hex is 64 characters.
	assert.Len(t, id, 64)
	// Must be lowercase hex.
	assert.Regexp(t, `^[0-9a-f]+$`, id)

	// Stable across calls.
	id2, err := generateMachineID()
	require.NoError(t, err)
	assert.Equal(t, id, id2)
}

// --- orDefault ---

func TestOrDefault(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "fallback", orDefault("", "fallback"))
	assert.Equal(t, "value", orDefault("value", "fallback"))
}

// --- loadOrAcquireGitHubToken ---

func TestLoadGitHubToken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "copilot_token")

	// Write token manually (persistence is handled by cmd/auth.go).
	require.NoError(t, os.WriteFile(tokenFile, []byte("ghu_testtoken123"), 0o600))

	// Check permissions.
	info, err := os.Stat(tokenFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Load the token via loadOrAcquireGitHubToken.
	g := &copilotGateway{tokenFile: tokenFile}
	err = g.loadOrAcquireGitHubToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghu_testtoken123", g.githubToken)
}

func TestLoadGitHubToken_MissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "copilot_token")

	g := &copilotGateway{tokenFile: tokenFile}
	err := g.loadOrAcquireGitHubToken(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "meow auth copilot")
}

// --- CountTokens ---

func TestCopilotCountTokens(t *testing.T) {
	t.Parallel()

	g := &copilotGateway{}

	tests := []struct {
		name    string
		texts   []string
		wantMin int
		wantMax int
	}{
		{name: "empty slice", texts: []string{}, wantMin: 0, wantMax: 0},
		{name: "nil slice", texts: nil, wantMin: 0, wantMax: 0},
		{name: "single word", texts: []string{"hello"}, wantMin: 1, wantMax: 3},
		{name: "multiple texts", texts: []string{"hello", "world"}, wantMin: 2, wantMax: 5},
		{name: "longer text", texts: []string{"the quick brown fox"}, wantMin: 5, wantMax: 8},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			count, err := g.CountTokens(context.Background(), "claude-sonnet-4.6", tc.texts)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, tc.wantMin)
			assert.LessOrEqual(t, count, tc.wantMax)
		})
	}
}

// --- buildRequestHeaders ---

func TestBuildRequestHeaders(t *testing.T) {
	t.Parallel()

	g := &copilotGateway{
		copilotToken:        "tok_abc",
		sessionID:           "sess_123",
		machineID:           "machine_abc",
		integrationID:       "vscode-chat",
		openAIOrganization:  "github-copilot",
		editorVersion:       "Neovim/0.6.1",
		editorPluginVersion: "copilot.vim/1.16.0",
		userAgent:           "GithubCopilot/1.155.0",
	}

	headers := g.buildRequestHeaders(false)

	assert.Equal(t, "Bearer tok_abc", headers["Authorization"])
	assert.Equal(t, "sess_123", headers["Vscode-Sessionid"])
	assert.Equal(t, "machine_abc", headers["Vscode-Machineid"])
	assert.Equal(t, "vscode-chat", headers["Copilot-Integration-Id"])
	assert.Equal(t, "github-copilot", headers["Openai-Organization"])
	assert.Equal(t, "conversation-edits", headers["Openai-Intent"])
	assert.Equal(t, "user", headers["x-initiator"])
	assert.Equal(t, "Neovim/0.6.1", headers["Editor-Version"])
	assert.Equal(t, "copilot.vim/1.16.0", headers["Editor-Plugin-Version"])
	assert.Equal(t, "GithubCopilot/1.155.0", headers["User-Agent"])
	assert.Equal(t, "application/json", headers["Content-Type"])
	assert.Equal(t, "none", headers["Sec-Fetch-Site"])
	assert.Equal(t, "no-cors", headers["Sec-Fetch-Mode"])
	assert.Equal(t, "empty", headers["Sec-Fetch-Dest"])
	// X-Request-Id is random — just check it's present and non-empty.
	assert.NotEmpty(t, headers["X-Request-Id"])

	// Agent initiator.
	agentHeaders := g.buildRequestHeaders(true)
	assert.Equal(t, "agent", agentHeaders["x-initiator"])
}

// --- refreshTokenIfNeeded: skip refresh if token still valid ---

func TestRefreshTokenIfNeeded_SkipsWhenValid(t *testing.T) {
	t.Parallel()

	g := &copilotGateway{
		copilotToken:        "existing_token",
		tokenExpiry:         time.Now().Add(10 * time.Minute),
		editorVersion:       defaultCopilotEditorVersion,
		editorPluginVersion: defaultCopilotEditorPluginVersion,
		userAgent:           defaultCopilotUserAgent,
		client:              &http.Client{},
	}

	err := g.refreshTokenIfNeeded(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "existing_token", g.copilotToken, "should not replace a valid token")
}

// --- GenerateContent (mocked server) ---

func TestCopilotGenerateContent(t *testing.T) {
	t.Parallel()

	futureExpiry := time.Now().Add(30 * time.Minute).Unix()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer copilot_tok", r.Header.Get("Authorization"))
		assert.NotEmpty(t, r.Header.Get("X-Request-Id"))
		assert.NotEmpty(t, r.Header.Get("Vscode-Sessionid"))
		assert.NotEmpty(t, r.Header.Get("Vscode-Machineid"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(copilotResponse{
			Choices: []copilotChoice{
				{Message: struct {
					Content   string            `json:"content"`
					ToolCalls []copilotToolCall `json:"tool_calls,omitempty"`
				}{Content: "Hello from Copilot!"}},
			},
			Usage: &struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		})
	}))
	defer server.Close()

	g := &copilotGateway{
		client:             server.Client(),
		baseURL:            server.URL,
		copilotToken:       "copilot_tok",
		tokenExpiry:        time.Unix(futureExpiry, 0),
		sessionID:          "test-session",
		machineID:          "test-machine",
		integrationID:      defaultCopilotIntegrationID,
		openAIOrganization: defaultCopilotOpenAIOrganization,

		editorVersion:       defaultCopilotEditorVersion,
		editorPluginVersion: defaultCopilotEditorPluginVersion,
		userAgent:           defaultCopilotUserAgent,
		githubToken:         "ghu_test",
	}

	req := domaingateway.NewGenerateContentRequest("claude-sonnet-4.6", "system prompt", "user prompt", 1000)

	resp, err := g.GenerateContent(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Blocks, 1)
	assert.Equal(t, "Hello from Copilot!", resp.Blocks[0].Text)
	require.NotNil(t, resp.Usage)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

// --- GenerateContentStream (mocked SSE server) ---

func TestCopilotGenerateContentStream(t *testing.T) {
	t.Parallel()

	futureExpiry := time.Now().Add(30 * time.Minute).Unix()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat/completions", r.URL.Path)

		// Verify stream=true in request body.
		var reqBody copilotRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&reqBody))
		assert.True(t, reqBody.Stream)

		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			`{"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`{"choices":[{"delta":{"content":" world"},"finish_reason":null}]}`,
			`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
		}
		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
		}
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer server.Close()

	g := &copilotGateway{
		client:             server.Client(),
		baseURL:            server.URL,
		copilotToken:       "copilot_tok",
		tokenExpiry:        time.Unix(futureExpiry, 0),
		sessionID:          "test-session",
		machineID:          "test-machine",
		integrationID:      defaultCopilotIntegrationID,
		openAIOrganization: defaultCopilotOpenAIOrganization,

		editorVersion:       defaultCopilotEditorVersion,
		editorPluginVersion: defaultCopilotEditorPluginVersion,
		userAgent:           defaultCopilotUserAgent,
		githubToken:         "ghu_test",
	}

	req := domaingateway.NewGenerateContentRequest("claude-sonnet-4.6", "", "hello", 1000)

	var deltas []string
	resp, err := g.GenerateContentStream(context.Background(), req, func(ev domaingateway.StreamEvent) error {
		if ev.Kind == domaingateway.StreamEventText {
			deltas = append(deltas, ev.Delta)
		}
		return nil
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Blocks, 1)
	assert.Equal(t, "Hello world", resp.Blocks[0].Text)
	assert.Equal(t, []string{"Hello", " world"}, deltas)
}

// --- API error handling ---

func TestCopilotGenerateContentAPIError(t *testing.T) {
	t.Parallel()

	futureExpiry := time.Now().Add(30 * time.Minute).Unix()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited","type":"rate_limit","code":"429"}}`))
	}))
	defer server.Close()

	g := &copilotGateway{
		client:             server.Client(),
		baseURL:            server.URL,
		copilotToken:       "copilot_tok",
		tokenExpiry:        time.Unix(futureExpiry, 0),
		sessionID:          "test-session",
		machineID:          "test-machine",
		integrationID:      defaultCopilotIntegrationID,
		openAIOrganization: defaultCopilotOpenAIOrganization,

		editorVersion:       defaultCopilotEditorVersion,
		editorPluginVersion: defaultCopilotEditorPluginVersion,
		userAgent:           defaultCopilotUserAgent,
		githubToken:         "ghu_test",
	}

	req := domaingateway.NewGenerateContentRequest("claude-sonnet-4.6", "", "hello", 1000)
	_, err := g.GenerateContent(context.Background(), req)
	require.Error(t, err)
	assert.True(t,
		strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate limit") || strings.Contains(err.Error(), "failed after"),
		"expected rate limit or retry error, got: %v", err)
}

// --- accumulateCopilotToolCalls ---

func TestAccumulateCopilotToolCalls(t *testing.T) {
	t.Parallel()

	// First delta starts a new tool call with ID.
	acc := accumulateCopilotToolCalls(nil, []copilotToolCall{
		{ID: "call_1", Type: "function", Function: copilotToolCallEntry{Name: "get_weather", Arguments: `{"loc`}},
	})
	require.Len(t, acc, 1)
	assert.Equal(t, "call_1", acc[0].ID)
	assert.Equal(t, `{"loc`, acc[0].Function.Arguments)

	// Second delta appends to arguments.
	acc = accumulateCopilotToolCalls(acc, []copilotToolCall{
		{Function: copilotToolCallEntry{Arguments: `ation":"Paris"}`}},
	})
	require.Len(t, acc, 1)
	assert.Equal(t, `{"location":"Paris"}`, acc[0].Function.Arguments)
}

// --- parseCopilotStream ---

func TestParseCopilotStream(t *testing.T) {
	t.Parallel()

	sseData := "data: {\"choices\":[{\"delta\":{\"content\":\"Foo\"},\"finish_reason\":null}]}\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"Bar\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n" +
		"data: [DONE]\n"

	var events []domaingateway.StreamEvent
	resp, err := parseCopilotStream(strings.NewReader(sseData), func(ev domaingateway.StreamEvent) error {
		events = append(events, ev)
		return nil
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Blocks, 1)
	assert.Equal(t, "FooBar", resp.Blocks[0].Text)

	// Should have received text events + done event.
	var textDeltas []string
	for _, ev := range events {
		if ev.Kind == domaingateway.StreamEventText {
			textDeltas = append(textDeltas, ev.Delta)
		}
	}
	assert.Equal(t, []string{"Foo", "Bar"}, textDeltas)
}

// --- parseCopilotStream: stream error ---

func TestParseCopilotStreamError(t *testing.T) {
	t.Parallel()

	sseData := "data: {\"error\":{\"message\":\"context_length_exceeded\",\"type\":\"invalid_request_error\",\"code\":\"context_length_exceeded\"}}\n"

	_, err := parseCopilotStream(strings.NewReader(sseData), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context_length_exceeded")
}

// --- buildCopilotRequest ---

func TestBuildCopilotRequest(t *testing.T) {
	t.Parallel()

	temp := 0.7
	req := domaingateway.NewGenerateContentRequest("claude-haiku-4.5", "sys", "usr", 512)
	req.WithTemperature(&temp)

	msgs := buildCopilotMessages(req)
	body := buildCopilotRequest(req, msgs, false)

	assert.Equal(t, "claude-haiku-4.5", *body.Model)
	assert.Equal(t, 512, body.MaxTokens)
	assert.InDelta(t, 0.7, *body.Temperature, 0.001)
	assert.False(t, body.Stream)
	require.NotNil(t, body.Messages)
	assert.Len(t, *body.Messages, 2) // system + user
}

func TestBuildCopilotRequestStream(t *testing.T) {
	t.Parallel()

	req := domaingateway.NewGenerateContentRequest("claude-sonnet-4.6", "", "hello", 1000)
	msgs := buildCopilotMessages(req)
	body := buildCopilotRequest(req, msgs, true)

	assert.True(t, body.Stream)
}
