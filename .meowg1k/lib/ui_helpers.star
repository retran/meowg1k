"""
UI Helpers Library for meowg1k Streaming

Reusable stream event handlers for use with ctx.llm.chat() and ctx.llm.agent_turn()
when stream=True.

## Quick Start

```python
load("//lib/ui_helpers.star", "make_markdown_stream_handler",
                              "make_plain_stream_handler",
                              "make_agentic_stream_handler")

def handler(ctx):
    # Stream LLM response rendered as markdown (live preview via TUI on TTY)
    on_event = make_markdown_stream_handler(ctx)
    result = ctx.llm.chat(
        prompt="Explain Go interfaces",
        preset="smart",
        stream=True,
        on_event=on_event,
    )
    ctx.output.writeline(result)  # Write final result to output buffer

    # Stream agent turn with tool event feedback
    on_event = make_agentic_stream_handler(ctx)
    result = ctx.llm.agent_turn(
        prompt="List files in src/",
        preset="smart",
        tools=[...],
        stream=True,
        on_event=on_event,
    )
    ctx.output.writeline(result)
```

## Available Handlers

- `make_markdown_stream_handler(ctx)` - Streams LLM text as live TUI preview via ctx.ui.stream()
- `make_plain_stream_handler(ctx)` - Streams raw text to output without rendering
- `make_agentic_stream_handler(ctx, abort_on_error=False, max_errors=3)` - Full handler
  for agent_turn: live preview, tool call progress, error handling

## Stream Event Kinds

Handlers receive dicts with the following shapes:

| kind              | extra fields                                              |
| ----------------- | --------------------------------------------------------- |
| text              | delta (str)                                               |
| thinking          | delta (str)                                               |
| usage             | usage {prompt, completion, total}                         |
| done              | usage {prompt, completion, total} (optional)              |
| error             | error (str), recoverable (bool)                           |
| tool_call_start   | tool_name, tool_id, arguments                             |
| tool_call_end     | tool_name, tool_id, duration_ms, arguments                |
| tool_call_error   | tool_name, tool_id, error, duration_ms, arguments         |

## Design

- ctx.ui.stream(delta, done) — live TUI preview on TTY, no-op on non-TTY
- ctx.output.writeline(result) — always written; goes to stdout buffer
- Scripts must explicitly write the return value of ctx.llm.chat() to ctx.output
"""

# ==============================================================================
# make_markdown_stream_handler
# ==============================================================================

def make_markdown_stream_handler(ctx):
    """Create an on_event callback that streams LLM text as live TUI preview.

    Token deltas are forwarded to ctx.ui.stream() for live display on TTY.
    On non-TTY, ctx.ui.stream() is a no-op.  The final result is NOT written
    here — the caller must write ctx.llm.chat()'s return value to ctx.output.

    Args:
        ctx: Handler context (provides ctx.ui).

    Returns:
        A callable suitable for passing as on_event= to ctx.llm.chat() or
        ctx.llm.agent_turn().

    Example:
        on_event = make_markdown_stream_handler(ctx)
        result = ctx.llm.chat(prompt="...", preset="smart",
                              stream=True, on_event=on_event)
        ctx.output.writeline(result)
    """
    def _on_event(event):
        kind = event.get("kind", "")

        if kind == "text":
            delta = event.get("delta", "")
            if delta:
                ctx.ui.stream(delta)

        elif kind == "done":
            ctx.ui.stream("", done=True)

        elif kind == "error":
            msg = event.get("error", "unknown error")
            recoverable = event.get("recoverable", False)
            if recoverable:
                ctx.ui.warn("Stream warning: " + msg)
            else:
                ctx.ui.error("Stream error: " + msg)

        # Ignore thinking / usage / tool_call_* for this simple handler

    return _on_event

# ==============================================================================
# make_plain_stream_handler
# ==============================================================================

def make_plain_stream_handler(ctx):
    """Create an on_event callback that writes streaming LLM text as plain text.

    Text deltas are written directly to ctx.output.write() without any
    rendering.  Useful for script-friendly output or when piping to another
    process.  No live TUI preview.

    Args:
        ctx: Handler context (provides ctx.output and ctx.ui).

    Returns:
        A callable suitable for passing as on_event= to ctx.llm.chat() or
        ctx.llm.agent_turn().

    Example:
        on_event = make_plain_stream_handler(ctx)
        result = ctx.llm.chat(prompt="...", preset="smart",
                              stream=True, on_event=on_event)
        # result is already written incrementally; no need to write again
    """
    def _on_event(event):
        kind = event.get("kind", "")

        if kind == "text":
            delta = event.get("delta", "")
            if delta:
                ctx.output.write(delta)

        elif kind == "error":
            msg = event.get("error", "unknown error")
            recoverable = event.get("recoverable", False)
            if recoverable:
                ctx.ui.warn("Stream warning: " + msg)
            else:
                ctx.ui.error("Stream error: " + msg)

        # Ignore thinking / usage / done / tool_call_* for this simple handler

    return _on_event

