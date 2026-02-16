# ADR-001: Streaming LLM API Redesign

**Status**: Proposed  
**Date**: 2026-02-16  
**Authors**: Development Team  
**Decision**: Redesign LLM API with streaming support, clean primitives, and session-centric architecture

## Context

The current `ctx.llm.generate()` API has several limitations:

1. **No streaming support**: Users cannot see real-time progress for long-running LLM calls
2. **Monolithic interface**: Single function tries to handle both chat and agentic workflows
3. **Unclear separation**: No distinction between conversational and tool-using patterns
4. **System prompt management**: System prompts passed per-call instead of session-managed
5. **Provider differences**: Thinking/reasoning capabilities not exposed consistently

Users want:
- Real-time streaming output with visual feedback
- Separate APIs for chat vs autonomous agent workflows
- Session-based system prompt management
- Provider-specific features (thinking/reasoning) exposed uniformly
- Tool execution visibility in agentic workflows

## Decision

We will implement a **complete API redesign** for v0.3.0 with three clean primitives:

### 1. Chat API - Conversational Primitive

```python
ctx.llm.chat(
    prompt,              # Required: str
    preset,              # Required: LLM preset name (NO DEFAULT)
    system=None,         # Optional: override session system
    use_session=True,    # Optional: session control (default: True)
    stream=False,        # Optional: streaming control (default: False)
    on_event=None        # Required if stream=True
) → str
```

**Purpose**: Single-turn or multi-turn conversations with streaming support.

**Design Choices**:
- Streaming is **opt-in** via `stream=True` + `on_event` callback
- System prompt comes from session by default, overridable per-call
- Returns final aggregated text (not trace/intermediate state)
- Callback receives normalized stream events

**Session Behavior** (`use_session=True`):
- Reads conversation history from session database
- Includes session history in LLM request context
- Appends user prompt and assistant response to session after completion
- System prompt retrieved from session metadata (`__system_prompt__`)

**Session Disabled** (`use_session=False`):
- Single-turn request (no history context)
- Response not persisted to session database
- Useful for one-off queries or stateless operations

### 2. Agentic API - Autonomous Tool Execution

```python
ctx.llm.agentic(
    prompt,              # Required: str
    preset,              # Required: LLM preset name (NO DEFAULT)
    tools,               # Required: list[tool]
    system=None,         # Optional: override session system
    use_session=True,    # Optional: session control (default: True)
    stream=False,        # Optional: streaming control (default: False)
    on_event=None,       # Required (always - for tool events)
    max_iterations=50    # Optional: iteration limit (default: 50)
) → str
```

**Purpose**: Autonomous workflows with tool execution and iteration.

**Design Choices**:
- `on_event` **always required** (tool execution visibility)
- Explicit tool list (no implicit access)
- Iteration limit prevents infinite loops
- Emits tool-specific events (start/end/error) in addition to LLM events

**Session Behavior** (`use_session=True`):
- Same as `chat()`: reads history, includes in context, persists after completion
- Tool calls and results are also added to session history
- Enables multi-turn agentic conversations with memory

**Session Disabled** (`use_session=False`):
- Single autonomous execution (no history context)
- Tool calls not persisted to session
- Useful for isolated tasks

### 3. Embed API - Unchanged

```python
ctx.llm.embed(
    texts,               # Required: list[str]
    preset               # Required: LLM preset name (NO DEFAULT)
) → list[list[float]]
```

**Purpose**: Convert text to embeddings (no changes).

### Supporting Modules

**Session Module** (new):
```python
ctx.session.set_system(prompt)  # Set system prompt in metadata
ctx.session.get_system() → str  # Get current system prompt
```

**Output Module** (enhanced):
```python
ctx.output.is_tty() → bool  # Check if terminal output
```

## Stream Event Design

### Normalized Event Types

All stream events follow a consistent structure:

```python
# LLM events (chat + agentic)
{"kind": "text", "delta": "..."}
{"kind": "thinking", "delta": "..."}  # Provider-specific
{"kind": "usage", "usage": {"prompt": 100, "completion": 50, "total": 150}}
{"kind": "done", "usage": {...}}
{"kind": "error", "error": "...", "recoverable": true/false}

# Tool events (agentic only)
{"kind": "tool_call_start", "tool_name": "...", "tool_id": "...", "arguments": {...}}
{"kind": "tool_call_end", "tool_name": "...", "tool_id": "...", "duration_ms": 342, "arguments": {...}}
{"kind": "tool_call_error", "tool_name": "...", "tool_id": "...", "error": "...", "duration_ms": 15, "arguments": {...}}
```

