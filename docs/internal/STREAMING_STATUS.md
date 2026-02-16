# Streaming LLM API Redesign - Status Tracker

**Last Updated**: February 16, 2026  
**Status**: Planning Complete, Ready for Implementation  
**Branch**: `feature/streaming-llm-api-implementation`

## Current Progress

### ✅ Completed
- [x] Design finalized (5 key decisions confirmed)
- [x] API signatures defined (chat, agentic, embed)
- [x] Stream event types designed
- [x] Provider research started (Anthropic confirmed, OpenAI/Gemini pending)
- [x] Command audit completed (7 generate + 2 agentic)
- [x] Implementation plan created (10 phases, 589 lines)
- [x] Branch created and checked out

### 🚧 In Progress
- [ ] None (ready to start Phase 0)

### 📋 Next Session Tasks (Priority Order)

1. **Phase 0 - Preparation** (8-12h)
   - [ ] Create `docs/decisions/ADR-001-streaming-api.md`
   - [ ] Create `docs/migration/v0.3-streaming.md`
   - [ ] Create `docs/internal/llm-usage-audit.md`

2. **Provider Research**
   - [ ] Test OpenAI o1 streaming to discover reasoning event format
   - [ ] Review `internal/adapters/gateway/gemini.go` for streaming structure
   - [ ] Document findings in implementation plan

3. **Phase 1 - Gateway Foundation** (8-12h)
   - [ ] Add `StreamEvent` types to `internal/domain/gateway/types.go`
   - [ ] Extend `GenerationGateway` interface in `internal/ports/types.go`
   - [ ] Implement OpenAI streaming in `internal/adapters/gateway/openai.go`

## Quick Reference

### Important Files
- **Implementation Plan**: `docs/internal/STREAMING_IMPLEMENTATION_PLAN.md` (589 lines)
- **Status Tracker**: `docs/internal/STREAMING_STATUS.md` (this file)

### Key Decisions
1. Hard break backward compatibility (delete `generate()`)
2. Instant cache replay (no simulated streaming)
3. Store final messages only (ephemeral stream events)
4. Provider-specific thinking detection
5. Return partial on abort

### API Surface
```python
ctx.llm.chat(prompt, preset="smart", system=None, use_session=True, stream=False, on_event=None) → str
ctx.llm.agentic(prompt, tools, on_event, preset="smart", system=None, use_session=True, max_iterations=50) → str
ctx.llm.embed(texts, preset="fast") → list[list[float]]
ctx.session.set_system(prompt)
ctx.session.get_system() → str
ctx.output.is_tty() → bool
```

### Event Types
- `text` - Text content delta
- `thinking` - Thinking/reasoning delta (provider-specific)
- `usage` - Token usage update
- `done` - Stream complete
- `error` - Error occurred
- `tool_call_start` - Tool execution started (agentic only)
- `tool_call_end` - Tool execution completed (agentic only)
- `tool_call_error` - Tool execution failed (agentic only)

### Commands to Migrate
**Streaming** (2): `write.star`, `code.star`  
**Non-streaming** (5): `commit.star`, `pr.star`, `search.star`, `extract.star`, `orchestrator-agent.star` (partial)  
**Agentic Update** (2): `orchestrator-agent.star`, `review-agent.star`

## Estimates

- **Total Effort**: 102-139 hours (~3-4 weeks)
- **Phase 0**: 8-12h (docs, ADR, audit)
- **Phase 1**: 8-12h (gateway foundation)
- **Phase 2**: 4-6h (session module)
- **Phase 3**: 2-3h (output module)
- **Phase 4**: 16-20h (LLM module rewrite)
- **Phase 5**: 4-6h (UI helpers)
- **Phase 6**: 16-20h (provider implementations)
- **Phase 7**: 8-12h (command migrations)
- **Phase 8**: 16-20h (testing)
- **Phase 9**: 12-16h (documentation)
- **Phase 10**: 8-12h (performance validation)

## Notes for Next Session

- Anthropic thinking detection is well-documented and confirmed
- OpenAI o1/o3 reasoning format needs testing to reverse-engineer
- Gemini thinking/reasoning support is unknown, need to check existing impl
- Tool calls can have "thinking" or "description" blocks (user reminder)
- Remember: streaming events are ephemeral, only final messages stored
- Cache should replay instantly, not simulate streaming delays
