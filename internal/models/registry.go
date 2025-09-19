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

// Package models provides information about supported AI models and their capabilities.
package models

// TokenizerType represents the type of tokenizer used by a model.
type TokenizerType string

const (
	// TokenizerCL100K is the tokenizer used by GPT-4 and similar OpenAI models.
	TokenizerCL100K TokenizerType = "cl100k_base"
	// TokenizerGPT2 is the tokenizer used by older OpenAI models.
	TokenizerGPT2 TokenizerType = "gpt2"
	// TokenizerSentencePiece is used by many open-source models.
	TokenizerSentencePiece TokenizerType = "sentencepiece"
	// TokenizerTikToken is the general tiktoken tokenizer.
	TokenizerTikToken TokenizerType = "tiktoken"
	// TokenizerGemini is used by Google Gemini models.
	TokenizerGemini TokenizerType = "gemini"
	// TokenizerLlama is used by Llama models.
	TokenizerLlama TokenizerType = "llama"
	// TokenizerUnknown is used when the tokenizer type is unknown.
	TokenizerUnknown TokenizerType = "unknown"
)

// ModelInfo contains comprehensive information about a specific AI model.
type ModelInfo struct {
	Provider              string        // The provider offering this model
	MaxContextTokens      int           // Maximum number of context tokens
	TokenizerType         TokenizerType // Type of tokenizer used
	Description           string        // Human-readable description
	DefaultEmbedDimension int           // Default embedding dimension (0 if not applicable)
}

