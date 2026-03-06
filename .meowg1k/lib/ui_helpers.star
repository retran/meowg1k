"""
UI Helpers Library for meowg1k Streaming

Reusable stream event handlers for use with ctx.llm.chat() and ctx.llm.agent_turn()
when stream=True.

The helpers now require a turn handle (from ctx.ui.assistant_turn()) so they can
stream tokens and report tool progress via the turn-based API.

## Quick Start

```python
load("//lib/ui_helpers.star", "make_markdown_stream_handler",
                              "make_plain_stream_handler",
                              "make_agentic_stream_handler")

def handler(ctx):
    ctx.ui.user_turn(prompt)
    turn = ctx.ui.assistant_turn()

    # Stream LLM response rendered as markdown (live preview via TUI on TTY)
    on_event = make_markdown_stream_handler(turn)
    result = ctx.llm.chat(
        prompt="Explain Go interfaces",
        preset="smart",
        stream=True,
        on_event=on_event,
    )
    turn.done()
    ctx.output.writeline(result)  # Write final result to output buffer

    # Stream agent turn with tool event feedback
    turn2 = ctx.ui.assistant_turn()
    on_event = make_agentic_stream_handler(turn2)
    result = ctx.llm.agent_turn(
        prompt="List files in src/",
        preset="smart",
        tools=[...],
        stream=True,
        on_event=on_event,
    )
    turn2.done()
    ctx.output.writeline(result)
```

## Available Handlers

- `make_markdown_stream_handler(turn)` - Streams LLM text as live TUI preview via turn.stream()
- `make_plain_stream_handler(turn)` - Streams raw text (turn used for error reporting only)
- `make_agentic_stream_handler(turn, abort_on_error=False, max_errors=3)` - Full handler
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

- turn.stream(delta, done) — live TUI preview on TTY, no-op on non-TTY
- ctx.output.writeline(result) — always written; goes to stdout buffer
- Scripts must explicitly write the return value of ctx.llm.chat() to ctx.output
"""

# ==============================================================================
# make_markdown_stream_handler
# ==============================================================================

def make_markdown_stream_handler(turn):
    """Create an on_event callback that streams LLM text as live TUI preview.

    Token deltas are forwarded to turn.stream() for live display on TTY.
    On non-TTY, turn.stream() is a no-op.  The final result is NOT written
    here — the caller must write ctx.llm.chat()'s return value to ctx.output.

    Args:
        turn: TurnHandle from ctx.ui.assistant_turn().

    Returns:
        A callable suitable for passing as on_event= to ctx.llm.chat() or
        ctx.llm.agent_turn().

    Example:
        turn = ctx.ui.assistant_turn()
        on_event = make_markdown_stream_handler(turn)
        result = ctx.llm.chat(prompt="...", preset="smart",
                              stream=True, on_event=on_event)
        turn.done()
        ctx.output.writeline(result)
    """
    def _on_event(event):
        kind = event.get("kind", "")

        if kind == "text":
            delta = event.get("delta", "")
            if delta:
                turn.stream(delta)

        elif kind == "done":
            turn.stream("", done=True)

        elif kind == "error":
            msg = event.get("error", "unknown error")
            recoverable = event.get("recoverable", False)
            if recoverable:
                turn.warn("Stream warning: " + msg)
            else:
                turn.warn("Stream error: " + msg)
                turn.stream("", done=True)

        # Ignore thinking / usage / tool_call_* for this simple handler

    return _on_event

# ==============================================================================
# make_plain_stream_handler
# ==============================================================================

def make_plain_stream_handler(turn):
    """Create an on_event callback that writes streaming LLM text as plain text.

    Text deltas are forwarded to turn.stream() without markdown rendering.
    Useful for script-friendly output or when piping to another process.

    Args:
        turn: TurnHandle from ctx.ui.assistant_turn().

    Returns:
        A callable suitable for passing as on_event= to ctx.llm.chat() or
        ctx.llm.agent_turn().

    Example:
        turn = ctx.ui.assistant_turn()
        on_event = make_plain_stream_handler(turn)
        result = ctx.llm.chat(prompt="...", preset="smart",
                              stream=True, on_event=on_event)
        turn.done()
    """
    def _on_event(event):
        kind = event.get("kind", "")

        if kind == "text":
            delta = event.get("delta", "")
            if delta:
                turn.stream(delta)

        elif kind == "done":
            turn.stream("", done=True)

        elif kind == "error":
            msg = event.get("error", "unknown error")
            recoverable = event.get("recoverable", False)
            if recoverable:
                turn.warn("Stream warning: " + msg)
            else:
                turn.warn("Stream error: " + msg)
                turn.stream("", done=True)

        # Ignore thinking / usage / done / tool_call_* for this simple handler

    return _on_event

# ==============================================================================
# _tool_step_label
# ==============================================================================

def _short(s, n):
    """Truncate string s to at most n chars, appending '...' if cut."""
    s = str(s)
    return s if len(s) <= n else s[:n - 3] + "..."

