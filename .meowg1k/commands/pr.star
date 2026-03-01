# ==============================================================================
# Pull Request Description Generator
# ==============================================================================
"""
Generate comprehensive Pull Request descriptions from git diffs with industry-standard templates.

FEATURES:
  - Multi-Source Analysis: Analyze staged changes or branch differences
  - 8 Built-in Styles: Industry-standard templates from major platforms and projects
  - Smart Diff Analysis: Adaptive map-reduce for large changes, unified for small diffs
  - Stdin Support: Pipe additional context directly into the generator
  - Custom Templates: Define your own PR format with custom instructions
  - Markdown Output: Clean, formatted descriptions ready to paste
  - Configurable: Add custom styles and set defaults via init.star

INSTALLATION:
  # In your .meowg1k/init.star
  load("//commands/pr.star", "setup")

  setup(
      # Add/override styles (merged with built-in styles)
      styles = {
          "acme": {
              "template": "## Summary\n{summary}\n\n## JIRA\n{ticket}",
              "examples": "## Summary\nAdded auth\n\n## JIRA\nACME-123"
          }
      },
      # Set defaults
      default_style = "github",
      default_source = "branch",
      default_base = "main",
      preset = "smart",
      summarize_preset = "fast"
  )

USAGE:
  # Generate PR description from current branch vs main
  meow pr

  # Generate from staged changes
  meow pr --source staged

  # Compare against different base branch
  meow pr --base develop

  # Add manual context
  meow pr --intent "Fixing race condition in auth middleware"

  # Pipe context from stdin
  echo "Resolves security vulnerability CVE-2024-1234" | meow pr

  # Use specific style
  meow pr --style gitlab
  meow pr --style chromium

STYLES:
  github       (Default) GitHub template with checkboxes and sections
  gitlab       GitLab merge request format with acceptance criteria
  conventional Based on Conventional Commits with type/scope
  chromium     Google Chromium style - concise and technical
  microsoft    Microsoft OSS style (TypeScript/VS Code) - detailed
  android      Android AOSP style with component prefix
  linear       Linear app style - user-focused and clean
  freeform     No strict format - natural language explanation

PARAMETERS:
  --intent, -i       Manual context or reasoning behind changes (supports piped input)
  --source          Source of changes: "branch" (default) or "staged"
  --base, -b        Base branch for comparison (default: "main", used with source=branch)
  --style, -s       PR style (default: "github")
  --custom-style, -c Custom style template, instructions, or examples

DEPENDENCIES:
  lib/diff.star - Shared diff analysis infrastructure
"""
# ==============================================================================

load("//lib/diff.star", "build_analysis_prompt")
load("//lib/help.star", "build_choices_desc", "build_preset_desc")

_SYSTEM_PROMPT = """You are an expert software engineer writing Pull Request descriptions.
Goal: Write clear, comprehensive PR descriptions that explain the WHY and enable efficient code review.
Rules:
- Title: Concise, action-oriented summary (ideally under 80 chars).
- Body: Explain motivation, approach, and key technical decisions.
- Impact: Highlight what reviewers should focus on.
- Context: Use the provided file summaries and developer intent."""

# =============================================================================
# Built-in Styles
# =============================================================================

