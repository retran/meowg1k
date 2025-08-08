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

// Package llama provides a client for interacting with the llama.cpp server endpoints.
package llama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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

// CompletionClient provides methods to interact with LLM completion API
type CompletionClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewCompletionClient creates a new client for interacting with the LLM completion endpoint.
func NewCompletionClient(baseURL string) (*CompletionClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL cannot be empty")
	}

	return &CompletionClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Minute},
	}, nil
}

// Complete generates a completion based on the provided request.
func (c *CompletionClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/completion"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var completionResp CompletionResponse
	if err := json.Unmarshal(bodyBytes, &completionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &completionResp, nil
}