def _tool_step_label(tool_name, args):
    """Build a human-friendly step label from a tool name and its arguments dict.

    Surfaces the most meaningful argument(s) so the user can see exactly what
    the agent is doing at each step, including key parameter values.

    Args:
        tool_name (str): The tool being called (e.g. "file_reader").
        args (dict): The tool's arguments dict from the tool_call_start event.

    Returns:
        str: A short descriptive label suitable for turn.step().
    """
    if tool_name == "file_reader":
        path = args.get("path", "")
        if path:
            return "Reading " + _short(path, 60)
    elif tool_name == "file_writer":
        path = args.get("path", "")
        if path:
            return "Writing " + _short(path, 60)
    elif tool_name == "file_editor":
        path = args.get("path", "")
        if path:
            return "Editing " + _short(path, 60)
    elif tool_name == "edit_file":
        path = args.get("path", "")
        if path:
            return "Editing " + _short(path, 60)
    elif tool_name == "replace_text":
        path = args.get("path", "")
        if path:
            return "Replacing in " + _short(path, 50)
    elif tool_name == "file_exists":
        path = args.get("path", "")
        if path:
            return "Checking " + _short(path, 60)
    elif tool_name == "list_directory":
        path = args.get("path", ".")
        pattern = args.get("pattern", "*")
        if pattern and pattern != "*":
            return "Listing " + _short(path, 40) + " [" + _short(pattern, 20) + "]"
        return "Listing " + _short(path, 60)
    elif tool_name == "search_text":
        pattern = args.get("pattern", "")
        path = args.get("path", ".")
        if pattern:
            return "Searching " + _short(path, 30) + ": " + _short(pattern, 30)
    elif tool_name == "shell_exec":
        command = args.get("command", "")
        if command:
            return "Running " + _short(command, 60)
    elif tool_name == "git_status":
        return "Git status"
    elif tool_name == "git_diff":
        staged = args.get("staged", False)
        if staged:
            return "Git diff (staged)"
        return "Git diff"
    elif tool_name == "web_search" or tool_name == "search":
        query = args.get("query", "")
        if query:
            return "Searching: " + _short(query, 55)
    elif tool_name == "code_search" or tool_name == "index_search":
        query = args.get("query", "")
        if query:
            return "Searching code: " + _short(query, 48)
    elif tool_name == "save_context":
        key = args.get("key", "")
        if key:
            return "Saving context: " + _short(key, 48)
    elif tool_name == "recall_context":
        key = args.get("key", "")
        if key:
            return "Recalling: " + _short(key, 52)
    elif tool_name == "list_context":
        return "Listing context"
    elif tool_name == "summarize_history":
        return "Summarizing history"
    elif tool_name == "get_session_info":
        return "Session info"

    # Generic fallback: show all key=value pairs up to ~80 chars total
    if args:
        params = ""
        for k in args:
            v = args[k]
            if params:
                params = params + ", "
            params = params + k + "=" + _short(v, 30)
        label = tool_name + "(" + params + ")"
        return _short(label, 80)

    return tool_name + "()"

# ==============================================================================
# make_agentic_stream_handler
# ==============================================================================

def make_agentic_stream_handler(turn, abort_on_error=False, max_errors=3):
    """Create a full-featured on_event callback for ctx.llm.agent_turn().

    Tool calls appear as steps on the turn. LLM text is streamed directly on
    the turn. No subturn/iteration nesting — just a flat log of thoughts and
    tool calls in the order they actually happen.

    Events handled:
    - iteration_start: ignored (no subturn labels)
    - iteration_end: ignored
    - tool_call_start: opens a step on the turn
    - tool_call_end: marks the step done with duration
    - tool_call_error: marks the step failed; counted against max_errors
    - text: live markdown streaming on the turn
    - thinking: ignored (not shown)
    - done: seals the stream; shows token total if available
    - error: warn or fail based on recoverable flag
    - usage: ignored (totals shown via done event)

    Args:
        turn: TurnHandle from ctx.ui.assistant_turn().
        abort_on_error (bool): If True, raise when tool errors exceed max_errors.
        max_errors (int): Maximum tolerated errors before aborting. Default 3.

    Returns:
        A callable suitable for passing as on_event= to ctx.llm.agent_turn().
    """
    state = {
        "error_count": 0,
        "step": None,
        "stream_started": False,
    }

    def _finish_stream():
        if state["stream_started"]:
            turn.stream("", done=True)
            state["stream_started"] = False

    def _on_event(event):
        kind = event.get("kind", "")

        if kind == "iteration_start" or kind == "iteration_end":
            pass

        elif kind == "text":
            delta = event.get("delta", "")
            if delta:
                state["stream_started"] = True
                turn.stream(delta)

        elif kind == "thinking":
            pass

        elif kind == "tool_call_start":
            _finish_stream()
            tool_name = event.get("tool_name", "unknown")
            args = event.get("arguments", {})
            label = _tool_step_label(tool_name, args)
            state["step"] = turn.step(label)

        elif kind == "tool_call_end":
            if state["step"] != None:
                state["step"].done()
                state["step"] = None

        elif kind == "tool_call_error":
            if state["step"] != None:
                err = event.get("error", "unknown error")
                state["step"].fail(err)
                state["step"] = None
            else:
                tool_name = event.get("tool_name", "unknown")
                err = event.get("error", "unknown error")
                s = turn.step(tool_name + "()")
                s.fail(err)

            state["error_count"] = state["error_count"] + 1
            if abort_on_error and state["error_count"] >= max_errors:
                fail("Too many tool errors (%d), aborting" % state["error_count"])

        elif kind == "done":
            _finish_stream()
            usage = event.get("usage", {})
            total = usage.get("total", 0)
            if total > 0:
                turn.info("Tokens: %d" % total)

        elif kind == "error":
            msg = event.get("error", "unknown error")
            recoverable = event.get("recoverable", False)
            if recoverable:
                turn.warn("Stream warning: " + msg)
            else:
                _finish_stream()
                turn.warn("Stream error: " + msg)
                if abort_on_error:
                    state["error_count"] = state["error_count"] + 1
                    if state["error_count"] >= max_errors:
                        fail("Too many stream errors (%d), aborting" % state["error_count"])

        elif kind == "usage":
            pass

    return _on_event
