# ==============================================================================
# Commit Message Generator
# ==============================================================================
"""
Generate semantic, well-formatted git commit messages automatically from staged changes or branch diffs.

FEATURES:
  - 9 Built-in Styles: Conventional, Gitmoji, Ticket-based, Linux Kernel, and more
  - Smart Matching: Automatically adapts to your repository's commit history
  - Adaptive Analysis: Uses map-reduce for large diffs, unified for small changes
  - Context Aware: Accepts developer notes via CLI or piped stdin
  - Branch Comparison: Generate commits for branch diffs, not just staged changes
  - Configurable: Add custom styles and set defaults via init.star

INSTALLATION:
  # In your .meowg1k/init.star
  load("//commands/commit.star", "setup")

  setup(
      # Add/override styles (merged with built-in styles)
      styles = {
          "acme": {
              "template": "[ACME-{ticket}] {description}",
              "examples": "[ACME-123] Add user auth"
          }
      },
      # Set defaults
      default_style = "conventional",
      preset = "smart",
      summarize_preset = "fast"
  )

USAGE:
  # Generate from staged changes (default)
  meow commit

  # With additional context
  meow commit -i "Implements JWT authentication for API"

  # From piped input
  echo "Fixes memory leak in image processor" | meow commit

  # Compare branch against main
  meow commit --source branch --base main

  # Use specific style
  meow commit --style conventional

STYLES:
  match               (Default) Adapts to your repository's commit history
  conventional        Standard type(scope): description format
  scoped              Strict component/path scoping
  gitmoji             Expressive emoji prefixes (✨, 🐛, ♻️)
  emoji-log           Simple emoji system (📦 NEW, 👌 IMPROVE, 🐛 FIX)
  imperative          Git default - present tense imperative
  ticket              Prefixed with issue/ticket ID
  ticket-conventional Combines ticket ID with Conventional Commits
  kernel              Linux kernel subsystem style
  freeform            No strict format - natural language

EXAMPLES:
  # Basic workflow
  git add .
  meow commit
  git commit -m "$(meow commit)"

  # With context from issue
  gh issue view 123 --json body -q .body | meow commit

  # Branch workflow
  git checkout -b feature/new-api
  # ... make changes ...
  meow commit --source branch --base main --style conventional

PARAMETERS:
  --intent, -i       Manual context or 'why' behind changes (supports piped input)
  --source          Source of changes: "staged" (default) or "branch"
  --base, -b        Base branch for comparison (default: "main", used with source=branch)
  --style, -s       Formatting standard (default: "match")
  --custom-style, -c Custom style template or instructions

DEPENDENCIES:
  lib/diff.star - Diff analysis strategies
"""
# ==============================================================================

load("//lib/diff.star", "build_analysis_prompt")
load("//lib/help.star", "build_choices_desc", "build_preset_desc")
load("//lib/ui_helpers.star", "make_markdown_stream_handler")

_SYSTEM_PROMPT = """You are an expert at writing clean, effective git commit messages.
Goal: Write messages that explain the 'WHY' and enable instant understanding.
Rules:
- Subject line: Max 50 chars, imperative mood (e.g., "Add", not "Added").
- Body: Wrap at 72 chars, separate from subject with a blank line.
- Context: Use the provided file summaries and developer intent."""

# =============================================================================
# Built-in Styles
# =============================================================================

