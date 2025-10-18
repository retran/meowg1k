/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package llama provides an HTTP client for interacting with Llama model APIs (local and hosted).
package llama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CompletionRequest represents the request body for /completion endpoint
type CompletionRequest struct {
	Prompt              any         `json:"prompt"`
	Temperature         float64     `json:"temperature,omitempty"`
	DynatempRange       float64     `json:"dynatemp_range,omitempty"`
	DynatempExponent    float64     `json:"dynatemp_exponent,omitempty"`
	TopK                int         `json:"top_k,omitempty"`
	TopP                float64     `json:"top_p,omitempty"`
	MinP                float64     `json:"min_p,omitempty"`
	NPredict            int         `json:"n_predict,omitempty"`
	NIndent             int         `json:"n_indent,omitempty"`
	NKeep               int         `json:"n_keep,omitempty"`
	Stream              bool        `json:"stream,omitempty"`
	Stop                []string    `json:"stop,omitempty"`
	TypicalP            float64     `json:"typical_p,omitempty"`
	RepeatPenalty       float64     `json:"repeat_penalty,omitempty"`
	RepeatLastN         int         `json:"repeat_last_n,omitempty"`
	PresencePenalty     float64     `json:"presence_penalty,omitempty"`
	FrequencyPenalty    float64     `json:"frequency_penalty,omitempty"`
	DryMultiplier       float64     `json:"dry_multiplier,omitempty"`
	DryBase             float64     `json:"dry_base,omitempty"`
	DryAllowedLength    int         `json:"dry_allowed_length,omitempty"`
	DryPenaltyLastN     int         `json:"dry_penalty_last_n,omitempty"`
	DrySequenceBreakers []string    `json:"dry_sequence_breakers,omitempty"`
	XtcProbability      float64     `json:"xtc_probability,omitempty"`
	XtcThreshold        float64     `json:"xtc_threshold,omitempty"`
	Mirostat            int         `json:"mirostat,omitempty"`
	MirostatTau         float64     `json:"mirostat_tau,omitempty"`
	MirostatEta         float64     `json:"mirostat_eta,omitempty"`
	Grammar             string      `json:"grammar,omitempty"`
	JSONSchema          any         `json:"json_schema,omitempty"`
	Seed                int         `json:"seed,omitempty"`
	IgnoreEOS           bool        `json:"ignore_eos,omitempty"`
	LogitBias           any         `json:"logit_bias,omitempty"`
	NProbs              int         `json:"n_probs,omitempty"`
	MinKeep             int         `json:"min_keep,omitempty"`
	TMaxPredictMs       int         `json:"t_max_predict_ms,omitempty"`
	ImageData           []ImageData `json:"image_data,omitempty"`
	IDSlot              int         `json:"id_slot,omitempty"`
	CachePrompt         bool        `json:"cache_prompt,omitempty"`
	ReturnTokens        bool        `json:"return_tokens,omitempty"`
	Samplers            []string    `json:"samplers,omitempty"`
	TimingsPerToken     bool        `json:"timings_per_token,omitempty"`
	PostSamplingProbs   bool        `json:"post_sampling_probs,omitempty"`
	ResponseFields      []string    `json:"response_fields,omitempty"`
	Lora                []Lora      `json:"lora,omitempty"`
}

// ImageData represents base64-encoded image data
type ImageData struct {
	Data string `json:"data"`
	ID   int    `json:"id"`
}

// Lora represents a LoRA adapter configuration
type Lora struct {
	ID    int     `json:"id"`
	Scale float64 `json:"scale"`
}

// CompletionResponse represents the response from /completion endpoint
type CompletionResponse struct {
	Content            string         `json:"content"`
	Tokens             []int          `json:"tokens,omitempty"`
	Stop               bool           `json:"stop,omitempty"`
	GenerationSettings map[string]any `json:"generation_settings,omitempty"`
	Model              string         `json:"model,omitempty"`
	Prompt             any            `json:"prompt,omitempty"`
	StopType           string         `json:"stop_type,omitempty"`
	Timings            map[string]any `json:"timings,omitempty"`
	TokensCached       int            `json:"tokens_cached,omitempty"`
	TokensEvaluated    int            `json:"tokens_evaluated,omitempty"`
	Truncated          bool           `json:"truncated,omitempty"`
	Probs              []TokenProb    `json:"probs,omitempty"`
}

// TokenProb represents token probability information
type TokenProb struct {
	ID          int         `json:"id"`
	Logprob     float64     `json:"logprob,omitempty"`
	Token       string      `json:"token"`
	Bytes       []int       `json:"bytes,omitempty"`
	TopLogprobs []TokenProb `json:"top_logprobs,omitempty"`
}

