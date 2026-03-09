# Contributing to meowg1k

Thank you for your interest in contributing to meowg1k! This guide will help you get started with development and
submitting contributions.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Testing Requirements](#testing-requirements)
- [Code Standards](#code-standards)
- [Commit Message Guidelines](#commit-message-guidelines)
- [Pull Request Process](#pull-request-process)
- [Code Review Guidelines](#code-review-guidelines)

## Code of Conduct

This project adheres to a Code of Conduct. By participating, you are expected to uphold this code. Please read
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) before contributing.

## Getting Started

### Prerequisites

- **Go**: Version 1.25.2 or higher (see `go.mod` for exact version)
- **Task**: Task build system (https://taskfile.dev)
- **Git**: For version control
- **golangci-lint**: For code linting (installed via Task)
- **pre-commit** (optional but recommended): For automated checks

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork:

   ```bash
   git clone https://github.com/YOUR_USERNAME/meowg1k.git
   cd meowg1k
   ```

3. Add upstream remote:

   ```bash
   git remote add upstream https://github.com/retran/meowg1k.git
   ```

## Development Setup

### Quick Setup

Run the automated development setup:

```bash
task dev:setup
```

This will:

- Install Go dependencies
- Install development tools (golangci-lint, pre-commit, etc.)
- Set up git hooks
- Verify your environment

### Manual Setup

If you prefer manual setup:

```bash
# 1. Install dependencies
task deps:install

# 2. Install development tools
task tools:install

# 3. Install git hooks (optional but recommended)
task hooks:install

# 4. Verify setup
task build
task check:test
```

## Development Workflow

### 1. Create a Feature Branch

Always work on a feature branch, not directly on `dev`:

```bash
git checkout dev
git pull upstream dev
git checkout -b feature/your-feature-name
```

Branch naming conventions:

- `feature/` - New features
- `fix/` - Bug fixes
- `refactor/` - Code refactoring
- `docs/` - Documentation updates
- `test/` - Test improvements
- `chore/` - Build/tooling updates

### 2. Make Your Changes

Edit code following our [code standards](#code-standards).

### 3. Run Tests Continuously

Use test watch mode during development:

```bash
task test:watch
```

Or run specific test suites:

```bash
task test:unit           # Unit tests only
task test:integration    # Integration tests
task test:race           # Race detector tests
```

### 4. Format and Lint Your Code

Before committing, ensure code is properly formatted and linted:

```bash
task fix:fmt     # Auto-format with gofumpt and goimports
task lint:fix    # Auto-fix lint issues
task check:lint  # Verify lint passes
```

Or use the pre-commit hook:

```bash
task ci:pre-commit
```

### 5. Run All Checks

Before pushing, run the full CI suite locally:

```bash
task ci:local
```

This runs:

- Linting (golangci-lint with 30+ linters)
- Security checks (gosec, govulncheck)
- Unit tests
- Integration tests
- Coverage validation (75% threshold)

### 6. Commit Your Changes

See [Commit Message Guidelines](#commit-message-guidelines) below.

### 7. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub targeting the `dev` branch.

## Testing Requirements

### Coverage Requirements

- Minimum **75% code coverage** required
- Test coverage is automatically checked in CI
- View coverage report: `task coverage:html`
- View function coverage: `task coverage:func`

### Test Organization

- Place tests alongside source: `service.go` → `service_test.go`
- Use table-driven tests for multiple scenarios
- Use testify for assertions: `require.NoError(t, err)`, `assert.Equal(t, expected, actual)`
- Integration tests use `_test` package suffix for black-box testing

### Running Tests

```bash
# Run all tests with coverage
task check:test

# Run only unit tests
task test:unit

# Run integration tests
task test:integration

# Run with race detector
task test:race

# Run verbose tests
task test:verbose

# Watch mode (re-run on file changes)
task test:watch
```

### Writing Good Tests

1. **Test one thing per test**: Keep tests focused
2. **Use descriptive names**: `TestServiceCreateUser_WithInvalidEmail_ReturnsError`
3. **Follow Arrange-Act-Assert pattern**:

   ```go
   func TestExample(t *testing.T) {
       // Arrange
       service := NewService()
       
       // Act
       result, err := service.DoSomething()
       
       // Assert
       require.NoError(t, err)
       assert.Equal(t, expected, result)
   }
   ```

4. **Use table-driven tests** for multiple scenarios:

   ```go
   func TestParseInput(t *testing.T) {
       tests := []struct {
           name    string
           input   string
           want    Result
           wantErr bool
       }{
           {"valid input", "test", Result{}, false},
           {"empty input", "", Result{}, true},
       }
       
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               got, err := ParseInput(tt.input)
               if tt.wantErr {
                   require.Error(t, err)
               } else {
                   require.NoError(t, err)
                   assert.Equal(t, tt.want, got)
               }
           })
       }
   }
   ```

## Code Standards

### Go Conventions

1. **Formatting**: Use `gofumpt` (stricter than `gofmt`) and `goimports`
2. **Naming**:
   - Packages: Short, lowercase, single-word (e.g., `index`, `gateway`)
   - Interfaces: Noun or noun phrase (e.g., `GitService`, `ModelRepository`)
   - Methods: Verb or verb phrase (e.g., `GetModel`, `CreateIndex`)
3. **Error Handling**: Always wrap errors with context using `fmt.Errorf` with `%w`
4. **Nil Checks**: Always check for nil before dereferencing pointers
5. **Interface Usage**: Depend on interfaces, not concrete types (especially in `core/`)

### License Headers

All Go source files must include the Apache 2.0 license header:

```go
// Copyright 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0
```

The pre-commit hook will automatically add this header if missing.

### Linting

We use golangci-lint with 30+ linters across categories:

- **Error Checking & Security**: errcheck, gosec, nilerr, errorlint, wrapcheck
- **Code Quality**: govet, staticcheck, unused, ineffassign, gocyclo, gocognit
- **Best Practices**: gocritic, revive, goconst, thelper
- **Performance**: prealloc, bodyclose, noctx
- **Style**: whitespace, godot, misspell

Run linters:

```bash
task lint:fast    # Quick essential linters
task lint:full    # All 30+ linters
task lint:fix     # Auto-fix issues
task lint:new     # Check only new code
```

### Architecture Patterns

meowg1k follows **Hexagonal Architecture** (Ports & Adapters):

- **Domain Layer** (`internal/domain/`): Core business types
- **Ports Layer** (`internal/ports/`): Service interfaces
- **Core Layer** (`internal/core/`): Business logic implementations
- **Adapters Layer** (`internal/adapters/`): Infrastructure implementations
- **Application Layer** (`internal/app/`): Dependency injection
- **UI Layer** (`internal/ui/`): Terminal rendering
- **CMD Layer** (`cmd/`): CLI commands

When adding new features:

1. Define domain types in `internal/domain/`
2. Define port interfaces in `internal/ports/`
3. Implement business logic in `internal/core/`
4. Implement adapters in `internal/adapters/`
5. Wire dependencies in `internal/app/container.go`

See [docs/agents/architecture.md](docs/agents/architecture.md) for detailed architecture guide.

## Commit Message Guidelines

We follow **Conventional Commits** specification:

### Format

```text
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `build`: Build system or dependency changes
- `ci`: CI/CD configuration changes
- `chore`: Maintenance tasks

### Scope (optional)

The scope indicates what part of the codebase is affected:

- `starlark`: Starlark runtime or modules
- `gateway`: LLM gateway adapters
- `index`: RAG indexing system
- `ui`: Terminal UI components
- `cmd`: CLI commands
- `core`: Core business logic
- `adapters`: Infrastructure adapters

### Examples

```text
feat(starlark): add http module for REST API calls

Implements new http module with request/response handling,
support for REST and GraphQL queries, and automatic JSON parsing.

Closes #123
```

```text
fix(gateway): handle nil pointer in anthropic adapter

Add nil check before dereferencing response pointer to prevent
panic when API returns empty response.

Fixes #456
```

```text
docs: update API reference with template module examples
```

### Breaking Changes

If your change breaks backward compatibility, add `BREAKING CHANGE:` in the footer:

```text
feat(starlark): rename fs.getcwd() to fs.cwd()

BREAKING CHANGE: fs.getcwd() has been renamed to fs.cwd() for consistency.
Users must update their .meowg1k/commands/*.star files.
```

## Pull Request Process

### Before Submitting

1. ✅ All tests pass: `task check:test`
2. ✅ All linters pass: `task check:lint`
3. ✅ Security checks pass: `task check:security`
4. ✅ Coverage meets 75% threshold
5. ✅ Code is properly formatted: `task fix:fmt`
6. ✅ Commit messages follow Conventional Commits
7. ✅ Changes are documented (API_REFERENCE.md if adding Starlark APIs)

Run full pre-push check:

```bash
task ci:pre-push
```

### PR Title and Description

- **Title**: Follow Conventional Commits format
- **Description**: Include:
  - Summary of changes
  - Motivation and context
  - Related issue numbers (e.g., "Closes #123")
  - Breaking changes (if any)
  - Testing performed
  - Screenshots (for UI changes)

### PR Checklist

Use this checklist in your PR description:

```markdown
## Checklist

- [ ] Tests added/updated with 75%+ coverage
- [ ] Documentation updated (API_REFERENCE.md if needed)
- [ ] Commit messages follow Conventional Commits
- [ ] All CI checks pass
- [ ] No breaking changes (or documented in commit message)
- [ ] Code follows Go conventions and architecture patterns
- [ ] License headers added to new files
```

### CI/CD Pipeline

Your PR will trigger the following CI checks:

**Stage 1: Fast Lint** (~1 min)

- Essential linters: gofmt, goimports, govet, errcheck, staticcheck

**Stage 2: Full Lint** (~3 min)

- All 30+ linters
- Markdown and YAML linting

**Stage 3: Security Scan** (~2 min)

- gosec, govulncheck, gitleaks

**Stage 4: Unit Tests** (~5 min)

- Ubuntu, macOS, Windows

**Stage 5: Integration Tests** (~3 min)

- End-to-end command testing

**Stage 6: Build Matrix** (~4 min)

- Cross-compile for 6 platforms

All stages must pass before merge.

## Code Review Guidelines

### For Contributors

- Be responsive to feedback
- Keep changes focused (one feature/fix per PR)
- Update PR based on review comments
- Ask questions if feedback is unclear

### For Reviewers

Review for:

1. **Correctness**: Does the code work as intended?
2. **Tests**: Are there adequate tests with good coverage?
3. **Architecture**: Does it follow hexagonal architecture patterns?
4. **Code Quality**: Is it readable, maintainable, and idiomatic Go?
5. **Security**: Are there any security concerns?
6. **Performance**: Are there obvious performance issues?
7. **Documentation**: Are changes documented?

### Review Process

1. PRs require at least one approval
2. All CI checks must pass
3. Resolve all review comments or discussions
4. Squash and merge into `dev` branch
5. Delete feature branch after merge

## Getting Help

- **Questions**: Open a [Discussion](https://github.com/retran/meowg1k/discussions)
- **Bugs**: Open an [Issue](https://github.com/retran/meowg1k/issues) with bug report template
- **Features**: Open an [Issue](https://github.com/retran/meowg1k/issues) with feature request template
- **Documentation**: See [docs/](docs/) directory and [API_REFERENCE.md](API_REFERENCE.md)

## License

By contributing to meowg1k, you agree that your contributions will be licensed under the Apache License 2.0.
See [LICENSE](LICENSE) for details.

---

**Thank you for contributing to meowg1k!**
