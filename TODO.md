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

### 10. Stream LLM output in terminal (2-3 weeks)

Show words as they arrive, not after completion.

**Build**:
- Add streaming support to ALL provider gateways:
  - ✅ OpenAI (via SSE)
  - ✅ Anthropic (via SSE)  
  - ✅ Gemini (via gRPC streaming)
  - ✅ OpenRouter (via SSE)
  - ❌ Ollama/Llama (needs implementation)
  - ❌ Voyage (embeddings only, N/A)
- Implement streaming UI renderer with Bubble Tea:
  - Progressive markdown rendering
  - Syntax highlighting during stream
  - Show tokens/second metrics
  - Handle Ctrl+C gracefully
  - Real-time progress indicators
- Add `stream=True` parameter to `llm.generate()` and `llm.agentic()`
- Buffer management for incomplete tokens
- Error handling mid-stream

**Example**:
```python
# Streaming in commands
result = ctx.llm.generate(
    prompt="Write a story...",
    stream=True,  # Enable streaming
)
# Text appears progressively in terminal

# Streaming with callback
def on_chunk(chunk):
    ctx.ui.append(chunk.text)
    ctx.ui.show_speed(chunk.tokens_per_sec)

ctx.llm.generate(
    prompt="Explain quantum computing...",
    stream=True,
    on_chunk=on_chunk
)
```

**UI Features**:
- Animated spinner while waiting for first token
- Character-by-character or word-by-word rendering
- Syntax highlighting updates in real-time for code blocks
- Final cleanup pass for complete markdown rendering

### 11. Plugin system (2-3 weeks)

Let users share Starlark libraries safely.

**Build**:
- Sandbox plugins (restrict file access)
- Plugin registry/marketplace
- Signature verification
- `meow plugin install` command

Works with #9 (import system).

### 12. Web UI for sessions (4-6 weeks)

View sessions in a browser:

- List all sessions
- Watch sessions run live
- Replay past sessions
- Share sessions with team

### 13. LSP for Starlark (3-4 weeks)

Make IDEs understand meowg1k code:

- Autocomplete for standard library
- Hover to see function docs
- Jump to definition
- Inline errors for schema validation

### 14. Speed up queries (ongoing)

**Profile and optimize**:
- Starlark execution time
- SQLite query speed
- Vector search (HNSW)
- File chunking strategies

### 15. Better error messages (2-3 days)

Make errors useful:

- "Syntax error line 42" → "Missing closing quote on line 42: `name = "foo`"
- "Invalid parameter" → "Expected string, got number. Try: `count="5"` instead of `count=5`"
- "Config not found" → "Create `.meowg1k/init.star` first. See docs/user/setup.md"
- Add "Did you mean?" for typos

### 16. Update GitHub templates (1 hour)

Change issue templates:

- Mention Starlark (not YAML)
- Add "Request Starlark API" template
- Add "Request new library" template

### 17. Improve CI/CD (1-2 days)

- Verify nightly builds work
- Run integration tests on CI
- Check for performance regressions
- Lint Starlark files
- Validate documentation links

### 18. Support MCP (Model Context Protocol) (3-4 weeks)

Add support for Anthropic's Model Context Protocol to integrate external tools and resources.

**Build**:
- MCP client implementation in Go
- Connect to MCP servers via stdio, HTTP, or SSE
- Expose MCP tools to Starlark as `ctx.mcp.call()`
- Support resource listing and reading
- Support prompts from MCP servers
- Handle tool discovery and schema conversion

**Example**:
```python
# .meowg1k/init.star
meow.mcp_server("filesystem",
    transport="stdio",
    command="npx",
    args=["-y", "@modelcontextprotocol/server-filesystem", "/Users/me/projects"]
)

# In a command
files = ctx.mcp.call("filesystem", "list_directory", {"path": "."})
content = ctx.mcp.call("filesystem", "read_file", {"path": "README.md"})
```

**Resources**:
- https://modelcontextprotocol.io/
- https://github.com/modelcontextprotocol

### 19. Support ACP (Agent Communication Protocol) (4-5 weeks)

Implement ACP for multi-agent orchestration and collaboration.

**Build**:
- ACP protocol implementation
- Agent-to-agent communication
- Task delegation and result aggregation
- Shared context and memory
- Agent lifecycle management
- Protocol negotiation

**Example**:
```python
# Define agents that communicate via ACP
code_agent = ctx.acp.spawn("code-reviewer", capabilities=["review", "suggest"])
test_agent = ctx.acp.spawn("test-writer", capabilities=["generate_tests"])

# Agents collaborate on a task
review = code_agent.request("review", {"file": "main.go"})
tests = test_agent.request("generate_tests", {"code": review.code})
```

**Resources**:
- Research existing agent communication protocols
- Design meowg1k-specific ACP extensions

### 20. Session compaction and summarization (2-3 weeks)

Automatically compress long conversation histories to fit within context windows.

**Build**:
- Detect when session approaches token limit
- Summarize older messages while preserving key information
- Keep recent messages verbatim for coherence
- Preserve critical context (function definitions, schemas, errors)
- Make compaction configurable (auto/manual, compression ratio)
- Store original uncompacted session for replay

**Example**:
```python
# Automatic compaction when approaching limit
session.compact(
    strategy="auto",
    keep_recent=10,  # Keep last 10 messages verbatim
    max_tokens=100000,  # Target total tokens
    preserve=["system", "tool_results"]  # Always keep these
)

# Manual summarization with LLM
summary = session.summarize(
    prompt="Summarize this conversation focusing on decisions made",
    max_length=500
)
session.replace_range(start=0, end=50, summary=summary)
```

**Features**:
- Token counting per message
- Intelligent message merging (combine related messages)
- Preserve code snippets and structured data
- Track compaction history
- Allow recovery of original messages

**Use cases**:
- Long debugging sessions
- Multi-step agent workflows
- Code review with many iterations
- Research tasks with extensive context

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
