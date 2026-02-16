# Go Coding Conventions and Standards

This document outlines Go-specific coding standards, conventions, and best practices for meowg1k.

## Go Version

**Go 1.25.5** - Use features available in this version and newer.

## File Headers

**MANDATORY**: All Go source files must include the Apache 2.0 license header:

```go
// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0
```

## Documentation and Comments

### Godoc Comment Rules

**"Chart by Exceptions" Philosophy**:
- **Public API (exported)**: MUST have full godoc comments
- **Internal implementation**: Comment by exception - only document what's non-obvious

### Public API Documentation (MANDATORY)

All exported identifiers (packages, types, functions, methods, constants, variables) MUST have godoc comments.

#### Package Comments

```go
// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package index provides core indexing operations for documents and snapshots.
// It coordinates RAG indexing workflows including chunking, embedding generation,
// and snapshot management for commit-based versioning.
package index
```

#### Type Comments

```go
// Service coordinates index persistence and snapshot reconciliation.
// It manages document versions, chunks, and their associations with Git snapshots.
type Service struct {
    indexRepo    ports.IndexRepository
    snapshotRepo ports.SnapshotRepository
}
```

#### Function/Method Comments

Start with the function name and describe what it does:

```go
// NewService creates a new index service with the given repositories.
// It returns an error if any repository is nil.
func NewService(
    indexRepo ports.IndexRepository,
    snapshotRepo ports.SnapshotRepository,
) (*Service, error) {
    // Implementation
}

// PrepareForProcessing deduplicates files and prepares them for indexing.
// It returns a PrepareOutput containing files to process and lookup maps.
func (s *Service) PrepareForProcessing(ctx context.Context, workspaceState interface{}) (interface{}, error) {
    // Implementation
}
```

#### Constant and Variable Comments

```go
// DefaultChunkSize is the default size in bytes for document chunks.
const DefaultChunkSize = 1024

// ErrInvalidHash is returned when a content hash is malformed.
var ErrInvalidHash = errors.New("invalid content hash")
```

#### Interface Comments

```go
// GenerationGateway defines the contract for LLM content generation.
// Implementations must handle streaming, caching, and error retry logic.
type GenerationGateway interface {
    // GenerateContent generates text content from the given messages.
    // It returns the generated response or an error if generation fails.
    GenerateContent(ctx context.Context, request *GenerateContentRequest) (*GenerateContentResponse, error)
}
```

### Internal Documentation (By Exception)

For **unexported** (internal) code, document only when:

1. **Non-obvious logic**:
```go
// computeChunkOverlap calculates overlap size to maintain context continuity
// across chunk boundaries. Uses 10% overlap with minimum 50 bytes.
func computeChunkOverlap(chunkSize int) int {
    overlap := chunkSize / 10
    if overlap < 50 {
        return 50
    }
    return overlap
}
```

2. **Complex algorithms**:
```go
// buildHNSWIndex constructs a hierarchical navigable small world graph
// for approximate nearest neighbor search. Uses M=16 connections per layer
// and efConstruction=200 for build quality.
func buildHNSWIndex(embeddings []Embedding) (*Index, error) {
    // Implementation
}
```

3. **Workarounds or gotchas**:
```go
// HACK: SQLite doesn't support concurrent writes, so we serialize
// all write operations through a single channel.
func (r *Repository) writeQueue() chan writeOp {
    // Implementation
}
```

4. **Why, not what** (when non-obvious):
```go
// We cache the compiled regex because it's used in hot path
// and compilation is expensive (~100µs per call).
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
```

**Do NOT over-comment obvious code**:

```go
// ❌ BAD: Obvious comment
// Set the name field to the provided name value
func (u *User) setName(name string) {
    u.name = name
}

// ✅ GOOD: No comment needed
func (u *User) setName(name string) {
    u.name = name
}

// ✅ GOOD: Comment explains non-obvious constraint
// setName truncates names longer than 255 characters to comply with
// database column constraints.
func (u *User) setName(name string) {
    if len(name) > 255 {
        name = name[:255]
    }
    u.name = name
}
```

### Comment Style

```go
// ✅ GOOD: Complete sentences with proper punctuation
// PrepareForProcessing deduplicates files and prepares them for indexing.

// ❌ BAD: Incomplete sentence, no punctuation
// prepare files

// ✅ GOOD: Multi-line comment
// BuildIndex constructs a vector index for semantic search.
// It uses HNSW algorithm with the following parameters:
//   - M: 16 connections per layer
//   - efConstruction: 200 for build quality
//   - efSearch: 50 for search performance

// ❌ BAD: Multi-line without proper formatting
// BuildIndex constructs a vector index for semantic search
// using HNSW with M=16, efConstruction=200, efSearch=50
```

### TODO Comments