### Provider-Specific Thinking Detection

**Anthropic Claude** (Extended Thinking):
- Detected via `delta.type == "thinking_delta"` in streaming
- Response blocks: `{"type": "thinking", "thinking": "...", "signature": "..."}`
- Mapped to generic `"thinking"` event kind

**OpenAI** (Reasoning - o1/o3):
- Research needed to determine event structure
- Will map to `"thinking"` event kind if detected

**Gemini**:
- Research needed to determine thinking/reasoning support
- Will map to `"thinking"` event kind if detected

**Tool Calls**:
- Can include "thinking" or "description" fields
- Emitted as part of tool event structure

**Fallback**: Providers without thinking support never emit `"thinking"` events.

## Session Management

### How `use_session` Works

**Default**: `use_session=True` - All LLM calls use session history by default.

When `use_session=True` (DEFAULT):

1. **Before LLM Request**:
   - Load conversation history from session database
   - Retrieve system prompt from session metadata (`__system_prompt__`)
   - Build message array: `[system] + [history messages] + [user prompt]`
   - Send complete context to LLM provider

2. **During Streaming**:
   - Stream events are ephemeral (not persisted)
   - Only used for real-time UI feedback

3. **After Completion**:
   - Persist user message to session database
   - Persist final assistant response (aggregated text) to session database
   - For `agentic()`: also persist tool call messages and results
   - Update session metadata if needed

When `use_session=False`:

1. **Before LLM Request**:
   - No history loaded from session
   - Build message array: `[system (if provided)] + [user prompt]`
   - Send single-turn context to LLM provider

2. **During Streaming**:
   - Same ephemeral behavior (events not persisted)

3. **After Completion**:
   - **Nothing persisted** to session database
   - Response returned to caller only

### System Prompt Storage

System prompts are stored in session metadata with key `"__system_prompt__"`:

```python
# Set system prompt (typically in command initialization)
ctx.session.set_system("You are a helpful coding assistant")

# Later, chat() automatically uses it
result = ctx.llm.chat(
    prompt="Write a function",
    preset="smart",
    use_session=True  # Reads system prompt from session
)

# Override system prompt for one request
result = ctx.llm.chat(
    prompt="Write a function",
    preset="smart",
    system="You are a senior architect",  # Overrides session system
    use_session=True  # Still uses history, but different system prompt
)
```

### Message History Structure

Session database stores messages in order:

```
Message 1: role=system, content="You are a helpful assistant"
Message 2: role=user, content="What is 2+2?"
Message 3: role=assistant, content="2+2 equals 4."
Message 4: role=user, content="What about 3+3?"
Message 5: role=assistant, content="3+3 equals 6."
```

When `chat()` is called with `use_session=True`, all messages are included in the LLM request context for continuity.

## Consequences

### Breaking Changes (v0.3.0)

1. **`ctx.llm.generate()` deleted** - No backward compatibility
2. **Signature changes** - All code using LLM must migrate
3. **Callback requirement** - Streaming/agentic require `on_event`

### Migration Path

**Old (v0.2.x)**:
```python
result = ctx.llm.generate(
    messages=[{"role": "user", "content": "Hello"}],
    preset="smart"
)
```

**New (v0.3.x)**:
```python
result = ctx.llm.chat(
    prompt="Hello",
    preset="smart"
)
```

**With streaming**:
```python
def on_event(event):
    if event["kind"] == "text":
        ctx.output.write(event["delta"])

result = ctx.llm.chat(
    prompt="Hello",
    preset="smart",
    stream=True,
    on_event=on_event
)
```

**Agentic (old)**:
```python
result = ctx.llm.agentic(
    system="You are a helper",
    prompt="Fix bugs",
    tools=[tool1, tool2]
)
```

**Agentic (new)**:
```python
def on_event(event):
    if event["kind"] == "tool_call_start":
        ctx.output.info(f"🔧 {event['tool_name']}")

ctx.session.set_system("You are a helper")
result = ctx.llm.agentic(
    prompt="Fix bugs",
    preset="smart",
    tools=[tool1, tool2],
    on_event=on_event
)
```

### Advantages

1. **Clarity**: Separate APIs for chat vs agentic workflows
2. **Streaming**: Real-time feedback for long-running operations
3. **Visibility**: Tool execution events in agentic mode
4. **Consistency**: Normalized events across all providers
5. **Session-centric**: System prompts managed at session level
6. **Extensibility**: Easy to add new event types or providers

