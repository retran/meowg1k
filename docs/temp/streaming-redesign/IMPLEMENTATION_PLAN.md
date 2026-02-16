# Streaming LLM API Implementation Plan

**Branch**: `feature/streaming-llm-api-implementation`  
**Target**: v0.3.0 (Breaking Change)  
**Estimate**: 96-131 hours (~3-4 weeks)

## Quick Reference

See detailed design decisions in: `docs/temp/streaming-redesign/ADR-001-streaming-api.md`

### New API Surface

```python
# Chat primitive
ctx.llm.chat(prompt, preset, system=None, use_session=True, stream=False, on_event=None) → str

# Agentic primitive
ctx.llm.agentic(prompt, preset, tools, system=None, use_session=True, stream=False, on_event=None, max_iterations=50) → str

# Embed unchanged
ctx.llm.embed(texts, preset) → list[list[float]]

# Session helpers
ctx.session.set_system(prompt)
ctx.session.get_system() → str

# Output helper
ctx.output.is_tty() → bool
```

**Note**: `preset` parameter has **NO DEFAULT** - must be explicitly provided in all APIs.

## Phase 0: Preparation (2-4h)

**Goal**: Finalize API design

### Tasks
- [x] Create ADR-001 (`docs/temp/streaming-redesign/ADR-001-streaming-api.md`)
- [x] Fix API parameter ordering (prompt, preset, tools, optional params)
- [x] Add `stream` parameter to `agentic()`
- [x] Remove default values from `preset` parameter

## Phase 1: Gateway Streaming Foundation (8-12h)

**Goal**: Add streaming support to gateway layer

### Tasks
- [ ] Define `StreamEvent` and `StreamEventKind` in `internal/domain/gateway/types.go`
- [ ] Add `StreamCallback` type
- [ ] Extend `GenerationGateway` interface with `GenerateContentStream()` in `internal/ports/types.go`
- [ ] Implement streaming in `internal/adapters/gateway/openai.go`
- [ ] Update caching gateway (`internal/adapters/gateway/caching.go`) for instant replay
- [ ] Update logging gateway (`internal/adapters/gateway/logging.go`) to log stream events
- [ ] Add unit tests for stream events

**Key Types**:
```go
type StreamEventKind int
const (
    StreamEventText
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
    Delta      string
    Usage      *UsageMetadata
    Error      string
    Recoverable bool
    ToolName   string
    ToolID     string
    Arguments  map[string]interface{}
    DurationMS int64
}

type StreamCallback func(event StreamEvent) error
```

## Phase 2: Session Module Enhancement (4-6h)

**Goal**: Add system prompt management to session

### Tasks
- [ ] Add `set_system()` method to `internal/core/starlark/module_session.go`
- [ ] Add `get_system()` method to `internal/core/starlark/module_session.go`
- [ ] Store system prompt in session metadata with key `"__system_prompt__"`
- [ ] Add unit tests for session system prompt methods

## Phase 3: Output Module Enhancement (2-3h)

**Goal**: Add TTY detection helper

### Tasks
- [ ] Add `IsTTY()` method to `internal/adapters/output/service.go`
- [ ] Expose as `is_tty()` in `internal/core/starlark/module_output.go`
- [ ] Add unit test for TTY detection

## Phase 4: LLM Module Complete Rewrite (16-20h)

**Goal**: Replace `generate()` with `chat()` and `agentic()`

### Tasks
- [ ] Delete `llmGenerate()` function from `internal/core/starlark/module_llm.go`
- [ ] Implement `llmChat()` with streaming support
- [ ] Implement `llmAgentic()` with tool execution
- [ ] Add callback validation (required for streaming/agentic)
- [ ] Integrate with session system prompt
- [ ] Handle streaming event emission from Go → Starlark callback
- [ ] Ensure final aggregated text is returned
- [ ] Complete rewrite of `internal/core/starlark/module_llm_test.go`

**Critical Logic**:
- `chat()`: `on_event` required if `stream=True`, otherwise optional
- `agentic()`: `on_event` always required (for tool events)
- Both return final aggregated string
- Stream events are ephemeral (not stored in session)
- Final messages stored in session

## Phase 5: UI Helpers Library (4-6h)

**Goal**: Create reusable streaming handlers

### Tasks
- [ ] Create `.meowg1k/lib/ui_helpers.star`
- [ ] Implement `make_markdown_stream_handler(ctx)`
- [ ] Implement `make_plain_stream_handler(ctx)`
- [ ] Implement `make_agentic_stream_handler(ctx, abort_on_error, max_errors)`
- [ ] Add usage examples in comments

## Phase 6: Provider Implementations (16-20h)

**Goal**: Implement streaming for all providers

