// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package ports defines port interfaces for hexagonal architecture, decoupling core business logic from adapters.
package ports

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/domain/session"
)

// OutputWriter writes output to the user (used in flows).
type OutputWriter interface {
	PrintLine(line string) error
}

// ConfigResolver reads the application configuration.
type ConfigResolver interface {
	Get() (*config.Config, error)
}

// PresetResolver resolves preset configurations.
type PresetResolver interface {
	Get(preset preset.Preset) (*preset.ResolvedPreset, error)
}

// GenerationGateway defines the contract for a client that generates content using an LLM.
type GenerationGateway interface {
	GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (*gateway.GenerateContentResponse, error)
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
	NewGenerationGateway(ctx context.Context, preset *preset.ResolvedPreset) (GenerationGateway, error)
}

// Host provides access to database connections.
type Host interface {
	GetMainDB() (*sql.DB, error)
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

	// GetWithTimeout returns a new HTTP client with custom timeout settings.
	// This is useful for operations that need different timeout characteristics.
	GetWithTimeout(timeout time.Duration) *http.Client

	// Close cleans up any resources held by the HTTP client.
	Close() error

	// Validate checks if the service is properly initialized.
	Validate() error
}

// MetaRepository defines the interface for metadata key-value storage.
type MetaRepository interface {
	// GetValue retrieves a metadata value by key.
	// Returns nil if the key does not exist.
	GetValue(ctx context.Context, key string) ([]byte, error)

	// SetValue stores a metadata value with the given key.
	// If the key already exists, the value is updated.
	SetValue(ctx context.Context, key string, value []byte) error

	// DeleteValue deletes a metadata value by key.
	// Does not return an error if the key does not exist.
	DeleteValue(ctx context.Context, key string) error
}

// IndexRepository defines the interface for document indexing operations.
// It manages document versions, content blobs, and chunks with embeddings.
type IndexRepository interface {
	// AddDocumentVersion adds a new document version with its content to the index.
	// Returns the ID of the newly created document version.
	AddDocumentVersion(ctx context.Context, doc *domainindex.DocumentVersion, content []byte) (int64, error)

	// AddDocumentVersionWithChunks adds a document version with its chunks in a single transaction.
	// This ensures atomicity and better performance compared to separate calls.
	// Returns the ID of the newly created document version.
	AddDocumentVersionWithChunks(ctx context.Context, doc *domainindex.DocumentVersion, content []byte, chunks []domainindex.Chunk) (int64, error)

	// AddChunks adds multiple chunks to the index in a single transaction.
	AddChunks(ctx context.Context, chunks []domainindex.Chunk) error

	// FindVersionByContentHash finds a document version by content hash and file path.
	// Returns nil if no matching version is found.
	FindVersionByContentHash(ctx context.Context, filePath, contentHash string) (*domainindex.DocumentVersion, error)

	// FindVersionsByContentHashes finds document versions for multiple content hashes.
	// Returns a map of contentHash to document version.
	// Only returns entries for versions that exist in the database.
	FindVersionsByContentHashes(ctx context.Context, contentHashes []string) (map[string]*domainindex.DocumentVersion, error)

	// FindContentBlob checks if a content blob exists by its hash.
	// Returns true if the blob exists, false otherwise.
	FindContentBlob(ctx context.Context, contentHash string) (bool, error)

	// GetContentBlob retrieves the content of a blob by its hash.
	// Returns nil if the blob does not exist.
	GetContentBlob(ctx context.Context, contentHash string) ([]byte, error)

	// FindVersionsByFilePath finds all versions of a document by file path.
	FindVersionsByFilePath(ctx context.Context, filePath string) ([]domainindex.DocumentVersion, error)

	// GetChunksByVersionID retrieves all chunks for a given document version.
	GetChunksByVersionID(ctx context.Context, versionID int64) ([]domainindex.Chunk, error)

	// GetChunksByIDs retrieves chunks by their IDs.
	// This is useful for efficiently fetching multiple chunks for RAG context assembly.
	GetChunksByIDs(ctx context.Context, chunkIDs []int64) ([]domainindex.Chunk, error)

	// GetAllEmbeddings retrieves all embeddings from the index.
	// Returns a map of chunk ID to embedding vector.
	GetAllEmbeddings(ctx context.Context) (map[int64]gateway.Embedding, error)

	// GetVersionsByIDs retrieves document versions by their IDs.
	GetVersionsByIDs(ctx context.Context, versionIDs []int64) ([]domainindex.DocumentVersion, error)

	// Checkpoint performs a WAL checkpoint to ensure all pending writes are visible to readers.
	Checkpoint(ctx context.Context) error
}