_BUILTIN_STYLES = {
    "conventional": {
        "name": "Conventional Commits",
        "description": "type(scope): description - for changelogs",
        "template": "<type>(<optional-scope>): <description>\n\n[optional body]\n\n[optional footer]",
        "types": "feat, fix, docs, style, refactor, perf, test, build, ci, chore, ops, revert",
        "examples": "feat(auth): add JWT support\n\nBREAKING CHANGE: The 'auth' header is now required.\n\nfix(ui): resolve button alignment"
    },
    "scoped": {
        "name": "Scoped (Monorepo/Component)",
        "description": "type(path/module): description - for monorepos",
        "template": "<type>(<path/to/module>): <description>",
        "examples": "feat(packages/ui): add new Button\nfix(services/payment): handle missing avatar"
    },
    "gitmoji": {
        "name": "Gitmoji",
        "description": "✨ feat: description - emoji prefixes",
        "template": "<emoji> <type>: <description>",
        "examples": "✨ feat: add dark mode\n🐛 fix: resolve memory leak"
    },
    "emoji-log": {
        "name": "Emoji Log",
        "description": "📦 NEW: description - simple emoji system",
        "template": "<PREFIX>: <description>",
        "prefixes": "📦 NEW, 👌 IMPROVE, 🐛 FIX, 📖 DOC, 🚀 RELEASE, 🤖 TEST",
        "examples": "📦 NEW: Add user profile page\n👌 IMPROVE: Refactor middleware"
    },
    "imperative": {
        "name": "Imperative (Git Default)",
        "description": "Add feature - simple imperative verb",
        "template": "<Verb> <description>",
        "examples": "Add user authentication endpoint\nFix navigation bug on mobile"
    },
    "ticket": {
        "name": "Ticket Prefix",
        "description": "[JIRA-123] description - issue tracker IDs",
        "template": "[<TICKET-ID>] <description>",
        "examples": "[PROJ-123] Add user authentication\n[JIRA-456] Fix navigation bug"
    },
    "ticket-conventional": {
        "name": "Ticket + Conventional",
        "description": "[JIRA-123] feat(scope): desc - combined",
        "template": "[<TICKET-ID>] <type>(<scope>): <description>",
        "examples": "[PROJ-123] feat(auth): add JWT\n[JIRA-456] fix(nav): resolve crash"
    },
    "kernel": {
        "name": "Linux Kernel",
        "description": "subsystem: description - Linux style",
        "template": "<subsystem>: <description>",
        "examples": "net/http: add context support\ndrivers/usb: fix race condition"
    },
    "match": {
        "name": "Match Repository Style",
        "description": "Auto-detect from recent commits",
        "template": "(Inferred from commit history)",
        "examples": "(Varies based on repository)"
    },
    "freeform": {
        "name": "Freeform",
        "description": "No format rules - natural language",
        "template": "<Any format>",
        "examples": "wip: saving work before meeting\nFixed the login bug Dave found"
    }
}

# =============================================================================
# Prompt Templates
# =============================================================================

_TEMPLATES = {
    "map_reduce": """Generate a git commit message based on these file summaries.

Summaries:
{summaries}

{context_block}

{style_section}

Requirements:
- Subject: Concise (ideally <50 chars), imperative, no period.
- Body: Explain the 'why' if non-trivial, wrap at 72 chars.
- STRICTLY follow the style template above.
""",
    "unified": """Generate a git commit message for these changes.

Diff:
{diff_text}

{context_block}

{style_section}

Requirements:
- Subject: Concise (ideally <50 chars), imperative, no period.
- Body: Explain the 'why' if non-trivial, wrap at 72 chars.
- STRICTLY follow the style template above.
"""
}

# =============================================================================
# Setup Function
# =============================================================================

