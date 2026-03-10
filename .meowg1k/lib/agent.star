"""
Agent Loop Library for meowg1k (lib/agent.star)

Shared helper for running an agentic LLM loop with:
  - A "Preparing" step that shows task/preset/stdin info
  - Optional context compaction via maybe_compact
  - agent_turn call with streaming and on_event handler
  - turn.done() + result return

## Usage

```python
load("//lib/agent.star", "run_agent_turn")
load("//lib/file_ops.star", "file_reader", "file_writer")

def handle(ctx):
    result = run_agent_turn(
        ctx=ctx,
        task=ctx.task,
        preset=ctx.preset,
        system=MY_SYSTEM_PROMPT,
        tools=[file_reader, file_writer],
        max_steps=ctx.max_steps,
    )
```

## API

### run_agent_turn(ctx, task, preset, system, tools, max_steps, compact_threshold=80)

Run the full agentic loop for a given task.

Args:
    ctx:               Handler context (from meow tool handler).
    task:              The user's task string.
    preset:            LLM preset name to use.
    system:            System prompt string.
    tools:             List of tool handles to make available.
    max_steps:         Maximum agent iterations.
    compact_threshold: Event count threshold for history compaction (default 80).

Returns:
    str — the final result string from agent_turn (the full LLM response).
"""

load("//lib/ui_helpers.star", "make_agentic_stream_handler")
load("//lib/compaction.star", "maybe_compact")

def run_agent_turn(ctx, task, preset, system, tools, max_steps, compact_threshold=80):
    """Run the full agentic loop: prep step, compaction, agent_turn, turn.done().

    Merges any piped stdin into the task automatically.

    Args:
        ctx:               Handler context.
        task:              The user's task string.
        preset:            LLM preset name.
        system:            System prompt string.
        tools:             List of tool handles.
        max_steps:         Maximum agent iterations.
        compact_threshold: Compact when event count >= this (default 80).

    Returns:
        str — the final result from agent_turn.
    """
    # --- Merge stdin context ---
    full_task = task
    stdin_len = 0
    if ctx.stdin.is_piped():
        stdin_content = ctx.stdin.read().strip()
        if stdin_content:
            stdin_len = len(stdin_content)
            full_task = task + "\n\n### Additional context (stdin):\n" + stdin_content

    # --- UI turn ---
    turn = ctx.ui.assistant_turn()

    # --- Phase 1: prepare ---
    prep_step = turn.step("Preparing")
    task_preview = task if len(task) <= 60 else task[:57] + "..."
    prep_step.info("Task: " + task_preview)
    prep_step.info("Preset: " + preset)
    prep_step.info("Max steps: " + str(max_steps))
    if stdin_len > 0:
        prep_step.info("Stdin context: " + str(stdin_len) + " chars")

    # --- Compact if session is already long ---
    compact_summary = maybe_compact(ctx, preset=preset, threshold=compact_threshold)
    if compact_summary:
        prep_step.info("History compacted")

    prep_step.done()

    on_event = make_agentic_stream_handler(turn)

    # Build prompt, prepending compaction summary when present
    prompt = full_task
    if compact_summary:
        prompt = ("## Compacted session history\n\n" + compact_summary +
                  "\n\n---\n\n## Current task\n\n" + full_task)

    # --- Run the agentic loop ---
    result = ctx.llm.agent_turn(
        prompt=prompt,
        preset=preset,
        system=system,
        tools=tools,
        max_iterations=max_steps,
        on_tool_error="return",
        stream=True,
        on_event=on_event,
    )

    turn.done()
    ctx.output.writeline(result)

    return result
