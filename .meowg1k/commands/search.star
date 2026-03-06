# ==============================================================================
# Search Commands - Semantic Search, RAG, and Index Management
# ==============================================================================
"""
Semantic code search and question answering using vector embeddings.

FEATURES:
  - Semantic Search: Find code by meaning, not just keywords
  - RAG-powered Q&A: Ask questions about your codebase with AI
  - Multi-snapshot: Search across HEAD, stage, and working directory
  - Smart Chunking: Automatic code splitting with context preservation
  - Index Management: Efficient incremental updates with deduplication
  - Relevance Scoring: Threshold-based filtering for quality results
  - Content Preview: Inline snippet preview in search results

INSTALLATION:
  # In your .meowg1k/init.star
  load("//commands/search.star", "setup")

  setup(
      # Optional: Configure defaults
      default_limit = 10,
      default_threshold = 0.3,
      default_context_size = 8,
      default_preset = "smart"
  )

USAGE:
  # Semantic search
  meow search "authentication middleware"

  # Ask questions with RAG
  meow ask "How does the auth system work?"
  meow ask "What testing frameworks are used?"

  # Build/rebuild index
  meow index

  # Advanced search options
  meow search "database queries" --limit 20 --threshold 0.7
  meow search "API handlers" --snapshots workdir,HEAD

  # Pipeline usage (stdin)
  echo "authentication middleware" | meow search
  echo "How does auth work?" | meow ask

COMMANDS:
  search   Semantic code search
  ask      Ask questions about codebase using RAG
  index    Build or rebuild search index

EXAMPLES:
  # Search working directory
  meow search "HTTP request handlers"

  # Search specific snapshots
  meow search "validation logic" --snapshots HEAD,stage

  # With threshold filter
  meow search "caching" --threshold 0.75

  # Full content output
  meow search "tests" --full

  # JSON output
  meow search "config" --format json

  # Ask with more context
  meow ask "Explain the caching strategy" --context-size 12

  # Show retrieved context
  meow ask "How is authentication implemented?" --show-context

PARAMETERS:
  Search:
    query              Search query string (required, min 3 chars, supports stdin)
    --limit, -n        Maximum results to return (default: 10)
    --snapshots, -s    Comma-separated snapshots (default: "workdir")
    --threshold, -t    Minimum similarity score 0.0-1.0 (default: 0.3)
    --format, -f       Output format: "text" or "json" (default: "text")
    --full             Include full content in output
    --preview          Characters of inline content preview (default: 120, 0=off)

  Ask:
    question           Question to ask (required, min 3 chars, supports stdin)
    --preset, -p       LLM preset: "fast" or "smart" (default: "smart")
    --context-size, -n Number of code snippets to retrieve (default: 8)
    --threshold, -t    Minimum similarity score (default: 0.5)
    --snapshots, -s    Comma-separated snapshots (default: "workdir")
    --show-context     Display retrieved context snippets in output

  Index:
    No parameters - rebuilds entire index

HOW IT WORKS:
  1. Indexing: Scans repository, chunks code files, generates embeddings
  2. Deduplication: Uses content hashes to avoid reprocessing identical files
  3. Vector Search: HNSW index for fast similarity search
  4. RAG: Retrieves relevant code snippets, provides to LLM as context
  5. Incremental: Only processes new/changed files on re-index

NOTES:
  - First run requires 'meow index' to build search database
  - Supports multiple git snapshots (HEAD, stage, workdir) for comprehensive search
  - Embeddings cached by content hash for efficiency
  - Default ignore patterns exclude node_modules, .git, build artifacts
  - Relevance threshold: 0.5 works well for most cases, adjust as needed
"""
# ==============================================================================

load("//lib/help.star", "build_choices_desc", "build_preset_desc")
load("//lib/ui_helpers.star", "make_markdown_stream_handler")

# ==============================================================================
# Constants
# ==============================================================================

