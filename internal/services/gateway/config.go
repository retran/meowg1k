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

	"github.com/retran/meowg1k/internal/models/gateway"
)

// Option defines a function that configures the gateway.
// It can return an error if an option is invalid.
type Option func(c *gateway.Config) error

// WithProvider sets the LLM provider (e.g., Llama, Gemini, Nebius, OpenAI, OpenRouter, Anthropic, Voyage). This is a required option.
func WithProvider(p gateway.Provider) Option {
	return func(c *gateway.Config) error {
		switch p {
		case gateway.Llama, gateway.Gemini, gateway.Nebius, gateway.OpenAI, gateway.OpenRouter, gateway.Anthropic, gateway.OpenAICompatible, gateway.Voyage:
			c.Provider = p
		default:
			return fmt.Errorf("unsupported provider: %s", p)
		}
		return nil
	}
}

// WithBaseURL sets the base URL for a local Llama-compatible server.
func WithBaseURL(url string) Option {
	return func(c *gateway.Config) error {
		c.BaseURL = url
		return nil
	}
}

// WithAPIKey sets the API key directly.
func WithAPIKey(key string) Option {
	return func(c *gateway.Config) error {
		c.APIKey = key
		return nil
	}
}
