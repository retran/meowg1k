# ==============================================================================
# Code Command - AI Coding Agent
# ==============================================================================
"""
Autonomous AI coding agent that can explore, plan, implement and fix code.

FEATURES:
  - Full agentic loop: the agent decides which tools to call and when to stop
  - Planning support: create_plan / decompose_task / execute_plan tools
  - Memory: save_context / recall_context / list_context tools for state across steps
  - File operations: read, write, edit (first-occurrence), search, replace, list
  - Shell execution: run build commands, tests, linters
  - Git awareness: read-only status and diff (the agent does NOT commit)
  - Semantic code search: vector-based search when an index exists
  - Context compaction: auto-compacts session history when it grows large

USAGE:
  meow code --task "add error handling to all HTTP handlers"
  meow code -t "fix failing tests" --preset smart
  cat error.log | meow code -t "diagnose this error"

PARAMETERS:
  --task, -t     What to do (required)
  --preset       LLM preset: smart (default) or fast
  --max-steps    Maximum agent iterations (default: 50)

INSTALLATION:
  # In your .meowg1k/init.star
  load("//commands/code.star", code_setup="setup")
  code_setup(preset="smart")
"""
# ==============================================================================

load("//lib/file_ops.star",
     "file_reader", "file_writer", "file_exists",
     "list_directory", "search_text", "replace_text", "edit_file")
load("//lib/shell.star", "shell_exec")
load("//lib/git.star", "git_status", "git_diff")
load("//lib/code_search.star", "code_search")
load("//lib/memory.star",
     "save_context", "recall_context", "list_context",
     "summarize_history", "get_session_info")
load("//lib/planning.star", "create_plan", "decompose_task", "execute_plan")
load("//lib/ui_helpers.star", "make_agentic_stream_handler")
load("//lib/compaction.star", "maybe_compact")
load("//lib/help.star", "build_preset_desc")

# ==============================================================================
# System prompt
# ==============================================================================

_SYSTEM_PROMPT = """You are an expert autonomous coding agent embedded in a developer's terminal.

## Mission
Complete the coding task given to you. Work methodically: explore before acting,
plan before implementing, verify after changing.

## Capabilities
You have access to the following tool groups:

### File Operations
- file_reader       — read a file's full contents
- file_writer       — write / overwrite a file
- edit_file         — replace the FIRST occurrence of a string in a file (preferred for edits)
- replace_text      — replace ALL occurrences of a string in a file
- file_exists       — check whether a path exists
- list_directory    — list files matching a glob pattern
- search_text       — grep for a regex pattern across files

### Shell
- shell_exec        — run any shell command (build, test, lint, …)

### Git (read-only)
- git_status        — current branch, staged/modified/untracked files
- git_diff          — unified diff (staged or unstaged)
  NOTE: do NOT commit; the user commits manually.

### Semantic Search
- code_search       — vector-based semantic search (requires built index)

### Planning
- create_plan       — generate a step-by-step plan from a goal (returns JSON)
- decompose_task    — break a complex task into subtasks (returns JSON)
- execute_plan      — run a plan using a sub-agentic loop

### Memory (within this session)
- save_context      — persist a key/value string to session metadata
- recall_context    — retrieve a previously saved value (returns "" if missing)
- list_context      — list all saved context keys
- summarize_history — ask the LLM to summarise recent session history
- get_session_info  — session ID, tool name, metadata count

## Workflow Guidelines

1. **Explore first**: before changing anything, read relevant files and understand
   the current state.
2. **Plan complex work**: for tasks with multiple steps, use create_plan or
   decompose_task, then follow the plan.
3. **Use memory**: save intermediate results (file lists, error counts, decisions)
   with save_context so you can recall them if the session is long.
4. **Edit surgically**: prefer edit_file (first-occurrence) over replace_text
   (all-occurrences) unless you intentionally want every instance changed.
5. **Verify changes**: after editing files, run relevant tests or builds with
   shell_exec to confirm correctness.
6. **Report clearly**: when done, summarise what was changed and why.

## Output Format
- Think step-by-step in your responses before calling tools.
- After all work is done, provide a concise markdown summary:
  - What was done
  - Files created / modified
  - Test/build outcome (if applicable)
  - Any caveats or follow-up work needed
"""

# ==============================================================================
# Setup
# ==============================================================================

def setup(preset=None):
    """Register the `code` command.

    Args:
        preset: Default LLM preset (default: "smart").
    """
    config_preset = preset if preset != None else "smart"

    _ALL_TOOLS = [
        # File ops
        file_reader, file_writer, file_exists,
        list_directory, search_text, replace_text, edit_file,
        # Shell
        shell_exec,
        # Git
        git_status, git_diff,
        # Semantic search
        code_search,
        # Planning
        create_plan, decompose_task, execute_plan,
        # Memory
        save_context, recall_context, list_context,
        summarize_history, get_session_info,
    ]

    def handle_code(ctx):
        task = ctx.task
        active_preset = ctx.preset
        max_steps = ctx.max_steps

        # --- Merge stdin context ---
        full_task = task
        if ctx.stdin.is_piped():
            stdin_content = ctx.stdin.read().strip()
            if stdin_content:
                full_task = task + "\n\n### Additional context (stdin):\n" + stdin_content

        # --- UI turn ---
        ctx.ui.user_turn(full_task)
        turn = ctx.ui.assistant_turn()
        on_event = make_agentic_stream_handler(turn)

        # --- Compact if session is already long (e.g. re-run in same session) ---
        compact_summary = maybe_compact(ctx, preset=active_preset, threshold=60)

        # Build prompt, prepending compaction summary when present
        prompt = full_task
        if compact_summary:
            prompt = ("## Compacted session history\n\n" + compact_summary +
                      "\n\n---\n\n## Current task\n\n" + full_task)

        # --- Run the agentic loop ---
        result = ctx.llm.agent_turn(
            prompt=prompt,
            preset=active_preset,
            system=_SYSTEM_PROMPT,
            tools=_ALL_TOOLS,
            max_iterations=max_steps,
            on_tool_error="return",
            stream=True,
            on_event=on_event,
        )

        turn.done()
        ctx.output.writeline(result)

    code_command = meow.tool(
        name="code",
        description="Autonomous AI coding agent: explore, plan, implement and fix code.",
        params={
            "task": meow.param(
                "string",
                default="",
                short="t",
                min_len=1,
                desc="The coding task to perform.",
            ),
            "preset": meow.param(
                "string",
                default=config_preset,
                choices=meow.presets(),
                desc=build_preset_desc(config_preset),
            ),
            "max_steps": meow.param(
                "int",
                default=50,
                desc="Maximum number of agent iterations.",
            ),
        },
        handler=handle_code,
    )
    meow.command(code_command)