// EmbeddingRequest represents the request body for /embedding endpoint
// Content can be either a single string or array of strings for batch processing
type EmbeddingRequest struct {
	Content    interface{} `json:"content"` // string or []string
	ImageData  []ImageData `json:"image_data,omitempty"`
	NormOutput bool        `json:"norm_output,omitempty"`
	Truncate   int         `json:"truncate,omitempty"`
	Lora       []Lora      `json:"lora,omitempty"`
}

// EmbeddingResponse represents the response from /embedding endpoint for single text
type EmbeddingResponse struct {
	Embedding       []float64      `json:"embedding"`
	Model           string         `json:"model,omitempty"`
	Timings         map[string]any `json:"timings,omitempty"`
	TokensEvaluated int            `json:"tokens_evaluated,omitempty"`
}

// EmbeddingBatchItem represents a single item in batch embedding response
type EmbeddingBatchItem struct {
	Index     int         `json:"index"`
	Embedding [][]float64 `json:"embedding"` // nested array from llama.cpp
	Object    string      `json:"object,omitempty"`
	Model     string      `json:"model,omitempty"`
}

// Client provides methods to interact with LLM completion API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new client for interacting with the LLM completion endpoint.
// The HTTP client is provided via dependency injection to allow for better resource management
// and connection pooling across multiple client instances.
func NewClient(baseURL, apiKey string, httpClient *http.Client) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL cannot be empty")
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

// Complete generates a completion based on the provided request.
func (c *Client) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("client is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/completion"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request to %q: %w", url, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %q: %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %q: %w", url, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request to %q failed with status %d: %s", url, resp.StatusCode, string(bodyBytes))
	}

	var completionResp CompletionResponse
	if err := json.Unmarshal(bodyBytes, &completionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response from %q: %w", url, err)
	}

	return &completionResp, nil
}

// Embedding generates an embedding vector for the provided content.
func (c *Client) Embedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("client is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/embedding"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request to %q: %w", url, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %q: %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %q: %w", url, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request to %q failed with status %d: %s", url, resp.StatusCode, string(bodyBytes))
	}

	// llama.cpp can return multiple formats:
	// 1. Nested array: [[0.1, 0.2, ...]]
	// 2. Simple array: [0.1, 0.2, ...]
	// 3. Object: {"embedding": [0.1, 0.2, ...], "model": "..."}

	// Try nested array first (most common format from llama.cpp)
	var nestedEmbedding [][]float64
	if err := json.Unmarshal(bodyBytes, &nestedEmbedding); err == nil && len(nestedEmbedding) > 0 {
		return &EmbeddingResponse{
			Embedding: nestedEmbedding[0],
		}, nil
	}

	// Try simple array
	var embedding []float64
	if err := json.Unmarshal(bodyBytes, &embedding); err == nil {
		return &EmbeddingResponse{
			Embedding: embedding,
		}, nil
	}

	// Finally, try object format
	var embeddingResp EmbeddingResponse
	if err := json.Unmarshal(bodyBytes, &embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response from %q: %w", url, err)
	}

	return &embeddingResp, nil
}

// EmbeddingBatch generates embedding vectors for multiple texts in a single request.
// Returns a slice of embeddings in the same order as input texts.
func (c *Client) EmbeddingBatch(ctx context.Context, texts []string, normOutput bool) ([][]float64, error) {
	if c == nil {
		return nil, fmt.Errorf("client is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if len(texts) == 0 {
		return [][]float64{}, nil
	}

	req := &EmbeddingRequest{
		Content:    texts,
		NormOutput: normOutput,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/embedding"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request to %q: %w", url, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %q: %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %q: %w", url, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request to %q failed with status %d: %s", url, resp.StatusCode, string(bodyBytes))
	}

	// Parse batch response: array of {index, embedding: [[...]]}
	var batchItems []EmbeddingBatchItem
	if err := json.Unmarshal(bodyBytes, &batchItems); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch response from %q: %w", url, err)
	}

	// Extract embeddings and sort by index to match input order
	results := make([][]float64, len(texts))
	for _, item := range batchItems {
		if item.Index < 0 || item.Index >= len(texts) {
			return nil, fmt.Errorf("invalid index %d in batch response (expected 0-%d)", item.Index, len(texts)-1)
		}
		if len(item.Embedding) == 0 {
			return nil, fmt.Errorf("empty embedding for index %d", item.Index)
		}
		// Extract first (and only) embedding from nested array
		results[item.Index] = item.Embedding[0]
	}

	return results, nil
}