_DEFAULT_SEARCH_RESULTS = 10
_DEFAULT_RAG_RESULTS = 8
_DEFAULT_SIMILARITY_THRESHOLD = 0.5
_DEFAULT_SEARCH_THRESHOLD = 0.3
_DEFAULT_PREVIEW_CHARS = 120
_DEFAULT_BATCH_SIZE = 20
_DEFAULT_SNAPSHOTS = "workdir"
_DEFAULT_IGNORE_PATTERNS = [
    # Version control
    ".git/**",
    # Tool configuration / project-specific
    ".meowg1k/**",
    # Dependencies
    "**/node_modules/**",
    "**/vendor/**",
    # Python
    "**/*.pyc",
    "**/__pycache__/**",
    # Environment / secrets
    ".env",
    ".env.*",
    # Lock files and checksums
    "**/*.lock",
    "**/*.sum",
    # OS noise
    "**/.DS_Store",
    "**/Thumbs.db",
    # Build output
    "bin/**",
    "**/dist/**",
    "**/build/**",
    "**/out/**",
    # Test / coverage artifacts
    "coverage.out",
    "coverage.html",
    # IDE / editor
    ".idea/**",
    ".vscode/**",
    # Minified assets
    "**/*.min.js",
    "**/*.min.css",
    # Generated code
    "**/*.pb.go",
    "**/*.gen.go",
]

_SYSTEM_PROMPT_ASK = """You are an expert AI programming assistant with deep knowledge of software engineering.

Your role:
- Answer questions about code based on retrieved context
- Provide accurate, detailed technical explanations
- Reference specific code snippets when relevant
- Explain WHY and HOW, not just WHAT

Guidelines:
- Ground all answers in the provided code context
- Cite file names when referencing specific code
- If context is insufficient, clearly state limitations
- Use technical terminology appropriately
"""

# =============================================================================
# Text Chunking (for indexing)
# =============================================================================

def _split_by_lines(text, max_lines=100, overlap_lines=10):
    """Split text into chunks by line count with overlap."""
    lines = text.split("\n")
    chunks = []
    step = max_lines - overlap_lines
    if step <= 0:
        step = max_lines

    num_lines = len(lines)
    starts = range(0, num_lines, step)

    for i in starts:
        end = min(i + max_lines, num_lines)
        chunk_lines = lines[i:end]
        chunk_text = "\n".join(chunk_lines)
        chunks.append({
            "text": chunk_text,
            "start_line": i + 1,
            "end_line": end,
        })

    return chunks

def _split_by_chars(text, max_chars=4000, overlap_chars=200):
    """Split text into chunks by character count with overlap."""
    chunks = []
    text_len = len(text)
    step = max_chars - overlap_chars
    if step <= 0:
        step = max_chars

    starts = range(0, text_len, step)

    for i in starts:
        end = min(i + max_chars, text_len)
        chunk_text = text[i:end]
        lines_before = text[:i].count("\n")
        lines_in_chunk = chunk_text.count("\n")
        chunks.append({
            "text": chunk_text,
            "start_line": lines_before + 1,
            "end_line": lines_before + lines_in_chunk + 1,
        })

    return chunks

def _split_by_paragraphs(text, max_chars=4000):
    """Split text by paragraphs, grouping them into chunks under max_chars."""
    paragraphs = text.split("\n\n")
    chunks = []
    current_chunk = []
    current_size = 0
    line_offset = 0

    for para in paragraphs:
        para_size = len(para)
        if current_size + para_size > max_chars and current_chunk:
            chunk_text = "\n\n".join(current_chunk)
            start_line = line_offset + 1
            end_line = start_line + chunk_text.count("\n")
            chunks.append({
                "text": chunk_text,
                "start_line": start_line,
                "end_line": end_line,
            })
            line_offset = end_line
            current_chunk = []
            current_size = 0

        current_chunk.append(para)
        current_size += para_size + 2

    if current_chunk:
        chunk_text = "\n\n".join(current_chunk)
        start_line = line_offset + 1
        end_line = start_line + chunk_text.count("\n")
        chunks.append({
            "text": chunk_text,
            "start_line": start_line,
            "end_line": end_line,
        })

    return chunks

def _chunk_file(content, path, strategy="lines"):
    """Chunk file content using the specified strategy."""
    if strategy == "chars":
        return _split_by_chars(content, max_chars=4000, overlap_chars=200)
    elif strategy == "paragraphs":
        return _split_by_paragraphs(content, max_chars=4000)
    else:
        return _split_by_lines(content, max_lines=100, overlap_lines=10)

