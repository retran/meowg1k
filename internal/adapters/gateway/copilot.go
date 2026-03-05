// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// Default Neovim-based identity constants for GitHub Copilot API.
const (
	defaultCopilotAppID               = "Iv1.b507a08c87ecfe98"
	defaultCopilotEditorVersion       = "Neovim/0.6.1"
	defaultCopilotEditorPluginVersion = "copilot.vim/1.16.0"
	defaultCopilotUserAgent           = "GithubCopilot/1.155.0"
	defaultCopilotIntegrationID       = "vscode-chat"
	defaultCopilotOpenAIOrganization  = "github-copilot"

	copilotAPITokenURL = "https://api.github.com/copilot_internal/v2/token"
)

// copilotGateway implements gateway interfaces for GitHub Copilot API.
type copilotGateway struct {
	client              *http.Client
	baseURL             string
	appID               string
	editorVersion       string
	editorPluginVersion string
	userAgent           string
	integrationID       string
	openAIOrganization  string
	tokenFile           string
	githubToken         string
	copilotToken        string
	tokenExpiry         time.Time
	sessionID           string
	machineID           string
	mu                  sync.Mutex
}

// copilotGatewayOptions holds all configurable parameters for creating a Copilot gateway.
type copilotGatewayOptions struct {
	BaseURL             string
	AppID               string
	EditorVersion       string
	EditorPluginVersion string
	UserAgent           string
	IntegrationID       string
	OpenAIOrganization  string
	HTTPClient          *http.Client
}

func orDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// newCopilotGateway creates a new GitHub Copilot gateway.
// It loads the GitHub OAuth token from disk (running the device flow if absent),
// then generates stable session/machine IDs.
func newCopilotGateway(ctx context.Context, opts copilotGatewayOptions) (ports.GenerationGateway, error) {
	if opts.HTTPClient == nil {
		return nil, fmt.Errorf("HTTP client is required for GitHub Copilot gateway")
	}

	baseURL := orDefault(opts.BaseURL, "https://api.githubcopilot.com")
	appID := orDefault(opts.AppID, defaultCopilotAppID)
	editorVersion := orDefault(opts.EditorVersion, defaultCopilotEditorVersion)
	editorPluginVersion := orDefault(opts.EditorPluginVersion, defaultCopilotEditorPluginVersion)
	userAgent := orDefault(opts.UserAgent, defaultCopilotUserAgent)
	integrationID := orDefault(opts.IntegrationID, defaultCopilotIntegrationID)
	openAIOrganization := orDefault(opts.OpenAIOrganization, defaultCopilotOpenAIOrganization)

	// Determine token file path.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	tokenFile := filepath.Join(homeDir, ".config", "meowg1k", "copilot_token")

	// Generate stable machine ID: SHA256 of hostname.
	machineID, err := generateMachineID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate machine ID: %w", err)
	}

	// Generate stable-per-process session ID: uuid4 + unix milliseconds.
	sessionID := fmt.Sprintf("%s%d", uuid.New().String(), time.Now().UnixMilli())

	g := &copilotGateway{
		client:              opts.HTTPClient,
		baseURL:             baseURL,
		appID:               appID,
		editorVersion:       editorVersion,
		editorPluginVersion: editorPluginVersion,
		userAgent:           userAgent,
		integrationID:       integrationID,
		openAIOrganization:  openAIOrganization,
		tokenFile:           tokenFile,
		sessionID:           sessionID,
		machineID:           machineID,
	}

	// Load or acquire GitHub OAuth token.
	if err := g.loadOrAcquireGitHubToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to acquire GitHub Copilot credentials: %w", err)
	}

	return g, nil
}

// generateMachineID returns a SHA256 hex digest of the system hostname.
func generateMachineID() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	sum := sha256.Sum256([]byte(hostname))
	return fmt.Sprintf("%x", sum), nil
}

// loadOrAcquireGitHubToken loads the persisted GitHub token from disk.
// If no token is found, it fails fast with a message directing the user to run 'meow auth copilot'.
func (g *copilotGateway) loadOrAcquireGitHubToken(_ context.Context) error {
	data, err := os.ReadFile(g.tokenFile)
	if err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			g.githubToken = token
			return nil
		}
	}

	return fmt.Errorf("no GitHub Copilot token found — run 'meow auth copilot' to authenticate")
}