// SnapshotRepository defines the interface for managing commit snapshots.
// A snapshot represents the state of all document versions at a specific commit.
type SnapshotRepository interface {
	// LinkVersionToSnapshot links a document version to a commit snapshot.
	LinkVersionToSnapshot(ctx context.Context, commitHash string, versionID int64) error

	// UnlinkVersionFromSnapshot removes a link between a document version and a snapshot.
	UnlinkVersionFromSnapshot(ctx context.Context, commitHash string, versionID int64) error

	// GetVersionIDsForSnapshot retrieves all document version IDs for a given snapshot.
	GetVersionIDsForSnapshot(ctx context.Context, commitHash string) ([]int64, error)

	// ClearSnapshotLinks removes all links for a given snapshot.
	ClearSnapshotLinks(ctx context.Context, commitHash string) error
}

// GitService defines the interface for Git operations.
type GitService interface {
	// ListFiles returns a list of all files in the specified commit/ref.
	ListFiles(ref string) ([]string, error)

	// ReadFileAtCommit reads the content of a file at a specific commit/ref.
	ReadFileAtCommit(ref, filePath string) (string, error)

	// ReadStagedFiles returns a list of files that are currently staged.
	ReadStagedFiles() ([]string, error)

	// ReadStagedFileContent reads the content of a staged file from Git index.
	ReadStagedFileContent(filePath string) (string, error)
}

// GitToolingService defines the interface for extended git operations used by agent tools.
type GitToolingService interface {
	Status() (string, error)
	Diff(ref, path string) (string, error)
	Show(ref string) (string, error)
	Log(limit int, path string) (string, error)
	Branches() ([]string, error)
	CurrentBranch() (string, error)
	Stage(paths []string) (string, error)
	Commit(message string) (string, error)
	HeadHash() (string, error)
}

// ChunkerService defines the interface for text chunking.
type ChunkerService interface {
	Chunk(content []byte, filePath string) ([]domainindex.ChunkData, error)
}

// WorkspaceService defines the interface for workspace operations.
type WorkspaceService interface {
	// Get returns the workspace root directory.
	Get() (string, error)
}

// ProjectStateService defines the interface for getting project file states.
type ProjectStateService interface {
	GetHeadState(ctx context.Context) (map[string]domainindex.FileState, error)
	GetStagingState(ctx context.Context) (map[string]domainindex.FileState, error)
	GetWorkdirState(ctx context.Context) (map[string]domainindex.FileState, error)
}

// VectorIndexService defines the interface for vector index operations.
type VectorIndexService interface {
	// BuildAndSave builds a vector index for the given snapshot and saves it.
	BuildAndSave(snapshotName string) error
}

// IndexService defines the interface for index management operations.
type IndexService interface {
	// PrepareForProcessing deduplicates files and prepares them for processing.
	PrepareForProcessing(ctx context.Context, workspaceState interface{}) (interface{}, error)
	// SaveNewVersion saves a new version of a file with its chunks and embeddings.
	SaveNewVersion(ctx context.Context, input interface{}) (interface{}, error)
	// FinalizeLiveSnapshots finalizes the live snapshots with version mappings.
	FinalizeLiveSnapshots(ctx context.Context, input interface{}) error
}