# =============================================================================
# Output Formatting Helpers
# =============================================================================

def _truncate(text, max_chars):
    """Truncate text to max_chars, replacing newlines with spaces."""
    # Collapse whitespace for inline display
    collapsed = " ".join(text.split())
    if len(collapsed) <= max_chars:
        return collapsed
    return collapsed[:max_chars - 1] + "…"

def _format_score(score):
    """Format a similarity score as a percentage string."""
    return "{}%".format(int(score * 100))

def _format_location(file_path, start_line, end_line):
    """Format a file location as file:start-end."""
    return "{}:{}-{}".format(file_path, start_line, end_line)

def _print_separator(ctx, char="-", width=72):
    """Print a separator line."""
    ctx.output.writeline(char * width)

def _print_result_text(ctx, r, index, preview_chars):
    """Print a single search result in text format."""
    location = _format_location(r.file_path, r.start_line, r.end_line)
    score = _format_score(r.score)
    ctx.output.writeline("[{}] {} ({})".format(index, location, score))
    if preview_chars > 0 and r.content:
        preview = _truncate(r.content, preview_chars)
        ctx.output.writeline("    {}".format(preview))

def _print_result_full(ctx, r, index):
    """Print a single search result with full content."""
    location = _format_location(r.file_path, r.start_line, r.end_line)
    score = _format_score(r.score)
    ctx.output.writeline("[{}] {} ({})".format(index, location, score))
    ctx.output.writeline("")
    ctx.output.writeline(r.content)
    ctx.output.writeline("")

# =============================================================================
# UI Helpers
# =============================================================================

def _display_file_stats(ctx, files, title="Changed Files"):
    """Display file statistics in table or summary format."""
    if not files:
        return
    if len(files) <= 15:
        rows = [{"file": f} for f in files]
        ctx.ui.table(rows, columns=["file"], title=title)

# =============================================================================
# Setup Function
# =============================================================================

