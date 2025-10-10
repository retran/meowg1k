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

package provider

import (
	"time"
)

// Provider defines an enumeration for supported LLM providers.
type Provider string

const (
	// Llama identifies the Llama provider.
	Llama Provider = "llama"
	// Gemini identifies the Gemini provider.
	Gemini Provider = "gemini"
	// OpenAI identifies the OpenAI provider.
	OpenAI Provider = "openai"
	// OpenRouter identifies the OpenRouter provider.
	OpenRouter Provider = "openrouter"
	// OpenAICompatible identifies OpenAI-compatible providers with custom base URLs.
	OpenAICompatible Provider = "openai-compatible"
	// Anthropic identifies the Anthropic provider.
	Anthropic Provider = "anthropic"
	// Voyage identifies the Voyage AI provider (embeddings only).
	Voyage Provider = "voyage"
)

// ProviderDefinition defines the characteristics of a provider.
type ProviderDefinition struct {
	Type            Provider      `json:"type"`
	Name            string        `json:"name"`
	DefaultModel    string        `json:"default_model"`
	DefaultBaseURL  string        `json:"default_base_url"`
	DefaultEnvVar   string        `json:"default_env_var"`
	RequiresAPIKey  bool          `json:"requires_api_key"`
	RequiresBaseURL bool          `json:"requires_base_url"`
	MaxInputTokens  int           `json:"max_input_tokens"`
	MaxOutputTokens int           `json:"max_output_tokens"`
	DefaultTimeout  time.Duration `json:"default_timeout"`
}
