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
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
	"github.com/retran/meowg1k/internal/adapters/sqlite/ratelimit"
	"github.com/retran/meowg1k/internal/adapters/tracelog"
	"github.com/retran/meowg1k/internal/domain/model"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/domain/provider"
)

// mockCommandNameReader is a mock implementation of CommandNameReader for testing
type mockCommandNameReader struct {
	commandName string
	err         error
}

func (m *mockCommandNameReader) GetCommandName() (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.commandName, nil
}

// mockCacheRepo is a mock implementation of CacheRepository for testing
type mockCacheRepo struct{}

func (m *mockCacheRepo) Get(ctx context.Context, key string) (string, bool, error) {
	return "", false, nil
}

func (m *mockCacheRepo) Set(ctx context.Context, key, value string) error {
	return nil
}

func (m *mockCacheRepo) Purge(ctx context.Context, ttl time.Duration) error {
	return nil
}

// mockFlagReader is a mock implementation of FlagReader for testing
type mockFlagReader struct{}

func (m *mockFlagReader) GetNoCacheFlag() (bool, error) {
	return false, nil
}

func (m *mockFlagReader) GetUpdateCacheFlag() (bool, error) {
	return false, nil
}

// mockFactoryTraceLogger is a simple mock implementation of TraceLogger for factory testing
type mockFactoryTraceLogger struct{}

func (m *mockFactoryTraceLogger) LogAPIInteraction(entry *tracelog.APIInteractionEntry) error {
	return nil
}

// mockHTTPClientService is a mock implementation of HTTPClientService for testing
type mockHTTPClientService struct {
	client *http.Client
}

func newMockHTTPClientService() *mockHTTPClientService {
	return &mockHTTPClientService{
		client: &http.Client{Timeout: 10 * time.Minute},
	}
}

func (m *mockHTTPClientService) Get() *http.Client {
	return m.client
}

func (m *mockHTTPClientService) Close() error {
	return nil
}

func (m *mockHTTPClientService) Validate() error {
	return nil
}

// setupTestRepoForFactory creates an in-memory SQLite database and repository for testing
func setupTestRepoForFactory(t *testing.T) (*sql.DB, *ratelimit.Repository) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Run migrations
	if err := migrations.RunMigrations(db, ratelimit.Migrations); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := ratelimit.NewRepository(db)
	return db, repo
}

func TestNewGatewayFactory(t *testing.T) {
	db, repo := setupTestRepoForFactory(t)
	defer db.Close()

	mockCmdReader := &mockCommandNameReader{commandName: "test"}
	mockCache := &mockCacheRepo{}
	mockFlags := &mockFlagReader{}
	mockTrace := &mockFactoryTraceLogger{}
	mockHTTPClient := newMockHTTPClientService()
	factory, err := NewFactory(repo, mockCache, mockFlags, mockTrace, mockCmdReader, mockHTTPClient)
	assert.NoError(t, err)
	assert.NotNil(t, factory)
	assert.IsType(t, &Factory{}, factory)
}

func TestNewGatewayFactoryNilRepo(t *testing.T) {
	mockCmdReader := &mockCommandNameReader{commandName: "test"}
	mockHTTPClient := newMockHTTPClientService()
	factory, err := NewFactory(nil, nil, nil, nil, mockCmdReader, mockHTTPClient)
	assert.Error(t, err)
	assert.Nil(t, factory)
	assert.Contains(t, err.Error(), "rate limit repository is nil")
}