def setup(default_limit=None, default_threshold=None, default_ask_threshold=None, default_context_size=None, default_preset=None, default_format=None, default_snapshots=None, default_preview=None, ignore_patterns=None):
    """Configure the search commands.

    Args:
        default_limit: Default number of search results.
        default_threshold: Default similarity threshold for search.
        default_ask_threshold: Default similarity threshold for ask.
        default_context_size: Default context size for ask.
        default_preset: Default LLM preset for ask.
        default_format: Default output format ("text" or "json").
        default_snapshots: Default snapshots to search (comma-separated).
        default_preview: Default preview characters (0 to disable).
        ignore_patterns: List of glob patterns to ignore during indexing.
    """
    cfg_default_limit = default_limit if default_limit != None else _DEFAULT_SEARCH_RESULTS
    cfg_default_threshold = default_threshold if default_threshold != None else _DEFAULT_SEARCH_THRESHOLD
    cfg_default_ask_threshold = default_ask_threshold if default_ask_threshold != None else _DEFAULT_SIMILARITY_THRESHOLD
    cfg_default_context_size = default_context_size if default_context_size != None else _DEFAULT_RAG_RESULTS
    cfg_default_preset = default_preset if default_preset != None else "smart"
    cfg_default_format = default_format if default_format != None else "text"
    cfg_default_snapshots = default_snapshots if default_snapshots != None else _DEFAULT_SNAPSHOTS
    cfg_default_preview = default_preview if default_preview != None else _DEFAULT_PREVIEW_CHARS
    cfg_ignore_patterns = ignore_patterns if ignore_patterns != None else list(_DEFAULT_IGNORE_PATTERNS)

    def _embed_query(ctx, query, preset="embeddings"):
        """Embed a query string and return the embedding vector."""
        embeddings = ctx.llm.embed(texts=[query], preset=preset)
        return embeddings[0]

    def semantic_search(ctx, query, snapshots=_DEFAULT_SNAPSHOTS, limit=_DEFAULT_SEARCH_RESULTS, threshold=_DEFAULT_SEARCH_THRESHOLD, format="text", full=False, preview=_DEFAULT_PREVIEW_CHARS):
        """Perform semantic code search."""
        if not query:
            turn = ctx.ui.assistant_turn()
            turn.fail("Query required")
            return []

        turn = ctx.ui.assistant_turn()
        search_step = turn.step("Searching")
        search_step.info("Query: '{}'".format(query))

        snapshot_list = [s.strip() for s in snapshots.split(",")]

        embedding = _embed_query(ctx, query)
        results = ctx.index.search(
            embedding=embedding,
            snapshots=snapshot_list,
            top_k=limit,
            min_score=threshold
        )

        if not results or len(results) == 0:
            search_step.fail("No results")
            turn.warn("Try lowering --threshold (current: {})".format(threshold))
            turn.done()
            return []

        search_step.done("{} matches".format(len(results)))

        if format == "json":
            ctx.output.writeline(ctx.json.stringify(results))
        elif full:
            ctx.output.writeline("")
            ctx.output.writeline("Search results for '{}' ({} matches):".format(query, len(results)))
            _print_separator(ctx)
            for i, r in enumerate(results):
                _print_result_full(ctx, r, i + 1)
                if i < len(results) - 1:
                    _print_separator(ctx, char="·")
                    ctx.output.writeline("")
            _print_separator(ctx)
        else:
            # TUI table for interactive sessions
            table_data = []
            for r in results:
                table_data.append({
                    "File": r.file_path,
                    "Lines": "{}-{}".format(r.start_line, r.end_line),
                    "Score": _format_score(r.score),
                })
            ctx.ui.table(table_data, columns=["File", "Lines", "Score"], title="Results for '{}'".format(query))

            # Persistent output with preview
            ctx.output.writeline("")
            ctx.output.writeline("Search results for '{}' ({} matches):".format(query, len(results)))
            _print_separator(ctx)
            for i, r in enumerate(results):
                _print_result_text(ctx, r, i + 1, preview)
            _print_separator(ctx)

        turn.done()
        return results

    def ask_question(ctx, question, preset="smart", snapshots=_DEFAULT_SNAPSHOTS, context_size=_DEFAULT_RAG_RESULTS, threshold=_DEFAULT_SIMILARITY_THRESHOLD, show_context=False):
        """Ask questions about codebase using RAG."""
        if not question:
            turn = ctx.ui.assistant_turn()
            turn.fail("Question required")
            return None

        turn = ctx.ui.assistant_turn()

        rag_step = turn.step("Retrieving Context")
        snapshot_list = [s.strip() for s in snapshots.split(",")]

        embedding = _embed_query(ctx, question)
        results = ctx.index.search(
            embedding=embedding,
            snapshots=snapshot_list,
            top_k=context_size,
            min_score=threshold
        )

        if not results or len(results) == 0:
            rag_step.fail("No context found")
            turn.warn("Try lowering --threshold (current: {}) or run 'meow index' first".format(threshold))
            turn.done()
            return None

        rag_step.done("{} snippets from {} file(s)".format(
            len(results),
            len({r.file_path: True for r in results})
        ))

        context_parts = []
        seen_files = {}

        for r in results:
            if r.file_path not in seen_files:
                seen_files[r.file_path] = 0
            seen_files[r.file_path] += 1

            context_parts.append(
                "### File: {} (lines {}-{})\n```\n{}\n```".format(
                    r.file_path, r.start_line, r.end_line, r.content
                )
            )

        context = "\n\n".join(context_parts)

        if show_context:
            ctx.ui.markdown("**Retrieved Context:**")
            file_list = list(seen_files.keys())
            _display_file_stats(ctx, file_list, title="Source Files")
            # Also write context to persistent output
            ctx.output.writeline("")
            ctx.output.writeline("Retrieved context ({} snippets from {} file(s)):".format(len(results), len(seen_files)))
            _print_separator(ctx)
            for i, r in enumerate(results):
                _print_result_full(ctx, r, i + 1)
            _print_separator(ctx)
            ctx.output.writeline("")

        prompt = """Question: {}

Retrieved Code Context:
{}

Please provide a comprehensive answer based on the code context above. Reference specific files and code snippets when relevant.""".format(
            question, context
        )

        ans_step = turn.step("Generating Answer")
        ans_step.info("{} snippets from {} files".format(len(results), len(seen_files)))

        on_event = make_markdown_stream_handler(turn)
        answer = ctx.llm.chat(
            preset=preset,
            system=_SYSTEM_PROMPT_ASK,
            prompt=prompt,
            stream=True,
            on_event=on_event,
        )
        ans_step.done()

        # TUI: render references table
        source_rows = []
        for r in results:
            source_rows.append({
                "File": r.file_path,
                "Lines": "{}-{}".format(r.start_line, r.end_line),
                "Relevance": _format_score(r.score),
            })
        ctx.ui.table(source_rows, columns=["File", "Lines", "Relevance"], title="References")

        turn.done()

        # Persistent output: answer + references
        ctx.output.writeline("")
        ctx.output.writeline(answer)
        ctx.output.writeline("")
        _print_separator(ctx)
        ctx.output.writeline("References ({} snippets):".format(len(results)))
        for r in results:
            ctx.output.writeline("  {} ({})".format(
                _format_location(r.file_path, r.start_line, r.end_line),
                _format_score(r.score),
            ))
        _print_separator(ctx)

        return {"answer": answer, "sources": results}

    def rebuild_index(ctx, custom_ignore_patterns=None, batch_size=None, chunking_strategy="lines"):
        """Rebuild search index from scratch."""
        if batch_size == None:
            batch_size = _DEFAULT_BATCH_SIZE
        if custom_ignore_patterns == None:
            custom_ignore_patterns = cfg_ignore_patterns

        ctx.ui.banner("Rebuilding Search Index")
        turn = ctx.ui.assistant_turn()

        __cleanup_snapshots(ctx, turn)
        file_counts = __scan_workspace(ctx, turn, custom_ignore_patterns)
        dedup_result = __deduplicate_files(ctx, turn, file_counts)

        if len(dedup_result["new_files"]) > 0:
            __process_new_files(ctx, turn, dedup_result["new_files"], dedup_result["existing_versions"], batch_size, chunking_strategy)

        __link_snapshots(ctx, turn, dedup_result["snapshot_map"], dedup_result["existing_versions"])
        __build_vector_indices(ctx, turn)

        # Summary
        total_unique = len(dedup_result["existing_versions"])
        new_count = len(dedup_result["new_files"])
        cached_count = total_unique - new_count
        turn.info("Done: {} files ({} new, {} cached)".format(total_unique, new_count, cached_count))
        turn.done()

        ctx.output.writeline("")
        ctx.output.writeline("Index rebuilt: {} files ({} new, {} cached)".format(total_unique, new_count, cached_count))

    def handle_search(ctx):
        return semantic_search(
            ctx,
            query=ctx.query,
            snapshots=ctx.snapshots,
            limit=ctx.limit,
            threshold=ctx.threshold,
            format=ctx.format,
            full=ctx.full,
            preview=ctx.preview,
        )

    def handle_ask(ctx):
        return ask_question(
            ctx,
            question=ctx.question,
            preset=ctx.preset,
            snapshots=ctx.snapshots,
            context_size=ctx.context_size,
            threshold=ctx.threshold,
            show_context=ctx.show_context
        )

    def handle_index(ctx):
        rebuild_index(ctx)

    def build_format_desc(default_format):
        formats = {
            "text": "Human-readable formatted output",
            "json": "Machine-readable JSON output"
        }
        return build_choices_desc("Output format (one of):", formats, default_format)

    def build_snapshots_desc():
        lines = ["Snapshots to search (comma-separated):"]
        lines.append("  workdir: Current working directory (default)")
        lines.append("  stage: Staged changes")
        lines.append("  HEAD: Latest commit")
        return "\n".join(lines)

    search_command = meow.tool(
        name="search",
        description="Semantic code search",
        params={
            "query": meow.param("string", required=True, from_stdin=True, min_len=3, short="q", desc="Search query."),
            "limit": meow.param("int", default=cfg_default_limit, short="n", desc="Maximum results to return."),
            "snapshots": meow.param("string", default=cfg_default_snapshots, short="s", desc=build_snapshots_desc()),
            "threshold": meow.param("float", default=cfg_default_threshold, short="t", desc="Minimum similarity score (0.0-1.0)."),
            "format": meow.param("string", default=cfg_default_format, choices=["text", "json"], short="f", desc=build_format_desc(cfg_default_format)),
            "full": meow.param("bool", default=False, desc="Include full content in output."),
            "preview": meow.param("int", default=cfg_default_preview, desc="Characters of inline preview per result (0 to disable)."),
        },
        handler=handle_search
    )
    meow.command(search_command)

    ask_command = meow.tool(
        name="ask",
        description="Ask questions about codebase using RAG",
        params={
            "question": meow.param("string", required=True, from_stdin=True, min_len=3, short="q", desc="Question to ask about the codebase."),
            "preset": meow.param("string", default=cfg_default_preset, short="p", choices=meow.presets(), desc=build_preset_desc(cfg_default_preset)),
            "context_size": meow.param("int", default=cfg_default_context_size, short="n", desc="Number of code snippets to retrieve."),
            "threshold": meow.param("float", default=cfg_default_ask_threshold, short="t", desc="Minimum similarity score (0.0-1.0)."),
            "snapshots": meow.param("string", default=cfg_default_snapshots, short="s", desc=build_snapshots_desc()),
            "show_context": meow.param("bool", default=False, short="v", desc="Display retrieved context snippets in output.")
        },
        handler=handle_ask
    )
    meow.command(ask_command)

    index_command = meow.tool(
        name="index",
        description="Build or rebuild search index",
        params={},
        handler=handle_index
    )
    meow.command(index_command)

