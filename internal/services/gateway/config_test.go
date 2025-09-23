package gateway

import (
	"testing"

	gatewaymodels "github.com/retran/meowg1k/internal/models/gateway"
	"github.com/stretchr/testify/assert"
)

func TestWithProvider(t *testing.T) {
	tests := []struct {
		name        string
		provider    gatewaymodels.Provider
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid OpenAI provider",
			provider:    gatewaymodels.OpenAI,
			expectError: false,
		},
		{
			name:        "Valid Anthropic provider",
			provider:    gatewaymodels.Anthropic,
			expectError: false,
		},
		{
			name:        "Valid Gemini provider",
			provider:    gatewaymodels.Gemini,
			expectError: false,
		},
		{
			name:        "Valid Llama provider",
			provider:    gatewaymodels.Llama,
			expectError: false,
		},
		{
			name:        "Valid OpenRouter provider",
			provider:    gatewaymodels.OpenRouter,
			expectError: false,
		},
		{
			name:        "Valid OpenAI-compatible provider",
			provider:    gatewaymodels.OpenAICompatible,
			expectError: false,
		},
		{
			name:        "Valid Nebius provider",
			provider:    gatewaymodels.Nebius,
			expectError: false,
		},
		{
			name:        "Valid Voyage provider",
			provider:    gatewaymodels.Voyage,
			expectError: false,
		},
		{
			name:        "Invalid provider",
			provider:    gatewaymodels.Provider("invalid"),
			expectError: true,
			errorMsg:    "unsupported provider: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &gatewaymodels.Config{}
			option := WithProvider(tt.provider)
			err := option(config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.provider, config.Provider)
			}
		})
	}
}

func TestWithBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{
			name:    "Valid HTTP URL",
			baseURL: "http://localhost:8080",
		},
		{
			name:    "Valid HTTPS URL",
			baseURL: "https://api.openai.com/v1",
		},
		{
			name:    "Empty URL",
			baseURL: "",
		},
		{
			name:    "URL with path",
			baseURL: "http://localhost:8080/api/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &gatewaymodels.Config{}
			option := WithBaseURL(tt.baseURL)
			err := option(config)

			assert.NoError(t, err)
			assert.Equal(t, tt.baseURL, config.BaseURL)
		})
	}
}

func TestWithAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
	}{
		{
			name:   "Valid API key",
			apiKey: "sk-test1234567890",
		},
		{
			name:   "Empty API key",
			apiKey: "",
		},
		{
			name:   "Long API key",
			apiKey: "sk-" + string(make([]byte, 100)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &gatewaymodels.Config{}
			option := WithAPIKey(tt.apiKey)
			err := option(config)

			assert.NoError(t, err)
			assert.Equal(t, tt.apiKey, config.APIKey)
		})
	}
}

func TestConfigCombined(t *testing.T) {
	config := &gatewaymodels.Config{}

	// Apply multiple options
	options := []Option{
		WithProvider(gatewaymodels.OpenAI),
		WithBaseURL("https://api.openai.com/v1"),
		WithAPIKey("sk-test123"),
	}

	for _, option := range options {
		err := option(config)
		assert.NoError(t, err)
	}

	assert.Equal(t, gatewaymodels.OpenAI, config.Provider)
	assert.Equal(t, "https://api.openai.com/v1", config.BaseURL)
	assert.Equal(t, "sk-test123", config.APIKey)
}
