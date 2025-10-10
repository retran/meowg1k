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

package profile

import (
	"time"

	"github.com/retran/meowg1k/internal/domain/model"
	"github.com/retran/meowg1k/internal/domain/provider"
)

// Profile defines an enumeration for configured profile names.
type Profile string

// ResolvedProfile represents a profile with all values resolved from both model and profile config.
type ResolvedProfile struct {
	// Profile information
	Name string

	// Model instance information (from model config)
	ModelID         string                // Model instance ID
	Provider        provider.Provider     // Provider type
	Model           string                // Model name
	MaxInputTokens  int                   // Maximum input tokens
	MaxOutputTokens int                   // Maximum output tokens (can be overridden by profile)
	BaseURL         string                // API base URL
	APIKey          string                // Resolved API key (actual value)
	APIKeyEnv       string                // Environment variable name for API key
	TokenizerType   model.Tokenizer       // Tokenizer type
	RateLimit       model.RateLimitConfig // Rate limiting config

	// Request-specific parameters (from profile config)
	Timeout     time.Duration // Request timeout
	Temperature *float64      // Temperature parameter (optional)
	TopP        *float64      // TopP parameter (optional)
	TopK        *int          // TopK parameter (optional)
}