def setup(styles=None, default_style=None, default_source=None, default_base=None, preset=None, summarize_preset=None):
    """Configure the commit command.

    Args:
        styles: Dict of custom styles to add/override. Merged with built-in styles.
        default_style: Default style when --style not specified.
        default_source: Default source of changes ("staged" or "branch").
        default_base: Default base branch for comparison.
        preset: LLM preset for final generation.
        summarize_preset: LLM preset for file summarization (map-reduce).
    """
    all_styles = dict(_BUILTIN_STYLES)
    if styles:
        for name, style_def in styles.items():
            all_styles[name] = style_def

    cfg_default_style = default_style if default_style != None else "match"
    cfg_default_source = default_source if default_source != None else "staged"
    cfg_default_base = default_base if default_base != None else "main"
    cfg_preset = preset if preset != None else ""
    cfg_summarize_preset = summarize_preset if summarize_preset != None else ""

    def build_style_section(style, custom_style):
        """Build style instruction section for LLM prompt."""
        if custom_style:
            return "### CUSTOM STYLE:\nInstructions: {}\n".format(custom_style)
        if style in all_styles:
            s = all_styles[style]
            if style == "freeform":
                return "### STYLE: Freeform\nNo strict formatting rules."
            return "### STYLE: {}\nFormat: {}\n\nExamples:\n{}".format(s["name"], s["template"], s["examples"])
        return ""

    def read_diff(ctx, source, base):
        """Read git diff based on source and validate. Returns (diff, target) tuple."""
        if source == "staged":
            target = "staged"
            diff = ctx.git.diff(target="staged")
        else:
            target = base
            diff = ctx.git.diff(target=base)

        if len(diff.files) == 0:
            ctx.ui.error("No changes")
            return None, None

        return diff, target

    def get_match_style_context(ctx):
        """Get recent commits for 'match' style."""
        commits = ctx.git.log(count=10)
        if not commits or len(commits) == 0:
            return ""
        commit_msgs = ["{}: {}".format(c.hash[:7], c.message.split("\n")[0]) for c in commits]
        return """### MATCH STYLE: Analyze and Mirror
Recent commits (use as style reference):
{}

Infer the commit message style from the patterns above and generate a message that matches.""".format("\n".join(commit_msgs))

    def generate_commit_message(ctx, intent="", source="staged", base="main", style="match", custom_style="", preset_arg="", summarize_preset_arg=""):
        """Generate a git commit message from diff."""
        # Get diff
        diff, target = read_diff(ctx, source, base)
        if not diff:
            return None

        style_section = ""
        if style == "match":
            style_section = get_match_style_context(ctx)
        else:
            style_section = build_style_section(style, custom_style)

        template_vars = {
            "context_block": "### Context:\n{}".format(intent) if intent else "",
            "style_section": style_section
        }

        # This internally handles collecting and summarization with proper UI stages
        prompt = build_analysis_prompt(ctx, diff.files, target, template_vars, _TEMPLATES, summarize_preset_arg)

        # Final stage: Generate commit message
        gen_step = ctx.ui.step("Generating Commit Message")
        ctx.ui.info("Style: {}".format(style))

        on_event = make_markdown_stream_handler(ctx)
        gen_step.done("Message ready")
        ctx.ui.divider("thick")
        commit_message = ctx.llm.chat(
            preset=preset_arg,
            system=_SYSTEM_PROMPT,
            prompt=prompt,
            stream=True,
            on_event=on_event,
        )
        ctx.output.writeline(commit_message)
        return commit_message

    def merge_intent_with_stdin(ctx, intent):
        """Merge intent flag with piped stdin content."""
        full_intent = intent or ""
        if ctx.stdin.is_piped():
            stdin_content = ctx.stdin.read().strip()
            if stdin_content:
                if full_intent:
                    full_intent = full_intent + "\n\n" + stdin_content
                else:
                    full_intent = stdin_content
        return full_intent

    def handle_commit(ctx):
        full_intent = merge_intent_with_stdin(ctx, ctx.intent)
        return generate_commit_message(
            ctx,
            intent=full_intent,
            source=ctx.source,
            base=ctx.base,
            style=ctx.style,
            custom_style=ctx.custom_style,
            preset_arg=ctx.preset,
            summarize_preset_arg=ctx.summarize_preset
        )

    def build_style_desc(default_style):
        return build_choices_desc("Message style (one of):", all_styles, default_style,
            lambda name, s: s["description"])

    def build_source_desc(default_source):
        sources = {
            "staged": "Analyze staged changes only",
            "branch": "Analyze full branch diff against base"
        }
        return build_choices_desc("Source of changes (one of):", sources, default_source)

    commit_command = meow.tool(
        name="commit",
        description="Generate commit messages from staged changes or branch diffs.",
        params={
            "intent": meow.param("string", default="", short="i", desc="Why you made these changes."),
            "source": meow.param("string", default=cfg_default_source, choices=["staged", "branch"], desc=build_source_desc(cfg_default_source)),
            "base": meow.param("string", default=cfg_default_base, short="b", desc="Branch to compare against (with --source=branch)."),
            "style": meow.param("string", default=cfg_default_style, short="s", choices=all_styles, desc=build_style_desc(cfg_default_style)),
            "custom_style": meow.param("string", default="", short="c", desc="Free-form style instructions (overrides --style)."),
            "preset": meow.param("string", default=cfg_preset, choices=meow.presets(), desc=build_preset_desc(cfg_preset)),
            "summarize_preset": meow.param("string", default=cfg_summarize_preset, choices=meow.presets(), desc=build_preset_desc(cfg_summarize_preset))
        },
        handler=handle_commit
    )
    meow.command(commit_command)