// ModelRegistry contains information about all known models.
var ModelRegistry = map[string]ModelInfo{
	// OpenAI models (provider: openai)
	"gpt-5": {
		Provider:         "openai",
		MaxContextTokens: 400000,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI GPT-5 - flagship model for coding, reasoning, and agentic tasks",
	},
	"gpt-5-mini": {
		Provider:         "openai",
		MaxContextTokens: 400000,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI GPT-5 mini - faster, cost-efficient version for well-defined tasks",
	},
	"gpt-5-nano": {
		Provider:         "openai",
		MaxContextTokens: 400000,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI GPT-5 nano - fastest, most cost-efficient version for summarization",
	},
	"gpt-4.1": {
		Provider:         "openai",
		MaxContextTokens: 1047576,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI GPT-4.1 - smartest non-reasoning model for instruction following",
	},

	// Google Gemini models (provider: gemini)
	"gemini-2.5-pro": {
		Provider:         "gemini",
		MaxContextTokens: 1048576,
		TokenizerType:    TokenizerGemini,
		Description:      "Google Gemini 2.5 Pro - state-of-the-art thinking model for complex reasoning",
	},
	"gemini-2.5-flash": {
		Provider:         "gemini",
		MaxContextTokens: 1048576,
		TokenizerType:    TokenizerGemini,
		Description:      "Google Gemini 2.5 Flash - best price-performance, optimized for scale and speed",
	},
	"gemini-2.5-flash-lite": {
		Provider:         "gemini",
		MaxContextTokens: 1048576,
		TokenizerType:    TokenizerGemini,
		Description:      "Google Gemini 2.5 Flash-Lite - optimized for cost-efficiency and high throughput",
	},

	// OpenRouter models - Popular and High-Performance Models

	// xAI Models
	"x-ai/grok-code-fast-1": {
		Provider:         "openrouter",
		MaxContextTokens: 256000,
		TokenizerType:    TokenizerCL100K,
		Description:      "xAI Grok Code Fast 1 - optimized for code generation",
	},
	"x-ai/grok-4": {
		Provider:         "openrouter",
		MaxContextTokens: 256000,
		TokenizerType:    TokenizerCL100K,
		Description:      "xAI Grok 4",
	},
	"x-ai/grok-3": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerCL100K,
		Description:      "xAI Grok 3",
	},
	"x-ai/grok-3-mini": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerCL100K,
		Description:      "xAI Grok 3 Mini",
	},

	// Anthropic models (provider: openrouter)
	"anthropic/claude-sonnet-4": {
		Provider:         "openrouter",
		MaxContextTokens: 1000000,
		TokenizerType:    TokenizerCL100K,
		Description:      "Anthropic Claude Sonnet 4",
	},
	"anthropic/claude-3.7-sonnet": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		TokenizerType:    TokenizerCL100K,
		Description:      "Anthropic Claude 3.7 Sonnet",
	},
	"anthropic/claude-3.5-sonnet": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		TokenizerType:    TokenizerCL100K,
		Description:      "Anthropic Claude 3.5 Sonnet",
	},
	"anthropic/claude-3.5-haiku": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		TokenizerType:    TokenizerCL100K,
		Description:      "Anthropic Claude 3.5 Haiku",
	},
	"anthropic/claude-opus-4.1": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		TokenizerType:    TokenizerCL100K,
		Description:      "Anthropic Claude Opus 4.1",
	},
	"anthropic/claude-3-haiku": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		TokenizerType:    TokenizerCL100K,
		Description:      "Anthropic Claude 3 Haiku",
	},

	// Google models (provider: openrouter)
	"google/gemini-2.5-flash": {
		Provider:         "openrouter",
		MaxContextTokens: 1048576,
		TokenizerType:    TokenizerGemini,
		Description:      "Google Gemini 2.5 Flash",
	},
	"google/gemini-2.0-flash-001": {
		Provider:         "openrouter",
		MaxContextTokens: 1048576,
		TokenizerType:    TokenizerGemini,
		Description:      "Google Gemini 2.0 Flash",
	},
	"google/gemini-2.5-pro": {
		Provider:         "openrouter",
		MaxContextTokens: 1048576,
		TokenizerType:    TokenizerGemini,
		Description:      "Google Gemini 2.5 Pro",
	},
	"google/gemini-2.5-flash-lite": {
		Provider:         "openrouter",
		MaxContextTokens: 1048576,
		TokenizerType:    TokenizerGemini,
		Description:      "Google Gemini 2.5 Flash Lite",
	},
	"google/gemini-flash-1.5": {
		Provider:         "openrouter",
		MaxContextTokens: 1000000,
		TokenizerType:    TokenizerGemini,
		Description:      "Google Gemini 1.5 Flash",
	},
	"google/gemini-pro-1.5": {
		Provider:         "openrouter",
		MaxContextTokens: 2000000,
		TokenizerType:    TokenizerGemini,
		Description:      "Google Gemini 1.5 Pro",
	},

	// OpenAI models (provider: openrouter)
	"openai/gpt-4.1-mini": {
		Provider:         "openrouter",
		MaxContextTokens: 1047576,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI GPT-4.1 Mini",
	},
	"openai/gpt-5": {
		Provider:         "openrouter",
		MaxContextTokens: 400000,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI GPT-5",
	},
	"openai/gpt-5-mini": {
		Provider:         "openrouter",
		MaxContextTokens: 400000,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI GPT-5 Mini",
	},
	"openai/gpt-4.1": {
		Provider:         "openrouter",
		MaxContextTokens: 1047576,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI GPT-4.1",
	},
	"openai/gpt-4o": {
		Provider:         "openrouter",
		MaxContextTokens: 128000,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI GPT-4o",
	},
	"openai/gpt-4o-mini": {
		Provider:         "openrouter",
		MaxContextTokens: 128000,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI GPT-4o Mini",
	},
	"openai/o3": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI o3",
	},
	"openai/o3-mini": {
		Provider:         "openrouter",
		MaxContextTokens: 200000,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI o3 Mini",
	},

	// DeepSeek models (provider: openrouter)
	"deepseek/deepseek-chat-v3.1": {
		Provider:         "openrouter",
		MaxContextTokens: 163840,
		TokenizerType:    TokenizerCL100K,
		Description:      "DeepSeek V3.1",
	},
	"deepseek/deepseek-chat-v3-0324": {
		Provider:         "openrouter",
		MaxContextTokens: 163840,
		TokenizerType:    TokenizerCL100K,
		Description:      "DeepSeek V3 0324",
	},
	"deepseek/deepseek-r1": {
		Provider:         "openrouter",
		MaxContextTokens: 163840,
		TokenizerType:    TokenizerCL100K,
		Description:      "DeepSeek R1",
	},
	"deepseek/deepseek-r1-0528": {
		Provider:         "openrouter",
		MaxContextTokens: 163840,
		TokenizerType:    TokenizerCL100K,
		Description:      "DeepSeek R1 0528",
	},

	// Meta Llama models (provider: openrouter)
	"meta-llama/llama-3.3-70b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerLlama,
		Description:      "Meta Llama 3.3 70B Instruct",
	},
	"meta-llama/llama-3.1-70b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerLlama,
		Description:      "Meta Llama 3.1 70B Instruct",
	},
	"meta-llama/llama-3.1-8b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 16384,
		TokenizerType:    TokenizerLlama,
		Description:      "Meta Llama 3.1 8B Instruct",
	},
	"meta-llama/llama-3.2-3b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 16384,
		TokenizerType:    TokenizerLlama,
		Description:      "Meta Llama 3.2 3B Instruct",
	},
	"meta-llama/llama-3.2-3b-instruct:free": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerLlama,
		Description:      "Meta Llama 3.2 3B Instruct (OpenRouter free tier)",
	},
	"meta-llama/llama-4-maverick": {
		Provider:         "openrouter",
		MaxContextTokens: 1048576,
		TokenizerType:    TokenizerLlama,
		Description:      "Meta Llama 4 Maverick",
	},
	"meta-llama/llama-4-scout": {
		Provider:         "openrouter",
		MaxContextTokens: 1048576,
		TokenizerType:    TokenizerLlama,
		Description:      "Meta Llama 4 Scout",
	},
	"meta-llama/llama-3.1-405b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 32768,
		TokenizerType:    TokenizerLlama,
		Description:      "Meta Llama 3.1 405B Instruct",
	},

	// Qwen models (provider: openrouter)
	"qwen/qwen3-235b-a22b-2507": {
		Provider:         "openrouter",
		MaxContextTokens: 262144,
		TokenizerType:    TokenizerCL100K,
		Description:      "Qwen3 235B A22B Instruct 2507",
	},
	"qwen/qwen3-coder": {
		Provider:         "openrouter",
		MaxContextTokens: 262144,
		TokenizerType:    TokenizerCL100K,
		Description:      "Qwen3 Coder 480B A35B",
	},
	"qwen/qwen3-30b-a3b": {
		Provider:         "openrouter",
		MaxContextTokens: 40960,
		TokenizerType:    TokenizerCL100K,
		Description:      "Qwen3 30B A3B",
	},
	"qwen/qwen3-max": {
		Provider:         "openrouter",
		MaxContextTokens: 256000,
		TokenizerType:    TokenizerCL100K,
		Description:      "Qwen3 Max",
	},
	"qwen/qwen-2.5-72b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 32768,
		TokenizerType:    TokenizerCL100K,
		Description:      "Qwen2.5 72B Instruct",
	},
	"qwen/qwq-32b": {
		Provider:         "openrouter",
		MaxContextTokens: 32768,
		TokenizerType:    TokenizerCL100K,
		Description:      "Qwen QwQ 32B - reasoning focused",
	},

	// Mistral models (provider: openrouter)
	"mistralai/mistral-nemo": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Mistral Nemo",
	},
	"mistralai/mistral-small-3.2-24b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 128000,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Mistral Small 3.2 24B",
	},
	"mistralai/mistral-large-2411": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Mistral Large 2411",
	},
	"mistralai/mixtral-8x7b-instruct": {
		Provider:         "openrouter",
		MaxContextTokens: 32768,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Mistral Mixtral 8x7B Instruct",
	},

	// Other popular models (provider: openrouter)
	"amazon/nova-lite-v1": {
		Provider:         "openrouter",
		MaxContextTokens: 300000,
		TokenizerType:    TokenizerCL100K,
		Description:      "Amazon Nova Lite 1.0",
	},
	"amazon/nova-pro-v1": {
		Provider:         "openrouter",
		MaxContextTokens: 300000,
		TokenizerType:    TokenizerCL100K,
		Description:      "Amazon Nova Pro 1.0",
	},
	"nousresearch/hermes-3-llama-3.1-70b": {
		Provider:         "openrouter",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerLlama,
		Description:      "Nous Hermes 3 70B Instruct",
	},

	// Nebius models (provider: nebius)
	// OpenAI-compatible models
	"gpt-oss-120b": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI-compatible 120B model optimized for code and reasoning",
	},
	"gpt-oss-20b": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerCL100K,
		Description:      "OpenAI-compatible 20B model for code and reasoning",
	},

	// Moonshot AI models
	"Kimi-K2-Instruct": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Moonshot AI Kimi K2 Instruct model",
	},

	// Qwen models
	"Qwen/Qwen3-Coder-480B-A35B-Instruct": {
		Provider:         "nebius",
		MaxContextTokens: 262144,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Qwen3 Coder 480B - specialized for coding and math",
	},
	"Qwen3-235B-A22B-Thinking-2507": {
		Provider:         "nebius",
		MaxContextTokens: 262144,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Qwen3 235B Thinking - advanced reasoning model",
	},
	"Qwen3-235B-A22B-Instruct-2507": {
		Provider:         "nebius",
		MaxContextTokens: 262144,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Qwen3 235B Instruct - large instruction-following model",
	},
	"Qwen3-30B-A3B-Thinking-2507": {
		Provider:         "nebius",
		MaxContextTokens: 262144,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Qwen3 30B Thinking - reasoning model",
	},
	"Qwen3-30B-A3B-Instruct-2507": {
		Provider:         "nebius",
		MaxContextTokens: 262144,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Qwen3 30B Instruct - instruction-following model",
	},
	"Qwen3-Coder-30B-A3B-Instruct": {
		Provider:         "nebius",
		MaxContextTokens: 262144,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Qwen3 Coder 30B - coding-specialized model",
	},
	"Qwen3-32B": {
		Provider:         "nebius",
		MaxContextTokens: 41984,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Qwen3 32B - general purpose model",
	},
	"Qwen3-14B": {
		Provider:         "nebius",
		MaxContextTokens: 41984,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Qwen3 14B - efficient general purpose model",
	},
	"Qwen2.5-72B-Instruct": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Qwen2.5 72B Instruct model",
	},
	"Qwen2.5-Coder-7B": {
		Provider:         "nebius",
		MaxContextTokens: 32768,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Qwen2.5 Coder 7B - fast coding model",
	},
	"QwQ-32B": {
		Provider:         "nebius",
		MaxContextTokens: 32768,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "QwQ 32B - reasoning and math specialized model",
	},

	// NousResearch models
	"Hermes-4-405B": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerLlama,
		Description:      "Hermes 4 405B - large conversational model",
	},
	"Hermes-4-70B": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerLlama,
		Description:      "Hermes 4 70B - efficient conversational model",
	},
	"Hermes-3-Llama-3.1-405B": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerLlama,
		Description:      "Hermes 3 Llama 405B - advanced conversational model",
	},

	// DeepSeek models
	"DeepSeek-R1-0528": {
		Provider:         "nebius",
		MaxContextTokens: 163840,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "DeepSeek R1 - advanced reasoning model",
	},
	"DeepSeek-V3-0324": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "DeepSeek V3 - versatile AI model",
	},
	"DeepSeek-V3": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "DeepSeek V3 - base model",
	},

	// Meta Llama models
	"Llama-3_1-Nemotron-Ultra-253B-v1": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerLlama,
		Description:      "Llama 3.1 Nemotron Ultra 253B - NVIDIA enhanced model",
	},
	"Llama-3.3-70B-Instruct": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerLlama,
		Description:      "Llama 3.3 70B Instruct - latest Meta model",
	},
	"Meta-Llama-3.1-8B-Instruct": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerLlama,
		Description:      "Meta Llama 3.1 8B Instruct - small efficient model",
	},
	"Meta-Llama-3.1-405B-Instruct": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerLlama,
		Description:      "Meta Llama 3.1 405B Instruct - largest Meta model",
	},

	// Zhipu AI models
	"GLM-4.5": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "GLM 4.5 - code and reasoning model",
	},
	"GLM-4.5-Air": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "GLM 4.5 Air - lightweight version",
	},

	// Google models
	"Gemma-2-2b-it": {
		Provider:         "nebius",
		MaxContextTokens: 8192,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Gemma 2 2B Instruct - small Google model",
	},
	"Gemma-2-9b-it": {
		Provider:         "nebius",
		MaxContextTokens: 8192,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Gemma 2 9B Instruct - efficient Google model",
	},

	// Mistral models
	"Devstral-Small-2505": {
		Provider:         "nebius",
		MaxContextTokens: 131072,
		TokenizerType:    TokenizerSentencePiece,
		Description:      "Devstral Small - Mistral coding model",
	},

	// Embedding models

	// Google Gemini embedding models
	"text-embedding-004": {
		Provider:              "gemini",
		MaxContextTokens:      2048,
		TokenizerType:         TokenizerGemini,
		DefaultEmbedDimension: 3072,
		Description:           "Google Gemini Text Embedding 004 - latest embedding model with 3072 dimensions",
	},

	// Voyage AI embedding models
	"voyage-3": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI 3 - high-performance embedding model",
	},
	"voyage-3.5": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI 3.5 - optimized for general-purpose and multilingual retrieval quality",
	},
	"voyage-3.5-lite": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI 3.5 Lite - optimized for latency and cost",
	},
	"voyage-3-large": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI 3 Large - best general-purpose and multilingual retrieval quality",
	},
	"voyage-code-3": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI Code 3 - optimized for code retrieval",
	},
	"voyage-3-lite": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 512,
		Description:           "Voyage AI 3 Lite - efficient embedding model",
	},
	"voyage-finance-2": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI Finance 2 - specialized for financial documents",
	},
	"voyage-multilingual-2": {
		Provider:              "voyage",
		MaxContextTokens:      32000,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI Multilingual 2 - optimized for 100+ languages",
	},
	"voyage-law-2": {
		Provider:              "voyage",
		MaxContextTokens:      16000,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 1024,
		Description:           "Voyage AI Law 2 - specialized for legal documents",
	},
	"voyage-code-2": {
		Provider:              "voyage",
		MaxContextTokens:      16000,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 1536,
		Description:           "Voyage AI Code 2 - optimized for code and technical documents",
	},

	// Nebius AI Studio embedding models
	"bge-multilingual-gemma2": {
		Provider:              "nebius",
		MaxContextTokens:      8192,
		TokenizerType:         TokenizerSentencePiece,
		DefaultEmbedDimension: 3584,
		Description:           "BGE Multilingual Gemma2 - BAAI multilingual embedding model with 3584 dimensions",
	},
	"BGE-ICL": {
		Provider:              "nebius",
		MaxContextTokens:      32768,
		TokenizerType:         TokenizerSentencePiece,
		DefaultEmbedDimension: 4096,
		Description:           "BGE-ICL - BAAI embedding model with 4096 dimensions and 32K context",
	},
	"e5-mistral-7b-instruct": {
		Provider:              "nebius",
		MaxContextTokens:      32768,
		TokenizerType:         TokenizerSentencePiece,
		DefaultEmbedDimension: 4096,
		Description:           "E5 Mistral 7B Instruct - intfloat embedding model with 4096 dimensions",
	},
	"Qwen3-Embedding-8B": {
		Provider:              "nebius",
		MaxContextTokens:      32768,
		TokenizerType:         TokenizerSentencePiece,
		DefaultEmbedDimension: 4096,
		Description:           "Qwen3 Embedding 8B - high-quality embedding model with 4096 dimensions",
	},

	// OpenAI embedding models
	"text-embedding-3-small": {
		Provider:              "openai",
		MaxContextTokens:      8192,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 1536,
		Description:           "OpenAI Text Embedding 3 Small - cost-effective embedding model with 1536 dimensions",
	},
	"text-embedding-3-large": {
		Provider:              "openai",
		MaxContextTokens:      8192,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 3072,
		Description:           "OpenAI Text Embedding 3 Large - high-performance embedding model with 3072 dimensions",
	},
	"text-embedding-ada-002": {
		Provider:              "openai",
		MaxContextTokens:      8192,
		TokenizerType:         TokenizerCL100K,
		DefaultEmbedDimension: 1536,
		Description:           "OpenAI Ada 002 - previous generation embedding model with 1536 dimensions",
	},
}

