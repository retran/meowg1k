# Hexagonal Architecture Deep Dive

This document provides a comprehensive overview of meowg1k's hexagonal architecture (also known as Ports & Adapters pattern).

## Architecture Overview

meowg1k strictly follows **Hexagonal Architecture** principles to achieve:
- **Testability**: Core business logic can be tested without external dependencies
- **Flexibility**: Easy to swap implementations (e.g., switch from SQLite to PostgreSQL)
- **Maintainability**: Clear separation of concerns across layers
- **Independence**: Core logic doesn't depend on frameworks or external libraries

## Layer Structure

### 1. Domain Layer (`internal/domain/`)

The **Domain Layer** contains pure business types, domain models, and value objects. These represent the core concepts of the application.

**Key Characteristics**:
- No dependencies on other internal packages
- Pure data structures and types
- Domain-specific logic encapsulated in methods
- Exported for use across all layers

**Example Packages**:
- `domain/config/` - Configuration domain types (Provider, Model, Preset)
- `domain/gateway/` - LLM gateway types (GenerateContentRequest, Embedding)
- `domain/index/` - RAG index types (DocumentVersion, Chunk, FileState)
- `domain/git/` - Git domain types
- `domain/preset/` - Preset configuration types

**Example Type**:
```go
// domain/index/types.go
type DocumentVersion struct {
    ID          int64
    FilePath    string
    ContentHash string
    IndexedAt   time.Time
}
```

### 2. Ports Layer (`internal/ports/`)

The **Ports Layer** defines **interfaces** that serve as contracts between the core and adapters. This is the **dependency inversion** layer.

**Key Characteristics**:
- All interfaces defined in `ports/types.go`
- Consumed by core services (dependencies injected)
- Implemented by adapters
- No implementation details, only contracts

**Example Interfaces**:
```go
// ports/types.go
type GenerationGateway interface {
    GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (*gateway.GenerateContentResponse, error)
}

type IndexRepository interface {
    AddDocumentVersion(ctx context.Context, doc *domainindex.DocumentVersion, content []byte) (int64, error)
    GetChunksByVersionID(ctx context.Context, versionID int64) ([]domainindex.Chunk, error)
}

type GitService interface {
    ListFiles(ref string) ([]string, error)
    ReadFileAtCommit(ref, filePath string) (string, error)
}
```

**Port Categories**:
- **Repositories**: Data persistence (IndexRepository, CacheRepository, MetaRepository)
- **Gateways**: External services (GenerationGateway, EmbeddingsGateway)
- **Services**: Domain operations (GitService, ChunkerService, WorkspaceService)

### 3. Core Layer (`internal/core/`)

The **Core Layer** contains business logic implementations and orchestrates domain operations.

**Key Characteristics**:
- Depends on ports (interfaces), not adapters
- Pure business logic with no infrastructure concerns
- Services accept dependencies through constructors (DI)
- Testable with mock implementations

**Example Packages**:
- `core/index/` - RAG indexing service (orchestrates chunking, embedding, storage)
- `core/model/` - Model management service
- `core/preset/` - Preset resolution service
- `core/provider/` - Provider management service
- `core/starlark/` - Starlark runtime and module implementations
- `core/chunker/` - Text chunking strategies
- `core/vector/` - HNSW vector index operations
- `core/shutdown/` - Graceful shutdown coordination

**Example Service**:
```go
// core/index/service.go
type Service struct {
    indexRepo    ports.IndexRepository
    snapshotRepo ports.SnapshotRepository
    chunker      ports.ChunkerService
    embeddings   ports.EmbeddingsGateway
}

func NewService(
    indexRepo ports.IndexRepository,
    snapshotRepo ports.SnapshotRepository,
    chunker ports.ChunkerService,
    embeddings ports.EmbeddingsGateway,
) *Service {
    return &Service{
        indexRepo:    indexRepo,
        snapshotRepo: snapshotRepo,
        chunker:      chunker,
        embeddings:   embeddings,
    }
}
```

### 4. Adapters Layer (`internal/adapters/`)

The **Adapters Layer** contains concrete implementations of ports, interfacing with external systems.