// refreshTokenIfNeeded exchanges the GitHub OAuth token for a short-lived Copilot API token,
// refreshing only when the current token has expired (or is about to).
func (g *copilotGateway) refreshTokenIfNeeded(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Refresh if token is missing or expires within 60 seconds.
	if g.copilotToken != "" && time.Now().Add(60*time.Second).Before(g.tokenExpiry) {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", copilotAPITokenURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create token exchange request: %w", err)
	}
	g.setEditorHeaders(req)
	req.Header.Set("Authorization", "token "+g.githubToken)

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("token exchange request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read token exchange response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token exchange returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse token exchange response: %w", err)
	}
	if tokenResp.Token == "" {
		return fmt.Errorf("token exchange returned empty token")
	}

	g.copilotToken = tokenResp.Token
	g.tokenExpiry = parseCopilotTokenExpiry(tokenResp.Token)
	return nil
}

// parseCopilotTokenExpiry extracts the exp= unix timestamp from the Copilot token string.
// The token is a semicolon-separated key=value string, e.g. "tid=...;exp=1234567890;sku=...".
func parseCopilotTokenExpiry(token string) time.Time {
	for _, part := range strings.Split(token, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == "exp" {
			var ts int64
			if _, err := fmt.Sscanf(strings.TrimSpace(kv[1]), "%d", &ts); err == nil {
				return time.Unix(ts, 0)
			}
		}
	}
	// Fallback: assume 25-minute TTL (Copilot tokens are ~30 min).
	return time.Now().Add(25 * time.Minute)
}

// setEditorHeaders sets the editor identity headers used for GitHub/Copilot API calls.
func (g *copilotGateway) setEditorHeaders(req *http.Request) {
	req.Header.Set("Editor-Version", g.editorVersion)
	req.Header.Set("Editor-Plugin-Version", g.editorPluginVersion)
	req.Header.Set("User-Agent", g.userAgent)
}

// isAgentRequest returns true when the request represents an agentic turn —
// i.e. the last message in the conversation is not a plain user message.
// This maps to x-initiator: "agent" per the Copilot API convention.
func isAgentRequest(request *gateway.GenerateContentRequest) bool {
	msgs := request.Messages()
	if len(msgs) == 0 {
		// Simple user-prompt request — always a user turn.
		return false
	}
	return msgs[len(msgs)-1].Role != gateway.MessageRoleUser
}

// buildRequestHeaders builds the full header set for a chat completions request.
// isAgent should be true when the last message in the conversation is not a plain user
// turn (i.e. an assistant continuation or tool result), signalling an agentic exchange
// to the Copilot API via the x-initiator header.
func (g *copilotGateway) buildRequestHeaders(isAgent bool) map[string]string {
	initiator := "user"
	if isAgent {
		initiator = "agent"
	}
	return map[string]string{
		"Authorization":          "Bearer " + g.copilotToken,
		"X-Request-Id":           uuid.New().String(),
		"Vscode-Sessionid":       g.sessionID,
		"Vscode-Machineid":       g.machineID,
		"Copilot-Integration-Id": g.integrationID,
		"Openai-Organization":    g.openAIOrganization,
		"Openai-Intent":          "conversation-edits",
		"x-initiator":            initiator,
		"Content-Type":           "application/json",
		"Editor-Version":         g.editorVersion,
		"Editor-Plugin-Version":  g.editorPluginVersion,
		"User-Agent":             g.userAgent,
		"Sec-Fetch-Site":         "none",
		"Sec-Fetch-Mode":         "no-cors",
		"Sec-Fetch-Dest":         "empty",
	}
}

// Copilot API request/response structures (OpenAI-compatible).
type copilotMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ToolCalls  []copilotToolCall `json:"tool_calls,omitempty"`
}