// SessionRepository defines the interface for session persistence operations.
type SessionRepository interface {
	// CreateSession inserts a new session.
	CreateSession(ctx context.Context, s *session.Session) error
	// GetSession retrieves a session by ID.
	GetSession(ctx context.Context, id string) (*session.Session, error)
	// UpdateSession updates an existing session's status and timestamp.
	UpdateSession(ctx context.Context, s *session.Session) error
	// ListSessions retrieves sessions with optional filters.
	ListSessions(ctx context.Context, filter *session.SessionFilter) ([]*session.Session, error)
	// AddEvent inserts a new event into a session.
	AddEvent(ctx context.Context, e *session.Event) error
	// GetEvents retrieves events for a session with pagination.
	GetEvents(ctx context.Context, sessionID string, limit, offset int) ([]*session.Event, error)
	// MarkEventsObsolete marks events as obsolete (soft delete for compaction).
	MarkEventsObsolete(ctx context.Context, eventIDs []string) error
	// InsertSummary inserts a system summary event after a specific event.
	InsertSummary(ctx context.Context, sessionID, afterEventID, summaryContent string) error
	// SetMetadata sets a metadata key-value pair for a session.
	SetMetadata(ctx context.Context, sessionID, key, value string) error
	// GetMetadata retrieves a metadata value for a session.
	GetMetadata(ctx context.Context, sessionID, key string) (string, error)
	// GetAllMetadata retrieves all metadata for a session.
	GetAllMetadata(ctx context.Context, sessionID string) (map[string]string, error)
}

// SessionService defines the interface for session management operations.
type SessionService interface {
	// CreateSession creates a new session with auto-generated UUID.
	CreateSession(ctx context.Context, parentID *string, toolName string) (*session.Session, error)
	// GetSession retrieves a session by ID.
	GetSession(ctx context.Context, id string) (*session.Session, error)
	// ListSessions retrieves sessions with optional filters.
	ListSessions(ctx context.Context, filter *session.SessionFilter) ([]*session.Session, error)
	// GetChildSessions retrieves all child sessions for a parent session.
	GetChildSessions(ctx context.Context, parentID string) ([]*session.Session, error)
	// CompleteSession marks a session as completed.
	CompleteSession(ctx context.Context, id string) error
	// FailSession marks a session as failed.
	FailSession(ctx context.Context, id string) error
	// AddUserMessage adds a user message event to the session.
	AddUserMessage(ctx context.Context, sessionID, content string) error
	// AddAssistantMessage adds an assistant message event to the session.
	AddAssistantMessage(ctx context.Context, sessionID, content string, toolCalls []session.ToolCall) error
	// AddToolResult adds a tool result event to the session.
	AddToolResult(ctx context.Context, sessionID, toolCallID, content string) error
	// AddSystemMessage adds a system message event to the session.
	AddSystemMessage(ctx context.Context, sessionID, content string) error
	// GetEvents retrieves events for a session with pagination.
	GetEvents(ctx context.Context, sessionID string, limit, offset int) ([]*session.Event, error)
	// GetAllEvents retrieves all non-obsolete events for a session.
	GetAllEvents(ctx context.Context, sessionID string) ([]*session.Event, error)
	// MarkEventsObsolete marks events as obsolete for compaction.
	MarkEventsObsolete(ctx context.Context, eventIDs []string) error
	// InsertSummary inserts a summary event after a specific event.
	InsertSummary(ctx context.Context, sessionID, afterEventID, summaryContent string) error
	// SetMetadata sets session metadata.
	SetMetadata(ctx context.Context, sessionID, key, value string) error
	// GetMetadata retrieves session metadata.
	GetMetadata(ctx context.Context, sessionID, key string) (string, error)
	// GetAllMetadata retrieves all session metadata.
	GetAllMetadata(ctx context.Context, sessionID string) (map[string]string, error)
	// GetChildMetadata retrieves metadata from a child session.
	GetChildMetadata(ctx context.Context, sessionID, childID, key string) (string, error)
}
