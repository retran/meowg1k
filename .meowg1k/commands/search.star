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

INSTALLATION:
  # In your .meowg1k/init.star
  load("//commands/search.star", "setup")

  setup(
      # Optional: Configure defaults
      default_limit = 10,
      default_threshold = 0.65,
      default_context_size = 8,
      default_preset = "smart"
  )

USAGE:
  # Semantic search
  meow query "authentication middleware"
  meow search "error handling patterns"  # alias for query

  # Ask questions with RAG
  meow ask "How does the auth system work?"
  meow ask "What testing frameworks are used?"

  # Build/rebuild index
  meow index

  # Advanced search options
  meow query "database queries" --limit 20 --threshold 0.7
  meow query "API handlers" --snapshots workdir,HEAD

COMMANDS:
  query    Semantic code search
  search   Alias for query
  ask      Ask questions about codebase using RAG
  index    Build or rebuild search index

EXAMPLES:
  # Search working directory
  meow query "HTTP request handlers"

  # Search specific snapshots
  meow query "validation logic" --snapshots HEAD,stage

  # With threshold filter
  meow query "caching" --threshold 0.75

  # Full content output
  meow query "tests" --full

  # JSON output
  meow query "config" --format json

  # Ask with more context
  meow ask "Explain the caching strategy" --context-size 12

  # Show retrieved context
  meow ask "How is authentication implemented?" --show-context

PARAMETERS:
  Query/Search:
    query              Search query string (required, min 3 chars, supports stdin)
    --limit, -n        Maximum results to return (default: 10)
    --snapshots, -s    Comma-separated snapshots (default: "workdir,stage,HEAD")
    --threshold, -t    Minimum similarity score 0.0-1.0 (default: 0.0)
    --format, -f       Output format: "text" or "json" (default: "text")
    --full             Include full content in output

  Ask:
    question           Question to ask (required, min 3 chars, supports stdin)
    --preset, -p       LLM preset: "fast" or "smart" (default: "smart")
    --context-size, -n Number of code snippets to retrieve (default: 8)
    --threshold, -t    Minimum similarity score (default: 0.65)
    --snapshots, -s    Comma-separated snapshots (default: "workdir,stage,HEAD")
    --show-context     Display retrieved context before answer

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
  - Relevance threshold: 0.65 works well for most cases, adjust as needed
