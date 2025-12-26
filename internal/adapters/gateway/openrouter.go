// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// openrouterGateway implements gateway interfaces for OpenRouter API.
type openrouterGateway struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

// NewOpenRouterGateway creates a new OpenRouter gateway with a shared HTTP client.
// The HTTP client is provided via dependency injection to allow for better resource management
// and connection pooling across multiple gateway instances.
func NewOpenRouterGateway(
	_ context.Context,
	baseURL string,
	apiKey string,
	httpClient *http.Client,
) (ports.GenerationGateway, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required for OpenRouter gateway")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API key is required for OpenRouter gateway")
	}

	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client is required for OpenRouter gateway")
	}

	return &openrouterGateway{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  httpClient,
	}, nil
}

// OpenRouter API request/response structures.
type openrouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openrouterRequest struct {
	FrequencyPenalty  *float64            `json:"frequency_penalty,omitempty"`
	TopA              *float64            `json:"top_a,omitempty"`
	N                 *int                `json:"n,omitempty"`
	Temperature       *float64            `json:"temperature,omitempty"`
	TopP              *float64            `json:"top_p,omitempty"`
	TopK              *int                `json:"top_k,omitempty"`
	RepetitionPenalty *float64            `json:"repetition_penalty,omitempty"`
	PresencePenalty   *float64            `json:"presence_penalty,omitempty"`
	Seed              *int                `json:"seed,omitempty"`
	MinP              *float64            `json:"min_p,omitempty"`
	Model             string              `json:"model"`
	Messages          []openrouterMessage `json:"messages"`
	Stop              []string            `json:"stop,omitempty"`
	MaxTokens         int                 `json:"max_tokens,omitempty"`
}

type openrouterChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type openrouterResponse struct {
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
	Choices []openrouterChoice `json:"choices"`
}

// GenerateContent sends a content generation request to OpenRouter API.
func (g *openrouterGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return "", fmt.Errorf("request cannot be nil")
	}

	messages := buildOpenRouterMessages(request)
	reqBody := buildOpenRouterRequest(request, messages)

	// Marshal request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		g.baseURL+"/chat/completions",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/retran/meowg1k")
	httpReq.Header.Set("X-Title", "meowg1k")

	// Send request
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request to OpenRouter: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // Defer close errors are not critical

	return parseOpenRouterResponse(resp)
}

func buildOpenRouterMessages(request *gateway.GenerateContentRequest) []openrouterMessage {
	messages := []openrouterMessage{}
	if request.SystemPrompt() != "" {
		messages = append(messages, openrouterMessage{
			Role:    "system",
			Content: request.SystemPrompt(),
		})
	}
	messages = append(messages, openrouterMessage{
		Role:    "user",
		Content: request.UserPrompt(),
	})
	return messages
}

func buildOpenRouterRequest(request *gateway.GenerateContentRequest, messages []openrouterMessage) openrouterRequest {
	reqBody := openrouterRequest{
		Model:     request.Model(),
		Messages:  messages,
		MaxTokens: request.MaxOutputTokens(),
	}

	applyOpenRouterSampling(&reqBody, request)
	applyOpenRouterPenalties(&reqBody, request)
	applyOpenRouterControlParams(&reqBody, request)

	return reqBody
}

func applyOpenRouterSampling(reqBody *openrouterRequest, request *gateway.GenerateContentRequest) {
	if temp := request.Temperature(); temp != nil {
		reqBody.Temperature = temp
	}
	if topP := request.TopP(); topP != nil {
		reqBody.TopP = topP
	}
	if topK := request.TopK(); topK != nil {
		reqBody.TopK = topK
	}
	if topA := request.TopA(); topA != nil {
		reqBody.TopA = topA
	}
	if minP := request.MinP(); minP != nil {
		reqBody.MinP = minP
	}
}

func applyOpenRouterPenalties(reqBody *openrouterRequest, request *gateway.GenerateContentRequest) {
	if fp := request.FrequencyPenalty(); fp != nil {
		reqBody.FrequencyPenalty = fp
	}
	if pp := request.PresencePenalty(); pp != nil {
		reqBody.PresencePenalty = pp
	}
	if rp := request.RepetitionPenalty(); rp != nil {
		reqBody.RepetitionPenalty = rp
	}
}

func applyOpenRouterControlParams(reqBody *openrouterRequest, request *gateway.GenerateContentRequest) {
	if seed := request.Seed(); seed != nil {
		reqBody.Seed = seed
	}
	if stop := request.Stop(); len(stop) > 0 {
		reqBody.Stop = stop
	}
	if n := request.CandidateCount(); n != nil {
		reqBody.N = n
	}
}

func parseOpenRouterResponse(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenRouter API returned status %d: %s", resp.StatusCode, string(body))
	}

	var openrouterResp openrouterResponse
	if err := json.Unmarshal(body, &openrouterResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if openrouterResp.Error != nil {
		return "", fmt.Errorf("OpenRouter API error: %s (type: %s, code: %s)",
			openrouterResp.Error.Message,
			openrouterResp.Error.Type,
			openrouterResp.Error.Code,
		)
	}

	if len(openrouterResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from OpenRouter API")
	}

	return openrouterResp.Choices[0].Message.Content, nil
}
