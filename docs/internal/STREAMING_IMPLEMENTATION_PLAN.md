# Streaming LLM API Redesign - Implementation Plan

**Status**: Planning Complete - Ready for Implementation  
**Version**: v0.3.0 (Breaking Change)  
**Branch**: `feature/streaming-llm-api-implementation`  
**Estimated Effort**: 102-139 hours (~3-4 weeks)

This plan defines the phased work to redesign the LLM API to support streaming, clean primitives
(`chat`, `agentic`, `embed`), and session-centric system prompts with synchronous callbacks.

## Session Summary (Feb 16, 2026)

### Key Decisions Made

1. **Backward Compatibility**: Hard break (Option A)
   - Delete `ctx.llm.generate()` immediately
   - No deprecation period or compatibility wrappers
   - Clean break for v0.3.0

2. **Cache Streaming**: Instant replay (Option A)
   - Cached responses return immediately without simulated delay
   - No artificial streaming simulation for cached content

3. **Session Storage**: Store final messages only (Option B)
   - Don't persist individual streaming deltas
   - Only store aggregated final messages in session database
   - Streaming events are ephemeral (UI-only)

4. **Thinking Detection**: Provider-specific (Option A)
   - Use actual provider APIs to detect thinking/reasoning content
   - Anthropic: `type: "thinking"` blocks with `thinking_delta` events
   - OpenAI: Research needed (o1/o3 models)
   - Gemini: Research needed
   - Tool calls: Can also have "thinking" or "description" blocks

5. **Abort Behavior**: Return partial (Option A)
   - When user aborts mid-stream, return what was received so far
   - No error thrown on user abort

### API Design (Finalized)

```python
# Chat - conversational primitive
ctx.llm.chat(
    prompt,              # Required: str
    preset="smart",      # Common: LLM preset
    system=None,         # Common: override session system
    use_session=True,    # Session control
    stream=False,        # Streaming control
    on_event=None        # Required if stream=True
) → str

# Agentic - autonomous tool execution
ctx.llm.agentic(
    prompt,              # Required: str
    tools,               # Required: list[tool]
    on_event,            # Required (always - for tool events)
    preset="smart",      # Common: LLM preset
    system=None,         # Common: override session system
    use_session=True,    # Session control
    max_iterations=50    # Advanced: iteration limit
) → str

# Embed - unchanged
ctx.llm.embed(texts, preset="fast") → list[list[float]]
```

### Stream Event Types

```python
# LLM events (chat + agentic)
{"kind": "text", "delta": "..."}
{"kind": "thinking", "delta": "..."}  # Provider-specific detection
{"kind": "usage", "usage": {"prompt": 100, "completion": 50, "total": 150}}
{"kind": "done", "usage": {...}}
{"kind": "error", "error": "...", "recoverable": true/false}

# Tool events (agentic only)
{"kind": "tool_call_start", "tool_name": "...", "tool_id": "...", "arguments": {...}}
{"kind": "tool_call_end", "tool_name": "...", "tool_id": "...", "duration_ms": 342, "arguments": {...}}
{"kind": "tool_call_error", "tool_name": "...", "tool_id": "...", "error": "...", "duration_ms": 15, "arguments": {...}}
```

### New Modules

**Session Module**:
```python
ctx.session.set_system(prompt)  # Set system prompt in metadata "__system_prompt__"
ctx.session.get_system()        # Get current system prompt
```

**Output Module**:
```python
ctx.output.is_tty() → bool      # Check if terminal output
```

### Commands Audit

**Using `llm.generate()` (7 files to migrate)**:
- `write.star` - Enable streaming (improves UX)
- `code.star` - Enable streaming (long responses)
- `commit.star` - Keep non-streaming (short response)
- `pr.star` - Keep non-streaming (short response)
- `search.star` - Keep non-streaming (structured output)
- `extract.star` - Keep non-streaming (structured output)
- `orchestrator-agent.star` - Multiple calls, mostly non-streaming

**Using `llm.agentic()` (2 files to update)**:
- `orchestrator-agent.star` - Update parameter order, add on_event
- `review-agent.star` - Update parameter order, add on_event

### Architecture Principles

