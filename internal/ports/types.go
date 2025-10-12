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

package ports

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/profile"
)

// OutputWriter writes output to the user (used in flows).
type OutputWriter interface {
	PrintLine(line string) error
}

// ConfigResolver reads the application configuration.
type ConfigResolver interface {
	Get() (*config.Config, error)
}

// ProfileResolver resolves profile configurations.
type ProfileResolver interface {
	Get(profile profile.Profile) (*profile.ResolvedProfile, error)
}

// GenerationGateway defines the contract for a client that generates content using an LLM.
type GenerationGateway interface {
	GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (string, error)
}

// EmbeddingsGateway defines the contract for a client that computes text embeddings
// and measures the distance between them.
type EmbeddingsGateway interface {
	ComputeEmbeddings(ctx context.Context, request *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error)
	ComputeDistance(first, second gateway.Embedding) (float64, error)
}

// Gateway defines the contract for a client that supports both content generation and embeddings.
type Gateway interface {
	GenerationGateway
	EmbeddingsGateway
}

// GenerationGatewayFactory creates generation gateways for LLM providers.
type GenerationGatewayFactory interface {
	NewGenerationGateway(ctx context.Context, profile *profile.ResolvedProfile) (GenerationGateway, error)
}

// Host provides access to database connections.
type Host interface {
	GetDB() (*sql.DB, error)
	GetProjectDB() (*sql.DB, error)
	Close() error
}

// CacheRepository defines the contract for LLM response caching.
type CacheRepository interface {
	// Get retrieves a cached value by key.
	// Returns the value, whether it was found, and any error.
	Get(ctx context.Context, key string) (string, bool, error)

	// Set stores a value in the cache with the given key.
	Set(ctx context.Context, key, value string) error

	// Purge removes cache entries older than the specified TTL.
	Purge(ctx context.Context, ttl time.Duration) error
}

// FlagReader defines the contract for reading command-line flags.
type FlagReader interface {
	GetNoCacheFlag() (bool, error)
	GetUpdateCacheFlag() (bool, error)
}

// CommandNameReader defines the contract for reading the current command name.
type CommandNameReader interface {
	GetCommandName() (string, error)
}

// HTTPClientService defines the contract for providing HTTP client instances.
// This service manages a shared HTTP client that can be reused across multiple gateways,
// which is more efficient than creating new clients for each gateway instance.
type HTTPClientService interface {
	// Get returns the shared HTTP client instance.
	Get() *http.Client

	// Close cleans up any resources held by the HTTP client.
	Close() error

	// Validate checks if the service is properly initialized.
	Validate() error
}
