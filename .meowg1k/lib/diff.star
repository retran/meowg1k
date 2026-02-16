# ==============================================================================
# Diff Analysis Library (lib/diff.star)
# ==============================================================================
# Adaptive diff analysis strategies for commit and PR generation.
#
# Features:
#   - Automatic strategy selection (map-reduce vs unified)
#   - File-by-file summarization for large changesets
#   - Memory-efficient caching of fetched diffs
#   - Configurable size thresholds
#   - Clean, structured output following terminal UI design system
#
# Public API:
#   - build_analysis_prompt(ctx, files, target, template_vars, templates)
# ==============================================================================

_MAX_UNIFIED_DIFF_CHARS = 50000
_MAX_FILE_DIFF_CHARS = 8000
_MULTI_FILE_ANALYSIS_THRESHOLD = 5
_LARGE_DIFF_CHARS_THRESHOLD = 35000

def _smart_truncate(text, max_chars, suffix="\n\n... (truncated)"):
    """Truncate text to max_chars, adding suffix if truncated."""
    if len(text) <= max_chars:
        return text
    truncation_point = max_chars - len(suffix)
    return text[:truncation_point] + suffix if truncation_point > 0 else suffix

_FILE_SUMMARY_PROMPT = """Summarize this file change in exactly 1-2 sentences.
Rules:
- State WHAT changed, not WHY or HOW.
- Do not speculate about intent or motivation.
- If the file was deleted, say only that it was removed and why in ≤6 words.
- No filler phrases like "This change...", "The file...", "This PR...".

File: {file_path}
Changes:
{diff_content}"""

def _summarize_file_changes(ctx, file_path, diff_content, preset=""):
    """Generate a concise 1-2 sentence summary of changes in a single file."""
    prompt = _FILE_SUMMARY_PROMPT.format(
        file_path=file_path,
        diff_content=_smart_truncate(diff_content, _MAX_FILE_DIFF_CHARS)
    )
    return ctx.llm.generate(preset=preset, prompt=prompt)

def _should_use_map_reduce(file_count, total_chars):
    """Decide whether to use map-reduce based on size metrics."""
    return (
        file_count >= _MULTI_FILE_ANALYSIS_THRESHOLD or
        total_chars > _LARGE_DIFF_CHARS_THRESHOLD
    )

def _clean_summary(text):
    """Clean LLM summary output - strip quotes and normalize whitespace."""
    text = text.strip()
    # Strip leading/trailing quotes if present
    if (text.startswith('"') and text.endswith('"')) or (text.startswith("'") and text.endswith("'")):
        text = text[1:-1].strip()
    return text

def _format_size(chars):
    """Format character count as human-readable size."""
    if chars < 1000:
        return "{} chars".format(chars)
    elif chars < 10000:
        # 1 decimal place, no format specifiers
        k = chars / 1000.0
        k_rounded = int(k * 10 + 0.5) / 10.0
        # Remove trailing .0 for whole numbers
        if k_rounded == int(k_rounded):
            return "{}K chars".format(int(k_rounded))
        else:
            return "{}K chars".format(k_rounded)
    else:
        return "{}K chars".format(int(chars / 1000))

def _format_line_stats(additions, deletions):
    """Format +/- line stats."""
    if additions > 0 and deletions > 0:
        return "+{}, -{}".format(additions, deletions)
    elif additions > 0:
        return "+{}".format(additions)
    elif deletions > 0:
        return "-{}".format(deletions)
    else:
        return "no changes"