# ==============================================================================
# Index Helper Functions
# ==============================================================================

def __cleanup_snapshots(ctx, turn):
    """Clear old snapshot links."""
    step = turn.step("Clearing old index data")
    ctx.index.clear_snapshot("HEAD")
    ctx.index.clear_snapshot("stage")
    ctx.index.clear_snapshot("workdir")
    step.done("Cleared")

def __scan_workspace(ctx, turn, ignore_patterns):
    """Scan all snapshots and return file lists."""
    step = turn.step("Scanning workspace files")

    head_files = ctx.git.glob(ref="HEAD", pattern="**/*", ignore=ignore_patterns)
    stage_files = ctx.git.glob(ref="stage", pattern="**/*", ignore=ignore_patterns)
    workdir_files = ctx.fs.glob(pattern="**/*", ignore=ignore_patterns)

    total = len(head_files) + len(stage_files) + len(workdir_files)
    step.info("HEAD: {} | Stage: {} | Working: {}".format(len(head_files), len(stage_files), len(workdir_files)))
    step.done("{} total files".format(total))

    return {"HEAD": head_files, "stage": stage_files, "workdir": workdir_files}

def __deduplicate_files(ctx, turn, file_counts):
    """Deduplicate files by content hash."""
    step = turn.step("Deduplicating by content hash")

    file_map = {}
    snapshot_map = {"HEAD": [], "stage": [], "workdir": []}

    for path in file_counts["HEAD"]:
        content = ctx.git.read("HEAD", path)
        content_hash = ctx.crypto.sha256(content)
        if content_hash not in file_map:
            file_map[content_hash] = {"path": path, "content": content, "ref": "HEAD"}
        snapshot_map["HEAD"].append(content_hash)

    for path in file_counts["stage"]:
        content = ctx.git.read("stage", path)
        content_hash = ctx.crypto.sha256(content)
        if content_hash not in file_map:
            file_map[content_hash] = {"path": path, "content": content, "ref": "stage"}
        snapshot_map["stage"].append(content_hash)

    for path in file_counts["workdir"]:
        content = ctx.fs.read(path)
        content_hash = ctx.crypto.sha256(content)
        if content_hash not in file_map:
            file_map[content_hash] = {"path": path, "content": content, "ref": "workdir"}
        snapshot_map["workdir"].append(content_hash)

    all_hashes = list(file_map.keys())
    existing_versions = ctx.index.find_versions(all_hashes)

    new_files = []
    for content_hash, file_info in file_map.items():
        if existing_versions[content_hash] == None:
            new_files.append({"hash": content_hash, "path": file_info["path"], "content": file_info["content"]})

    new_files = sorted(new_files, key=lambda f: f["path"])

    unique_files = len(all_hashes)
    cached_count = unique_files - len(new_files)
    step.info("{} unique | {} new | {} cached".format(unique_files, len(new_files), cached_count))
    step.done("{} unique files".format(unique_files))

    return {"new_files": new_files, "existing_versions": existing_versions, "snapshot_map": snapshot_map}