- **Streaming events are ephemeral** (for UI only, not stored in session DB)
- **Final messages are persisted** (only aggregated text stored in session)
- **Callbacks are synchronous** (no async/await needed in Starlark)
- **Both APIs return final text** (not traces or intermediate state)
- **Go emits events, Starlark handles presentation**

## Executive Summary

Goals:
- Replace `ctx.llm.generate()` with `ctx.llm.chat()` and `ctx.llm.agentic()`.
- Add streaming callbacks with provider-normalized events.
- Store only final messages in sessions (no delta persistence).
- Keep streaming ephemeral: Go emits events, Starlark renders output.

Confirmed Decisions:
- Backward compatibility: hard break, delete `llm.generate()`.
- Cache streaming: instant replay when cached.
- Session storage: store final aggregated messages only.
- Thinking detection: provider-specific (Anthropic supports thinking blocks; others TBD).
- Abort behavior: return partial output when aborted mid-stream.

## Scope and Non-Goals

In scope:
- Gateway streaming with normalized events.
- New Starlark API for `chat` and `agentic`.
- Session system prompt helpers.
- Output helper (`is_tty`) for UI decisions.
- Migration of internal Starlark commands.
- Tests and documentation.

Out of scope:
- Backward compatibility wrappers.
- Async/await or coroutine support in Starlark.
- Persisting stream deltas in session storage.

## Phased Plan

### Phase 0: Preparation and Deprecation Materials (8-12h)

Deliverables:
- ADR documenting the break and streaming design.
- Migration guide for v0.3.0.
- LLM usage audit documentation.

Tasks:
- Create `docs/decisions/ADR-001-streaming-api.md`.
- Create `docs/migration/v0.3-streaming.md`.
- Create `docs/internal/llm-usage-audit.md`.

### Phase 1: Gateway Streaming Foundation (8-12h)

Deliverables:
- Stream event types.
- Gateway interface supports streaming.
- OpenAI streaming adapter implemented.

Tasks:
- Add `StreamEvent` and `StreamEventKind` in `internal/domain/gateway/types.go`.
- Extend `GenerationGateway` with `GenerateContentStream` in `internal/ports/types.go`.
- Implement streaming in `internal/adapters/gateway/openai.go`.
- Update caching and logging gateways for streaming.

### Phase 2: Session Module Enhancement (4-6h)

Deliverables:
- Starlark session helpers for system prompts.

Tasks:
- Add `ctx.session.set_system()` and `ctx.session.get_system()`.
- Store system prompt in session metadata key `__system_prompt__`.

### Phase 3: Output Module Enhancement (2-3h)

Deliverables:
- Starlark output helper for TTY detection.

Tasks:
- Add `IsTTY()` to output service and expose as `ctx.output.is_tty()`.

### Phase 4: LLM Module Rewrite (16-20h)

Deliverables:
- New `chat()` and `agentic()` implementations.
- Remove `generate()` completely.

Tasks:
- Rewrite `internal/core/starlark/module_llm.go`.
- Enforce `on_event` required for streaming.
- Enforce `on_event` required for agentic tool events.
- Ensure `chat()` and `agentic()` return final aggregated content.

### Phase 5: UI Helpers Library (4-6h)

Deliverables:
- Starlark streaming helpers for markdown/plain/agentic.

Tasks:
- Create `.meowg1k/lib/ui_helpers.star` with:
  - `make_markdown_stream_handler(ctx)`
  - `make_plain_stream_handler(ctx)`
  - `make_agentic_stream_handler(ctx, abort_on_error, max_errors)`

### Phase 6: Provider Implementations (16-20h)

Deliverables:
- Anthropic, Gemini, OpenRouter, Llama streaming implementations.

Tasks:
- Implement `GenerateContentStream` per provider.
- Normalize thinking events where supported.
- Add tests for each provider stream path.

### Phase 7: Command Migrations (8-12h)

Deliverables:
- All Starlark commands migrated to new API.

Tasks:
- Update commands in `.meowg1k/commands/`:
  - `write.star`, `code.star` -> streaming `chat()`.
  - `commit.star`, `pr.star`, `search.star`, `extract.star` -> non-streaming `chat()`.
  - `orchestrator-agent.star`, `review-agent.star` -> new `agentic()` signature.