**Key Characteristics**:
- Implements port interfaces
- Handles infrastructure concerns (DB, HTTP, filesystem)
- Can depend on third-party libraries
- Not used directly by core (injected via ports)

**Adapter Categories**:

#### Infrastructure Adapters
- `adapters/sqlite/` - SQLite database repositories
  - `sqlite/index/` - IndexRepository implementation
  - `sqlite/cache/` - CacheRepository implementation
  - `sqlite/meta/` - MetaRepository implementation
  - `sqlite/migrations/` - Database migrations
- `adapters/git/` - Git operations via exec
- `adapters/workspace/` - Workspace path detection
- `adapters/httpclient/` - Shared HTTP client service

#### Gateway Adapters (LLM Providers)
- `adapters/gateway/anthropic.go` - Anthropic Claude adapter
- `adapters/gateway/openai.go` - OpenAI GPT adapter
- `adapters/gateway/gemini.go` - Google Gemini adapter
- `adapters/gateway/llama.go` - Ollama/Llama adapter
- `adapters/gateway/voyage.go` - Voyage embeddings adapter
- `adapters/gateway/openrouter.go` - OpenRouter adapter
- `adapters/gateway/factory.go` - Gateway factory (creates appropriate adapter)
- `adapters/gateway/caching.go` - Caching decorator for gateways
- `adapters/gateway/retry.go` - Retry logic with exponential backoff

#### Service Adapters
- `adapters/config/` - Configuration file service
- `adapters/output/` - Terminal output service
- `adapters/progress/` - Progress logging
- `adapters/tracelog/` - Trace logging for debugging
- `adapters/command/` - Command-line flag parsing

**Example Adapter**:
```go
// adapters/gateway/anthropic.go
type AnthropicGateway struct {
    client     *anthropic.Client
    httpClient ports.HTTPClientService
    logger     *slog.Logger
}

func (g *AnthropicGateway) GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (*gateway.GenerateContentResponse, error) {
    // Implementation details for Anthropic API
}
```

### 5. Application Layer (`internal/app/`)

The **Application Layer** orchestrates the entire application lifecycle and wires dependencies together.

**Key Characteristics**:
- Dependency Injection container
- Initializes all services and adapters
- Manages application lifecycle (startup/shutdown)
- Connects to Cobra commands via context

**Container Structure**:
```go
// app/container.go
type Container struct {
    Logger              *slog.Logger
    ShutdownService     *shutdown.Service
    CommandService      *command.Service
    ConfigService       *adapterConfig.Service
    OutputService       Writer
    ProgressLogger      progress.Logger
    TraceLogger         *tracelog.Logger
    // ... more services (lazy initialized)
}

func NewAppContainer(cmd *cobra.Command) (*Container, error) {
    // 1. Create logger
    // 2. Initialize config service
    // 3. Create output service
    // 4. Wire up dependencies
    // 5. Attach container to command context
}
```

**Lazy Initialization Pattern**:
Many dependencies are lazily initialized to improve startup time:
```go
func (c *Container) GetIndexRepository() (ports.IndexRepository, error) {
    c.initDBOnce() // Ensures DB initialized only once
    if c.indexRepo == nil {
        c.indexRepo = indexRepo.NewRepository(c.dbHost)
    }
    return c.indexRepo, nil
}
```

### 6. UI Layer (`internal/ui/`)

The **UI Layer** handles terminal presentation logic with Bubble Tea and Lip Gloss.

**Key Characteristics**:
- Bubble Tea interactive components
- Lip Gloss styling and theming
- Markdown rendering with Glamour
- Syntax highlighting with Chroma
- Independent of core business logic

**Components**:
- `ui/theme.go` - Color schemes and styling
- `ui/markdown.go` - Markdown rendering
- `ui/code.go` - Code syntax highlighting
- `ui/diff.go` - Diff visualization
- `ui/prompt.go` - User input prompts
- `ui/select.go` - Selection menus
- `ui/progress_bar.go` - Progress indicators
- `ui/table.go` - Tabular data display
- `ui/panel.go` - Panel layouts
- `ui/banner.go` - Banner text

### 7. CMD Layer (`cmd/`)

The **CMD Layer** defines CLI commands using Cobra and serves as the application entry point.

