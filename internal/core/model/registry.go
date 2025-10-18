// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package model

import model2 "github.com/retran/meowg1k/internal/domain/model"

// Registry is the private implementation of the Registry interface.
type Registry struct {
	models map[string]model2.ModelInfo
}

func NewRegistry() *Registry {
	return &Registry{
		models: models,
	}
}

// Get returns information about a specific model.
func (r *Registry) Get(modelName string) model2.ModelInfo {
	if r == nil || r.models == nil {
		// Return sensible defaults for nil service
		return model2.ModelInfo{
			Provider:         "unknown",
			MaxContextTokens: 8192,
			TokenizerType:    model2.TokenizerUnknown,
			Description:      "Unknown model (service is nil)",
		}
	}

	if info, exists := r.models[modelName]; exists {
		return info
	}

	// Return sensible defaults for unknown models
	return model2.ModelInfo{
		Provider:         "unknown",
		MaxContextTokens: 8192,
		TokenizerType:    model2.TokenizerUnknown,
		Description:      "Unknown model",
	}
}

// GetMaxContextTokens returns the maximum context tokens for a model.
func (r *Registry) GetMaxContextTokens(modelName string) int {
	return r.Get(modelName).MaxContextTokens
}

// GetTokenizerType returns the tokenizer type for a model.
func (r *Registry) GetTokenizerType(modelName string) model2.Tokenizer {
	return r.Get(modelName).TokenizerType
}

// GetDefaultEmbedDimension returns the default embedding dimension for a model.
func (r *Registry) GetDefaultEmbedDimension(modelName string) int {
	return r.Get(modelName).DefaultEmbedDimension
}

// GetProvider returns the provider for a model.
func (r *Registry) GetProvider(modelName string) string {
	return r.Get(modelName).Provider
}

// GetMaxOutputTokens returns the maximum output tokens for a model.
// Returns 4096 as a safe default if the model is not found or has no limit specified.
func (r *Registry) GetMaxOutputTokens(modelName string) int {
	maxOutputTokens := r.Get(modelName).MaxOutputTokens
	if maxOutputTokens <= 0 {
		return 4096 // Safe default
	}

	return maxOutputTokens
}

// ListKnownModels returns a list of all models in the registry.
func (r *Registry) ListKnownModels() []string {
	models := make([]string, 0, len(r.models))
	for model := range r.models {
		models = append(models, model)
	}

	return models
}