_BUILTIN_STYLES = {
    "github": {
        "name": "GitHub",
        "description": "Sections + checkboxes (Description, Testing, Checklist)",
        "template": "## Description\n[Summary of changes]\n\n## Type of Change\n- [ ] Bug fix\n- [ ] New feature\n- [ ] Breaking change\n- [ ] Documentation update\n\n## How Has This Been Tested?\n[Test description]\n\n## Checklist\n- [ ] My code follows the style guidelines\n- [ ] I have performed a self-review\n- [ ] I have commented my code, particularly in hard-to-understand areas\n- [ ] I have made corresponding changes to the documentation\n- [ ] My changes generate no new warnings\n- [ ] I have added tests that prove my fix is effective or that my feature works\n- [ ] New and existing unit tests pass locally with my changes",
        "examples": "## Description\nAdds JWT authentication middleware to protect API endpoints.\n\n## Type of Change\n- [x] New feature\n- [ ] Bug fix\n\n## How Has This Been Tested?\nAdded integration tests for auth middleware, tested with expired/invalid tokens.\n\n## Checklist\n- [x] Code follows style guidelines\n- [x] Self-review performed\n- [x] Added tests for new feature"
    },
    "gitlab": {
        "name": "GitLab",
        "description": "Problem/solution structure (What, Why, How)",
        "template": "## What does this MR do?\n[Brief explanation]\n\n## Why was this MR needed?\n[Problem statement]\n\n## What are the relevant issue numbers?\nCloses #[issue]\n\n## Screenshots (if relevant)\n\n## Does this MR meet the acceptance criteria?\n- [ ] Documentation created/updated\n- [ ] Tests added for this feature/bug\n- [ ] Conforms to the code review guidelines\n- [ ] Conforms to the merge request performance guidelines",
        "examples": "## What does this MR do?\nImplements rate limiting for API endpoints.\n\n## Why was this MR needed?\nPrevents API abuse and improves service stability.\n\n## What are the relevant issue numbers?\nCloses #456\n\n## Does this MR meet the acceptance criteria?\n- [x] Documentation updated in API_REFERENCE.md\n- [x] Integration tests added\n- [x] Reviewed by @security-team"
    },
    "conventional": {
        "name": "Conventional",
        "description": "Type/Scope headers (feat, fix, docs, etc.)",
        "template": "## Type: [feat|fix|docs|refactor|test|chore]\n## Scope: [component/module]\n\n### Summary\n[Brief description]\n\n### Changes\n- [Change 1]\n- [Change 2]\n\n### Breaking Changes\n[If any]\n\n### Related Issues\n[Links]",
        "examples": "## Type: feat\n## Scope: auth\n\n### Summary\nAdd OAuth2 support for Google login.\n\n### Changes\n- Implement OAuth2 flow with PKCE\n- Add Google provider configuration\n- Update user model with external ID\n\n### Breaking Changes\nNone\n\n### Related Issues\nResolves #789"
    },
    "chromium": {
        "name": "Chromium",
        "description": "Concise technical (summary + Bug: + Test:)",
        "template": "[One line summary]\n\n[Detailed description explaining why the change is being made]\n\nBug: [bug number or 'none']\nTest: [description of tests]\nChange-Id: [generated]",
        "examples": "Optimize image loading pipeline for large files\n\nPreviously, images were loaded entirely into memory before processing,\ncausing OOM errors with files >100MB. This change implements streaming\nprocessing with configurable chunk sizes.\n\nPerformance improves by 60% on large files and memory usage is now\nbounded to chunk_size * 2.\n\nBug: 12345\nTest: Added MemoryBoundedImageLoadingTest, verified with 500MB test images"
    },
    "microsoft": {
        "name": "Microsoft",
        "description": "Problem/Solution/Risk/Test Plan sections",
        "template": "Fixes #[issue]\n\n### Problem\n[What problem does this solve?]\n\n### Solution\n[How does this change solve it?]\n\n### Risk\n[What's the risk? Low/Medium/High]\n\n### Test Plan\n[How was this tested?]\n\n### Screenshots/GIFs\n[If UI changes]",
        "examples": "Fixes #4567\n\n### Problem\nEditor crashes when opening files >10MB due to synchronous parsing.\n\n### Solution\nMoved parsing to background worker thread with progress reporting.\nImplemented cancellation support for better responsiveness.\n\n### Risk\nMedium - Changes core parsing pipeline, but isolated behind feature flag.\n\n### Test Plan\n- Unit tests for worker thread communication\n- Smoke tested with 50MB files\n- Verified cancellation behavior\n- Regression tests pass"
    },
    "android": {
        "name": "Android AOSP",
        "description": "Component: summary + Bug: + Test: + Change-Id:",
        "template": "[Component]: [Brief summary]\n\n[Detailed explanation of changes and rationale]\n\nBug: [bug ID or omit]\nTest: [test description]\n\n[Additional technical details]\n\nChange-Id: [gerrit ID]",
        "examples": "framework/base: Optimize WindowManager layout performance\n\nReduce layout passes by caching view measurements and detecting\nunnecessary invalidations. This eliminates redundant calculations\nwhen view hierarchy remains stable.\n\nMeasured 40% reduction in layout time for complex screens with\n50+ views. No behavioral changes, pure optimization.\n\nTest: WindowManagerLayoutTest, manual testing on Pixel devices\nBenchmarked with systrace showing improved frame times\n\nChange-Id: Iabcd1234ef567890"
    },
    "linear": {
        "name": "Linear",
        "description": "User-focused (Changes, Context, Details, Testing)",
        "template": "## Changes\n[What changed, in user-facing terms]\n\n## Context\n[Why this change was needed]\n\n## Details\n[Technical implementation notes]\n\n## Testing\n[How to verify]\n\n## Related\n[Issues/PRs]",
        "examples": "## Changes\nUsers can now export their data in CSV format from the settings page.\n\n## Context\nMultiple customers requested data export for compliance and backup.\nCSV was chosen for broad compatibility with spreadsheet tools.\n\n## Details\n- Added ExportService with streaming CSV writer\n- Implemented background job for large exports\n- Added progress notification\n- Rate limited to prevent abuse\n\n## Testing\n- Export 10K records - completes in <5s\n- Verified CSV format with Excel/Google Sheets\n- Tested error handling for failed exports\n\n## Related\nResolves LIN-123, LIN-456"
    },
    "freeform": {
        "name": "Freeform",
        "description": "No format rules - natural language",
        "template": "[Natural description of changes]",
        "examples": "This PR adds dark mode support. I've updated the theme system to handle color schemes dynamically and added a toggle in user settings."
    }
}

