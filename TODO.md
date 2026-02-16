# What's Left to Do

**Branch**: `retran/simplify` (PR #80)  
**Started from**: v0.2.0

---

## Critical for users

### 1. Write user docs (2-3 days)

Developers can read the code. Users need guides.

**Create**:
- `docs/user/install.md` - How to install
- `docs/user/setup.md` - How to configure `.meowg1k/init.star`
- `docs/user/commands.md` - What commands exist
- `docs/user/examples.md` - Common use cases
- `docs/user/faq.md` - "How do I..." answers
- `docs/user/troubleshoot.md` - When things break

### 2. Write README.md (1 day)

The project has no front page. We deleted the old one during cleanup.

**Include**:
- What is meowg1k - one paragraph
- Install instructions
- Quick example
- Link to docs

### 3. Generate man pages (1 day)

We deleted the old man pages. Generate new ones from Starlark commands.

Use `cobra` or write a custom generator.

### 4. Clean up test commands (1 week)

11 test commands live in `.meowg1k/commands/test-*.star`. Each needs a decision:

- **Keep** → rename, document, make official
- **Delete** → move to examples or remove

**Test commands**:
```
test-agentic-simple.star
test-agentic-tools.star
test-agentic.star
test-child-session.star
test-event-flow.star
test-llm-events.star
test-persistence.star
test-session.star
test-system-message.star
test-tool-objects.star
test-tool-value-run.star
```

---

## Good to have

### 5. Add examples to libraries (2-3 days)

`planning.star` and `memory.star` have 10+ examples each. Other libraries don't.

**Add examples to**:
- `file_ops.star`
- `shell.star`
- `git.star`
- `http.star`
- `diff.star`
- `validators.star`

### 6. Write integration tests (3-4 days)

Coverage jumped from 44% to 65%. Go higher:

- Test all production commands end-to-end (write, code, commit, pr, search)
- Test session persistence across restarts
- Test parent-child sessions
- Test tool execution and errors
- Benchmark RAG search speed

### 7. Create examples/ folder (1-2 days)

Show users what's possible:

```
examples/
├── commands/         # Custom commands
│   ├── analyze.star
│   └── review.star
├── workflows/        # Multi-step agents
│   ├── research.star
│   └── refactor.star
└── libraries/        # How to use each library
    ├── planning-example.star
    └── memory-example.star
```

### 8. Delete leftover files (30 min)

Merge artifacts and debug code:

- `internal/adapters/gateway/factory_test.go.orig`
- `internal/adapters/gateway/factory_test.go.rej`
- `debug_ui.go`

---

## Future ideas

### 9. Import Starlark libs from GitHub (3-4 weeks)

Like Bazel's `load("@repo//lib:file.star")`.

**Build**:
- Parse remote URLs in `load()` statements
- Download and cache libraries
- Pin versions (like `go.mod`)
- Check file integrity (SHA256)
- Support private repos

**Example**:
```python
# .meowg1k/workspace.star
github_library(
    name = "utils",
    repo = "github.com/user/meow-utils",
    version = "v1.2.3",
    sha256 = "abc123...",
)

# .meowg1k/commands/mycommand.star
load("@utils//lib:helpers.star", "format_code")
```

### 10. Parse YAML/XML/TOML/CSV (1-2 weeks)

JSON works. Add other formats.

**New modules**:
- `yaml` - Parse and write YAML
- `xml` - Parse with XPath support
- `toml` - Parse and write TOML v1.0
- `csv` - Read/write with custom delimiters

**Example**:
```python
load("//lib/yaml.star", "yaml")
load("//lib/xml.star", "xml")

config = yaml.parse(fs.read("config.yaml"))
xml_output = xml.encode(config, root="config")
```

### 11. Validate LLM responses (2-3 weeks)

Force LLMs to return structured data.

**Add**:
- JSON Schema validation
- `response_format` parameter in `llm.generate()`
- Auto-retry on validation errors
- Support for OpenAI/Anthropic/Gemini structured outputs

**Example**:
```python
load("//lib/llm.star", "llm")
load("//lib/schema.star", "schema")

ReviewSchema = schema.object({
    "score": schema.integer(min=1, max=10),
    "summary": schema.string(max_length=200),
    "approved": schema.boolean(),
})

review = llm.generate(
    prompt="Review this code...",
    response_format=ReviewSchema,
    validate=True,  # Retry on invalid JSON
)

print(review.score)  # Guaranteed to exist
```

### 12. Stream LLM output in terminal (2-3 weeks)

Show words as they arrive, not after completion.

**Build**:
- Render text as tokens stream in
- Progressive markdown rendering
- Syntax highlighting during stream
- Show tokens/second metrics
- Handle Ctrl+C gracefully

**Example**:
```python
load("//lib/llm.star", "llm")
load("//lib/ui.star", "ui")

for chunk in llm.stream("Write a story..."):
    ui.append(chunk.text)  # Render immediately
    ui.show_speed(chunk.tokens_per_sec)
```

### 13. Plugin system (2-3 weeks)

Let users share Starlark libraries safely.

**Build**:
- Sandbox plugins (restrict file access)
- Plugin registry/marketplace
- Signature verification
- `meow plugin install` command

Works with #9 (import system).

### 14. Web UI for sessions (4-6 weeks)

View sessions in a browser:

- List all sessions
- Watch sessions run live
- Replay past sessions
- Share sessions with team

### 15. LSP for Starlark (3-4 weeks)

Make IDEs understand meowg1k code:

- Autocomplete for standard library
- Hover to see function docs
- Jump to definition
- Inline errors for schema validation

### 16. Speed up queries (ongoing)

**Profile and optimize**:
- Starlark execution time
- SQLite query speed
- Vector search (HNSW)
- File chunking strategies

### 17. Better error messages (2-3 days)

Make errors useful:

- "Syntax error line 42" → "Missing closing quote on line 42: `name = "foo`"
- "Invalid parameter" → "Expected string, got number. Try: `count="5"` instead of `count=5`"
- "Config not found" → "Create `.meowg1k/init.star` first. See docs/user/setup.md"
- Add "Did you mean?" for typos

### 18. Update GitHub templates (1 hour)

Change issue templates:

- Mention Starlark (not YAML)
- Add "Request Starlark API" template
- Add "Request new library" template

### 19. Improve CI/CD (1-2 days)

- Verify nightly builds work
- Run integration tests on CI
- Check for performance regressions
- Lint Starlark files
- Validate documentation links

---

## How to use this file

**Priority order**:
1. Write user docs (#1, #2, #3) - users can't learn without docs
2. Clean up commands (#4) - confusing to have test-* in production
3. Everything else when we have time

**Before you start a task**:
- Check if it breaks existing code
- Update LIBRARY_INDEX.md if you change the API
- Keep test coverage above 65%
- Write a CHANGELOG entry for user-facing changes

---

**Updated**: 2026-02-16  
**PR**: #80
