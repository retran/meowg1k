# Testing Standards and Best Practices

This document outlines testing patterns, requirements, and best practices for meowg1k.

## Testing Requirements

### Coverage Threshold

**CRITICAL**: All code must maintain **75% test coverage** minimum.

- Measured via `task check:test`
- Enforced in CI/CD pipeline
- Coverage report: `coverage.out`
- View HTML report: `go tool cover -html=coverage.out`

### Test Execution

```bash
# Run all tests with coverage
task check:test

# Run specific package tests
go test -v ./internal/core/index/...

# Run with race detector
go test -race ./...

# Run specific test
go test -v -run TestIndexService_AddDocument ./internal/core/index/
```

## Testing Framework

meowg1k uses **testify** for assertions and test utilities.

### Required Imports

```go
import (
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

**Assertion Styles**:
- `require.*` - Fail immediately if assertion fails (use for critical checks)
- `assert.*` - Continue test execution after failure (use for non-critical checks)

## Testing Patterns

### 1. Table-Driven Tests

**Preferred pattern** for testing multiple scenarios:

```go
func TestChunker_Chunk(t *testing.T) {
    tests := []struct {
        name           string
        input          string
        expectedChunks int
        expectedError  bool
    }{
        {
            name:           "empty input",
            input:          "",
            expectedChunks: 0,
            expectedError:  false,
        },
        {
            name:           "single paragraph",
            input:          "This is a single paragraph.",
            expectedChunks: 1,
            expectedError:  false,
        },
        {
            name:           "multiple paragraphs",
            input:          "First paragraph.\n\nSecond paragraph.",
            expectedChunks: 2,
            expectedError:  false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            chunker := NewChunker()
            
            chunks, err := chunker.Chunk([]byte(tt.input), "test.txt")
            
            if tt.expectedError {
                require.Error(t, err)
                return
            }
            
            require.NoError(t, err)
            assert.Equal(t, tt.expectedChunks, len(chunks))
        })
    }
}
```

### 2. Mocking Dependencies

Use **interface-based mocking** for hexagonal architecture:

```go
// Define mock in test file
type mockIndexRepo struct {
    addDocumentVersionFunc func(ctx context.Context, doc *domainindex.DocumentVersion, content []byte) (int64, error)
}

func (m *mockIndexRepo) AddDocumentVersion(ctx context.Context, doc *domainindex.DocumentVersion, content []byte) (int64, error) {
    if m.addDocumentVersionFunc != nil {
        return m.addDocumentVersionFunc(ctx, doc, content)
    }
    return 0, nil
}

// Use in test
func TestIndexService_AddDocument(t *testing.T) {
    mockRepo := &mockIndexRepo{
        addDocumentVersionFunc: func(ctx context.Context, doc *domainindex.DocumentVersion, content []byte) (int64, error) {
            return 123, nil
        },
    }
    
    service := NewService(mockRepo, nil, nil, nil)
    
    id, err := service.AddDocument(context.Background(), "test.go", []byte("package main"))
    
    require.NoError(t, err)
    assert.Equal(t, int64(123), id)
}
```

### 3. Testing Core Services (Unit Tests)

Core services should be tested **without real adapters**:

```go
func TestModelService_GetModel(t *testing.T) {
    // Arrange
    mockConfig := &mockConfigResolver{
        config: &config.Config{
            Models: map[string]config.ModelConfig{
                "test-model": {
                    Name:            "test-model",
                    Provider:        "test-provider",
                    Model:           "test-model-v1",
                    MaxInputTokens:  1000,
                    MaxOutputTokens: 500,
                },
            },
        },
    }
    
    service := model.NewService(mockConfig)
    
    // Act
    model, err := service.GetModel("test-model")
    
    // Assert
    require.NoError(t, err)
    assert.Equal(t, "test-model", model.Name)
    assert.Equal(t, "test-provider", model.Provider)
}
```

### 4. Testing Adapters (Integration Tests)

Adapters should be tested against **real dependencies** (in-memory where possible):

```go
func TestIndexRepository_AddDocumentVersion(t *testing.T) {
    // Setup in-memory SQLite
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    defer db.Close()
    
    // Run migrations
    err = migrations.Apply(db)
    require.NoError(t, err)
    
    // Create repository with real DB
    repo := indexRepo.NewRepository(&sqliteHost{mainDB: db})
    
    // Test against real database
    doc := &domainindex.DocumentVersion{
        FilePath:    "test.go",
        ContentHash: "abc123",
        IndexedAt:   time.Now(),
    }
    
    id, err := repo.AddDocumentVersion(context.Background(), doc, []byte("package main"))
    
    require.NoError(t, err)
    assert.Greater(t, id, int64(0))
    
    // Verify persistence
    retrieved, err := repo.FindVersionByContentHash(context.Background(), "test.go", "abc123")
    require.NoError(t, err)
    assert.Equal(t, doc.FilePath, retrieved.FilePath)
}
```

### 5. Testing Error Cases

Always test error scenarios:

```go
func TestGateway_GenerateContent_ErrorHandling(t *testing.T) {
    tests := []struct {
        name          string
        request       *gateway.GenerateContentRequest
        mockResponse  error
        expectedError string
    }{
        {
            name:          "nil request",
            request:       nil,
            expectedError: "request cannot be nil",
        },
        {
            name: "empty messages",
            request: &gateway.GenerateContentRequest{
                Messages: []gateway.Message{},
            },
            expectedError: "messages cannot be empty",
        },
        {
            name: "API error",
            request: &gateway.GenerateContentRequest{
                Messages: []gateway.Message{
                    {Role: "user", Content: "test"},
                },
            },
            mockResponse:  fmt.Errorf("API rate limit exceeded"),
            expectedError: "API rate limit exceeded",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            gateway := newTestGateway(tt.mockResponse)
            
            _, err := gateway.GenerateContent(context.Background(), tt.request)
            
            require.Error(t, err)
            assert.Contains(t, err.Error(), tt.expectedError)
        })
    }
}
```

### 6. Using testify Assertions

**Common Assertions**:

```go
// Equality
assert.Equal(t, expected, actual)
assert.NotEqual(t, unexpected, actual)