**Key Characteristics**:
- Cobra command definitions
- Flag parsing and validation
- Command routing
- Application initialization trigger

**Structure**:
- `cmd/meow/main.go` - Entry point
- `cmd/root.go` - Root command and lifecycle hooks
- `cmd/init.go` - Project initialization command
- `cmd/starlark.go` - Starlark command loader
- `cmd/version.go` - Version command

## Dependency Flow

```
┌─────────────────────────────────────────────────────────────┐
│                         CMD Layer                            │
│                    (Cobra Commands)                          │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                         │
│              (Container / Dependency Injection)              │
└──────────┬───────────────────────────────────┬──────────────┘
           │                                   │
           ▼                                   ▼
┌──────────────────────┐          ┌──────────────────────────┐
│     Core Layer       │          │     Adapters Layer       │
│  (Business Logic)    │◄─────────┤   (Implementations)      │
│                      │  Injected│                          │
│  Depends on Ports ───┼─────────►│  Implements Ports        │
│  (Interfaces)        │          │                          │
└──────────┬───────────┘          └──────────────────────────┘
           │
           ▼
┌──────────────────────┐
│    Ports Layer       │
│   (Interfaces)       │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│   Domain Layer       │
│  (Domain Types)      │
└──────────────────────┘
```

**Key Principle**: Dependencies point **inward**. Core never depends on adapters. Adapters depend on ports.

## Testing Strategy

### Unit Testing Core Services
Core services are tested with mock implementations of ports:

```go
// Mock implementation
type mockIndexRepo struct {
    addDocumentVersionCalled bool
}

func (m *mockIndexRepo) AddDocumentVersion(ctx context.Context, doc *domainindex.DocumentVersion, content []byte) (int64, error) {
    m.addDocumentVersionCalled = true
    return 1, nil
}

// Test
func TestIndexService(t *testing.T) {
    mockRepo := &mockIndexRepo{}
    service := NewService(mockRepo, ...)
    
    // Test core logic without real database
}
```

### Integration Testing Adapters
Adapters are tested against real dependencies (e.g., SQLite in-memory):

```go
func TestIndexRepository(t *testing.T) {
    db := setupInMemoryDB(t)
    repo := NewRepository(db)
    
    // Test against real SQLite
}
```

## Adding New Features

### Example: Adding a New LLM Provider

1. **Define domain types** (if needed) in `domain/gateway/types.go`
2. **Create adapter** in `adapters/gateway/newprovider.go` implementing `GenerationGateway`
3. **Register in factory** in `adapters/gateway/factory.go`
4. **Test adapter** in `adapters/gateway/newprovider_test.go`
5. **No changes needed to core** - dependency injection handles it

### Example: Adding a New Storage Backend

1. **Port already exists** - `ports.IndexRepository`
2. **Create new adapter** in `adapters/postgres/` implementing the port
3. **Update app container** to use new adapter
4. **Core services unchanged** - they depend on the interface

## Benefits of This Architecture

1. **Testability**: Core logic tested without infrastructure
2. **Flexibility**: Swap SQLite for PostgreSQL without touching core
3. **Maintainability**: Changes localized to specific layers
4. **Clarity**: Clear boundaries and responsibilities
5. **Scalability**: Easy to add new providers, repositories, services
6. **Independence**: Core logic independent of frameworks

## Anti-Patterns to Avoid

❌ **Don't**: Import adapters in core
```go
// BAD: core/index/service.go
import "github.com/retran/meowg1k/internal/adapters/sqlite"
```

✅ **Do**: Depend on ports
```go
// GOOD: core/index/service.go
import "github.com/retran/meowg1k/internal/ports"

type Service struct {
    repo ports.IndexRepository // Interface, not concrete type
}
```

❌ **Don't**: Put business logic in adapters

✅ **Do**: Keep adapters thin, business logic in core

❌ **Don't**: Skip dependency injection

✅ **Do**: Inject all dependencies through constructors

## Summary

meowg1k's hexagonal architecture ensures:
- Core business logic remains pure and testable
- Easy to add new LLM providers, storage backends, or features
- Clear separation between what the system does (core) and how it does it (adapters)
- Maintainable codebase that scales with complexity