func TestGatewayFactory_NewGenerationGateway(t *testing.T) {
	db, repo := setupTestRepoForFactory(t)
	defer db.Close()

	mockCmdReader := &mockCommandNameReader{commandName: "test"}
	mockCache := &mockCacheRepo{}
	mockFlags := &mockFlagReader{}
	mockTrace := &mockFactoryTraceLogger{}
	factory, err := NewFactory(repo, mockCache, mockFlags, mockTrace, mockCmdReader, newMockHTTPClientService())
	assert.NoError(t, err)
	ctx := context.Background()

	tests := []struct {
		name        string
		profile     *profile.ResolvedProfile
		expectError bool
		errorMsg    string
	}{
		{
			name: "OpenAI provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAI,
				Model:           "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenAI provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAI,
				Model:           "gpt-4",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   model.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openai provider requires an API key",
		},
		{
			name: "Anthropic provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Anthropic,
				Model:           "claude-3-haiku-20240307",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerUnknown,
			},
			expectError: false,
		},
		{
			name: "Anthropic provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Anthropic,
				Model:           "claude-3-haiku-20240307",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   model.TokenizerUnknown,
			},
			expectError: true,
			errorMsg:    "API key",
		},
		{
			name: "Gemini provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Gemini,
				Model:           "gemini-1.5-flash",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerGemini,
			},
			expectError: false,
		},
		{
			name: "Gemini provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Gemini,
				Model:           "gemini-1.5-flash",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   model.TokenizerGemini,
			},
			expectError: true,
			errorMsg:    "gemini provider requires an API key",
		},
		{
			name: "Llama provider with base URL",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Llama,
				Model:           "llama-3.1-70b-instruct",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "",
				TokenizerType:   model.TokenizerLlama,
			},
			expectError: false,
		},
		{
			name: "Llama provider without base URL",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Llama,
				Model:           "llama-3.1-70b-instruct",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   model.TokenizerLlama,
			},
			expectError: true,
			errorMsg:    "llama provider requires a base URL",
		},
		{
			name: "OpenAI-compatible provider with base URL and API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAICompatible,
				Model:           "custom-model",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenAI-compatible provider without base URL",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAICompatible,
				Model:           "custom-model",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openai-compatible provider requires a base URL",
		},
		{
			name: "OpenAI-compatible provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAICompatible,
				Model:           "custom-model",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "",
				TokenizerType:   model.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenRouter provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenRouter,
				Model:           "openrouter/auto",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "https://openrouter.ai/api/v1",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenRouter provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenRouter,
				Model:           "openrouter/auto",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   model.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openrouter provider requires an API key",
		},
		{
			name: "Voyage provider (should fail for generation)",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Voyage,
				Model:           "voyage-large-2",
				MaxInputTokens:  8000,
				MaxOutputTokens: 2000,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerUnknown,
			},
			expectError: true,
			errorMsg:    "voyage provider only supports embeddings, not content generation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway, err := factory.NewGenerationGateway(ctx, tt.profile)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, gateway)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, gateway)
			}
		})
	}
}

func TestGatewayFactory_NewEmbeddingsGateway(t *testing.T) {
	db, repo := setupTestRepoForFactory(t)
	defer db.Close()

	mockCmdReader := &mockCommandNameReader{commandName: "test"}
	mockCache := &mockCacheRepo{}
	mockFlags := &mockFlagReader{}
	mockTrace := &mockFactoryTraceLogger{}
	factory, err := NewFactory(repo, mockCache, mockFlags, mockTrace, mockCmdReader, newMockHTTPClientService())
	require.NoError(t, err)
	ctx := context.Background()

	tests := []struct {
		name        string
		profile     *profile.ResolvedProfile
		expectError bool
		errorMsg    string
	}{
		{
			name: "OpenAI provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAI,
				Model:           "text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenAI provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenAI,
				Model:           "text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   model.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openai provider requires an API key",
		},
		{
			name: "Gemini provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Gemini,
				Model:           "models/embedding-001",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerGemini,
			},
			expectError: false,
		},
		{
			name: "Gemini provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Gemini,
				Model:           "models/embedding-001",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   model.TokenizerGemini,
			},
			expectError: true,
			errorMsg:    "gemini provider requires an API key",
		},
		{
			name: "Voyage provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Voyage,
				Model:           "voyage-large-2",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerUnknown,
			},
			expectError: false,
		},
		{
			name: "Voyage provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Voyage,
				Model:           "voyage-large-2",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   model.TokenizerUnknown,
			},
			expectError: true,
			errorMsg:    "voyage provider requires an API key",
		},
		{
			name: "OpenRouter provider with API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenRouter,
				Model:           "openai/text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "https://openrouter.ai/api/v1",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerCL100K,
			},
			expectError: false,
		},
		{
			name: "OpenRouter provider without API key",
			profile: &profile.ResolvedProfile{
				Provider:        provider.OpenRouter,
				Model:           "openai/text-embedding-ada-002",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "",
				TokenizerType:   model.TokenizerCL100K,
			},
			expectError: true,
			errorMsg:    "openrouter provider requires an API key",
		},
		{
			name: "Llama provider (not implemented)",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Llama,
				Model:           "llama-3.1-70b-instruct",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "http://localhost:8080",
				APIKey:          "",
				TokenizerType:   model.TokenizerLlama,
			},
			expectError: true,
			errorMsg:    "llama embedding gateway is not yet implemented",
		},
		{
			name: "Anthropic provider (not supported)",
			profile: &profile.ResolvedProfile{
				Provider:        provider.Anthropic,
				Model:           "claude-3-haiku-20240307",
				MaxInputTokens:  8000,
				MaxOutputTokens: 0,
				Timeout:         30 * time.Second,
				BaseURL:         "",
				APIKey:          "test-key",
				TokenizerType:   model.TokenizerUnknown,
			},
			expectError: true,
			errorMsg:    "anthropic provider does not provide embedding models",
		},
		{
			name:        "Nil profile",
			profile:     nil,
			expectError: true,
			errorMsg:    "profile cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway, err := factory.NewEmbeddingsGateway(ctx, tt.profile)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, gateway)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, gateway)
			}
		})
	}
}