// Nil checks
assert.Nil(t, value)
assert.NotNil(t, value)

// Errors
assert.NoError(t, err)
assert.Error(t, err)
assert.EqualError(t, err, "expected error message")
assert.ErrorContains(t, err, "partial message")

// Boolean
assert.True(t, condition)
assert.False(t, condition)

// Collections
assert.Empty(t, collection)
assert.NotEmpty(t, collection)
assert.Len(t, collection, expectedLength)
assert.Contains(t, collection, element)

// Comparison
assert.Greater(t, actual, threshold)
assert.GreaterOrEqual(t, actual, threshold)
assert.Less(t, actual, threshold)

// require variants (fail immediately)
require.NoError(t, err)
require.NotNil(t, value)
```

### 7. Test Helpers

Create helper functions for common test setup:

```go
// testhelper.go in test package
func newTestDB(t *testing.T) *sql.DB {
    t.Helper()
    
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    
    t.Cleanup(func() {
        db.Close()
    })
    
    return db
}

func createTestFile(t *testing.T, dir, name, content string) string {
    t.Helper()
    
    path := filepath.Join(dir, name)
    err := os.WriteFile(path, []byte(content), 0644)
    require.NoError(t, err)
    
    return path
}

// Use in tests
func TestSomething(t *testing.T) {
    db := newTestDB(t)
    // Test uses db
}
```

### 8. Context and Timeout Handling

Always test with proper context handling:

```go
func TestService_WithTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()
    
    service := NewService()
    
    // Should respect context timeout
    err := service.LongRunningOperation(ctx)
    
    assert.Error(t, err)
    assert.ErrorIs(t, err, context.DeadlineExceeded)
}
```

## Test Organization

### File Naming

- Test files: `filename_test.go`
- Same directory as source: `service.go` → `service_test.go`
- Integration tests: `integration_test.go`

### Package Naming

**Unit tests** (white-box): Same package as source
```go
package index

func TestService_InternalMethod(t *testing.T) {
    // Can access unexported methods
}
```

**Integration tests** (black-box): Use `_test` suffix
```go
package index_test

import "github.com/retran/meowg1k/internal/core/index"

func TestPublicAPI(t *testing.T) {
    // Only access exported API
}
```

### Test Structure

Follow **Arrange-Act-Assert (AAA)** pattern:

```go
func TestExample(t *testing.T) {
    // Arrange - Setup test data and dependencies
    mockRepo := &mockRepository{}
    service := NewService(mockRepo)
    input := "test input"
    
    // Act - Execute the code under test
    result, err := service.Process(input)
    
    // Assert - Verify the results
    require.NoError(t, err)
    assert.Equal(t, "expected output", result)
}
```

## Test Categories

### Unit Tests
- Test single function/method
- Mock all dependencies
- Fast execution (< 10ms per test)
- High coverage of edge cases
- Located with source files

### Integration Tests
- Test multiple components together
- Use real adapters (in-memory DB, etc.)
- Slower execution acceptable
- Verify end-to-end flows
- Located in `*_test.go` with `_test` package

### Examples

**Unit Test**:
```go
// internal/core/chunker/chunker_test.go
package chunker