// GetModelInfo returns information about a specific model.
// If the model is not found, returns a default ModelInfo with unknown tokenizer.
func GetModelInfo(modelName string) ModelInfo {
	if info, exists := ModelRegistry[modelName]; exists {
		return info
	}

	// Return sensible defaults for unknown models
	return ModelInfo{
		Provider:         "unknown",
		MaxContextTokens: 8192,
		TokenizerType:    TokenizerUnknown,
		Description:      "Unknown model",
	}
}

// GetMaxContextTokens returns the maximum context tokens for a model.
// Returns a default value if the model is not found.
func GetMaxContextTokens(modelName string) int {
	return GetModelInfo(modelName).MaxContextTokens
}

// GetTokenizerType returns the tokenizer type for a model.
// Returns TokenizerUnknown if the model is not found.
func GetTokenizerType(modelName string) TokenizerType {
	return GetModelInfo(modelName).TokenizerType
}

// GetDefaultEmbedDimension returns the default embedding dimension for a model.
// Returns 0 if the model is not found or has no default dimension.
func GetDefaultEmbedDimension(modelName string) int {
	return GetModelInfo(modelName).DefaultEmbedDimension
}

// GetProvider returns the provider for a model.
// Returns "unknown" if the model is not found.
func GetProvider(modelName string) string {
	return GetModelInfo(modelName).Provider
}

// ListKnownModels returns a list of all models in the registry.
func ListKnownModels() []string {
	models := make([]string, 0, len(ModelRegistry))
	for model := range ModelRegistry {
		models = append(models, model)
	}
	return models
}
