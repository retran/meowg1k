# TODO - Post-Starlark Refactoring Tasks

This document tracks tasks that were identified during the major Starlark refactoring (PR #80) but were not completed in this branch.

**Branch**: `retran/simplify`  
**Base Version**: v0.2.0 (commit d4f7539)  
**Last Updated**: 2026-02-16

---

## High Priority

### 1. User-Facing Documentation
**Status**: Not Started  
**Complexity**: Medium  
**Estimated Effort**: 2-3 days

The current documentation in `docs/` is focused on development and architecture. We need user-facing documentation:

- [ ] Create user installation guide (replacing deleted docs/01-INSTALLATION.md)
- [ ] Create user configuration guide (how to set up `.meowg1k/init.star`)
- [ ] Create command reference guide (replacing deleted docs/03-COMMAND-REFERENCE.md)
- [ ] Create examples guide with common use cases (replacing deleted docs/06-EXAMPLES.md)
- [ ] Create FAQ for common questions
- [ ] Create troubleshooting guide

**Files to Create**:
- `docs/user/installation.md`
- `docs/user/configuration.md`
- `docs/user/commands.md`
- `docs/user/examples.md`
- `docs/user/faq.md`
- `docs/user/troubleshooting.md`

**Related**: This was mentioned in the old documentation but removed during cleanup.

---

### 2. Man Pages
**Status**: Not Started  
**Complexity**: Low  
**Estimated Effort**: 1 day

Man pages were deleted during refactoring (`docs/man/*.1`, `docs/man/*.5`, `docs/man/*.7`). These should be regenerated for the new Starlark-based commands:

- [ ] Generate man pages from new command definitions
- [ ] Update man page generation in build process
- [ ] Add man page installation to Taskfile.yaml

**Files Deleted**:
- `docs/man/meow.1`
- `docs/man/meow-*.1` (various subcommands)
- `docs/man/meow-config.5`
- `docs/man/meow-security.7`

**Related Tools**: Could use `cobra` built-in man page generation or a custom generator.

---

### 3. README.md
**Status**: Missing  
**Complexity**: Medium  
**Estimated Effort**: 1 day

The main README.md was deleted and needs to be recreated with:

- [ ] Project overview and value proposition
- [ ] Quick start guide
- [ ] Installation instructions
- [ ] Basic usage examples
- [ ] Links to comprehensive documentation
- [ ] Contributing guidelines
- [ ] License information

**File to Create**: `README.md` (root level)

---

### 4. Command Implementation Gaps
**Status**: Identified  
**Complexity**: High  
**Estimated Effort**: 5-7 days

Several Starlark test commands exist that should be promoted to production commands or removed:

**Test Commands to Review**:
- `.meowg1k/commands/test-agentic-simple.star`
- `.meowg1k/commands/test-agentic-tools.star`
- `.meowg1k/commands/test-agentic.star`
- `.meowg1k/commands/test-child-session.star`
- `.meowg1k/commands/test-event-flow.star`
- `.meowg1k/commands/test-llm-events.star`
- `.meowg1k/commands/test-persistence.star`
- `.meowg1k/commands/test-session.star`
- `.meowg1k/commands/test-system-message.star`
- `.meowg1k/commands/test-tool-objects.star`
- `.meowg1k/commands/test-tool-value-run.star`

**Actions**:
- [ ] Review each test command for production readiness
- [ ] Either promote to production command or move to `examples/` or `tests/`
- [ ] Document decision for each command

---

## Medium Priority

### 5. Library Documentation Improvements
**Status**: Partially Complete  
**Complexity**: Medium  
**Estimated Effort**: 2-3 days

While `planning.star` and `memory.star` have comprehensive documentation (10+ examples each), other libraries could be improved:

- [ ] Add more examples to `file_ops.star` (currently basic)
- [ ] Add examples to `shell.star`
- [ ] Add examples to `git.star`
- [ ] Add examples to `http.star`
- [ ] Add examples to `diff.star`
- [ ] Add examples to `validators.star`

**Goal**: Match the quality of `planning.star` and `memory.star` documentation.

---

### 6. Integration Tests
**Status**: Partially Complete  
**Complexity**: High  
**Estimated Effort**: 3-4 days

Coverage improved from 44.2% → 64.9%, but integration tests could be expanded:

- [ ] End-to-end tests for all production commands (write, code, commit, pr, search)
- [ ] Integration tests for session persistence across restarts
- [ ] Integration tests for parent-child session hierarchies
- [ ] Integration tests for tool execution and error handling
- [ ] Performance benchmarks for RAG search

**Related Files**: 
- `internal/core/starlark/*_test.go`
- `internal/adapters/sqlite/session/repository_test.go`

---

### 7. Starlark Library Examples
**Status**: Not Started  
**Complexity**: Low  
**Estimated Effort**: 1-2 days

Create an `examples/` directory with real-world Starlark library usage:

- [ ] Create `examples/commands/` with sample commands
- [ ] Create `examples/libraries/` with sample library usage
- [ ] Create `examples/workflows/` with multi-step agentic workflows
- [ ] Add examples to documentation

**Structure**:
```
examples/
├── commands/
│   ├── custom-write.star
│   ├── custom-review.star
│   └── custom-analysis.star
├── libraries/
│   ├── using-planning.star
│   ├── using-memory.star
│   └── using-file-ops.star
└── workflows/
    ├── research-workflow.star
    └── refactor-workflow.star
```

---

### 8. Cleanup Residual Files
**Status**: Identified  
**Complexity**: Low  
**Estimated Effort**: 30 minutes

Some files appear to be merge artifacts or leftovers:

- [ ] Review and delete `internal/adapters/gateway/factory_test.go.orig`
- [ ] Review and delete `internal/adapters/gateway/factory_test.go.rej`
- [ ] Review and delete `debug_ui.go` (appears to be debug code)

**Files**:
- `internal/adapters/gateway/factory_test.go.orig`
- `internal/adapters/gateway/factory_test.go.rej`
- `debug_ui.go`

---

## Low Priority

### 9. Performance Optimization
**Status**: Not Started  
**Complexity**: High  
**Estimated Effort**: Ongoing

Opportunities for performance improvements:

- [ ] Profile Starlark execution performance
- [ ] Optimize session query performance (SQLite)
- [ ] Add caching for frequently accessed Starlark modules
- [ ] Benchmark vector search performance (HNSW)
- [ ] Optimize chunking strategies for large files

---

### 10. Enhanced Error Messages
**Status**: Partial  
**Complexity**: Medium  
**Estimated Effort**: 2-3 days

Improve user-facing error messages:

- [ ] Better error messages for Starlark syntax errors
- [ ] Contextual help when tool parameters are invalid
- [ ] Friendly errors when configuration is missing
- [ ] Add "did you mean?" suggestions for command typos

---

### 11. GitHub Issue Templates
**Status**: Partially Complete  
**Complexity**: Low  
**Estimated Effort**: 1 hour

Update issue templates to reflect new architecture:

- [ ] Review existing issue templates (`.github/ISSUE_TEMPLATE/`)
- [ ] Update templates to mention Starlark configuration
- [ ] Add template for "Starlark API request"
- [ ] Add template for "New tool library request"

---

### 12. CI/CD Improvements
**Status**: Working  
**Complexity**: Medium  
**Estimated Effort**: 1-2 days

Enhance continuous integration:

- [ ] Add nightly builds (nightly.yaml already exists, verify it works)
- [ ] Add integration test suite to CI
- [ ] Add performance regression tests
- [ ] Add Starlark linting/validation to CI
- [ ] Add documentation link checking

---

## Future Enhancements

### 13. Library Import System (Bazel-style)
**Status**: Idea  
**Complexity**: Very High  
**Estimated Effort**: 3-4 weeks

Implement a Bazel-style library import system for downloading and managing Starlark dependencies from GitHub:

- [ ] Design `load()` syntax for remote libraries (e.g., `load("@github.com/user/repo//lib:foo.star", "func")`)
- [ ] Implement dependency resolution and version pinning
- [ ] Add caching mechanism for downloaded libraries
- [ ] Implement integrity checking (SHA256 hashes)
- [ ] Create lock file format (similar to `go.sum` or `package-lock.json`)
- [ ] Add `meow deps update` command for dependency management
- [ ] Support private GitHub repositories via authentication
- [ ] Add dependency graph visualization
- [ ] Implement workspace concept (similar to Bazel `WORKSPACE` file)

**Example**:
```python
# .meowg1k/deps.star (workspace file)
github_archive(
    name = "awesome_lib",
    repo = "github.com/user/awesome-meow-lib",
    ref = "v1.2.3",
    sha256 = "abc123...",
)

# .meowg1k/commands/mycommand.star
load("@awesome_lib//lib:util.star", "helper_func")
```

---

### 14. Structured Data Libraries
**Status**: Idea  
**Complexity**: Medium  
**Estimated Effort**: 1-2 weeks

Expand data format support beyond JSON:

**YAML Support**:
- [ ] Add `yaml` Starlark module for parsing and serialization
- [ ] Support YAML anchors and references
- [ ] Add YAML validation helpers

**XML Support**:
- [ ] Add `xml` Starlark module for parsing and serialization
- [ ] Support XPath queries
- [ ] Add XML schema validation

**TOML Support**:
- [ ] Add `toml` Starlark module for parsing and serialization
- [ ] Support TOML v1.0 specification
- [ ] Add TOML validation helpers

**CSV Support**:
- [ ] Add `csv` Starlark module for parsing and writing
- [ ] Support custom delimiters and quoting
- [ ] Add CSV to JSON/dict conversion helpers

**Example**:
```python
load("//lib/yaml.star", "yaml")
load("//lib/xml.star", "xml")
load("//lib/toml.star", "toml")

# Parse YAML
config = yaml.parse(fs.read("config.yaml"))

# Convert to XML
xml_str = xml.encode(config, root="config")

# Write as TOML
fs.write("config.toml", toml.encode(config))
```

---

### 15. Structured LLM Responses
**Status**: Idea  
**Complexity**: High  
**Estimated Effort**: 2-3 weeks

Add support for structured/typed LLM responses with schema validation:

- [ ] Implement JSON Schema-based response validation
- [ ] Add `response_format` parameter to `llm.generate()`
- [ ] Support OpenAI structured outputs API
- [ ] Support Anthropic tool use for structured data
- [ ] Support Gemini function calling for structured responses
- [ ] Add Pydantic-style schema definitions in Starlark
- [ ] Implement automatic retry on schema validation failure
- [ ] Add response streaming for structured data
- [ ] Support partial structured responses

**Example**:
```python
load("//lib/llm.star", "llm")
load("//lib/schema.star", "schema")

# Define response schema
ReviewSchema = schema.object({
    "score": schema.integer(min=1, max=10),
    "summary": schema.string(max_length=200),
    "issues": schema.array(schema.string()),
    "approved": schema.boolean(),
})

# Request structured response
response = llm.generate(
    prompt="Review this code: ...",
    response_format=ReviewSchema,
    validate=True,  # Auto-retry on validation failure
)

# response is guaranteed to match schema
print(response.score)  # 8
print(response.approved)  # True
```

---

### 16. UI Streaming Support
**Status**: Idea  
**Complexity**: High  
**Estimated Effort**: 2-3 weeks

Enhance terminal UI to support real-time streaming of LLM responses:

- [ ] Implement streaming text rendering with word wrapping
- [ ] Add progress indicators for streaming responses
- [ ] Support partial markdown rendering (render as tokens arrive)
- [ ] Add syntax highlighting for streamed code blocks
- [ ] Implement cancellation support (Ctrl+C during streaming)
- [ ] Add visual indicators for thinking/processing state
- [ ] Support multi-column streaming (side-by-side comparisons)
- [ ] Add streaming diff visualization
- [ ] Implement token-per-second metrics display
- [ ] Add buffer management for very long responses

**Example**:
```python
load("//lib/llm.star", "llm")
load("//lib/ui.star", "ui")

# Stream response with live UI updates
for chunk in llm.stream(prompt="Write a story..."):
    ui.append(chunk.content)  # Live rendering
    ui.update_metrics(tokens=chunk.tokens, tps=chunk.tps)
```

**UI Features**:
- Live word-wrapping as content streams
- Progressive markdown rendering
- Spinner/progress for long pauses
- Token count and speed metrics
- Graceful handling of interruption

---

### 17. Plugin System
**Status**: Idea  
**Complexity**: Very High  
**Estimated Effort**: 2-3 weeks

Allow third-party Starlark libraries with sandboxing and security:

- [ ] Design plugin discovery mechanism
- [ ] Implement plugin sandboxing (restrict file system access)
- [ ] Create plugin registry/marketplace
- [ ] Add plugin management commands (`meow plugin install`, etc.)
- [ ] Implement plugin signing and verification
- [ ] Add plugin dependency resolution
- [ ] Support plugin configuration
- [ ] Add plugin lifecycle hooks (install, uninstall, update)

**Related**: Works well with Library Import System (#13)

---

### 18. Web UI
**Status**: Idea  
**Complexity**: Very High  
**Estimated Effort**: 4-6 weeks

Optional web interface for session management:

- [ ] Design web UI architecture
- [ ] Implement session viewer
- [ ] Implement live session monitoring
- [ ] Add session replay functionality
- [ ] Support real-time streaming in browser
- [ ] Add collaborative session sharing

---

### 19. Language Server Protocol (LSP)
**Status**: Idea  
**Complexity**: Very High  
**Estimated Effort**: 3-4 weeks

IDE support for Starlark commands:

- [ ] Implement Starlark LSP server
- [ ] Add autocomplete for meowg1k standard library
- [ ] Add hover documentation
- [ ] Add go-to-definition for library functions
- [ ] Support remote library imports (from #13)
- [ ] Add inline diagnostics for schema validation

---

## Notes

### Migration Strategy

When implementing these tasks:
1. **User Documentation**: Should be the top priority for adoption
2. **README.md**: Critical for first impressions
3. **Command Cleanup**: Important for clarity and maintenance
4. **Examples**: Help users understand the Starlark system

### Breaking Changes Tracking

Any tasks that introduce breaking changes should:
1. Update LIBRARY_INDEX.md migration guide
2. Add deprecation warnings before removal
3. Update version number appropriately
4. Document in CHANGELOG.md (to be created)

### Testing Requirements

All new features should maintain or improve the 64.9% test coverage threshold.

---

**Last Updated**: 2026-02-16  
**Maintainer**: retran  
**Related PR**: #80