def __process_new_files(ctx, turn, new_files, existing_versions, batch_size, chunking_strategy):
    """Chunk, embed, and save new files incrementally."""
    step = turn.step("Processing new files")

    total_chunks = 0
    for file_info in new_files:
        chunks = _chunk_file(file_info["content"], file_info["path"], strategy=chunking_strategy)
        file_info["chunks"] = chunks
        total_chunks += len(chunks)
    step.info("{} files → {} chunks".format(len(new_files), total_chunks))
    step.done()

    sub = turn.subturn("Computing embeddings")
    __process_files_incrementally(ctx, sub, new_files, existing_versions, batch_size, total_chunks)
    sub.done()

def __process_files_incrementally(ctx, turn, new_files, existing_versions, batch_size, total_chunks):
    """Process files incrementally with batched embeddings and progress bar."""
    step = turn.step("Batch size: {}".format(batch_size))

    chunk_queue = []
    for file_info in new_files:
        for chunk in file_info["chunks"]:
            chunk_queue.append({"text": chunk["text"], "file": file_info, "chunk_data": chunk})

    total = len(chunk_queue)
    processed = 0
    saved_files = 0
    num_batches = (total + batch_size - 1) // batch_size
    batch_num = 0

    # Progress bar for embedding batches
    progress = ctx.ui.progress_bar(num_batches, message="Embedding batches")

    for batch_start in range(0, total, batch_size):
        batch_num += 1
        batch_end = min(batch_start + batch_size, total)
        batch_items = chunk_queue[batch_start:batch_end]

        batch_texts = [item["text"] for item in batch_items]
        embeddings = ctx.llm.embed(texts=batch_texts, preset="embeddings")
        step.update("Batch {}/{}".format(batch_num, num_batches))
        progress.inc()

        for i, item in enumerate(batch_items):
            item["embedding"] = embeddings[i]

        processed += len(batch_items)

        files_to_save = __collect_completed_files(batch_items, chunk_queue[:batch_end])
        for file_info in files_to_save:
            if existing_versions[file_info["hash"]] == None:
                __save_single_file(ctx, file_info, existing_versions)
                saved_files += 1

    progress.done("{} batches complete".format(num_batches))
    step.done("{}/{} files embedded".format(saved_files, len(new_files)))