### Phase 8: Testing and Validation (16-20h)

Deliverables:
- Unit and integration tests for streaming.

Tasks:
- Add unit tests for starlark llm/session/output modules.
- Add integration tests for streaming in `tests/integration/`.

### Phase 9: Documentation Updates (12-16h)

Deliverables:
- Updated API documentation and guides.

Tasks:
- Update `docs/api/API_REFERENCE.md`.
- Update `docs/guides/starlark-system.md` with streaming patterns.
- Add CHANGELOG entry for v0.3.0.

### Phase 10: Performance and UX Validation (8-12h)

Deliverables:
- Benchmarks and streaming performance verification.

Tasks:
- Add benchmark tests in `tests/benchmarks/`.
- Validate TTY streaming behavior for markdown/plain.

## Provider-Specific Thinking/Reasoning Detection

### Anthropic Claude (CONFIRMED)

**Extended Thinking Models**: Claude Opus 4.x, Sonnet 4.x, Haiku 4.5

**Streaming Events**:
- Thinking content emitted as `type: "thinking_delta"` events during streaming
- Final response includes thinking blocks: `{"type": "thinking", "thinking": "...", "signature": "..."}`
- Thinking blocks are summarized versions (charged for full tokens, shows summary in response)

**Enabling Thinking**:
```json
{
  "thinking": {"type": "enabled", "budget_tokens": N}
}
```
or for Opus 4.6+:
```json
{
  "thinking": {"type": "adaptive"}
}
```

**Important Notes**:
- Thinking blocks must be preserved when using tool use
- Redacted thinking blocks may appear when safety systems trigger
- Detection: Check `block.type == "thinking"` or `delta.type == "thinking_delta"`

**Implementation**: Map to generic `StreamEventKind.StreamEventThinking` in gateway layer.

---

### OpenAI (o1/o3 models)

**Reasoning Models**: o1-preview, o1, o3 series

**Current Status**: 
- OpenAI's o1/o3 models have reasoning capabilities
- **API structure unknown** - official docs blocked during research
- Known from cookbook: reasoning is used for complex problem-solving

**Research Needed**:
- [ ] Determine if reasoning tokens appear in streaming responses
- [ ] Identify event types for reasoning deltas (if any)
- [ ] Check if reasoning appears as separate content blocks or message roles
- [ ] Understand token counting for reasoning (prompt vs completion tokens)

**Hypothesis** (based on Anthropic pattern):
- Likely uses content block structure similar to Anthropic
- May have `reasoning` or `thought` content type in streaming chunks
- Possibly separate token counts for reasoning vs output

**Action**: Test with o1 model streaming to reverse-engineer event format.

---

### Google Gemini

**Current Status**:
- **API structure unknown** - official docs inaccessible during research
- Gemini API exists but thinking/reasoning capabilities unclear

**Research Needed**:
- [ ] Check if Gemini has extended thinking/reasoning mode
- [ ] Identify streaming event structure for content generation
- [ ] Determine if there are special content blocks for thinking
- [ ] Verify token counting behavior

**Hypothesis**:
- Gemini API uses gRPC streaming (already implemented in codebase)
- Likely follows standard content/part structure from Google AI SDK
- May not have explicit "thinking" blocks (different from Anthropic approach)

**Action**: Review existing `internal/adapters/gateway/gemini.go` implementation and test streaming behavior.

---

### Implementation Strategy

**Gateway Layer Responsibilities**:
1. Detect provider-specific thinking/reasoning content
2. Normalize to generic `StreamEventKind.StreamEventThinking`
3. Emit thinking events separately from text events
4. Preserve thinking metadata in final message storage (if applicable)

**Event Normalization**:
```go
// Anthropic: delta.type == "thinking_delta" → StreamEventThinking
// OpenAI: TBD (research needed)
// Gemini: TBD (research needed)
```

**Fallback Behavior**:
- If provider doesn't support thinking, never emit `StreamEventThinking` events
- Only emit `StreamEventText` for all content
- Tools/commands should handle absence of thinking events gracefully

## Detailed Implementation Examples

### Gateway Layer - Stream Event Types

