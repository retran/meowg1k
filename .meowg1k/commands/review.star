# ==============================================================================
# Review Command - AI Staged Changes Reviewer
# ==============================================================================
"""
Review staged git changes with AI-powered analysis and actionable feedback.

FEATURES:
  - Staged Diff Analysis: Focuses exclusively on what is staged and ready to commit
  - Adaptive Map-Reduce: Handles large diffs by summarising per-file before synthesis
  - Focused Lenses: Choose between a security, performance, correctness, or full review
  - Context Aware: Accepts developer notes via CLI or piped stdin
  - Streaming Output: Results stream to the terminal as they are generated
  - Configurable: Set defaults for lens and presets via init.star

INSTALLATION:
  # In your .meowg1k/init.star
  load("//commands/review.star", review_setup="setup")

  review_setup(
      default_lens = "full",
      preset        = "smart",
      summarize_preset = "fast",
  )

USAGE:
  # Review everything that is staged
  meow review

  # Add developer context / intent
  meow review -i "Refactoring auth middleware for OIDC"

  # Pipe context from another tool
  gh issue view 42 --json body -q .body | meow review

  # Focus the review on a specific concern
  meow review --lens security
  meow review --lens performance

  # Override the preset for this run
  meow review --preset smart

LENSES:
  full          (Default) Comprehensive review: correctness, style, security, performance
  security      Threat modelling, injection risks, secrets exposure, auth/authz
  performance   Algorithmic complexity, hot paths, memory allocation, I/O patterns
  correctness   Logic bugs, edge cases, error handling, concurrency hazards
  style         Naming, readability, comments, idiomatic patterns

PARAMETERS:
  --intent, -i       Context or reasoning behind the staged changes (supports piped input)
  --lens, -l         Review focus (default: "full")
  --preset           LLM preset to use (default: configured in setup)
  --summarize-preset LLM preset for per-file summarisation in large diffs

DEPENDENCIES:
  lib/diff.star - Shared diff analysis and map-reduce infrastructure
"""
# ==============================================================================

load("//lib/diff.star", "build_analysis_prompt")
load("//lib/help.star", "build_choices_desc", "build_preset_desc")
load("//lib/ui_helpers.star", "make_markdown_stream_handler")

_SYSTEM_PROMPT = """You are a meticulous senior software engineer performing a code review.
Goal: Provide clear, actionable feedback that improves code quality before it is committed.
Rules:
- Be specific: reference file names, function names, and line context when possible.
- Be constructive: explain *why* something is a problem and suggest a concrete fix.
- Prioritise findings: lead with critical issues, then moderate, then minor/nits.
- Be concise: avoid padding. Every sentence should add value.
- Format: use markdown headings, bullet lists, and code blocks for readability."""

# =============================================================================
# Built-in Lenses
# =============================================================================

_LENSES = {
    "full": {
        "name": "Full Review",
        "description": "Correctness, security, performance, style — everything",
        "focus": """Perform a comprehensive review covering all of the following dimensions:
1. **Correctness** — logic bugs, wrong assumptions, edge cases, error handling, concurrency.
2. **Security** — injection risks, secret exposure, auth/authz gaps, unsafe deserialization.
3. **Performance** — unnecessary allocations, O(n²) patterns, blocking calls, excessive I/O.
4. **Style & Readability** — naming, comments, idiomatic patterns, dead code.

For each issue found, state: severity (critical / moderate / minor), location, problem, and suggested fix.""",
    },
    "security": {
        "name": "Security Review",
        "description": "Threat model, injection, secrets, auth/authz, unsafe ops",
        "focus": """Focus exclusively on security concerns:
- Injection vulnerabilities (SQL, shell, template, path traversal).
- Hardcoded secrets, credentials, or tokens.
- Authentication and authorisation gaps or bypasses.
- Insecure deserialization or unsafe type assertions.
- Exposed sensitive data in logs or error messages.
- Cryptographic weaknesses (weak algorithms, improper key handling).
- Race conditions or TOCTOU issues.

Rate each finding: critical / high / medium / low. Provide a concrete remediation for each.""",
    },
    "performance": {
        "name": "Performance Review",
        "description": "Complexity, allocations, hot paths, I/O, caching",
        "focus": """Focus exclusively on performance concerns:
- Algorithmic complexity (identify O(n²) or worse where O(n log n) or better is feasible).
- Unnecessary memory allocations or copies in hot paths.
- Blocking or synchronous I/O where async or batched alternatives exist.
- Missing caching for repeated expensive computations.
- Database query patterns (N+1, missing indices, over-fetching).
- Unnecessary serialisation/deserialisation cycles.

For each issue: estimate the impact (high / medium / low) and suggest an optimisation.""",
    },
    "correctness": {
        "name": "Correctness Review",
        "description": "Bugs, edge cases, error handling, concurrency hazards",
        "focus": """Focus exclusively on correctness concerns:
- Logic bugs and incorrect assumptions.
- Unhandled edge cases (empty input, nil/null, integer overflow, off-by-one).
- Missing or swallowed errors — every error path should be handled or explicitly justified.
- Concurrency hazards: data races, deadlocks, improper use of shared state.
- Resource leaks: file handles, connections, goroutines, memory.
- Incorrect use of third-party APIs or standard library functions.

For each finding: explain what can go wrong and provide a corrected snippet.""",
    },
    "style": {
        "name": "Style Review",
        "description": "Naming, readability, comments, idiomatic patterns, dead code",
        "focus": """Focus exclusively on style and readability:
- Naming: are identifiers clear, consistent, and idiomatic for the language?
- Comments: missing doc-comments on exported symbols, outdated or redundant comments.
- Idiomatic patterns: are language-specific idioms followed (e.g., error wrapping in Go, list comps in Python)?
- Dead code: unused imports, variables, functions, or commented-out blocks.
- Duplication: opportunities to extract reusable helpers.
- Complexity: functions that are too long or deeply nested — suggest refactors.

Keep feedback constructive. For each nit, explain the benefit of the suggested change.""",
    },
}