def _collect_diffs(ctx, files, target):
    """
    Stage 1: Collect all file diffs.

    Returns tuple: (fetched_diffs, total_chars, use_map_reduce)
    fetched_diffs is list of (file_path, raw_diff, additions, deletions)
    """
    total_files = len(files)
    step = ctx.ui.step("Collecting Changes")

    fetched_diffs = []
    total_chars = 0
    total_additions = 0
    total_deletions = 0

    for i, file_path in enumerate(files):
        file_diff = ctx.git.diff_file(file=file_path, target=target)
        fetched_diffs.append((file_path, file_diff.raw, file_diff.additions, file_diff.deletions))
        total_chars += len(file_diff.raw)
        total_additions += file_diff.additions
        total_deletions += file_diff.deletions

        # Show file with line stats and progress
        ctx.ui.info("[{}/{}] {} ({})".format(i + 1, total_files, file_path, _format_line_stats(file_diff.additions, file_diff.deletions)))

    # Decide strategy
    use_map_reduce = _should_use_map_reduce(total_files, total_chars)
    if use_map_reduce:
        step.done("{} files (+{}, -{}) · {} → summarizing".format(total_files, total_additions, total_deletions, _format_size(total_chars)))
    else:
        step.done("{} files (+{}, -{}) · {} → unified".format(total_files, total_additions, total_deletions, _format_size(total_chars)))

    return fetched_diffs, total_chars, use_map_reduce

def _summarize_diffs(ctx, fetched_diffs, preset=""):
    """
    Stage 2: Generate and display summaries one by one.

    Returns formatted summaries for prompt.
    """
    total_files = len(fetched_diffs)
    step = ctx.ui.step("Summarizing Changes")

    file_summaries = []

    for i, (file_path, file_diff, additions, deletions) in enumerate(fetched_diffs):
        # Generate summary
        summary = _summarize_file_changes(ctx, file_path, file_diff, preset)
        clean = _clean_summary(summary)
        file_summaries.append((file_path, clean))

        # Output: filename with line stats and progress, then LLM summary
        ctx.ui.info("[{}/{}] {} ({})".format(i + 1, total_files, file_path, _format_line_stats(additions, deletions)))
        for line in clean.split("\n"):
            ctx.ui.think(line)

    step.done("{} summaries generated".format(total_files))

    # Return formatted summaries for prompt
    formatted = []
    for file_path, summary in file_summaries:
        formatted.append("**{}**\n  {}".format(file_path, summary))
    return "\n\n".join(formatted)

def _fetch_and_decide_strategy(ctx, files, target, summarize_preset=""):
    """
    Fetch all file diffs and select optimal analysis strategy.

    Args:
        ctx: Command context
        files: List of file paths
        target: Git target
        summarize_preset: Preset to use for file summarization

    Returns tuple: ("map_reduce"|"unified", content)
    Strategy selection based on file count and total size.
    """
    # Stage 1: Collect diffs (includes strategy decision)
    fetched_diffs, total_chars, use_map_reduce = _collect_diffs(ctx, files, target)

    # Execute strategy
    if use_map_reduce:
        # Stage 2: Summarize
        result = _summarize_diffs(ctx, fetched_diffs, summarize_preset)
        return ("map_reduce", result)
    else:
        concatenated = "\n".join([diff for _, diff, _, _ in fetched_diffs])
        return ("unified", concatenated)

def build_analysis_prompt(ctx, files, target, template_vars, templates, summarize_preset=""):
    """
    Build LLM prompt using adaptive analysis strategy.

    Args:
        ctx: Command context with git, llm, ui access
        files: List of file paths to analyze
        target: Git target ("staged", branch name)
        template_vars: Dict of variables to inject into templates
        templates: Dict with "map_reduce" and "unified" template strings
        summarize_preset: Preset to use for file summarization in map-reduce

    Returns:
        Formatted prompt string ready for LLM generation
    """
    strategy, content = _fetch_and_decide_strategy(ctx, files, target, summarize_preset)

    if strategy == "map_reduce":
        template_vars["summaries"] = content
        return templates["map_reduce"].format(**template_vars)
    else:
        diff_text = _smart_truncate(content, _MAX_UNIFIED_DIFF_CHARS)
        template_vars["diff_text"] = diff_text
        return templates["unified"].format(**template_vars)