type copilotRequest struct {
	Messages         *[]copilotMessage `json:"messages"`
	Stop             *[]string         `json:"stop,omitempty"`
	Tools            *[]copilotTool    `json:"tools,omitempty"`
	Model            *string           `json:"model"`
	ToolChoice       *string           `json:"tool_choice,omitempty"`
	FrequencyPenalty *float64          `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64          `json:"presence_penalty,omitempty"`
	Temperature      *float64          `json:"temperature,omitempty"`
	TopP             *float64          `json:"top_p,omitempty"`
	N                *int              `json:"n,omitempty"`
	Seed             *int              `json:"seed,omitempty"`
	MaxTokens        int               `json:"max_tokens,omitempty"`
	Stream           bool              `json:"stream"`
}

type copilotTool struct {
	Function copilotToolFunction `json:"function"`
	Type     string              `json:"type"`
}

type copilotToolFunction struct {
	Parameters  map[string]any `json:"parameters,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
}

type copilotToolCall struct {
	ID       string               `json:"id,omitempty"`
	Type     string               `json:"type,omitempty"`
	Function copilotToolCallEntry `json:"function"`
}

type copilotToolCallEntry struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

type copilotChoice struct {
	Message struct {
		Content   string            `json:"content"`
		ToolCalls []copilotToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
}

type copilotResponse struct {
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
	Choices []copilotChoice `json:"choices"`
	Usage   *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// Streaming response structures.
type copilotStreamDelta struct {
	Role      string            `json:"role,omitempty"`
	Content   string            `json:"content,omitempty"`
	ToolCalls []copilotToolCall `json:"tool_calls,omitempty"`
}

type copilotStreamChoice struct {
	Delta        copilotStreamDelta `json:"delta"`
	FinishReason *string            `json:"finish_reason"`
}

type copilotStreamChunk struct {
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
	Choices []copilotStreamChoice `json:"choices"`
	Usage   *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// GenerateContent sends a content generation request to GitHub Copilot API.
func (g *copilotGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (*gateway.GenerateContentResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	return RetryWithBackoff(ctx, DefaultRetryConfig(), func(ctx context.Context) (*gateway.GenerateContentResponse, error) {
		if err := g.refreshTokenIfNeeded(ctx); err != nil {
			return nil, fmt.Errorf("failed to refresh Copilot token: %w", err)
		}

		messages := buildCopilotMessages(request)
		reqBody := buildCopilotRequest(request, messages, false)
		if tools := request.Tools(); len(tools) > 0 {
			toolList := buildCopilotTools(tools)
			if len(toolList) > 0 {
				reqBody.Tools = &toolList
			}
			reqBody.ToolChoice = stringPtr("auto")
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", g.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}
		for k, v := range g.buildRequestHeaders(isAgentRequest(request)) {
			httpReq.Header.Set(k, v)
		}

		resp, err := g.client.Do(httpReq)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }() //nolint:errcheck

		return parseCopilotResponse(resp)
	}, fmt.Sprintf("GitHub Copilot GenerateContent for model %q", request.Model()))
}

// GenerateContentStream implements streaming for GitHub Copilot using native SSE streaming.
func (g *copilotGateway) GenerateContentStream(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
	callback gateway.StreamCallback,
) (*gateway.GenerateContentResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := g.refreshTokenIfNeeded(ctx); err != nil {
		if callback != nil {
			_ = callback(gateway.StreamEvent{Kind: gateway.StreamEventError, Error: err.Error(), Recoverable: false})
		}
		return nil, fmt.Errorf("failed to refresh Copilot token: %w", err)
	}

	messages := buildCopilotMessages(request)
	reqBody := buildCopilotRequest(request, messages, true)
	if tools := request.Tools(); len(tools) > 0 {
		toolList := buildCopilotTools(tools)
		if len(toolList) > 0 {
			reqBody.Tools = &toolList
		}
		reqBody.ToolChoice = stringPtr("auto")
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		if callback != nil {
			_ = callback(gateway.StreamEvent{Kind: gateway.StreamEventError, Error: err.Error(), Recoverable: false})
		}
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		if callback != nil {
			_ = callback(gateway.StreamEvent{Kind: gateway.StreamEventError, Error: err.Error(), Recoverable: false})
		}
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	for k, v := range g.buildRequestHeaders(isAgentRequest(request)) {
		httpReq.Header.Set(k, v)
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		if callback != nil {
			_ = callback(gateway.StreamEvent{Kind: gateway.StreamEventError, Error: err.Error(), Recoverable: false})
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		streamErr := fmt.Errorf("GitHub Copilot API returned status %d: %s", resp.StatusCode, string(body))
		if callback != nil {
			_ = callback(gateway.StreamEvent{Kind: gateway.StreamEventError, Error: streamErr.Error(), Recoverable: false})
		}
		return nil, streamErr
	}

	return parseCopilotStream(resp.Body, callback)
}

// parseCopilotStream reads the SSE stream and fires callback events.
func parseCopilotStream(body io.Reader, callback gateway.StreamCallback) (*gateway.GenerateContentResponse, error) {
	var fullContent strings.Builder
	var toolCallAccumulator []copilotToolCall
	var usage *gateway.UsageMetadata

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk copilotStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed chunks
		}

		if chunk.Error != nil {
			streamErr := fmt.Errorf("GitHub Copilot stream error: %s (type: %s, code: %s)",
				chunk.Error.Message, chunk.Error.Type, chunk.Error.Code)
			if callback != nil {
				_ = callback(gateway.StreamEvent{Kind: gateway.StreamEventError, Error: streamErr.Error(), Recoverable: false})
			}
			return nil, streamErr
		}

		if chunk.Usage != nil {
			usage = &gateway.UsageMetadata{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fullContent.WriteString(choice.Delta.Content)
				if callback != nil {
					if err := callback(gateway.StreamEvent{
						Kind:  gateway.StreamEventText,
						Delta: choice.Delta.Content,
					}); err != nil {
						return nil, err
					}
				}
			}
			// Accumulate tool call deltas.
			toolCallAccumulator = accumulateCopilotToolCalls(toolCallAccumulator, choice.Delta.ToolCalls)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream read error: %w", err)
	}

	// Build final response.
	blocks := make([]gateway.ContentBlock, 0)
	if content := fullContent.String(); content != "" {
		blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockText, Text: content})
	}

	toolCalls, err := parseCopilotToolCalls(toolCallAccumulator)
	if err != nil {
		return nil, err
	}
	for i := range toolCalls {
		call := toolCalls[i]
		blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockToolCall, ToolCall: &call})
	}

	finalResp := &gateway.GenerateContentResponse{Blocks: blocks, Usage: usage}

	if callback != nil {
		if err := callback(gateway.StreamEvent{Kind: gateway.StreamEventDone, Usage: usage}); err != nil {
			return finalResp, err
		}
	}

	return finalResp, nil
}

// accumulateCopilotToolCalls merges streaming tool call deltas into an accumulator slice.
func accumulateCopilotToolCalls(acc []copilotToolCall, deltas []copilotToolCall) []copilotToolCall {
	for _, delta := range deltas {
		// Tool calls have an index; expand slice as needed.
		// The Copilot stream sends index implicitly via order; we append sequentially.
		if len(acc) == 0 || delta.ID != "" {
			acc = append(acc, copilotToolCall{
				ID:   delta.ID,
				Type: delta.Type,
				Function: copilotToolCallEntry{
					Name:      delta.Function.Name,
					Arguments: delta.Function.Arguments,
				},
			})
		} else {
			// Accumulate arguments into the last tool call.
			last := &acc[len(acc)-1]
			last.Function.Arguments += delta.Function.Arguments
			if delta.Function.Name != "" {
				last.Function.Name += delta.Function.Name
			}
		}
	}
	return acc
}

func buildCopilotMessages(request *gateway.GenerateContentRequest) []copilotMessage {
	msgs := request.Messages()
	if len(msgs) > 0 {
		mapped := make([]copilotMessage, 0, len(msgs))
		for i := range msgs {
			mapped = append(mapped, mapToCopilotMessage(&msgs[i]))
		}
		return mapped
	}

	messages := []copilotMessage{}
	if request.SystemPrompt() != "" {
		messages = append(messages, copilotMessage{Role: "system", Content: request.SystemPrompt()})
	}
	messages = append(messages, copilotMessage{Role: "user", Content: request.UserPrompt()})
	return messages
}

func mapToCopilotMessage(m *gateway.Message) copilotMessage {
	cm := copilotMessage{Role: string(m.Role), Content: m.Content}
	if len(m.ToolCalls) > 0 {
		cm.ToolCalls = mapCopilotToolCalls(m.ToolCalls)
	}
	if m.Role == gateway.MessageRoleTool {
		cm.Role = "tool"
		cm.ToolCallID = m.ToolCallID
	}
	return cm
}

func mapCopilotToolCalls(toolCalls []gateway.ToolCall) []copilotToolCall {
	calls := make([]copilotToolCall, 0, len(toolCalls))
	for _, c := range toolCalls {
		args, err := json.Marshal(c.Arguments)
		if err != nil {
			args = []byte("{}")
		}
		calls = append(calls, copilotToolCall{
			ID:   c.ID,
			Type: "function",
			Function: copilotToolCallEntry{
				Name:      c.Name,
				Arguments: string(args),
			},
		})
	}
	return calls
}

func buildCopilotRequest(request *gateway.GenerateContentRequest, messages []copilotMessage, stream bool) copilotRequest {
	messageList := messages
	reqBody := copilotRequest{
		Model:     stringPtr(request.Model()),
		Messages:  &messageList,
		MaxTokens: request.MaxOutputTokens(),
		Stream:    stream,
	}
	if temp := request.Temperature(); temp != nil {
		reqBody.Temperature = temp
	}
	if topP := request.TopP(); topP != nil {
		reqBody.TopP = topP
	}
	if fp := request.FrequencyPenalty(); fp != nil {
		reqBody.FrequencyPenalty = fp
	}
	if pp := request.PresencePenalty(); pp != nil {
		reqBody.PresencePenalty = pp
	}
	if seed := request.Seed(); seed != nil {
		reqBody.Seed = seed
	}
	if stop := request.Stop(); len(stop) > 0 {
		reqBody.Stop = &stop
	}
	if n := request.CandidateCount(); n != nil {
		reqBody.N = n
	}
	return reqBody
}

func buildCopilotTools(tools []gateway.ToolDefinition) []copilotTool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]copilotTool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, copilotTool{
			Type: "function",
			Function: copilotToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}
	return result
}

