# ==============================================================================
# New Command - AI Starlark Command Generator
# ==============================================================================
"""
Autonomous agent that creates new Starlark commands in .meowg1k/commands/.

FEATURES:
  - Full agentic loop: agent reads existing commands and the API reference
  - Writes a new .meowg1k/commands/<name>.star and registers it in init.star
  - Reads docs/api/API_REFERENCE.md for authoritative API knowledge
  - Reads existing commands and libs as style examples
  - Memory tools available for longer generation sessions

USAGE:
  meow new-command --name commit2 --task "generate conventional commit messages"
  meow new-command -n review -t "do a full code review on staged changes" --preset smart

PARAMETERS:
  --name, -n     Filename for the new command (without .star extension, required)
  --task, -t     What the command should do (required)
  --preset       LLM preset: smart (default) or fast
  --max-steps    Maximum agent iterations (default: 30)

INSTALLATION:
  # In your .meowg1k/init.star
  load("//commands/new_command.star", new_command_setup="setup")
  new_command_setup(preset="smart")
"""
# ==============================================================================

load("//lib/file_ops.star",
     "file_reader", "file_writer", "file_exists", "list_directory", "search_text")
load("//lib/memory.star",
     "save_context", "recall_context", "list_context",
     "summarize_history", "get_session_info")
load("//lib/agent.star", "run_agent_turn")
load("//lib/help.star", "build_preset_desc")

# ==============================================================================
# System prompt
# ==============================================================================

_SYSTEM_PROMPT = """You are an expert meowg1k Starlark command author.

Your mission is to create a new, well-structured `.meowg1k/commands/<name>.star` file
and update `.meowg1k/init.star` to register it — based on the task description given.

## Step-by-step approach

1. **Read the API reference** first:
   - Use file_reader on `docs/api/API_REFERENCE.md` to understand all available APIs,
     modules, param types, handler context fields, and tool definitions.

2. **Read existing commands** for style and patterns:
   - Use list_directory on `.meowg1k/commands` to see all commands.
   - Use file_reader on 1–2 similar commands (e.g. `code.star`, `commit.star`) as templates.
   - Use list_directory on `.meowg1k/lib` and read relevant libs you plan to load.
   - Use search_text to grep for specific patterns or usages across existing files.

3. **Read init.star** to understand how to register the command:
   - file_reader on `.meowg1k/init.star`

4. **Write the new command file**:
   - Path: `.meowg1k/commands/<name>.star`
   - Follow the existing command style: module docstring, `_SYSTEM_PROMPT` constant,
     `setup(preset=None)` function, inner `_ALL_TOOLS` list, inner handler, `meow.tool(...)`,
     `meow.command(...)`.
   - Use `load("//lib/agent.star", "run_agent_turn")` if the command needs an agentic loop.
   - Use `load("//lib/help.star", "build_preset_desc")` for the preset param description.
   - All `meow.param(...)` calls must use `desc=` (not `description=`).
   - Use `ctx.json.stringify(...)` not `ctx.json.encode(...)`.
   - No implicit string concatenation — always use `+`.
   - No re-assignment of module-level globals — define each name exactly once.
   - Starlark has no `str.join()` — build joined strings with a loop if needed.

5. **Update init.star** to register the new command:
   - Append the load + setup call at the bottom of the REGISTER COMMANDS section.
   - Use a unique alias for the setup function to avoid name collisions.
   - Example:
       load("//commands/<name>.star", <name>_setup = "setup")
       <name>_setup(preset="smart")

6. **Verify** with file_reader that both files were written correctly.

## Key Starlark rules

- `meow.param` uses `desc=` (not `description=`)
- `ctx.json` has `stringify` and `parse` only (no `encode`/`decode`)
- No implicit string concatenation — use `+` operator explicitly
- No reassignment of module-level globals
- Starlark has no `str.join()` — iterate and concatenate manually
- `load("//lib/...", "name")` uses `//` prefix for .meowg1k-relative paths

## Output Format

When done, provide a concise markdown summary:
- Command name and file created
- What the command does
- Parameters it accepts
- How to invoke it (example CLI usage)
- The init.star registration that was added
"""

# ==============================================================================
# Setup
# ==============================================================================

def setup(preset=None):
    """Register the `new-command` command.

    Args:
        preset: Default LLM preset (default: "smart").
    """
    config_preset = preset if preset != None else "smart"

    _ALL_TOOLS = [
        file_reader,
        file_writer,
        file_exists,
        list_directory,
        search_text,
        save_context,
        recall_context,
        list_context,
        summarize_history,
        get_session_info,
    ]

    def handle_new_command(ctx):
        name = ctx.name
        task = ctx.task
        active_preset = ctx.preset
        max_steps = ctx.max_steps

        # Build a rich task description that tells the agent exactly what to create
        full_task = (
            "Create a new meowg1k Starlark command.\n\n" +
            "**Command name (filename without .star):** " + name + "\n\n" +
            "**What the command should do:**\n" + task + "\n\n" +
            "Follow the step-by-step approach in your system prompt:\n" +
            "1. Read docs/api/API_REFERENCE.md\n" +
            "2. Read existing commands for style examples\n" +
            "3. Read .meowg1k/init.star\n" +
            "4. Write .meowg1k/commands/" + name + ".star\n" +
            "5. Update .meowg1k/init.star to register the command\n" +
            "6. Verify both files\n"
        )

        run_agent_turn(
            ctx=ctx,
            task=full_task,
            preset=active_preset,
            system=_SYSTEM_PROMPT,
            tools=_ALL_TOOLS,
            max_steps=max_steps,
        )

    new_command_tool = meow.tool(
        name="new-command",
        description="AI agent that creates a new Starlark command in .meowg1k/commands/.",
        params={
            "name": meow.param(
                "string",
                default="",
                short="n",
                min_len=1,
                desc="Name for the new command (filename without .star extension).",
            ),
            "task": meow.param(
                "string",
                default="",
                short="t",
                min_len=1,
                desc="Description of what the new command should do.",
            ),
            "preset": meow.param(
                "string",
                default=config_preset,
                choices=meow.presets(),
                desc=build_preset_desc(config_preset),
            ),
            "max_steps": meow.param(
                "int",
                default=30,
                desc="Maximum number of agent iterations.",
            ),
        },
        handler=handle_new_command,
    )
    meow.command(new_command_tool)