func TestFixedSizeChunker_Chunk(t *testing.T) {
    chunker := NewFixedSizeChunker(100)
    // Test chunking logic only
}
```

**Integration Test**:
```go
// internal/adapters/sqlite/index/repository_test.go
package index_test

func TestRepository_EndToEnd(t *testing.T) {
    // Test full repository lifecycle with real SQLite
}
```

## Best Practices

### ✅ Do

1. **Test behavior, not implementation**
```go
// Good: Test what it does
func TestCalculateTotal_ReturnsSum(t *testing.T) {
    total := CalculateTotal([]int{1, 2, 3})
    assert.Equal(t, 6, total)
}

// Bad: Test how it does it
func TestCalculateTotal_UsesForLoop(t *testing.T) {
    // Testing implementation details
}
```

2. **Use descriptive test names**
```go
// Good
func TestIndexService_AddDocument_WithDuplicateHash_ReturnsExistingVersion(t *testing.T)

// Bad
func TestAddDoc(t *testing.T)
```

3. **Test one thing per test**
```go
// Good: Focused test
func TestValidation_EmptyEmail_ReturnsError(t *testing.T)

// Bad: Testing multiple things
func TestValidation(t *testing.T) {
    // Tests empty email, invalid format, etc.
}
```

4. **Use t.Helper() in test utilities**
```go
func assertNoError(t *testing.T, err error) {
    t.Helper() // Shows correct line number in failure
    require.NoError(t, err)
}
```

5. **Clean up resources**
```go
func TestWithTempFile(t *testing.T) {
    f, err := os.CreateTemp("", "test")
    require.NoError(t, err)
    
    t.Cleanup(func() {
        os.Remove(f.Name())
    })
    
    // Test logic
}
```

### ❌ Don't

1. **Don't skip error checks in tests**
```go
// Bad
result, _ := service.Process(input)

// Good
result, err := service.Process(input)
require.NoError(t, err)
```

2. **Don't use magic numbers**
```go
// Bad
assert.Equal(t, 42, len(results))

// Good
expectedCount := 42
assert.Equal(t, expectedCount, len(results))
```

3. **Don't test external services directly**
```go
// Bad: Depends on external API
func TestRealAnthropicAPI(t *testing.T) {
    client := anthropic.NewClient(apiKey)
    // Test against real API
}

// Good: Mock the gateway
func TestAnthropicGateway(t *testing.T) {
    mockClient := &mockAnthropicClient{}
    // Test with mock
}
```

4. **Don't use time.Sleep for synchronization**
```go
// Bad
go asyncOperation()
time.Sleep(100 * time.Millisecond)
assert.True(t, operationComplete)

// Good: Use channels or sync primitives
done := make(chan bool)
go func() {
    asyncOperation()
    done <- true
}()
select {
case <-done:
    assert.True(t, operationComplete)
case <-time.After(1 * time.Second):
    t.Fatal("timeout waiting for operation")
}
```

## Coverage Analysis

### Viewing Coverage

```bash
# Generate coverage report
task check:test

# View HTML report
go tool cover -html=coverage.out

# View per-function coverage
go tool cover -func=coverage.out
```

### Improving Coverage

**Focus on**:
1. Error paths
2. Edge cases
3. Boundary conditions
4. Core business logic

**Less critical**:
1. Generated code
2. Simple getters/setters
3. Trivial wrappers

## CI/CD Integration

Tests run automatically on:
- Push to `dev` or `main` branches
- Pull request creation
- See `.github/workflows/ci.yaml`

**Requirements**:
- ✅ All tests must pass
- ✅ Coverage ≥ 75%
- ✅ No race conditions
- ✅ Linter passes

## Summary

- **75% coverage minimum** - enforced in CI/CD
- **testify framework** - use `require.*` for critical checks, `assert.*` for non-critical
- **Table-driven tests** - preferred pattern for multiple scenarios
- **Mock dependencies** - use interfaces for testability
- **Test what, not how** - focus on behavior, not implementation
- **AAA pattern** - Arrange, Act, Assert structure
- **Clean up resources** - use `t.Cleanup()` for resource management