# =============================================================================
# Prompt Templates (map-reduce + unified, injected into build_analysis_prompt)
# =============================================================================

_TEMPLATES = {
    "map_reduce": """Review the staged changes based on these per-file summaries.

Summaries:
{summaries}

{context_block}

{lens_section}

Structure your response as:
## Summary
One-paragraph overview of what the staged changes do.

## Findings
List findings grouped by severity. For each:
- **[Severity] File / area**: description of issue and suggested fix.

## Verdict
One of: ✅ Looks good | ⚠️ Minor issues | ❌ Needs work — with a one-line rationale.
""",
    "unified": """Review the following staged git diff.

Diff:
{diff_text}

{context_block}

{lens_section}

Structure your response as:
## Summary
One-paragraph overview of what the staged changes do.

## Findings
List findings grouped by severity. For each:
- **[Severity] File / area**: description of issue and suggested fix.

## Verdict
One of: ✅ Looks good | ⚠️ Minor issues | ❌ Needs work — with a one-line rationale.
""",
}

# =============================================================================
# Setup Function
# =============================================================================

def setup(default_lens=None, preset=None, summarize_preset=None):
    """Register the `review` command.

    Args:
        default_lens:      Default review lens (default: "full").
        preset:            LLM preset for final review generation.
        summarize_preset:  LLM preset for per-file summarisation (map-reduce).
    """
    cfg_default_lens      = default_lens      if default_lens      != None else "full"
    cfg_preset            = preset            if preset            != None else ""
    cfg_summarize_preset  = summarize_preset  if summarize_preset  != None else ""

    # -------------------------------------------------------------------------

    def build_lens_section(lens):
        """Return the focus instructions for the chosen lens."""
        if lens in _LENSES:
            l = _LENSES[lens]
            return "### REVIEW LENS: {}\n\n{}".format(l["name"], l["focus"])
        return ""

    def build_lens_desc(default_lens):
        return build_choices_desc(
            "Review focus (one of):",
            _LENSES,
            default_lens,
            lambda name, l: l["description"],
        )

    # -------------------------------------------------------------------------

    def generate_review(ctx, intent="", lens="full", preset_arg="", summarize_preset_arg=""):
        """Core review logic: diff → analysis prompt → LLM → stream output."""
        turn = ctx.ui.assistant_turn()

        # Fetch staged diff
        diff = ctx.git.diff(target="staged")
        if len(diff.files) == 0:
            turn.fail("No staged changes found. Stage some files with `git add` first.")
            return None

        lens_section = build_lens_section(lens)

        template_vars = {
            "context_block": "### Developer Context:\n{}".format(intent) if intent else "",
            "lens_section": lens_section,
        }

        # build_analysis_prompt handles the map-reduce vs unified decision and
        # shows intermediate UI steps inside `turn`.
        prompt = build_analysis_prompt(
            ctx,
            turn,
            diff.files,
            "staged",
            template_vars,
            _TEMPLATES,
            summarize_preset_arg,
        )

        # Final generation step
        gen_step = turn.step("Reviewing staged changes")
        gen_step.info("Lens: {}  |  Files: {}  |  +{} -{}".format(
            lens, len(diff.files), diff.additions, diff.deletions,
        ))

        on_event = make_markdown_stream_handler(turn)
        review_text = ctx.llm.chat(
            preset=preset_arg,
            system=_SYSTEM_PROMPT,
            prompt=prompt,
            stream=True,
            on_event=on_event,
            use_session=False,
        )
        gen_step.done()
        turn.done()

        ctx.output.writeline(review_text)
        return review_text

    # -------------------------------------------------------------------------

    def merge_intent_with_stdin(ctx, intent):
        """Combine --intent flag value with anything piped via stdin."""
        full_intent = intent or ""
        if ctx.stdin.is_piped():
            stdin_content = ctx.stdin.read().strip()
            if stdin_content:
                full_intent = (full_intent + "\n\n" + stdin_content) if full_intent else stdin_content
        return full_intent

    def handle_review(ctx):
        full_intent = merge_intent_with_stdin(ctx, ctx.intent)
        return generate_review(
            ctx,
            intent=full_intent,
            lens=ctx.lens,
            preset_arg=ctx.preset,
            summarize_preset_arg=ctx.summarize_preset,
        )

    # -------------------------------------------------------------------------

    review_command = meow.tool(
        name="review",
        description="Review staged git changes with AI-powered analysis and feedback.",
        params={
            "intent": meow.param(
                "string",
                default="",
                short="i",
                desc="Context or reasoning behind the staged changes (supports piped input).",
            ),
            "lens": meow.param(
                "string",
                default=cfg_default_lens,
                short="l",
                choices=_LENSES,
                desc=build_lens_desc(cfg_default_lens),
            ),
            "preset": meow.param(
                "string",
                default=cfg_preset,
                choices=meow.presets(),
                desc=build_preset_desc(cfg_preset),
            ),
            "summarize_preset": meow.param(
                "string",
                default=cfg_summarize_preset,
                choices=meow.presets(),
                desc=build_preset_desc(cfg_summarize_preset),
            ),
        },
        handler=handle_review,
    )
    meow.command(review_command)
