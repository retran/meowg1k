package profile

import (
	"time"

	"github.com/retran/meowg1k/internal/core/model"
	"github.com/retran/meowg1k/internal/core/provider"
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
	TokenizerType   model.TokenizerType   // Tokenizer type
	RateLimit       model.RateLimitConfig // Rate limiting config

	// Request-specific parameters (from profile config)
	Timeout     time.Duration // Request timeout
	Temperature *float64      // Temperature parameter (optional)
	TopP        *float64      // TopP parameter (optional)
	TopK        *int          // TopK parameter (optional)
}
