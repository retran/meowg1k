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

package gateway

import (
	"fmt"
)

// Provider defines an enumeration for supported LLM providers.
type Provider string

const (
	// Llama identifies the Llama provider.
	Llama Provider = "llama"
	// Gemini identifies the Gemini provider.
	Gemini Provider = "gemini"
	// Nebius identifies the Nebius AI Studio provider.
	Nebius Provider = "nebius"
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

// Config holds all possible configuration for any gateway provider.
type Config struct {
	Provider Provider
	BaseURL  string // Used by Llama
	APIKey   string // Used by Gemini
}

// Option defines a function that configures the gateway.
// It can return an error if an option is invalid.
type Option func(c *Config) error

// WithProvider sets the LLM provider (e.g., Llama, Gemini, Nebius, OpenAI, OpenRouter, Anthropic, Voyage). This is a required option.
func WithProvider(p Provider) Option {
	return func(c *Config) error {
		switch p {
		case Llama, Gemini, Nebius, OpenAI, OpenRouter, Anthropic, OpenAICompatible, Voyage:
			c.Provider = p
		default:
			return fmt.Errorf("unsupported provider: %s", p)
		}
		return nil
	}
}

// WithBaseURL sets the base URL for a local Llama-compatible server.
func WithBaseURL(url string) Option {
	return func(c *Config) error {
		c.BaseURL = url
		return nil
	}
}

// WithAPIKey sets the API key directly.
func WithAPIKey(key string) Option {
	return func(c *Config) error {
		c.APIKey = key
		return nil
	}
}
