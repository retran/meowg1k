# meowg1k - AI-Powered CLI Tool

meowg1k is a fast, script-friendly AI companion CLI tool built in Go. It provides AI-assisted code generation, commit messages, PR descriptions, and semantic code search capabilities with Retrieval-Augmented Generation (RAG).

## Project Type

**Language**: Go 1.25.5  
**Architecture**: Hexagonal (Ports & Adapters)  
**CLI Framework**: Cobra  
**TUI Framework**: Bubble Tea + Lip Gloss  
**Build System**: Task (Taskfile.yaml)  
**Testing**: testify with 65% coverage requirement  
**License**: Apache 2.0

## Project Structure

```
.
├── cmd/                      # CLI commands and entry points
│   ├── meow/                # Main binary entry point
│   ├── root.go              # Root command setup
│   ├── init.go              # Project initialization command
│   ├── starlark.go          # Starlark command loader
│   └── version.go           # Version command
├── internal/                # Internal packages (not importable externally)
│   ├── adapters/            # Infrastructure adapters (implementations)
│   │   ├── gateway/         # LLM provider adapters (Anthropic, OpenAI, Gemini, etc.)
│   │   ├── sqlite/          # SQLite repositories for indexing and caching
│   │   ├── git/             # Git service implementation
│   │   ├── config/          # Configuration service
│   │   ├── output/          # Output rendering service
│   │   └── ...
│   ├── core/                # Core business logic services
│   │   ├── index/           # RAG indexing and retrieval
│   │   ├── model/           # Model management
│   │   ├── preset/          # Preset management
│   │   ├── provider/        # Provider management
│   │   ├── starlark/        # Starlark runtime and modules
│   │   ├── chunker/         # Text chunking strategies
│   │   ├── vector/          # Vector operations (HNSW)
│   │   └── ...
│   ├── domain/              # Domain models and types
│   │   ├── config/          # Configuration domain types
│   │   ├── gateway/         # Gateway interfaces and types
│   │   ├── git/             # Git domain types
│   │   ├── index/           # Index domain types
│   │   └── ...
│   ├── ports/               # Port interfaces (contracts)
│   │   └── types.go         # Service port definitions
│   ├── ui/                  # Terminal UI components
│   │   ├── theme.go         # Color schemes and styling
│   │   ├── markdown.go      # Markdown rendering
│   │   ├── code.go          # Code highlighting
│   │   ├── diff.go          # Diff visualization
│   │   ├── prompt.go        # User prompts
│   │   ├── select.go        # Selection menus
│   │   └── ...
│   ├── app/                 # Application layer
│   │   └── container.go     # Dependency injection container
│   └── version/             # Version information
├── .meowg1k/                # User configuration directory
│   ├── init.star            # Main configuration (providers, models, presets)
│   ├── commands/            # User-defined Starlark commands
│   │   ├── write.star       # AI-assisted writing
│   │   ├── commit.star      # Commit message generation
│   │   ├── pr.star          # PR description generation
│   │   ├── code.star        # Code Q&A
│   │   └── search.star      # Semantic search
│   └── lib/                 # Shared Starlark libraries
├── docs/                    # Documentation
│   ├── api/                 # API references
│   │   └── API_REFERENCE.md # Complete Starlark API documentation
│   └── guides/              # Development guides
│       ├── agentic-system.md
│       ├── architecture.md
│       ├── go-conventions.md
│       ├── starlark-system.md
│       ├── testing-standards.md
│       └── ui-patterns.md
├── Taskfile.yaml            # Task build system configuration
├── go.mod                   # Go module definition
├── .golangci.yaml           # golangci-lint configuration
└── .goreleaser.yaml         # GoReleaser configuration

```

## Key Features

### 1. Multi-Provider LLM Support
- Extensible gateway pattern for multiple LLM providers
- Built-in adapters: Anthropic, OpenAI, Google Gemini, Ollama/Llama, Voyage, OpenRouter
- Response caching for performance and cost optimization
- Retry logic with exponential backoff

### 2. Starlark Extension System
- User-defined commands via Starlark scripting (`.meowg1k/`)
- Rich standard library: fs, git, llm, shell, index, ui, json, path, crypto, time, regexp
- Provider/Model/Preset configuration pattern
- Tool system with parameter validation and automatic CLI integration

### 3. RAG and Code Search
- Semantic code indexing using vector embeddings (HNSW algorithm)
- SQLite-backed persistence
- Configurable chunking strategies
- Context-aware retrieval for code Q&A