```go
// internal/domain/gateway/types.go

type StreamEventKind int

const (
    StreamEventText StreamEventKind = iota
    StreamEventThinking
    StreamEventUsage
    StreamEventDone
    StreamEventError
    StreamEventToolCallStart
    StreamEventToolCallEnd
    StreamEventToolCallError
)

type StreamEvent struct {
    Kind       StreamEventKind
    Delta      string                 // For text/thinking events
    Usage      *UsageMetadata         // For usage/done events
    Error      string                 // For error events
    Recoverable bool                  // For error events
    ToolName   string                 // For tool events
    ToolID     string                 // For tool events
    Arguments  map[string]interface{} // For tool events
    DurationMS int64                  // For tool_call_end/error events
}

type StreamCallback func(event StreamEvent) error
```

### Starlark LLM Module - Chat Implementation

```python
# Example usage in .meowg1k/commands/write.star

def on_stream_event(event):
    if event["kind"] == "text":
        ctx.output.write(event["delta"])
    elif event["kind"] == "thinking":
        ctx.output.write_dimmed(event["delta"])
    elif event["kind"] == "usage":
        # Optional: show token usage
        pass
    elif event["kind"] == "done":
        ctx.output.write("\n")
    elif event["kind"] == "error":
        ctx.output.error(event["error"])

result = ctx.llm.chat(
    prompt="Write a story about a cat",
    preset="smart",
    stream=True,
    on_event=on_stream_event
)
```

### Starlark LLM Module - Agentic Implementation

```python
# Example usage in .meowg1k/commands/orchestrator-agent.star

def on_agentic_event(event):
    if event["kind"] == "text":
        ctx.output.write(event["delta"])
    elif event["kind"] == "thinking":
        ctx.output.write_dimmed(f"[Thinking: {event['delta']}]")
    elif event["kind"] == "tool_call_start":
        ctx.output.info(f"🔧 Calling {event['tool_name']}...")
    elif event["kind"] == "tool_call_end":
        ctx.output.success(f"✓ {event['tool_name']} completed ({event['duration_ms']}ms)")
    elif event["kind"] == "tool_call_error":
        ctx.output.error(f"✗ {event['tool_name']} failed: {event['error']}")

result = ctx.llm.agentic(
    prompt="Analyze this codebase and fix bugs",
    tools=[search_tool, edit_tool, test_tool],
    on_event=on_agentic_event,
    preset="smart",
    max_iterations=50
)
```

### Session Module - System Prompt Management

```python
# Example: Setting system prompt for a tool
def initialize_agent(ctx):
    ctx.session.set_system("""
You are a helpful code review assistant.
- Focus on best practices
- Suggest improvements
- Be concise and actionable
""")

# Example: Getting current system prompt
def show_current_instructions(ctx):
    system = ctx.session.get_system()
    if system:
        ctx.output.info(f"Current instructions:\n{system}")
    else:
        ctx.output.warn("No system prompt set")
```

### UI Helpers Library - Streaming Handlers

```python
# .meowg1k/lib/ui_helpers.star

def make_markdown_stream_handler(ctx):
    """Returns event handler that streams markdown with syntax highlighting"""
    def handler(event):
        if event["kind"] == "text":
            ctx.output.stream_markdown(event["delta"])
        elif event["kind"] == "thinking":
            ctx.output.write_dimmed(event["delta"])
        elif event["kind"] == "done":
            ctx.output.stream_markdown("", done=True)  # Flush
    return handler

def make_plain_stream_handler(ctx):
    """Returns event handler that streams plain text"""
    def handler(event):
        if event["kind"] == "text":
            ctx.output.write(event["delta"])
        elif event["kind"] == "done":
            ctx.output.write("\n")
    return handler

def make_agentic_stream_handler(ctx, abort_on_error=True, max_errors=3):
    """Returns event handler for agentic workflows with tool visibility"""
    error_count = {"value": 0}
    
    def handler(event):
        if event["kind"] == "text":
            ctx.output.write(event["delta"])
        elif event["kind"] == "thinking":
            ctx.output.write_dimmed(f"💭 {event['delta']}")
        elif event["kind"] == "tool_call_start":
            ctx.output.info(f"🔧 {event['tool_name']}({event['arguments']})")
        elif event["kind"] == "tool_call_end":
            ctx.output.success(f"✓ {event['tool_name']} ({event['duration_ms']}ms)")
        elif event["kind"] == "tool_call_error":
            error_count["value"] += 1
            ctx.output.error(f"✗ {event['tool_name']}: {event['error']}")
            if abort_on_error and error_count["value"] >= max_errors:
                raise RuntimeError(f"Too many tool errors ({max_errors})")
        elif event["kind"] == "error":
            ctx.output.error(f"LLM Error: {event['error']}")
            if not event["recoverable"]:
                raise RuntimeError(event["error"])
    
    return handler
```