func parseCopilotResponse(resp *http.Response) (*gateway.GenerateContentResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub Copilot API returned status %d: %s", resp.StatusCode, string(body))
	}

	var copilotResp copilotResponse
	if err := json.Unmarshal(body, &copilotResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if copilotResp.Error != nil {
		return nil, fmt.Errorf("GitHub Copilot API error: %s (type: %s, code: %s)",
			copilotResp.Error.Message, copilotResp.Error.Type, copilotResp.Error.Code)
	}

	if len(copilotResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from GitHub Copilot API")
	}

	message := copilotResp.Choices[0].Message
	toolCalls, err := parseCopilotToolCalls(message.ToolCalls)
	if err != nil {
		return nil, err
	}

	blocks := make([]gateway.ContentBlock, 0, 1+len(toolCalls))
	if message.Content != "" {
		blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockText, Text: message.Content})
	}
	for i := range toolCalls {
		call := toolCalls[i]
		blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockToolCall, ToolCall: &call})
	}

	var usage *gateway.UsageMetadata
	if copilotResp.Usage != nil {
		usage = &gateway.UsageMetadata{
			PromptTokens:     copilotResp.Usage.PromptTokens,
			CompletionTokens: copilotResp.Usage.CompletionTokens,
			TotalTokens:      copilotResp.Usage.TotalTokens,
		}
	}

	return &gateway.GenerateContentResponse{Blocks: blocks, Usage: usage}, nil
}

func parseCopilotToolCalls(calls []copilotToolCall) ([]gateway.ToolCall, error) {
	if len(calls) == 0 {
		return nil, nil
	}
	result := make([]gateway.ToolCall, 0, len(calls))
	for _, call := range calls {
		var args map[string]any
		if call.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool call arguments: %w", err)
			}
		}
		if args == nil {
			args = map[string]any{}
		}
		result = append(result, gateway.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: args,
		})
	}
	return result, nil
}

// CountTokens estimates token count for Copilot models using character-based estimation.
// Approximately (chars + 2) / 3 tokens.
func (g *copilotGateway) CountTokens(_ context.Context, _ string, texts []string) (int, error) {
	if g == nil {
		return 0, fmt.Errorf("copilot gateway is nil")
	}
	if len(texts) == 0 {
		return 0, nil
	}
	totalChars := 0
	for _, text := range texts {
		totalChars += len(text)
	}
	return (totalChars + 2) / 3, nil
}