### 4. Rich Terminal UI
- Bubble Tea-based interactive components
- Syntax highlighting with Chroma
- Markdown rendering with Glamour
- Custom widgets: progress bars, spinners, selections, prompts, diffs, tables

## Architecture Overview

meowg1k follows **Hexagonal Architecture** (Ports & Adapters):

- **Domain Layer** (`internal/domain/`): Core business types and domain models
- **Ports Layer** (`internal/ports/`): Service interfaces defining contracts
- **Core Layer** (`internal/core/`): Business logic implementations
- **Adapters Layer** (`internal/adapters/`): Infrastructure implementations (DB, HTTP, Git, etc.)
- **Application Layer** (`internal/app/`): Dependency injection and application setup
- **UI Layer** (`internal/ui/`): Presentation logic (terminal rendering)
- **CMD Layer** (`cmd/`): CLI command definitions and entry points

This architecture ensures:
- Clear separation of concerns
- Testability through dependency injection
- Easy extensibility for new providers and features
- Independence from external frameworks and libraries

## Build and Development

### Available Task Commands

```bash
# Dependencies
task deps:install          # Download and tidy Go modules

# Testing and Quality
task check:all            # Run all checks (lint, test, security)
task check:lint           # Run golangci-lint
task check:test           # Run tests with coverage (65% threshold)
task check:security       # Run gosec and govulncheck

# Code Formatting
task fix:fmt              # Format code with gofumpt and goimports
task fix:lint             # Auto-fix lint issues

# Building
task build                # Build binary to bin/meow
task install              # Install to GOBIN

# Release
task release              # Release via GoReleaser
task release:snapshot     # Build snapshot release

# Cleanup
task clean                # Clean artifacts
```

### Quick Start for Development

```bash
# 1. Install dependencies
task deps:install

# 2. Run tests
task check:test

# 3. Build the binary
task build

# 4. Run the binary
./bin/meow --help
```

## Code Standards

### General Conventions

1. **File Headers**: All Go source files must include Apache 2.0 license header
2. **Package Documentation**: All packages should have a package-level comment
3. **Error Handling**: Always wrap errors with context using `fmt.Errorf` with `%w`
4. **Nil Checks**: Always check for nil before dereferencing pointers
5. **Interface Usage**: Depend on interfaces, not concrete types (especially in core/)
6. **Testing**: Aim for 65%+ test coverage; tests must pass before commit

### Go-Specific

- Use `gofumpt` for formatting (stricter than `gofmt`)
- Follow standard Go project layout conventions
- Use Cobra for CLI commands, Bubble Tea for TUI
- Prefer table-driven tests with testify assertions
- Use meaningful variable names (avoid single letters except in short scopes)

### Naming Conventions

- **Packages**: Short, lowercase, single-word names (e.g., `index`, `gateway`, `model`)
- **Interfaces**: Noun or noun phrase (e.g., `GitService`, `ModelRepository`)
- **Methods**: Verb or verb phrase (e.g., `GetModel`, `CreateIndex`, `UpdateCache`)
- **Files**: Match the primary type/function they contain (e.g., `service.go`, `types.go`)

### Testing Patterns

- Test files alongside source: `service.go` → `service_test.go`
- Use testify for assertions: `require.NoError(t, err)`, `assert.Equal(t, expected, actual)`
- Mock dependencies using interfaces
- Table-driven tests for multiple scenarios
- Integration tests should use `_test` package suffix for black-box testing

## Configuration System

### Provider/Model/Preset Pattern

Configuration follows a three-tier system defined in `.meowg1k/init.star`:

1. **Provider**: Connection details for an LLM provider (API key, base URL)
2. **Model**: Specific model configuration (model name, token limits, provider reference)
3. **Preset**: Reusable model + parameters combination (model reference, temperature, etc.)

Example:
```python
# Define provider
meow.provider("gemini", type="gemini", api_key=env.get("MEOW_GEMINI_API_KEY"))

# Define model
meow.model("gemini-flash", provider="gemini", model="gemini-3-flash-preview", 
           max_input_tokens=1048576, max_output_tokens=65536)

# Define preset
meow.preset("fast", model="gemini-flash", temperature=0.2)
```

### User-Defined Commands

Users can create custom commands in `.meowg1k/commands/*.star`. See docs/api/API_REFERENCE.md for details on the tool system, handler context, and available modules.

## Important Files

- **docs/api/API_REFERENCE.md**: Complete Starlark API reference
- **Taskfile.yaml**: Build commands and automation
- **.golangci.yaml**: Linter configuration
- **.meowg1k/init.star**: User configuration entry point
- **go.mod**: Dependencies and Go version

