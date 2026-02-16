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

### 13. Plugin System
**Status**: Idea  
**Complexity**: Very High  
**Estimated Effort**: 2-3 weeks

Allow third-party Starlark libraries:

- [ ] Design plugin discovery mechanism
- [ ] Implement plugin sandboxing
- [ ] Create plugin registry/marketplace
- [ ] Add plugin management commands (`meow plugin install`, etc.)

---

### 14. Web UI
**Status**: Idea  
**Complexity**: Very High  
**Estimated Effort**: 4-6 weeks

Optional web interface for session management:

- [ ] Design web UI architecture
- [ ] Implement session viewer
- [ ] Implement live session monitoring
- [ ] Add session replay functionality

---

### 15. Language Server Protocol (LSP)
**Status**: Idea  
**Complexity**: Very High  
**Estimated Effort**: 3-4 weeks

IDE support for Starlark commands:

- [ ] Implement Starlark LSP server
- [ ] Add autocomplete for meowg1k standard library
- [ ] Add hover documentation
- [ ] Add go-to-definition for library functions

---

## Completed in This Branch ✓

- [x] Starlark migration (all core logic moved from Go to Starlark)
- [x] Unified session system with SQLite persistence
- [x] Agentic capabilities with hierarchical planning
- [x] Library ecosystem (14 libraries)
- [x] Comprehensive library documentation (planning.star, memory.star, LIBRARY_INDEX.md)
- [x] Test coverage improvement (44.2% → 64.9%)
- [x] Documentation restructuring (docs/api/, docs/guides/)
- [x] Legacy documentation cleanup (archive/, old man pages)
- [x] Squashed commit with comprehensive message
- [x] PR created with detailed description (#80)
- [x] Tagged v0.2.0 as baseline before refactoring

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