### Disadvantages

1. **Breaking change**: All existing code must migrate
2. **Learning curve**: Users must understand new API surface
3. **Callback requirement**: Streaming requires event handler
4. **Provider research**: OpenAI/Gemini thinking detection TBD

## Alternatives Considered

### Alternative 1: Deprecation Period

**Rejected** - Keep `generate()` with deprecation warnings for 6 months.

**Pros**: Smoother migration, no immediate breakage  
**Cons**: Maintenance burden, unclear timeline, users delay migration

**Decision**: Hard break preferred - clean slate for v0.3.0.

### Alternative 2: Simulated Streaming for Cache

**Rejected** - Simulate streaming delays for cached responses.

**Pros**: Consistent UX (always shows streaming animation)  
**Cons**: Artificial delays waste time, misleading to users

**Decision**: Instant replay for cached responses (no delay).

### Alternative 3: Store Stream Deltas in Session

**Rejected** - Persist every streaming delta to session database.

**Pros**: Full replay capability, detailed history  
**Cons**: Database bloat, performance impact, unnecessary for most use cases

**Decision**: Store only final aggregated messages, streaming events are ephemeral.

### Alternative 4: Generic Thinking Detection

**Rejected** - Try to detect thinking via heuristics (e.g., certain phrases).

**Pros**: Works across all providers  
**Cons**: Unreliable, false positives, misses provider-native features

**Decision**: Provider-specific detection using official APIs.

### Alternative 5: Async/Await in Starlark

**Rejected** - Add async/await primitives to Starlark runtime.

**Pros**: More "native" async pattern  
**Cons**: Complex implementation, Starlark doesn't support coroutines natively

**Decision**: Synchronous callbacks - simpler, works today.

## Implementation Strategy

### Phase 0: Preparation (2-4h)
- Create ADR (this document)
- Finalize API design

### Phase 1: Gateway Foundation (8-12h)
- Define stream event types
- Extend gateway interface
- Implement OpenAI streaming

### Phase 2-3: Supporting Modules (6-9h)
- Session system prompt helpers
- Output TTY detection

### Phase 4: LLM Module Rewrite (16-20h)
- Delete `generate()`
- Implement `chat()` and `agentic()`
- Callback validation

### Phase 5-6: Providers and Helpers (20-26h)
- Implement streaming for all providers
- Create UI helpers library
- Normalize thinking events

### Phase 7: Command Migration (8-12h)
- Migrate 7 commands using `generate()`
- Update 2 commands using `agentic()`

### Phase 8-10: Testing and Documentation (36-48h)
- Unit and integration tests
- Update API reference
- Migration guide
- Performance validation

**Total Estimate**: 96-131 hours (~3-4 weeks)

## Success Metrics

1. All 9 commands successfully migrated
2. Streaming works across 5 providers (OpenAI, Anthropic, Gemini, OpenRouter, Llama)
3. Test coverage ≥75% maintained
4. Thinking events normalized for Anthropic (+ OpenAI/Gemini if supported)
5. Zero regressions in non-streaming use cases
6. Documentation complete with migration guide and examples

## Open Questions

1. **OpenAI Reasoning Format**: What events does o1 emit for reasoning tokens?
   - **Action**: Test with o1-preview streaming to discover format
   
2. **Gemini Thinking Support**: Does Gemini have thinking/reasoning modes?
   - **Action**: Review existing `gemini.go` implementation and docs

3. **Tool Call Thinking**: How should tool "thinking" fields be exposed?
   - **Hypothesis**: Include in `tool_call_start` event arguments
   - **Action**: Verify during implementation

4. **Abort Recovery**: What should happen if stream aborted mid-tool-call?
   - **Decision**: Return partial text, mark tool call as incomplete
   - **Action**: Document in migration guide

5. **Cache Key Strategy**: How to cache streaming vs non-streaming responses?
   - **Hypothesis**: Same cache key, instant replay for cached streams
   - **Action**: Verify in caching gateway implementation

## References

- [Anthropic Extended Thinking Docs](https://docs.anthropic.com/claude/docs/extended-thinking)
- [OpenAI Streaming Guide](https://platform.openai.com/docs/api-reference/streaming)
- [Implementation Plan](IMPLEMENTATION_PLAN.md)

## Decision Log

- **2026-02-16**: ADR created, decision proposed
- **Next review**: After Phase 0 completion and stakeholder feedback