# ==============================================================================
# make_agentic_stream_handler
# ==============================================================================

def make_agentic_stream_handler(ctx, abort_on_error=False, max_errors=3):
    """Create a full-featured on_event callback for ctx.llm.agent_turn().

    Handles all stream event kinds:
    - text: live TUI preview via ctx.ui.stream()
    - thinking: shown as a dimmed info message (if TTY)
    - tool_call_start: prints an activity indicator with the tool name
    - tool_call_end: marks the activity as successful with duration
    - tool_call_error: marks the activity as failed; errors counted against
      max_errors; raises StarlarkError when limit exceeded (if abort_on_error)
    - usage: shown via "done" event totals only
    - done: seals the TUI stream block
    - error: shows warning or error based on recoverable flag

    The caller must write the return value of ctx.llm.agent_turn() to
    ctx.output explicitly after the call returns.

    Args:
        ctx: Handler context.
        abort_on_error (bool): If True, raise an error when tool or stream
            errors exceed max_errors. Default False.
        max_errors (int): Maximum tolerated errors before aborting. Default 3.

    Returns:
        A callable suitable for passing as on_event= to ctx.llm.agent_turn().

    Example:
        on_event = make_agentic_stream_handler(ctx, abort_on_error=True, max_errors=2)
        result = ctx.llm.agent_turn(
            prompt="...",
            preset="smart",
            tools=my_tools,
            stream=True,
            on_event=on_event,
        )
        ctx.output.writeline(result)
    """
    # Mutable state carried in a dict (Starlark dicts are not frozen)
    state = {
        "error_count": 0,
        "activity": None,
        "in_stream": False,
    }

    def _finish_stream():
        if state["in_stream"]:
            ctx.ui.stream("", done=True)
            state["in_stream"] = False

    def _on_event(event):
        kind = event.get("kind", "")

        if kind == "text":
            delta = event.get("delta", "")
            if delta:
                if not state["in_stream"]:
                    state["in_stream"] = True
                ctx.ui.stream(delta)

        elif kind == "thinking":
            delta = event.get("delta", "")
            if delta:
                ctx.ui.info("[thinking] " + delta)

        elif kind == "tool_call_start":
            _finish_stream()
            tool_name = event.get("tool_name", "unknown")
            state["activity"] = ctx.ui.activity("Calling " + tool_name + "...")

        elif kind == "tool_call_end":
            if state["activity"] != None:
                tool_name = event.get("tool_name", "unknown")
                dur = event.get("duration_ms", 0)
                state["activity"].success(tool_name + " completed (" + str(dur) + "ms)")
                state["activity"] = None

        elif kind == "tool_call_error":
            if state["activity"] != None:
                tool_name = event.get("tool_name", "unknown")
                err = event.get("error", "unknown error")
                dur = event.get("duration_ms", 0)
                state["activity"].fail(tool_name + " failed: " + err + " (" + str(dur) + "ms)")
                state["activity"] = None

            state["error_count"] = state["error_count"] + 1
            if abort_on_error and state["error_count"] >= max_errors:
                fail("Too many tool errors (%d), aborting" % state["error_count"])

        elif kind == "done":
            _finish_stream()
            usage = event.get("usage", {})
            total = usage.get("total", 0)
            if total > 0:
                ctx.ui.info("Tokens used: %d" % total)

        elif kind == "error":
            msg = event.get("error", "unknown error")
            recoverable = event.get("recoverable", False)
            if recoverable:
                ctx.ui.warn("Stream warning: " + msg)
            else:
                ctx.ui.error("Stream error: " + msg)
                if abort_on_error:
                    state["error_count"] = state["error_count"] + 1
                    if state["error_count"] >= max_errors:
                        fail("Too many stream errors (%d), aborting" % state["error_count"])

        elif kind == "usage":
            # usage events are intermediate; shown via "done" event totals only
            pass

    return _on_event
