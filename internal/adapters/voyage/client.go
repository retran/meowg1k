// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package voyage provides an HTTP client for the Voyage AI embedding API.
package voyage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	// DefaultBaseURL is the default base URL for the Voyage AI API.
	DefaultBaseURL = "https://api.voyageai.com/v1"
	// DefaultModel is the default model for embeddings.
	DefaultModel = "voyage-3.5"
)

// Client represents a client for the Voyage AI API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient creates a new Voyage AI client with the given configuration.
// The HTTP client is provided via dependency injection to allow for better resource management
// and connection pooling across multiple client instances.
func NewClient(baseURL, apiKey string, httpClient *http.Client) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("voyage API key is required")
	}

	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client cannot be nil")
	}

	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: httpClient,
	}, nil
}

// EmbeddingRequest represents a request to the Voyage AI embeddings endpoint.
type EmbeddingRequest struct {
	Model           string   `json:"model"`
	InputType       string   `json:"input_type,omitempty"`
	Input           []string `json:"input"`
	OutputDimension int      `json:"output_dimension,omitempty"`
}

// EmbeddingData represents a single embedding item in a response.
type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingUsage represents token usage in a response.
type EmbeddingUsage struct {
	TotalTokens int `json:"total_tokens"`
}

// EmbeddingResponse represents a response from the Voyage AI embeddings endpoint.
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Model  string          `json:"model"`
	Data   []EmbeddingData `json:"data"`
	Usage  EmbeddingUsage  `json:"usage"`
}

// ErrorResponse represents an error response from the Voyage AI API.
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// CreateEmbeddings sends a request to create embeddings for the given input texts.
func (c *Client) CreateEmbeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("client is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if req.Model == "" {
		req.Model = DefaultModel
	}

	requestBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	url := c.baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request to %q: %w", url, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %q: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // Defer close errors are not critical

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %q: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("API request to %q failed with status %d: %s", url, resp.StatusCode, string(body))
		}

		return nil, fmt.Errorf("API request to %q failed: %s", url, errorResp.Error.Message)
	}

	var embeddingResp EmbeddingResponse
	if err := json.Unmarshal(body, &embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal embedding response from %q: %w", url, err)
	}

	return &embeddingResp, nil
}