# =============================================================================
# Prompt Templates
# =============================================================================

_TEMPLATES = {
    "map_reduce": """Generate a Pull Request description based on these file summaries.

Summaries:
{summaries}

{context_block}

{style_section}

Requirements:
- Title: Clear, action-oriented summary (ideally under 80 chars).
- Body: Explain motivation, key changes, and technical approach.
- Structure: Use markdown formatting for clarity.
- Focus: What reviewers need to know and what to pay attention to.
- STRICTLY follow the style template above.
""",
    "unified": """Generate a Pull Request description for these changes.

Diff:
{diff_text}

{context_block}

{style_section}

Requirements:
- Title: Clear, action-oriented summary (ideally under 80 chars).
- Body: Explain motivation, key changes, and technical approach.
- Structure: Use markdown formatting for clarity.
- Focus: What reviewers need to know and what to pay attention to.
- STRICTLY follow the style template above.
"""
}

# =============================================================================
# Setup Function
# =============================================================================

def setup(styles=None, default_style=None, default_source=None, default_base=None, preset=None, summarize_preset=None):
    """Configure the PR command.

    Args:
        styles: Dict of custom styles to add/override. Merged with built-in styles.
        default_style: Default style when --style not specified.
        default_source: Default source (branch/staged).
        default_base: Default base branch.
        preset: LLM preset for final generation.
        summarize_preset: LLM preset for file summarization (map-reduce).
    """
    all_styles = dict(_BUILTIN_STYLES)
    if styles:
        for name, style_def in styles.items():
            all_styles[name] = style_def

    cfg_default_style = default_style if default_style != None else "github"
    cfg_default_source = default_source if default_source != None else "branch"
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

    def generate_pr_description(ctx, intent="", source="branch", base="main", style="github", custom_style="", preset_arg="", summarize_preset_arg=""):
        """Generate a Pull Request description from git diff."""
        # Get diff
        diff, target = read_diff(ctx, source, base)
        if not diff:
            return None

        style_section = build_style_section(style, custom_style)

        template_vars = {
            "context_block": "### Context:\n{}".format(intent) if intent else "",
            "style_section": style_section
        }

        # This internally handles collecting and summarization with proper UI stages
        prompt = build_analysis_prompt(ctx, diff.files, target, template_vars, _TEMPLATES, summarize_preset_arg)

        # Final stage: Generate PR description
        gen_step = ctx.ui.step("Generating PR Description")
        ctx.ui.info("Style: {}".format(style))
        activity = ctx.ui.activity("Writing...")

        pr_description = ctx.llm.chat(
            preset=preset_arg,
            system=_SYSTEM_PROMPT,
            prompt=prompt
        )

        activity.done()
        gen_step.done("Description ready")

        ctx.ui.divider("thick")
        ctx.output.markdown(pr_description)

        return pr_description

    def handle_pr(ctx):
        full_intent = merge_intent_with_stdin(ctx, ctx.intent)
        return generate_pr_description(
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
        return build_choices_desc("PR style (one of):", all_styles, default_style,
            lambda name, s: s["description"])

    def build_source_desc(default_source):
        sources = {
            "branch": "Analyze full branch diff against base",
            "staged": "Analyze staged changes only"
        }
        return build_choices_desc("Source of changes (one of):", sources, default_source)

    pr_command = meow.tool(
        name="pr",
        description="Generate pull request descriptions from branch diffs.",
        params={
            "intent": meow.param("string", default="", short="i", desc="Why you made these changes."),
            "source": meow.param("string", default=cfg_default_source, choices=["branch", "staged"], desc=build_source_desc(cfg_default_source)),
            "base": meow.param("string", default=cfg_default_base, short="b", desc="Branch to compare against."),
            "style": meow.param("string", default=cfg_default_style, short="s", choices=all_styles, desc=build_style_desc(cfg_default_style)),
            "custom_style": meow.param("string", default="", short="c", desc="Free-form style instructions (overrides --style)."),
            "preset": meow.param("string", default=cfg_preset, choices=meow.presets(), desc=build_preset_desc(cfg_preset)),
            "summarize_preset": meow.param("string", default=cfg_summarize_preset, choices=meow.presets(), desc=build_preset_desc(cfg_summarize_preset))
        },
        handler=handle_pr
    )
    meow.command(pr_command)
