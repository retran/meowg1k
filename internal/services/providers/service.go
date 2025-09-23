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

package providers

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/retran/meowg1k/internal/models/config"
	gatewaymodels "github.com/retran/meowg1k/internal/models/gateway"
	"github.com/retran/meowg1k/internal/services/llm/registry"
)

// Service provides provider registry capabilities.
type Service interface {
	// RegisterProvider registers a new provider definition.
	RegisterProvider(name string, definition config.ProviderDefinition) error

	// GetProvider retrieves a provider definition by name.
	GetProvider(name string) (config.ProviderDefinition, error)

	// ListProviders returns all registered provider names.
	ListProviders() []string

	// HasProvider checks if a provider is registered.
	HasProvider(name string) bool

	// GetDefaultProfile returns default profile settings for a provider.
	GetDefaultProfile(providerType gatewaymodels.Provider) config.Profile
}

// serviceImpl is the concrete implementation of the registry service.
type serviceImpl struct {
	mu        sync.RWMutex
	providers map[string]config.ProviderDefinition
}

// Compile-time interface satisfaction check
var _ Service = (*serviceImpl)(nil)

// NewService creates a new provider registry service with default providers.
func NewService() Service {
	s := &serviceImpl{
		providers: make(map[string]config.ProviderDefinition),
	}

	// Register default providers
	s.registerDefaultProviders()

	return s
}

// RegisterProvider registers a new provider definition.
func (s *serviceImpl) RegisterProvider(name string, definition config.ProviderDefinition) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.providers[name] = definition
	return nil
}

// GetProvider retrieves a provider definition by name.
func (s *serviceImpl) GetProvider(name string) (config.ProviderDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provider, exists := s.providers[name]
	if !exists {
		return config.ProviderDefinition{}, fmt.Errorf("provider '%s' not found", name)
	}

	return provider, nil
}

// ListProviders returns all registered provider names.
func (s *serviceImpl) ListProviders() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.providers))
	for name := range s.providers {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// HasProvider checks if a provider is registered.
func (s *serviceImpl) HasProvider(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.providers[name]
	return exists
}

// GetDefaultProfile returns default profile settings for a provider.
func (s *serviceImpl) GetDefaultProfile(providerType gatewaymodels.Provider) config.Profile {
	// Find the provider definition by type
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, def := range s.providers {
		if def.Type == providerType {
			return config.Profile{
				Provider:        def.Name,
				Model:           def.DefaultModel,
				MaxInputTokens:  def.MaxInputTokens,
				MaxOutputTokens: def.MaxOutputTokens,
				Timeout:         def.DefaultTimeout,
				TokenizerType:   def.TokenizerType,
			}
		}
	}

	// Fallback defaults
	return config.Profile{
		MaxInputTokens:  128000,
		MaxOutputTokens: 4096,
		Timeout:         5 * time.Minute,
		TokenizerType:   registry.TokenizerCL100K,
	}
}

// registerDefaultProviders registers all the default providers.
func (s *serviceImpl) registerDefaultProviders() {
	defaultProviders := map[string]config.ProviderDefinition{
		"gemini": {
			Type:            gatewaymodels.Gemini,
			Name:            "Google Gemini",
			DefaultModel:    "gemini-2.5-flash",
			DefaultEnvVar:   "MEOW_GEMINI_API_KEY",
			RequiresAPIKey:  true,
			RequiresBaseURL: false,
			TokenizerType:   registry.TokenizerGemini,
			MaxInputTokens:  1000000,
			MaxOutputTokens: 8192,
			DefaultTimeout:  5 * time.Minute,
		},
		"openai": {
			Type:            gatewaymodels.OpenAI,
			Name:            "OpenAI",
			DefaultModel:    "gpt-4o-mini",
			DefaultBaseURL:  "https://api.openai.com/v1",
			DefaultEnvVar:   "MEOW_OPENAI_API_KEY",
			RequiresAPIKey:  true,
			RequiresBaseURL: false,
			TokenizerType:   registry.TokenizerCL100K,
			MaxInputTokens:  128000,
			MaxOutputTokens: 16384,
			DefaultTimeout:  5 * time.Minute,
		},
		"anthropic": {
			Type:            gatewaymodels.Anthropic,
			Name:            "Anthropic Claude",
			DefaultModel:    "claude-3-5-haiku-20241022",
			DefaultEnvVar:   "MEOW_ANTHROPIC_API_KEY",
			RequiresAPIKey:  true,
			RequiresBaseURL: false,
			TokenizerType:   registry.TokenizerCL100K,
			MaxInputTokens:  200000,
			MaxOutputTokens: 8192,
			DefaultTimeout:  5 * time.Minute,
		},
		"llama": {
			Type:            gatewaymodels.Llama,
			Name:            "Meta Llama",
			DefaultModel:    "llama3.2:3b",
			DefaultEnvVar:   "", // Llama typically doesn't use API keys
			RequiresAPIKey:  false,
			RequiresBaseURL: true,
			TokenizerType:   registry.TokenizerLlama,
			MaxInputTokens:  128000,
			MaxOutputTokens: 4096,
			DefaultTimeout:  10 * time.Minute,
		},
		"nebius": {
			Type:            gatewaymodels.Nebius,
			Name:            "Nebius AI",
			DefaultModel:    "Qwen/Qwen3-Coder-30B-A3B-Instruct",
			DefaultBaseURL:  "https://api.studio.nebius.com/v1",
			DefaultEnvVar:   "MEOW_NEBIUS_API_KEY",
			RequiresAPIKey:  true,
			RequiresBaseURL: false,
			TokenizerType:   registry.TokenizerCL100K,
			MaxInputTokens:  32768,
			MaxOutputTokens: 8192,
			DefaultTimeout:  5 * time.Minute,
		},
		"openrouter": {
			Type:            gatewaymodels.OpenRouter,
			Name:            "OpenRouter",
			DefaultModel:    "anthropic/claude-3.5-haiku",
			DefaultBaseURL:  "https://openrouter.ai/api/v1",
			DefaultEnvVar:   "MEOW_OPENROUTER_API_KEY",
			RequiresAPIKey:  true,
			RequiresBaseURL: false,
			TokenizerType:   registry.TokenizerCL100K,
			MaxInputTokens:  200000,
			MaxOutputTokens: 8192,
			DefaultTimeout:  5 * time.Minute,
		},
		"voyage": {
			Type:            gatewaymodels.Voyage,
			Name:            "Voyage AI",
			DefaultModel:    "voyage-3",
			DefaultBaseURL:  "https://api.voyageai.com/v1",
			DefaultEnvVar:   "MEOW_VOYAGE_API_KEY",
			RequiresAPIKey:  true,
			RequiresBaseURL: false,
			TokenizerType:   registry.TokenizerCL100K,
			MaxInputTokens:  32000,
			MaxOutputTokens: 0, // Embeddings don't have output tokens
			DefaultTimeout:  5 * time.Minute,
		},
		"openai-compatible": {
			Type:            gatewaymodels.OpenAICompatible,
			Name:            "OpenAI Compatible",
			DefaultModel:    "",    // Must be specified by user
			DefaultEnvVar:   "",    // Depends on the service
			RequiresAPIKey:  false, // Depends on the service
			RequiresBaseURL: true,
			TokenizerType:   registry.TokenizerCL100K,
			MaxInputTokens:  128000,
			MaxOutputTokens: 4096,
			DefaultTimeout:  5 * time.Minute,
		},
	}

	for name, def := range defaultProviders {
		s.providers[name] = def
	}
}