## Dependencies

Key external dependencies:
- `github.com/spf13/cobra` - CLI framework
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/charmbracelet/glamour` - Markdown rendering
- `github.com/alecthomas/chroma` - Syntax highlighting
- `github.com/anthropics/anthropic-sdk-go` - Anthropic API
- `github.com/openai/openai-go` - OpenAI API
- `google.golang.org/genai` - Google Gemini API
- `github.com/ncruces/go-sqlite3` - SQLite database
- `github.com/coder/hnsw` - HNSW vector search
- `go.starlark.net` - Starlark runtime
- `github.com/stretchr/testify` - Testing utilities

## Development Workflow

1. **Make Changes**: Edit source files following code standards
2. **Format Code**: `task fix:fmt` (runs gofumpt, goimports)
3. **Run Linter**: `task check:lint` (must pass before commit)
4. **Run Tests**: `task check:test` (must maintain 65%+ coverage)
5. **Build**: `task build` (creates `bin/meow`)
6. **Commit**: Use conventional commit messages
7. **CI/CD**: GitHub Actions runs lint, security, and tests automatically

## References

For detailed information on specific topics, see the modular instruction files:

- **docs/guides/architecture.md** - Deep dive into hexagonal architecture
- **docs/guides/starlark-system.md** - Starlark extension system guide
- **docs/guides/agentic-system.md** - Agentic system and autonomous agents guide
- **docs/guides/testing-standards.md** - Testing patterns and best practices
- **docs/guides/go-conventions.md** - Go-specific coding standards
- **docs/guides/ui-patterns.md** - Bubble Tea UI component patterns
- **docs/api/API_REFERENCE.md** - Complete Starlark API documentation

## OpenCode Configuration Maintenance

**IMPORTANT**: The OpenCode configuration (AGENTS.md, opencode.json, and docs/guides/*.md files) is part of this project and should be maintained as the project evolves.

When making significant changes to the project, **always consider updating the OpenCode configuration**:

### When to Update OpenCode Configuration

1. **New Architecture Patterns**: Adding new layers, services, or architectural patterns
   - Update `docs/guides/architecture.md` with new patterns
   - Document new dependency injection patterns in `app/container.go`

2. **New Starlark Modules or Commands**: Adding functionality to the Starlark extension system
   - Update `docs/guides/starlark-system.md` with new modules or APIs
   - Document new standard library functions
   - Add examples of new command patterns

3. **Testing Changes**: New testing patterns, tools, or coverage requirements
   - Update `docs/guides/testing-standards.md` with new patterns
   - Document new mock patterns or testing utilities
   - Update coverage thresholds if changed

4. **Code Standards Changes**: New Go conventions or style guidelines
   - Update `docs/guides/go-conventions.md` with new standards
   - Document new linter rules or formatting requirements
   - Add examples of preferred patterns

5. **UI Component Additions**: New Bubble Tea components or UI patterns
   - Update `docs/guides/ui-patterns.md` with new widgets
   - Document theming conventions for new components
   - Add examples of interactive component patterns

6. **Build System Changes**: Task command changes or new build steps
   - Update AGENTS.md with new Task commands
   - Update `opencode.json` if formatter configuration changes
   - Document new CI/CD workflows

7. **Project Structure Changes**: New directories or reorganization
   - Update the Project Structure section in AGENTS.md
   - Update file exclusion patterns in `opencode.json` if needed
   - Document the purpose of new directories

### How to Update

- **Small Changes**: Directly edit the relevant `docs/guides/*.md` file
- **Structural Changes**: Update AGENTS.md and relevant instruction files
- **Build Changes**: Update both AGENTS.md and `opencode.json`
- **New Patterns**: Add examples to the appropriate instruction file

### Validation

After updating OpenCode configuration:
1. Ensure all referenced files in `opencode.json` still exist
2. Verify documentation accurately reflects current code structure
3. Test that OpenCode can successfully parse the configuration
4. Consider having OpenCode review its own config files for accuracy

**Treat OpenCode configuration as living documentation** - keep it synchronized with the codebase to maximize AI assistance effectiveness.

---

## Getting Help

- **GitHub Repository**: https://github.com/retran/meowg1k
- **Issues**: Report bugs and request features on GitHub Issues
- **Documentation**: See `docs/` directory for user guides
- **API Reference**: See docs/api/API_REFERENCE.md for Starlark API details