// models contains information about all known models.
var models = map[string]model2.ModelInfo{
	// OpenAI models (provider: openai)
	"gpt-5": {
		Provider:         "openai",
		MaxContextTokens: 400000,
		MaxOutputTokens:  128000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-5 - flagship model for coding, reasoning, and agentic tasks",
	},
	"gpt-5-mini": {
		Provider:         "openai",
		MaxContextTokens: 400000,
		MaxOutputTokens:  128000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-5 mini - faster, cost-efficient version for well-defined tasks",
	},
	"gpt-5-nano": {
		Provider:         "openai",
		MaxContextTokens: 400000,
		MaxOutputTokens:  128000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-5 nano - fastest, most cost-efficient version for summarization",
	},
	"gpt-4.1": {
		Provider:         "openai",
		MaxContextTokens: 1047576,
		MaxOutputTokens:  32768,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-4.1 - smartest non-reasoning model for instruction following, 1M context",
	},
	"gpt-4.1-mini": {
		Provider:         "openai",
		MaxContextTokens: 1047576,
		MaxOutputTokens:  32768,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-4.1 Mini - long-context at lower latency/cost",
	},
	"gpt-4.1-nano": {
		Provider:         "openai",
		MaxContextTokens: 1047576,
		MaxOutputTokens:  32768,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-4.1 Nano - cheapest model with 1M context",
	},
	"gpt-4o": {
		Provider:         "openai",
		MaxContextTokens: 128000,
		MaxOutputTokens:  32768,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-4o - flagship multimodal model",
	},
	"gpt-4o-mini": {
		Provider:         "openai",
		MaxContextTokens: 128000,
		MaxOutputTokens:  32768,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-4o Mini - fast, efficient small model",
	},
	"o1-preview": {
		Provider:         "openai",
		MaxContextTokens: 200000,
		MaxOutputTokens:  100000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI o1 Preview - powerful reasoning model",
	},
	"o1-mini": {
		Provider:         "openai",
		MaxContextTokens: 200000,
		MaxOutputTokens:  100000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI o1 Mini - fast, cost-efficient reasoning model",
	},

	// Google Gemini models (provider: gemini)
	"gemini-2.5-pro": {
		Provider:         "gemini",
		MaxContextTokens: 1048576,
		MaxOutputTokens:  65536,
		TokenizerType:    model2.TokenizerGemini,
		Description:      "Google Gemini 2.5 Pro - state-of-the-art thinking model for complex reasoning, 1M context",
	},
	"gemini-2.5-flash": {
		Provider:         "gemini",
		MaxContextTokens: 1048576,
		MaxOutputTokens:  65536,
		TokenizerType:    model2.TokenizerGemini,
		Description:      "Google Gemini 2.5 Flash - best price-performance, optimized for scale and speed, 1M context",
	},
	"gemini-2.5-flash-lite": {
		Provider:         "gemini",
		MaxContextTokens: 1048576,
		MaxOutputTokens:  65536,
		TokenizerType:    model2.TokenizerGemini,
		Description:      "Google Gemini 2.5 Flash-Lite - optimized for cost-efficiency and high throughput, 1M context",
	},
	"gemini-live-2.5-flash-preview": {
		Provider:         "gemini",
		MaxContextTokens: 1048576,
		MaxOutputTokens:  8192,
		TokenizerType:    model2.TokenizerGemini,
		Description:      "Google Gemini Live 2.5 Flash - for live conversations",
	},
	"gemini-2.5-flash-image-preview": {
		Provider:         "gemini",
		MaxContextTokens: 32768,
		MaxOutputTokens:  32768,
		TokenizerType:    model2.TokenizerGemini,
		Description:      "Google Gemini 2.5 Flash Image - state-of-the-art image generation",
	},

	// OpenRouter models - Popular and High-Performance Models

	// xAI Models
	"x-ai/grok-4": {
		Provider:         "openrouter",
		MaxContextTokens: 256000,
		MaxOutputTokens:  256000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "xAI Grok 4 - flagship pure-reasoning model, 1M context",
	},
	"x-ai/grok-code-fast-1": {
		Provider:         "openrouter",
		MaxContextTokens: 256000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "xAI Grok Code Fast 1 - optimized for agentic code generation",
	},
	"x-ai/grok-3": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "xAI Grok 3 - legacy model for general tasks",
	},
	"x-ai/grok-3-mini": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "xAI Grok 3 Mini - legacy model, fast and efficient",
	},

	// Anthropic models (provider: openrouter)
	"anthropic/claude-sonnet-4-5": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		MaxOutputTokens:  64000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Sonnet 4.5 (via OpenRouter) - best for complex agents and coding",
	},
	"anthropic/claude-opus-4-1": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		MaxOutputTokens:  32000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Opus 4.1 (via OpenRouter) - exceptional for specialized complex tasks",
	},
	"anthropic/claude-sonnet-4": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		MaxOutputTokens:  64000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Sonnet 4 (via OpenRouter) - high-performance model",
	},
	"anthropic/claude-opus-4": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		MaxOutputTokens:  32000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Opus 4 (via OpenRouter) - previous flagship model",
	},
	"anthropic/claude-3-7-sonnet": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		MaxOutputTokens:  64000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude 3.7 Sonnet (via OpenRouter) - high-performance with early extended thinking",
	},
	"anthropic/claude-3-5-haiku": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		MaxOutputTokens:  8192,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude 3.5 Haiku (via OpenRouter) - fastest model",
	},

	// Anthropic models (direct provider: anthropic)
	// Claude 4.5 and Claude 4 - Latest generation (2025)
	"claude-sonnet-4-5-20250929": {
		Provider:         "anthropic",
		MaxContextTokens: 200000, // 1M beta available with context-1m-2025-08-07 header
		MaxOutputTokens:  64000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Sonnet 4.5 (Sep 2025) - best model for complex agents and coding with highest intelligence",
	},
	"claude-sonnet-4-5": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  64000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Sonnet 4.5 (alias) - latest snapshot",
	},
	"claude-opus-4-1-20250805": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  32000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Opus 4.1 (Aug 2025) - exceptional model for specialized complex tasks",
	},
	"claude-opus-4-1": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  32000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Opus 4.1 (alias) - latest snapshot",
	},
	"claude-sonnet-4-20250514": {
		Provider:         "anthropic",
		MaxContextTokens: 200000, // 1M beta available with context-1m-2025-08-07 header
		MaxOutputTokens:  64000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Sonnet 4 (May 2025) - high-performance model",
	},
	"claude-sonnet-4-0": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  64000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Sonnet 4 (alias) - latest snapshot",
	},
	"claude-sonnet-4": {
		Provider:         "anthropic",
		MaxContextTokens: 1000000, // Extended thinking with 1M context
		MaxOutputTokens:  64000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Sonnet 4 (alias) - latest snapshot with extended context",
	},
	"claude-opus-4-20250514": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  32000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Opus 4 (May 2025) - previous flagship model",
	},
	"claude-opus-4-0": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  32000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Opus 4 (alias) - latest snapshot",
	},
	"claude-3-7-sonnet-20250219": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  64000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Sonnet 3.7 (Feb 2025) - high-performance with early extended thinking",
	},
	"claude-3-7-sonnet-latest": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  64000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude Sonnet 3.7 (alias) - latest snapshot",
	},

	// Claude 3.5 generation (2024)
	"claude-3-5-haiku-20241022": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  8192,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude 3.5 Haiku (Oct 2024) - fastest model for near-instant responsiveness",
	},
	"claude-3-5-haiku-latest": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  8192,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude 3.5 Haiku (alias) - latest snapshot",
	},
	"claude-3-5-sonnet-20241022": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  8192,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude 3.5 Sonnet (Oct 2024) - DEPRECATED - balanced performance and speed",
	},

	// Claude 3 generation (legacy, 2024)
	"claude-3-haiku-20240307": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  4096,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude 3 Haiku (Mar 2024) - fast and compact model",
	},
	"claude-3-opus-20240229": {
		Provider:         "anthropic",
		MaxContextTokens: 200000,
		MaxOutputTokens:  4096,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Anthropic Claude 3 Opus (Feb 2024) - DEPRECATED - most capable model for complex tasks",
	},

	// Google models (provider: openrouter)
	"google/gemini-2.5-pro": {
		Provider:         "openrouter",
		MaxContextTokens: 1048576,
		MaxOutputTokens:  65536,
		TokenizerType:    model2.TokenizerGemini,
		Description:      "Google Gemini 2.5 Pro (via OpenRouter)",
	},
	"google/gemini-2.5-flash": {
		Provider:         "openrouter",
		MaxContextTokens: 1048576,
		MaxOutputTokens:  65536,
		TokenizerType:    model2.TokenizerGemini,
		Description:      "Google Gemini 2.5 Flash (via OpenRouter)",
	},
	"google/gemini-2.5-flash-lite": {
		Provider:         "openrouter",
		MaxContextTokens: 1048576,
		MaxOutputTokens:  65536,
		TokenizerType:    model2.TokenizerGemini,
		Description:      "Google Gemini 2.5 Flash Lite (via OpenRouter)",
	},
	"google/gemini-pro-1.5": {
		Provider:         "openrouter",
		MaxContextTokens: 2000000,
		TokenizerType:    model2.TokenizerGemini,
		Description:      "Google Gemini 1.5 Pro - DEPRECATED, unique 2M token context",
	},

	// OpenAI models (provider: openrouter)
	"openai/gpt-5": {
		Provider:         "openrouter",
		MaxContextTokens: 400000,
		MaxOutputTokens:  128000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-5 (via OpenRouter)",
	},
	"openai/gpt-5-mini": {
		Provider:         "openrouter",
		MaxContextTokens: 400000,
		MaxOutputTokens:  128000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-5 Mini (via OpenRouter)",
	},
	"openai/gpt-4.1": {
		Provider:         "openrouter",
		MaxContextTokens: 1047576,
		MaxOutputTokens:  32768,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-4.1 (via OpenRouter)",
	},
	"openai/gpt-4.1-mini": {
		Provider:         "openrouter",
		MaxContextTokens: 1047576,
		MaxOutputTokens:  32768,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-4.1 Mini (via OpenRouter)",
	},
	"openai/gpt-4o": {
		Provider:         "openrouter",
		MaxContextTokens: 128000,
		MaxOutputTokens:  32768,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-4o - flagship multimodal model",
	},
	"openai/gpt-4o-mini": {
		Provider:         "openrouter",
		MaxContextTokens: 128000,
		MaxOutputTokens:  32768,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI GPT-4o Mini - fast, efficient small model",
	},
	"openai/o1-preview": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		MaxOutputTokens:  100000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI o1 Preview - powerful reasoning model for code, math, science",
	},
	"openai/o1-mini": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		MaxOutputTokens:  100000,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "OpenAI o1 Mini - fast, cost-efficient reasoning model",
	},

	// DeepSeek models (provider: openrouter)
	"deepseek/deepseek-chat-v3.1": {
		Provider:         "openrouter",
		MaxContextTokens: 163840,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "DeepSeek V3.1 - latest chat model",
	},
	"deepseek/deepseek-r1-0528": {
		Provider:         "openrouter",
		MaxContextTokens: 163840,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "DeepSeek R1 0528 - reasoning-focused model",
	},

	// Meta Llama models (provider: openrouter)
	"meta-llama/llama-4-maverick": {
		Provider:         "openrouter",
		MaxContextTokens: 1048576,
		MaxOutputTokens:  4096,
		TokenizerType:    model2.TokenizerLlama,
		Description:      "Meta Llama 4 Maverick - natively multimodal model for image and text",
	},
	"meta-llama/llama-4-scout": {
		Provider:         "openrouter",
		MaxContextTokens: 10000000,
		TokenizerType:    model2.TokenizerLlama,
		Description:      "Meta Llama 4 Scout - natively multimodal with superior visual intelligence, 10M context",
	},
	"meta-llama/llama-3.3-70b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    model2.TokenizerLlama,
		Description:      "Meta Llama 3.3 70B Instruct - latest 70B model",
	},
	"meta-llama/llama-3.3-8b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 128000,
		TokenizerType:    model2.TokenizerLlama,
		Description:      "Meta Llama 3.3 8B Instruct - latest 8B model",
	},
	"meta-llama/llama-3.2-3b-instruct:free": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		MaxOutputTokens:  4096,
		TokenizerType:    model2.TokenizerLlama,
		Description:      "Meta Llama 3.2 3B Instruct (Free Tier) - compact instruction-tuned model",
	},
	"meta-llama/llama-3.1-405b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    model2.TokenizerLlama,
		Description:      "Meta Llama 3.1 405B Instruct - largest Llama 3 model",
	},

	// Qwen models (provider: openrouter)
	"qwen/qwen3-coder-plus": {
		Provider:         "openrouter",
		MaxContextTokens: 128000,
		MaxOutputTokens:  65536,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Qwen3 Coder Plus - powerful proprietary coding agent",
	},
	"qwen/qwen3-next-80b-a3b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 262144,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Qwen3 Next 80B Instruct - optimized for fast, stable responses",
	},
	"qwen/qwq-32b": {
		Provider:         "openrouter",
		MaxContextTokens: 32768,
		TokenizerType:    model2.TokenizerCL100K,
		Description:      "Qwen QwQ 32B - reasoning and math focused",
	},

	// Mistral models (provider: openrouter)
	"mistralai/mistral-nemo": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		MaxOutputTokens:  128000,
		TokenizerType:    model2.TokenizerSentencePiece,
		Description:      "Mistral Nemo - best multilingual open-source model",
	},
	"mistralai/mistral-large-2411": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    model2.TokenizerSentencePiece,
		Description:      "Mistral Large 2.1 - top-tier model for high-complexity tasks",
	},
	"mistralai/mistral-small-3.2-24b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 128000,
		TokenizerType:    model2.TokenizerSentencePiece,
		Description:      "Mistral Small 3.2 24B - leading small model with image understanding",
	},

	// Other popular models (provider: openrouter)
	"nousresearch/hermes-4-70b": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    model2.TokenizerLlama,
		Description:      "Nous Hermes 4 70B - fine-tuned on Llama 3.1 with hybrid reasoning",
	},

	// Embedding models

	// OpenAI embedding models
	"text-embedding-3-large": {
		Provider:              "openai",
		MaxContextTokens:      8192,
		TokenizerType:         model2.TokenizerCL100K,
		DefaultEmbedDimension: 3072,
		Description:           "OpenAI Text Embedding 3 Large - high-performance embedding model with 3072 dimensions",
	},
	"text-embedding-3-small": {
		Provider:              "openai",
		MaxContextTokens:      8192,
		TokenizerType:         model2.TokenizerCL100K,
		DefaultEmbedDimension: 1536,
		Description:           "OpenAI Text Embedding 3 Small - cost-effective embedding model with 1536 dimensions",
	},
	"text-embedding-ada-002": {
		Provider:              "openai",
		MaxContextTokens:      8192,
		TokenizerType:         model2.TokenizerCL100K,
		DefaultEmbedDimension: 1536,
		Description:           "OpenAI Ada 002 - previous generation embedding model with 1536 dimensions",
	},

	// Google Gemini embedding models
	"text-embedding-004": {
		Provider:              "gemini",
		MaxContextTokens:      2048,
		TokenizerType:         model2.TokenizerGemini,
		DefaultEmbedDimension: 768,
		Description:           "Google Gemini Text Embedding 004 - latest embedding model with 768 dimensions",
	},

	// Voyage AI embedding models
	"voyage-3-large": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         model2.TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI 3 Large - best general-purpose and multilingual retrieval quality",
	},
	"voyage-3": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         model2.TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI 3 - optimized for general-purpose and multilingual retrieval quality",
	},
	"voyage-3-lite": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         model2.TokenizerCL100K,
		DefaultEmbedDimension: 512,
		Description:           "Voyage AI 3 Lite - optimized for latency and cost",
	},
	"voyage-code-3": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         model2.TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI Code 3 - optimized for code retrieval",
	},
	"voyage-law-2": {
		Provider:              "voyage",
		MaxContextTokens:      16000,
		TokenizerType:         model2.TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI Law 2 - specialized for legal documents",
	},
	"voyage-finance-2": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         model2.TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI Finance 2 - specialized for financial documents",
	},
	"voyage-multimodal-3": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         model2.TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI Multimodal 3 - for interleaved text and content-rich images",
	},
}