### Tasks
- [ ] **Anthropic** (`internal/adapters/gateway/anthropic.go`)
  - [ ] Implement `GenerateContentStream()`
  - [ ] Detect `thinking_delta` events → `StreamEventThinking`
  - [ ] Add tests for thinking detection
- [ ] **Gemini** (`internal/adapters/gateway/gemini.go`)
  - [ ] Review existing streaming implementation
  - [ ] Implement `GenerateContentStream()`
  - [ ] Research thinking/reasoning support
  - [ ] Add tests
- [ ] **OpenRouter** (`internal/adapters/gateway/openrouter.go`)
  - [ ] Implement `GenerateContentStream()`
  - [ ] Add tests
- [ ] **Llama** (`internal/adapters/gateway/llama.go`)
  - [ ] Implement `GenerateContentStream()`
  - [ ] Add tests
- [ ] **OpenAI** (already done in Phase 1, but enhance)
  - [ ] Test with o1-preview to discover reasoning event format
  - [ ] Add thinking detection if supported
  - [ ] Document findings

## Phase 7: Command Migrations (8-12h)

**Goal**: Migrate all commands to new API

### Tasks
- [ ] **write.star** - Migrate to `chat()` with streaming
- [ ] **code.star** - Migrate to `chat()` with streaming
- [ ] **commit.star** - Migrate to `chat()` without streaming
- [ ] **pr.star** - Migrate to `chat()` without streaming
- [ ] **search.star** - Migrate to `chat()` without streaming
- [ ] **extract.star** - Migrate to `chat()` without streaming
- [ ] **orchestrator-agent.star** - Update `agentic()` signature, migrate `generate()` calls
- [ ] **review-agent.star** - Update `agentic()` signature

## Phase 8: Testing & Validation (16-20h)

**Goal**: Comprehensive test coverage

### Tasks
- [ ] Unit tests for all gateway streaming implementations
- [ ] Unit tests for LLM module (`module_llm_test.go`)
- [ ] Unit tests for session module (`module_session_test.go`)
- [ ] Integration tests (`tests/integration/streaming_test.go`)
  - [ ] End-to-end streaming test
  - [ ] Cache replay test
  - [ ] Error handling test
  - [ ] Abort behavior test
- [ ] Manual testing of all 9 migrated commands
- [ ] Verify 75%+ test coverage maintained

## Phase 9: Documentation (12-16h)

**Goal**: Update all public documentation

### Tasks
- [ ] Update `docs/api/API_REFERENCE.md` - Complete LLM section rewrite
- [ ] Update `docs/guides/starlark-system.md` - Add streaming patterns
- [ ] Finalize migration guide (`docs/temp/streaming-redesign/MIGRATION.md`)
- [ ] Move ADR to permanent location (`docs/decisions/ADR-001-streaming-api.md`)
- [ ] Add CHANGELOG entry for v0.3.0
- [ ] Update README examples (if applicable)

## Phase 10: Performance & Benchmarks (8-12h)

**Goal**: Validate performance and UX

### Tasks
- [ ] Create benchmarks (`tests/benchmarks/streaming_bench_test.go`)
  - [ ] Streaming vs non-streaming overhead
  - [ ] Cache replay performance
  - [ ] Event emission throughput
- [ ] Validate TTY streaming UX (markdown rendering)
- [ ] Measure provider-specific thinking detection overhead
- [ ] Profile memory usage for streaming

## Rollout Checklist

Before merging to `dev`:

- [ ] All 10 phases complete
- [ ] All tests passing (75%+ coverage)
- [ ] All 9 commands migrated and tested
- [ ] Documentation complete and reviewed
- [ ] Performance validated
- [ ] ADR finalized and moved to `docs/decisions/`
- [ ] Migration guide complete
- [ ] CHANGELOG updated
- [ ] Remove temp directory (`docs/temp/streaming-redesign/`)

## Known Research Gaps

### OpenAI Reasoning (o1/o3 models)
- [ ] Test o1-preview streaming to discover reasoning event format
- [ ] Document findings in ADR
- [ ] Implement detection if reasoning tokens exposed

### Gemini Thinking
- [ ] Review `internal/adapters/gateway/gemini.go` for streaming structure
- [ ] Check if Gemini supports thinking/reasoning modes
- [ ] Document findings in ADR

### Tool Call Thinking
- [ ] Verify how tool "thinking"/"description" fields are exposed
- [ ] Decide on event structure (include in `tool_call_start` arguments?)
- [ ] Document in ADR

## Notes

- All stream events are **ephemeral** (not stored in session database)
- Only final aggregated messages are persisted
- Cached responses replay **instantly** (no simulated delay)
- Abort mid-stream returns **partial output** (no error)
- Providers without thinking support never emit `StreamEventThinking`

## References

- **ADR**: `docs/temp/streaming-redesign/ADR-001-streaming-api.md`