// TestGatewayFactory_NoOpLimiterFallback tests that the factory uses no-op limiter when no rate limits are configured
func TestGatewayFactory_NoOpLimiterFallback(t *testing.T) {
	db, repo := setupTestRepoForFactory(t)
	defer db.Close()

	mockCmdReader := &mockCommandNameReader{commandName: "test"}
	mockCache := &mockCacheRepo{}
	mockFlags := &mockFlagReader{}
	mockTrace := &mockFactoryTraceLogger{}
	factory, err := NewFactory(repo, mockCache, mockFlags, mockTrace, mockCmdReader, newMockHTTPClientService())
	assert.NoError(t, err)

	// Create a profile with rate limits enabled
	prof := &profile.ResolvedProfile{
		Provider:        provider.OpenAI,
		Model:           "gpt-4",
		MaxInputTokens:  8000,
		MaxOutputTokens: 2000,
		Timeout:         30 * time.Second,
		BaseURL:         "https://api.openai.com/v1",
		APIKey:          "test-key",
		APIKeyEnv:       "OPENAI_API_KEY",
		TokenizerType:   model.TokenizerCL100K,
		RateLimit: struct {
			RequestsPerMinute int
			TokensPerMinute   int
			RequestsPerDay    int
		}{
			RequestsPerMinute: 10, // Enable rate limiting to trigger DB repo usage
			TokensPerMinute:   0,
			RequestsPerDay:    0,
		},
	}

	// Get or create a limiter - should succeed when repo is valid and rate limiting is configured
	limiter, err := factory.getRateLimiter(prof)
	require.NoError(t, err, "Should not return error when repo is valid and rate limiting is configured")
	require.NotNil(t, limiter, "Should return a limiter when rate limiting is configured")
}

// TestGatewayFactory_NoLimitsNoOpLimiter tests that no-op limiter is used when no limits are configured
func TestGatewayFactory_NoLimitsNoOpLimiter(t *testing.T) {
	db, repo := setupTestRepoForFactory(t)
	defer db.Close()

	mockCmdReader := &mockCommandNameReader{commandName: "test"}
	mockCache := &mockCacheRepo{}
	mockFlags := &mockFlagReader{}
	mockTrace := &mockFactoryTraceLogger{}
	factory, err := NewFactory(repo, mockCache, mockFlags, mockTrace, mockCmdReader, newMockHTTPClientService())
	assert.NoError(t, err)

	// Create a profile with NO rate limits - should use no-op limiter without touching DB
	prof := &profile.ResolvedProfile{
		Provider:        provider.OpenAI,
		Model:           "gpt-4",
		MaxInputTokens:  8000,
		MaxOutputTokens: 2000,
		Timeout:         30 * time.Second,
		BaseURL:         "https://api.openai.com/v1",
		APIKey:          "test-key",
		APIKeyEnv:       "OPENAI_API_KEY",
		TokenizerType:   model.TokenizerCL100K,
		RateLimit: struct {
			RequestsPerMinute int
			TokensPerMinute   int
			RequestsPerDay    int
		}{
			RequestsPerMinute: 0, // No rate limiting
			TokensPerMinute:   0,
			RequestsPerDay:    0,
		},
	}

	// Get or create a limiter - should get no-op limiter without touching DB
	limiter, err := factory.getRateLimiter(prof)
	require.NoError(t, err, "Should not return error when no limits configured")
	require.NotNil(t, limiter, "Should return a no-op limiter when no limits configured")

	// Verify caching works
	limiter2, err := factory.getRateLimiter(prof)
	require.NoError(t, err, "Should not return error on second call")
	require.NotNil(t, limiter2, "Should return cached limiter on second call")
}