"""
# ==============================================================================

load("//lib/help.star", "build_choices_desc", "build_preset_desc")
load("//lib/ui_helpers.star", "make_markdown_stream_handler")

# ==============================================================================
# Constants
# ==============================================================================

_DEFAULT_SEARCH_RESULTS = 10
_DEFAULT_RAG_RESULTS = 8
_DEFAULT_SIMILARITY_THRESHOLD = 0.65
_DEFAULT_BATCH_SIZE = 20
_DEFAULT_IGNORE_PATTERNS = [
    ".git/**",
    "node_modules/**",
    "**/*.pyc",
    "__pycache__/**",
    ".env",
    "*.lock",
    "**/.DS_Store",
    "**/dist/**",
    "**/build/**",
    "**/*.min.js",
    "**/*.min.css",
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

def setup(default_limit=None, default_threshold=None, default_ask_threshold=None, default_context_size=None, default_preset=None, default_format=None, default_snapshots=None, ignore_patterns=None):
    """Configure the search commands.

    Args:
        default_limit: Default number of search results.
        default_threshold: Default similarity threshold for query/search.
        default_ask_threshold: Default similarity threshold for ask.
        default_context_size: Default context size for ask.
        default_preset: Default LLM preset for ask.
        default_format: Default output format ("text" or "json").
        default_snapshots: Default snapshots to search (comma-separated).
        ignore_patterns: List of glob patterns to ignore during indexing.
    """
    cfg_default_limit = default_limit if default_limit != None else _DEFAULT_SEARCH_RESULTS
    cfg_default_threshold = default_threshold if default_threshold != None else 0.0
    cfg_default_ask_threshold = default_ask_threshold if default_ask_threshold != None else _DEFAULT_SIMILARITY_THRESHOLD
    cfg_default_context_size = default_context_size if default_context_size != None else _DEFAULT_RAG_RESULTS
    cfg_default_preset = default_preset if default_preset != None else "smart"
    cfg_default_format = default_format if default_format != None else "text"
    cfg_default_snapshots = default_snapshots if default_snapshots != None else "workdir,stage,HEAD"
    cfg_ignore_patterns = ignore_patterns if ignore_patterns != None else list(_DEFAULT_IGNORE_PATTERNS)

    def semantic_search(ctx, query, snapshots="workdir,stage,HEAD", limit=_DEFAULT_SEARCH_RESULTS, threshold=0.0, format="text", full=False):
        """Perform semantic code search."""
        if not query:
            turn = ctx.ui.assistant_turn()
            turn.fail("Query required")
            return []

        turn = ctx.ui.assistant_turn()
        search_step = turn.step("Searching")
        search_step.info("Query: '{}'".format(query))

        snapshot_list = [s.strip() for s in snapshots.split(",")]

        results = ctx.index.search(
            query=query,
            snapshots=snapshot_list,
            top_k=limit,
            min_score=threshold
        )

        if not results or len(results) == 0:
            search_step.fail("No results")
            turn.warn("Try lowering threshold (current: {:.2f})".format(threshold))
            turn.done()
            return []

        search_step.done("{} matches".format(len(results)))

        if format == "json":
            ctx.output.writeline(ctx.json.encode(results))
        else:
            for r in results:
                ctx.output.writef("%s:%d-%d [%.2f]\n", r.file_path, r.start_line, r.end_line, r.score)
                if full:
                    ctx.output.writeline(r.content)
                    ctx.output.writeline("-" * 40)

            if not full:
                table_data = []
                for r in results:
                    table_data.append({
                        "File": r.file_path,
                        "Lines": "{}-{}".format(r.start_line, r.end_line),
                        "Score": "{}%".format(int(r.score*100))
                    })
                ctx.ui.table(table_data, columns=["File", "Lines", "Score"], title="Results for '{}'".format(query))

        turn.done()
        return results

    def ask_question(ctx, question, preset="smart", snapshots="workdir,stage,HEAD", context_size=_DEFAULT_RAG_RESULTS, threshold=_DEFAULT_SIMILARITY_THRESHOLD, show_context=False):
        """Ask questions about codebase using RAG."""
        if not question:
            turn = ctx.ui.assistant_turn()
            turn.fail("Question required")
            return None

        turn = ctx.ui.assistant_turn()

        rag_step = turn.step("Retrieving Context")
        snapshot_list = [s.strip() for s in snapshots.split(",")]

        results = ctx.index.search(
            query=question,
            snapshots=snapshot_list,
            top_k=context_size,
            min_score=threshold
        )

        if not results or len(results) == 0:
            rag_step.fail("No context found")
            turn.warn("Try lowering threshold (current: {:.2f}) or indexing more code".format(threshold))
            turn.done()
            return None

        rag_step.done("{} snippets".format(len(results)))

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
        ctx.output.writeline(answer)

        source_rows = []
        for r in results:
            source_rows.append({
                "File": r.file_path,
                "Lines": "{}-{}".format(r.start_line, r.end_line),
                "Relevance": "{}%".format(int(r.score*100))
            })
        ctx.ui.table(source_rows, columns=["File", "Lines", "Relevance"], title="References")

        turn.done()
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

        turn.done()

    def handle_query(ctx):
        return semantic_search(
            ctx,
            query=ctx.query,
            snapshots=ctx.snapshots,
            limit=ctx.limit,
            threshold=ctx.threshold,
            format=ctx.format,
            full=ctx.full
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
        # Not using build_choices_desc since this is informational, not a strict enum
        lines = ["Snapshots to search (comma-separated):"]
        lines.append("  workdir: Current working directory")
        lines.append("  stage: Staged changes")
        lines.append("  HEAD: Latest commit")
        return "\n".join(lines)

    query_command = meow.tool(
        name="query",
        description="Semantic code search",
        params={
            "query": meow.param("string", required=True, from_stdin=True, min_len=3, desc="Search query."),
            "limit": meow.param("int", default=cfg_default_limit, short="n", desc="Maximum results to return."),
            "snapshots": meow.param("string", default=cfg_default_snapshots, short="s", desc=build_snapshots_desc()),
            "threshold": meow.param("float", default=cfg_default_threshold, short="t", desc="Minimum similarity score (0.0-1.0)."),
            "format": meow.param("string", default=cfg_default_format, choices=["text", "json"], short="f", desc=build_format_desc(cfg_default_format)),
            "full": meow.param("bool", default=False, desc="Include full content in output.")
        },
        handler=handle_query
    )
    meow.command(query_command)

    search_command = meow.tool(
        name="search",
        description="Semantic code search (alias for query)",
        params={
            "query": meow.param("string", required=True, from_stdin=True, min_len=3, desc="Search query."),
            "limit": meow.param("int", default=cfg_default_limit, short="n", desc="Maximum results to return."),
            "snapshots": meow.param("string", default=cfg_default_snapshots, short="s", desc=build_snapshots_desc()),
            "threshold": meow.param("float", default=cfg_default_threshold, short="t", desc="Minimum similarity score (0.0-1.0)."),
            "format": meow.param("string", default=cfg_default_format, choices=["text", "json"], short="f", desc=build_format_desc(cfg_default_format)),
            "full": meow.param("bool", default=False, desc="Include full content in output.")
        },
        handler=handle_query
    )
    meow.command(search_command)

    ask_command = meow.tool(
        name="ask",
        description="Ask questions about codebase using RAG",
        params={
            "question": meow.param("string", required=True, from_stdin=True, min_len=3, desc="Question to ask about the codebase."),
            "preset": meow.param("string", default=cfg_default_preset, short="p", choices=meow.presets(), desc=build_preset_desc(cfg_default_preset)),
            "context_size": meow.param("int", default=cfg_default_context_size, short="n", desc="Number of code snippets to retrieve."),
            "threshold": meow.param("float", default=cfg_default_ask_threshold, short="t", desc="Minimum similarity score (0.0-1.0)."),
            "snapshots": meow.param("string", default=cfg_default_snapshots, short="s", desc=build_snapshots_desc()),
            "show_context": meow.param("bool", default=False, short="v", desc="Display retrieved context before answer.")
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
    __process_files_incrementally(ctx, sub, new_files, existing_versions, batch_size)
    sub.done()

def __process_files_incrementally(ctx, turn, new_files, existing_versions, batch_size):
    """Process files incrementally with batched embeddings."""
    step = turn.step("Batch size: {}".format(batch_size))

    chunk_queue = []
    for file_info in new_files:
        for chunk in file_info["chunks"]:
            chunk_queue.append({"text": chunk["text"], "file": file_info, "chunk_data": chunk})

    total_chunks = len(chunk_queue)
    processed = 0
    saved_files = 0
    num_batches = (total_chunks + batch_size - 1) // batch_size
    batch_num = 0

    for batch_start in range(0, total_chunks, batch_size):
        batch_num += 1
        batch_end = min(batch_start + batch_size, total_chunks)
        batch_items = chunk_queue[batch_start:batch_end]

        batch_texts = [item["text"] for item in batch_items]
        embeddings = ctx.llm.embed(texts=batch_texts, preset="embeddings")
        step.update("Batch {}/{}".format(batch_num, num_batches))

        for i, item in enumerate(batch_items):
            item["embedding"] = embeddings[i]

        processed += len(batch_items)

        files_to_save = __collect_completed_files(batch_items, chunk_queue[:batch_end])
        for file_info in files_to_save:
            if existing_versions[file_info["hash"]] == None:
                __save_single_file(ctx, file_info, existing_versions)
                saved_files += 1

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

