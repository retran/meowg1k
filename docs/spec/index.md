# Index

Status: draft
Elaborates: docs/design/0.3.0-architecture.md sections 4, 9

## Scope

`meow-index` makes a workspace searchable by meaning. It walks files, splits
them into chunks, embeds the chunks, stores the vectors, and answers a query
with ranked chunks.

It does not decide what to do with a result and does not call a generation
model. `search.code` is a thin tool over it.

## Boundary

The index API: build, update, query, stats, clear. The rows it writes through
`meow-store`. The ranked results a tool returns to a model.

## Requirements

### Walking

**[R-INDEX-001]** Indexing MUST respect `.gitignore`, `.meowignore`, and the
workspace's own `.meow/.data/` exclusion, and MUST NOT index a file any of
them excludes.

**[R-INDEX-002]** Indexing MUST skip files it detects as binary, and MUST
record how many it skipped.

**[R-INDEX-003]** Indexing MUST skip a file larger than a configured size and
MUST report each skip, so a missing result is explainable.

**[R-INDEX-004]** Indexing MUST NOT follow a symlink that leaves the
workspace.

**[R-INDEX-005]** Indexing MUST cover every text file the exclusions allow,
including Markdown and other prose, not only source code.

### Chunking

**[R-INDEX-010]** Chunking MUST be deterministic: the same file content MUST
produce the same chunks, with the same boundaries, on every run.

**[R-INDEX-011]** A chunk MUST carry its source path, its byte range, and its
line range, so a result can be cited precisely.

**[R-INDEX-012]** Chunks MUST overlap by a configured number of tokens, so a
definition split across a boundary is retrievable from either side.

**[R-INDEX-013]** A chunk MUST NOT exceed the embedding model's input limit.

**[R-INDEX-014]** Chunk boundaries MUST fall on line boundaries. A chunk MUST
NOT begin or end part-way through a line.

### Embedding

**[R-INDEX-020]** Chunks MUST be embedded in batches, and a batch rejected for
exceeding a provider limit MUST be split and retried.

**[R-INDEX-021]** A single chunk that a provider rejects as too large MUST
fail with an error naming the file and the chunk's line range, not a generic
size error.

**[R-INDEX-022]** Embedding MUST be resumable: an interrupted build MUST NOT
re-embed chunks whose vectors are already stored.

### Incremental update

**[R-INDEX-030]** Update MUST re-embed a file only when its content hash
differs from the stored one.

**[R-INDEX-031]** Update MUST remove the chunks of a file that no longer
exists or that has become excluded.

**[R-INDEX-032]** Update MUST report how many files were added, changed,
removed, and unchanged.

### Query

**[R-INDEX-040]** A query MUST return results ranked by descending similarity,
each carrying the path, the line range, the chunk text, and the score.

**[R-INDEX-041]** A query against an empty or absent index MUST return no
results and say the index is empty, and MUST NOT build one implicitly.

**[R-INDEX-042]** A query MUST accept a result limit and a minimum score, and
MUST apply both.

**[R-INDEX-043]** A query MUST be answerable without a network call when the
embedding of the query itself is cached.

**[R-INDEX-044]** A query MUST accept a path filter, expressed as globs, and
MUST apply it before ranking.

### Storage

**[R-INDEX-050]** Vectors MUST be stored through `meow-store` in the same
database as everything else.

**[R-INDEX-051]** The index MUST record which embedding model produced it, and
a query against an index built by a different model MUST fail with both names
rather than returning meaningless scores.

**[R-INDEX-052]** `clear` MUST remove every vector and chunk, and MUST leave
sessions and the key-value store untouched.

## Changes from v0.2.x

The v0.2.x index lives in its own SQLite database under
`internal/adapters/sqlite/index/`, separate from sessions and cache.
[R-INDEX-050] moves it into the one database, which is what lets a chunk share
the blob table with a tool result quoting the same file.

`llmEmbed` already splits a batch that exceeds a rate limit, recursively, and
that behaviour is kept by [R-INDEX-020]. Its failure message for a single
oversized chunk is generic; [R-INDEX-021] requires naming the file.

Nothing in v0.2.x records which embedding model built the index, so changing
the model silently produces nonsense scores.

## Decisions

**Chunking is line-oriented**, by [R-INDEX-014]. A syntax-aware splitter using
tree-sitter would give better boundaries and costs a grammar per language, a
build dependency, and a fallback for every language without one. The line
splitter is the thing to measure against; replacing it is an amendment once
there is a recall number that justifies the dependency.

**The vector index is `hnsw_rs`.** It is pure Rust, so it keeps the single
static binary and the `unsafe_code = "deny"` lint intact, which `usearch`
would not. The reason to revisit is a measurement, not a preference: if recall
or build time on this repository is unacceptable, the C++ implementation earns
its cost then.

**Prose is indexed alongside code**, by [R-INDEX-005], and a query narrows with
a path filter, by [R-INDEX-044]. Excluding prose would make the design
documents unsearchable by the agents most likely to need them, and dilution is
the caller's problem to solve with a filter rather than the index's to solve by
guessing.
