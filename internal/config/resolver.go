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

package config

import (
	"fmt"
	"os"
	"time"

	"github.com/retran/meowg1k/internal/llm/gateway"
	"github.com/retran/meowg1k/internal/models"
)

// ResolvedProfile represents a profile with all values resolved.
type ResolvedProfile struct {
	Provider        gateway.Provider
	Model           string
	MaxInputTokens  int
	MaxOutputTokens int
	Timeout         time.Duration
	BaseURL         string
	APIKey          string
	TokenizerType   models.TokenizerType
}

// ResolveProfile resolves a profile by name, applying defaults if necessary.
func (c *Config) ResolveProfile(profileName string) (*ResolvedProfile, error) {
	if c.Profiles == nil {
		return nil, fmt.Errorf("no profiles defined in configuration")
	}

	profile, exists := c.Profiles[profileName]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found in configuration", profileName)
	}

	// Apply defaults for missing values
	resolved := &ResolvedProfile{
		Model:           profile.Model,
		MaxInputTokens:  profile.MaxInputTokens,
		MaxOutputTokens: profile.MaxOutputTokens,
		Timeout:         profile.Timeout,
		BaseURL:         profile.BaseURL,
		TokenizerType:   profile.TokenizerType,
	}

	// Parse and validate provider
	switch profile.Provider {
	case "gemini":
		resolved.Provider = gateway.Gemini
	case "llama":
		resolved.Provider = gateway.Llama
	case "nebius":
		resolved.Provider = gateway.Nebius
	case "openai":
		resolved.Provider = gateway.OpenAI
	case "openrouter":
		resolved.Provider = gateway.OpenRouter
	case "anthropic":
		resolved.Provider = gateway.Anthropic
	case "voyage":
		resolved.Provider = gateway.Voyage
	case "openai-compatible":
		resolved.Provider = gateway.OpenAICompatible
	default:
		return nil, fmt.Errorf("unknown provider '%s' in profile '%s'", profile.Provider, profileName)
	}

	// Apply defaults for missing values
	if resolved.Model == "" {
		switch resolved.Provider {
		case gateway.Gemini:
			resolved.Model = "gemini-2.5-flash"
		case gateway.Nebius:
			resolved.Model = "Qwen2.5-Coder-7B"
		case gateway.OpenAI:
			resolved.Model = "gpt-5-mini"
		case gateway.OpenRouter:
			resolved.Model = "meta-llama/llama-3.2-3b-instruct:free"
		case gateway.Anthropic:
			resolved.Model = "claude-3-5-haiku-20241022"
		case gateway.Voyage:
			resolved.Model = "voyage-3.5"
		case gateway.Llama, gateway.OpenAICompatible:
			// For llama and openai-compatible, model is determined by the server
			resolved.Model = ""
		}
	}

	// Apply default baseURL if not specified
	if resolved.BaseURL == "" {
		switch resolved.Provider {
		case gateway.OpenAI:
			resolved.BaseURL = "https://api.openai.com/v1"
		case gateway.OpenRouter:
			resolved.BaseURL = "https://openrouter.ai/api/v1"
		case gateway.Gemini:
			// Gemini doesn't use baseURL, it has its own client configuration
			resolved.BaseURL = ""
		case gateway.Nebius:
			resolved.BaseURL = "https://api.studio.nebius.com/v1"
		case gateway.Voyage:
			resolved.BaseURL = "https://api.voyageai.com/v1"
		case gateway.Llama:
			// Llama requires explicit baseURL
			return nil, fmt.Errorf("llama provider in profile '%s' requires baseURL", profileName)
		case gateway.OpenAICompatible:
			// OpenAI-compatible requires explicit baseURL
			return nil, fmt.Errorf("openai-compatible provider in profile '%s' requires baseURL", profileName)
		}
	}

	// Apply default API key environment variable if not specified
	apiKeyEnv := profile.APIKeyEnv
	if apiKeyEnv == "" {
		switch resolved.Provider {
		case gateway.Gemini:
			apiKeyEnv = "MEOW_GEMINI_API_KEY"
		case gateway.Nebius:
			apiKeyEnv = "MEOW_NEBIUS_API_KEY"
		case gateway.OpenAI:
			apiKeyEnv = "MEOW_OPENAI_API_KEY"
		case gateway.OpenRouter:
			apiKeyEnv = "MEOW_OPENROUTER_API_KEY"
		case gateway.Anthropic:
			apiKeyEnv = "MEOW_ANTHROPIC_API_KEY"
		case gateway.Llama:
			apiKeyEnv = "MEOW_LLAMA_API_KEY"
		case gateway.Voyage:
			apiKeyEnv = "MEOW_VOYAGE_API_KEY"
		case gateway.OpenAICompatible:
			apiKeyEnv = "MEOW_OPENAI_COMPATIBLE_API_KEY"
		}
	}
	resolved.APIKey = os.Getenv(apiKeyEnv)

	// Validate that required API keys are present
	switch resolved.Provider {
	case gateway.Gemini, gateway.Nebius, gateway.OpenAI, gateway.OpenRouter,
		gateway.Anthropic, gateway.Voyage, gateway.OpenAICompatible:
		if resolved.APIKey == "" {
			return nil, fmt.Errorf("API key environment variable '%s' not set for %s provider in profile '%s'",
				apiKeyEnv, resolved.Provider, profileName)
		}
	case gateway.Llama:
		// Llama API key is optional, so no validation needed
	}

	// Apply default tokenizer if not specified
	if resolved.TokenizerType == "" {
		// First try to get tokenizer from model info
		if resolved.Model != "" {
			modelTokenizer := models.GetTokenizerType(resolved.Model)
			if modelTokenizer != models.TokenizerUnknown {
				resolved.TokenizerType = modelTokenizer
			}
		}

		// If still not resolved, fall back to provider defaults
		if resolved.TokenizerType == "" {
			switch resolved.Provider {
			case gateway.Gemini:
				resolved.TokenizerType = models.TokenizerGemini
			case gateway.OpenAI, gateway.OpenRouter, gateway.Anthropic:
				resolved.TokenizerType = models.TokenizerCL100K
			case gateway.Nebius:
				resolved.TokenizerType = models.TokenizerSentencePiece
			case gateway.Llama:
				resolved.TokenizerType = models.TokenizerLlama
			case gateway.OpenAICompatible:
				resolved.TokenizerType = models.TokenizerUnknown
			}
		}
	}

	if resolved.MaxInputTokens == 0 {
		resolved.MaxInputTokens = 8192
	}

	// Apply default MaxOutputTokens if not specified
	if resolved.MaxOutputTokens == 0 {
		if resolved.Model != "" {
			// Get max output tokens from model registry
			resolved.MaxOutputTokens = models.GetMaxOutputTokens(resolved.Model)
		} else {
			// Fallback to safe default
			resolved.MaxOutputTokens = 4096
		}
	}

	if resolved.Timeout == 0 {
		resolved.Timeout = 5 * time.Minute
	}

	return resolved, nil
}

// GetGenerateTask retrieves a generate task by name.
func (c *Config) GetGenerateTask(taskName string) (*GenerateTask, error) {
	if c.Generate == nil || c.Generate.Tasks == nil {
		return nil, fmt.Errorf("no generate tasks defined in configuration")
	}

	task, exists := c.Generate.Tasks[taskName]
	if !exists {
		return nil, fmt.Errorf("generate task '%s' not found in configuration", taskName)
	}

	return task, nil
}

// GetDefaultGenerateProfile returns the default profile for generate command.
func (c *Config) GetDefaultGenerateProfile() string {
	if c.Generate != nil && c.Generate.Default != nil && c.Generate.Default.Profile != "" {
		return c.Generate.Default.Profile
	}
	return "fast" // fallback default
}

// GetDefaultGenerateSystemPrompt returns the default system prompt for generate command.
func (c *Config) GetDefaultGenerateSystemPrompt() string {
	if c.Generate != nil && c.Generate.Default != nil {
		return c.Generate.Default.SystemPrompt
	}
	return ""
}