## File Touch List (Primary)

### Core Implementation Files
- `internal/domain/gateway/types.go` - Stream event types
- `internal/ports/types.go` - GenerateContentStream interface
- `internal/adapters/gateway/openai.go` - OpenAI streaming
- `internal/adapters/gateway/anthropic.go` - Anthropic streaming + thinking
- `internal/adapters/gateway/gemini.go` - Gemini streaming
- `internal/adapters/gateway/openrouter.go` - OpenRouter streaming
- `internal/adapters/gateway/llama.go` - Llama streaming
- `internal/adapters/gateway/caching.go` - Cache instant replay
- `internal/adapters/gateway/logging.go` - Stream event logging
- `internal/core/starlark/module_llm.go` - Complete rewrite (chat/agentic)
- `internal/core/starlark/module_session.go` - System prompt helpers
- `internal/core/starlark/module_output.go` - is_tty() helper

### Command Migration Files
- `.meowg1k/commands/write.star` - Migrate to streaming chat()
- `.meowg1k/commands/code.star` - Migrate to streaming chat()
- `.meowg1k/commands/commit.star` - Migrate to non-streaming chat()
- `.meowg1k/commands/pr.star` - Migrate to non-streaming chat()
- `.meowg1k/commands/search.star` - Migrate to non-streaming chat()
- `.meowg1k/commands/extract.star` - Migrate to non-streaming chat()
- `.meowg1k/commands/orchestrator-agent.star` - Update agentic() signature
- `.meowg1k/commands/review-agent.star` - Update agentic() signature

### New Files
- `.meowg1k/lib/ui_helpers.star` - Streaming handlers library
- `docs/decisions/ADR-001-streaming-api.md` - Architecture decision record
- `docs/migration/v0.3-streaming.md` - Migration guide
- `docs/internal/llm-usage-audit.md` - Usage audit
- `tests/integration/streaming_test.go` - Integration tests
- `tests/benchmarks/streaming_bench_test.go` - Performance benchmarks

### Test Files (Update)
- `internal/adapters/gateway/openai_test.go` - Add streaming tests
- `internal/adapters/gateway/anthropic_test.go` - Add streaming + thinking tests
- `internal/adapters/gateway/gemini_test.go` - Add streaming tests
- `internal/adapters/gateway/openrouter_test.go` - Add streaming tests
- `internal/adapters/gateway/llama_test.go` - Add streaming tests
- `internal/core/starlark/module_llm_test.go` - Complete rewrite for new API
- `internal/core/starlark/module_session_test.go` - Add system prompt tests

### Documentation Files (Update)
- `docs/api/API_REFERENCE.md` - Complete LLM section rewrite
- `docs/guides/starlark-system.md` - Add streaming patterns
- `CHANGELOG.md` - Add v0.3.0 breaking changes

## Success Criteria

- All Starlark commands use `chat` or `agentic`.
- Streaming is supported across all providers (OpenAI, Anthropic, Gemini, OpenRouter, Llama).
- Unit and integration tests pass at 75%+ coverage.
- Documentation updated with migration guide and API reference.
- UI helpers library provides reusable streaming handlers.
- Session system prompt management works correctly.
- Thinking/reasoning events properly normalized across providers.
- Cache instant replay works for streaming responses.
- Abort behavior returns partial output correctly.

## Next Session Tasks

1. **Phase 0 Start**: Create ADR, migration guide, and audit docs
2. **OpenAI Research**: Test o1 streaming to discover reasoning event format
3. **Gemini Research**: Review existing implementation for streaming structure
4. **Phase 1 Start**: Implement gateway streaming foundation
5. **Testing Setup**: Create integration test skeleton