```go
// TODO(username): Description of what needs to be done
// TODO(alice): Implement retry logic with exponential backoff
```

## Code Formatting

### Use gofumpt

meowg1k uses **gofumpt** (stricter than gofmt):

```bash
# Format code
task fix:fmt

# Or manually
golangci-lint run --fix --disable-all -E gofumpt -E goimports
```

### Import Grouping

Group imports in this order:
1. Standard library
2. External packages
3. Internal packages

```go
import (
    "context"
    "fmt"
    "time"
    
    "github.com/spf13/cobra"
    "go.starlark.net/starlark"
    
    "github.com/retran/meowg1k/internal/domain/config"
    "github.com/retran/meowg1k/internal/ports"
)
```

## Naming Conventions

### Packages

- Short, lowercase, single-word names
- No underscores or camelCase
- Descriptive of contents

```go
// ✅ GOOD
package index
package gateway
package chunker

// ❌ BAD
package index_service
package gatewayImpl
package my_awesome_chunker
```

### Types

- Use **PascalCase** for exported types
- Use **camelCase** for unexported types
- Avoid stuttering (don't repeat package name)

```go
// ✅ GOOD
type Service struct {}           // index.Service (not index.IndexService)
type generationRequest struct {} // Unexported

// ❌ BAD
type IndexService struct {}      // Stutters: index.IndexService
type Generation_Request struct{} // Underscores
```

### Interfaces

- Name describes capability or behavior
- Single-method interfaces often end in `-er`

```go
// ✅ GOOD
type Reader interface { Read([]byte) (int, error) }
type GenerationGateway interface { GenerateContent(...) }
type IndexRepository interface { AddDocument(...) }

// ❌ BAD
type IReader interface {}        // Don't use "I" prefix
type ReaderInterface interface{} // Don't use "Interface" suffix
```

### Functions and Methods

- Use **camelCase** for unexported
- Use **PascalCase** for exported
- Start with verb (Get, Set, New, Create, Update, Delete, etc.)

```go
// ✅ GOOD
func NewService() *Service
func (s *Service) GetModel(name string) (*Model, error)
func (s *Service) validateConfig() error  // Unexported

// ❌ BAD
func service() *Service           // Missing "New"
func (s *Service) Model(name string) // Ambiguous: getter or something else?
```

### Variables

- Short names for short-lived variables
- Descriptive names for longer-lived variables

```go
// ✅ GOOD: Short scope
for i, v := range items {
    // i and v are clear in context
}

// ✅ GOOD: Longer scope
type Service struct {
    indexRepository    ports.IndexRepository  // Descriptive
    snapshotRepository ports.SnapshotRepository
}

// ❌ BAD
for index, value := range items {  // Too verbose for short scope
}

type Service struct {
    ir ports.IndexRepository  // Too cryptic for struct field
}
```

### Constants

- Use **PascalCase** for exported
- Use **camelCase** for unexported
- Group related constants

```go
// ✅ GOOD
const (
    DefaultChunkSize = 1024
    MaxChunkSize     = 8192
    MinChunkSize     = 128
)

const (
    chunkOverlapRatio = 0.1
    minOverlap        = 50
)

// ❌ BAD
const DEFAULT_CHUNK_SIZE = 1024  // Snake case
const default_chunk_size = 1024  // Exported but lowercase
```

## Error Handling

### Always Check Errors

```go
// ✅ GOOD
file, err := os.Open("config.yaml")
if err != nil {
    return fmt.Errorf("failed to open config: %w", err)
}
defer file.Close()

// ❌ BAD
file, _ := os.Open("config.yaml")  // Ignoring error
```

### Wrap Errors with Context

Use `%w` to wrap errors:

```go
// ✅ GOOD
if err := validate(config); err != nil {
    return fmt.Errorf("config validation failed: %w", err)
}

// ❌ BAD
if err != nil {
    return err  // Lost context
}

// ❌ BAD
if err != nil {
    return fmt.Errorf("validation failed: %v", err)  // %v doesn't wrap
}
```

### Custom Error Types

```go
// Define sentinel errors
var (
    ErrNotFound     = errors.New("resource not found")
    ErrInvalidInput = errors.New("invalid input")
)

// Use errors.Is for checking
if errors.Is(err, ErrNotFound) {
    // Handle not found
}
```

### Error Messages

- Start with lowercase
- No punctuation at end
- Provide context

```go
// ✅ GOOD
return fmt.Errorf("failed to parse config at line %d: %w", line, err)

// ❌ BAD
return fmt.Errorf("Error!")  // Uppercase, no context
return fmt.Errorf("failed to parse config.")  // Punctuation
```

## Nil Checks

### Always Check Before Dereferencing

```go
// ✅ GOOD
func (s *Service) Process(input *Input) error {
    if s == nil {
        return fmt.Errorf("service is nil")
    }
    if input == nil {
        return fmt.Errorf("input cannot be nil")
    }
    // Process input
}

// ❌ BAD
func (s *Service) Process(input *Input) error {
    // Missing nil checks - potential panic
    return s.repo.Save(input.Data)
}
```

### Constructor Validation

```go
// ✅ GOOD
func NewService(repo ports.Repository) (*Service, error) {
    if repo == nil {
        return nil, fmt.Errorf("repository cannot be nil")
    }
    return &Service{repo: repo}, nil
}
```

## Interface Usage

### Accept Interfaces, Return Structs

```go
// ✅ GOOD
func NewService(repo ports.IndexRepository) *Service {
    return &Service{repo: repo}
}

// ❌ BAD
func NewService(repo *sqlite.Repository) *Service {
    // Depends on concrete type
}
```

### Define Interfaces in Consumer Package

```go
// ✅ GOOD: Define interface where it's used
// internal/core/index/service.go
type Repository interface {
    Save(doc *Document) error
}

// ❌ BAD: Define interface in implementation package
// internal/adapters/sqlite/repository.go
type Repository interface {  // Wrong place!
    Save(doc *Document) error
}
```

## Struct Initialization

### Use Named Fields

```go
// ✅ GOOD
user := User{
    Name:  "Alice",
    Email: "alice@example.com",
    Age:   30,
}

// ❌ BAD
user := User{"Alice", "alice@example.com", 30}  // Positional
```

### Zero Values

```go
// Understand zero values
var s string       // ""
var i int          // 0
var b bool         // false
var p *int         // nil
var slice []int    // nil
var m map[string]int // nil
```

## Concurrency

### Use Channels for Communication

```go
// ✅ GOOD
func worker(jobs <-chan Job, results chan<- Result) {
    for job := range jobs {
        results <- process(job)
    }
}
```

### Protect Shared State

```go
// ✅ GOOD
type SafeCounter struct {
    mu    sync.Mutex
    count int
}

func (c *SafeCounter) Inc() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}
```

### Context for Cancellation

```go
// ✅ GOOD
func (s *Service) Process(ctx context.Context, input string) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        // Continue processing
    }
    // Process input
}
```

## Code Organization

### Group Related Code

```go
// ✅ GOOD: Group constants, vars, types together
const (
    maxRetries = 3
    timeout    = 30 * time.Second
)

var (
    ErrTimeout = errors.New("operation timed out")
    ErrRetry   = errors.New("retry exhausted")
)

type Config struct {
    MaxRetries int
    Timeout    time.Duration
}
```

### Order of Declaration

Within a file, order should be:
1. Package comment
2. Imports
3. Constants
4. Variables
5. Types
6. Functions/Methods

## Linting

### Run golangci-lint

```bash
# Check for issues
task check:lint

# Auto-fix issues
task fix:lint
```

### Key Linters Enabled

See `.golangci.yaml` for complete configuration. Key linters:
- `gofumpt` - Stricter formatting
- `goimports` - Import organization
- `errcheck` - Unchecked errors
- `gosec` - Security issues
- `govet` - Suspicious constructs
- `staticcheck` - Static analysis
- `unused` - Unused code
- `ineffassign` - Ineffectual assignments

## Best Practices

### Use Table-Driven Tests
See `.opencode/testing-standards.md`

### Avoid Naked Returns

```go
// ✅ GOOD
func divide(a, b int) (int, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}

// ❌ BAD
func divide(a, b int) (result int, err error) {
    if b == 0 {
        err = errors.New("division by zero")
        return  // Naked return
    }
    result = a / b
    return  // Naked return
}
```

### Use defer for Cleanup

```go
// ✅ GOOD
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()  // Always closes, even on panic
    
    // Process file
}
```

### Avoid init() When Possible

```go
// ❌ AVOID: Global state in init
var db *sql.DB

func init() {
    var err error
    db, err = sql.Open("sqlite3", "data.db")
    if err != nil {
        panic(err)  // Can't return error from init
    }
}

// ✅ BETTER: Explicit initialization
func NewService() (*Service, error) {
    db, err := sql.Open("sqlite3", "data.db")
    if err != nil {
        return nil, err
    }
    return &Service{db: db}, nil
}
```

## Summary

- **Apache 2.0 header** required in all files
- **Public API**: Full godoc comments (MANDATORY)
- **Internal code**: Comment by exception (non-obvious logic only)
- **gofumpt formatting** via `task fix:fmt`
- **Error wrapping** with `%w`
- **Nil checks** before dereferencing
- **Interfaces** in consumer packages, not implementation packages
- **Named struct fields** for initialization
- **75% test coverage** minimum
- **golangci-lint** must pass

Follow these conventions to maintain consistency and quality across the codebase.