def __collect_completed_files(batch_items, all_processed):
    """Find files where all chunks have been embedded."""
    files_in_batch = {}
    for item in batch_items:
        file_hash = item["file"]["hash"]
        if file_hash not in files_in_batch:
            files_in_batch[file_hash] = item["file"]

    completed = []
    for file_hash, file_info in files_in_batch.items():
        expected_chunks = len(file_info["chunks"])
        embedded_count = 0
        for item in all_processed:
            if item["file"]["hash"] == file_hash and "embedding" in item:
                embedded_count += 1

        if embedded_count == expected_chunks:
            file_embeddings = []
            for item in all_processed:
                if item["file"]["hash"] == file_hash:
                    file_embeddings.append(item["embedding"])
            file_info["embeddings"] = file_embeddings
            completed.append(file_info)

    return completed

def __save_single_file(ctx, file_info, existing_versions):
    """Save a single file with its chunks and embeddings."""
    version_id = ctx.index.save_version(
        path=file_info["path"],
        content=file_info["content"],
        content_hash=file_info["hash"],
        chunks=file_info["chunks"],
        embeddings=file_info["embeddings"],
    )
    existing_versions[file_info["hash"]] = version_id

def __link_snapshots(ctx, turn, snapshot_map, existing_versions):
    """Link versions to snapshots."""
    step = turn.step("Linking files to snapshots")

    for snapshot_name, hash_list in snapshot_map.items():
        version_ids = []
        for content_hash in hash_list:
            version_id = existing_versions[content_hash]
            if version_id != None:
                version_ids.append(version_id)
        ctx.index.link_snapshot(snapshot_name, version_ids)
    step.done("3 snapshots linked")

def __build_vector_indices(ctx, turn):
    """Build HNSW vector indices for all snapshots."""
    step = turn.step("Building vector search indices")

    ctx.index.build_vector_index("HEAD")
    ctx.index.build_vector_index("stage")
    ctx.index.build_vector_index("workdir")
    step.done("Search index ready")
