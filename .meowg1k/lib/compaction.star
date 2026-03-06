"""
Compaction Library for meowg1k

Context compaction: when an agentic session grows long, the full message history
is fed back to the LLM on every iteration, consuming tokens and approaching
context-window limits. Compaction solves this by:

  1. Fetching all current session events
  2. Asking the LLM to produce a dense summary that preserves every decision,
     file change, tool result, and unresolved task
  3. Marking all old events obsolete (they are excluded from future history loads)
  4. Inserting the summary as a system event so the LLM receives it as context
     on the next iteration

The agentic loop in loadSessionHistory() (module_llm.go) already calls
GetAllEvents, which skips obsolete events. The injected system summary
is NOT converted to a message (EventTypeSystem is ignored in the message
replay), so the summary must be injected as a user or assistant message,
or the caller must prepend it to the next prompt. We inject it as a
special "system" event via insert_summary, which IS included in the
next context load as a gateway.Message with role=system — that is the
right place for a compact summary.

Wait — checking module_llm.go loadSessionHistory:
  EventTypeSystem → "System events are not replayed as messages"

So insert_summary events are silently dropped from the message list.
Instead we insert the summary as an assistant_message event so it
is included in history replay and the LLM sees it as prior context.

We work around this by calling ctx.session.insert_summary() to persist
it (for audit), and then returning the summary text so the caller can
prepend it to the next agent_turn prompt.

## Usage

```python
load("//lib/compaction.star", "maybe_compact", "compact_now")

def handler(ctx):
    turn = ctx.ui.assistant_turn()
    on_event = make_agentic_stream_handler(turn)

    # Compact automatically when event count exceeds threshold
    compact_summary = maybe_compact(ctx, preset="fast", threshold=80)

    result = ctx.llm.agent_turn(
        prompt=build_prompt(task, compact_summary),
        preset="smart",
        tools=my_tools,
        stream=True,
        on_event=on_event,
        max_iterations=50,
        on_tool_error="return",
    )
    turn.done()
    ctx.output.writeline(result)
```

## API

### compact_now(ctx, preset, instructions="")

Unconditionally compact the current session history.

Args:
    ctx:           Handler context.
    preset:        LLM preset to use for summarisation (recommend "fast").
    instructions:  Extra instructions appended to the summarisation prompt.

Returns:
    string — the summary text, or "" if there were no events to compact.

### maybe_compact(ctx, preset, threshold=80, instructions="")

Compact only when the event count reaches the threshold.

Args:
    ctx:          Handler context.
    preset:       LLM preset for summarisation.
    threshold:    Compact when event count >= this value (default 80).
    instructions: Extra instructions for the summarisation prompt.

Returns:
    string — summary text if compaction ran, otherwise "".

### count_events(ctx)

Return the current number of non-obsolete events in the session.
"""

# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

_DEFAULT_COMPACT_SYSTEM = """You are a session historian for an AI coding agent.

Your job: produce a **dense, lossless summary** of the conversation so far that
lets the agent continue its work without re-reading the full history.

Rules:
- Preserve every file that was read or written (name + what changed / was found).
- Preserve every shell command run and its outcome (pass/fail, key output).
- Preserve every error encountered and whether it was resolved.
- Preserve the current state of any open tasks or sub-goals.
- Preserve decisions made and the reasoning behind them.
- Use bullet points. Be terse but complete. No waffle.
- Do NOT omit tool results just because they seemed unimportant.
- Format: markdown, starting with "## Session Summary"
"""

def _build_compact_prompt(events, instructions):
    """Build the summarisation prompt from a list of event dicts."""
    lines = ["Summarise the following session history.\n"]
    if instructions:
        lines.append("Additional instructions: " + instructions + "\n")
    lines.append("---\n")

    for e in events:
        etype = e.get("type", "unknown")
        content = e.get("content", "")
        if etype == "user_message":
            lines.append("[USER] " + content)
        elif etype == "assistant_message":
            lines.append("[ASSISTANT] " + content)
        elif etype == "tool_result":
            tool_call_id = e.get("tool_call_id", "")
            if tool_call_id:
                lines.append("[TOOL:" + tool_call_id + "] " + content)
            else:
                lines.append("[TOOL] " + content)
        elif etype == "system":
            lines.append("[SYSTEM] " + content)
        else:
            lines.append("[" + etype.upper() + "] " + content)

    return "\n".join(lines)

# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def count_events(ctx):
    """Return the current number of session events."""
    events = ctx.session.get_events(limit=10000)
    return len(events)

def compact_now(ctx, preset, instructions=""):
    """Compact the session history unconditionally.

    Fetches all events, asks the LLM to summarise, marks them all obsolete,
    and inserts a system summary event. Returns the summary text so the
    caller can prepend it to the next agent_turn prompt.

    Args:
        ctx:          Handler context.
        preset:       LLM preset to use for summarisation.
        instructions: Extra instructions for the summarisation prompt.

    Returns:
        str — summary text, or "" if there were no events to compact.
    """
    events = ctx.session.get_events(limit=10000)
    if len(events) == 0:
        return ""

    prompt = _build_compact_prompt(events, instructions)

    summary = ctx.llm.chat(
        prompt=prompt,
        preset=preset,
        system=_DEFAULT_COMPACT_SYSTEM,
        use_session=False,  # Don't add this call to the session history itself
    )

    # Mark all current events obsolete
    event_ids = [e.get("id", "") for e in events if e.get("id", "") != ""]
    if event_ids:
        ctx.session.mark_obsolete(event_ids)

    # Insert the summary as a system event for audit purposes.
    # Note: system events are NOT replayed into the message list by
    # loadSessionHistory, so we return the summary text for the caller
    # to prepend to the next prompt instead.
    if event_ids:
        last_id = event_ids[len(event_ids) - 1]
        ctx.session.insert_summary(
            after_event_id=last_id,
            content=summary,
        )

    return summary

def maybe_compact(ctx, preset, threshold=80, instructions=""):
    """Compact the session history if the event count meets the threshold.

    Args:
        ctx:          Handler context.
        preset:       LLM preset for summarisation.
        threshold:    Run compaction when event count >= this value (default 80).
        instructions: Extra instructions for the summarisation prompt.

    Returns:
        str — summary text if compaction ran, otherwise "".
    """
    n = count_events(ctx)
    if n >= threshold:
        return compact_now(ctx, preset=preset, instructions=instructions)
    return ""
