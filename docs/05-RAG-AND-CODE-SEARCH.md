# RAG and Code Search

This guide covers `meowg1k`'s Retrieval-Augmented Generation (RAG) capabilities for intelligent code search and question answering. With RAG, you can semantically search your codebase and ask natural language questions that are answered based on your actual code.

[Back to Documentation Index](./README.md)

## Overview

`meowg1k` implements a complete RAG pipeline that allows you to:

1. **Index your codebase** — Convert your code into searchable embeddings using vector databases
2. **Search semantically** — Find relevant code chunks using natural language queries
3. **Ask questions** — Get AI-generated answers based on the actual context from your codebase

Unlike traditional text search that relies on exact keyword matching, RAG uses semantic similarity to understand the **meaning** of your queries and match them with relevant code, even when the exact words don't appear in your code.

## Architecture

The RAG system in `meowg1k` consists of three core components:

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│              │     │              │     │              │
│    Index     │────▶│    Query     │────▶│     Ask      │
│              │     │              │     │              │
└──────────────┘     └──────────────┘     └──────────────┘
     Build              Search              Answer
   embeddings        by similarity      with context
```

### 1. Indexing (`meow index`)

- Scans three workspace states: **workdir** (uncommitted changes), **stage** (staged for commit), and **head** (last commit)
- Chunks files into manageable pieces with configurable overlap
- Computes embeddings for each chunk using your configured embedding model
- Builds HNSW (Hierarchical Navigable Small World) vector indices for fast similarity search
- Stores everything locally in SQLite with smart deduplication

### 2. Querying (`meow query`)

- Computes an embedding for your search query
- Searches across HNSW indices for the most similar code chunks
- Returns results with similarity scores, file paths, and line numbers
- Supports filtering by snapshot (workdir/stage/head) and minimum score

### 3. Asking (`meow ask`)

- Performs a semantic search (like `query`)
- Retrieves the top-K most relevant code chunks
- Sends the retrieved context plus your question to an LLM
- Returns an answer grounded in your actual codebase

## Quick Start

### Step 1: Configure Embedding Model

Add an embedding model and configure the RAG system in your `.meowg1k.yaml`:

```yaml
# .meowg1k.yaml
models:
  gemini-embeddings:
    provider: "gemini"
    model: "text-embedding-004"

profiles:
  embeddings:
    model: "gemini-embeddings"

index:
  profile: "embeddings"
  chunker:
    maxRunes: 1024
    overlapRunes: 128
  batchSize: 64

ask:
  profile: "smart"
  topK: 5
  minScore: 0.7
```

**Note:** This is a minimal configuration. For detailed explanations of all available options including rate limiting, caching, different providers (OpenAI, Anthropic, local llama.cpp), and advanced chunking strategies, see the [Configuration Guide](./02-CONFIGURATION.md#index).

Set your API key:

```bash
export MEOW_GEMINI_API_KEY="your-api-key-here"
```

### Step 2: Index Your Codebase

```bash
# Index all files in your workspace
meow index
```

The indexing process will:

- Scan your workspace (workdir, stage, head)
- Deduplicate files that haven't changed
- Chunk new/modified files
- Compute embeddings in batches
- Build HNSW vector indices

**Initial indexing can take time** depending on your codebase size and the speed of your embedding model. Subsequent runs are much faster due to deduplication.

### Step 3: Search Your Code

#### Semantic Search with `query`

Find relevant code chunks using natural language:

```bash
# Search for authentication-related code
meow query "authentication logic"

# Search with higher precision (more results, lower threshold)
meow query "error handling" --top-k 20 --min-score 0.5

# Output results as JSON
meow query "database connection" --json

# Search only in uncommitted changes
meow query "new feature" --snapshots _workdir_
```

#### Question Answering with `ask`

Get AI-powered answers based on your codebase:

```bash
# Ask a question about your code
meow ask "How does authentication work in this project?"

# Use a more powerful model for complex questions
meow ask "What's the error handling strategy?" --profile smart

# Search more thoroughly
meow ask "Where are the API routes defined?" --top-k 10 --min-score 0.5

# Ask from stdin
echo "Explain the database layer" | meow ask
```

**Example output:**

```
Based on the codebase, authentication in this project works as follows:

1. **Token Validation**: The `AuthService` in `internal/auth/service.go` receives
   a JWT token and validates it using the `JWTService`.

2. **Claims Extraction**: Upon successful validation, user claims (including UserID)
   are extracted from the token.

3. **User Lookup**: The system queries the user repository to find the user by ID.

4. **Error Handling**: Both token validation and user lookup have proper error
   handling with context-aware error messages.

The main authentication flow is implemented in the `Authenticate` method at
`internal/auth/service.go:45-67`.
```

## Commands and Configuration

The RAG system uses three main commands:

- **[`meow index`](./03-COMMAND-REFERENCE.md#meow-index)** — Index your codebase by computing embeddings and building vector indices
- **[`meow query`](./03-COMMAND-REFERENCE.md#meow-query-text)** — Search for code using semantic similarity
- **[`meow ask`](./03-COMMAND-REFERENCE.md#meow-ask-question)** — Ask questions about your codebase with AI-powered answers

For complete command details including all flags and options, see the [Command Reference](./03-COMMAND-REFERENCE.md).

For configuration details of the `index` and `ask` sections, see the [Configuration Guide](./02-CONFIGURATION.md#index).

## 🎛️ Advanced Topics

### Chunking Strategy

Chunking is the process of splitting files into smaller pieces for embedding. The goal is to balance:

- **Context preservation**: Chunks should be large enough to contain meaningful context
- **Semantic coherence**: Each chunk should represent a cohesive concept
- **Search precision**: Smaller chunks = more precise matches but less context

**Recommended settings by use case:**

| Use Case                 | maxRunes | overlapRunes | Reason                              |
| ------------------------ | -------- | ------------ | ----------------------------------- |
| Code search              | 1024     | 128          | Good balance for most codebases     |
| Documentation            | 2048     | 256          | Longer chunks for narrative text    |
| Configuration files      | 512      | 64           | Smaller, focused chunks             |
| Large files (>10k lines) | 1536     | 192          | Larger chunks to reduce total count |

**Overlap importance:**

Overlap ensures that code spanning chunk boundaries doesn't get split awkwardly. For example:

```
Chunk 1 (lines 1-30):
  func ProcessUser(u *User) error {
      if err := validateUser(u); err != nil {
          return err
      }
      // [overlap starts here]
      return saveUser(u)
  }

Chunk 2 (lines 25-55):
  // [overlap from previous chunk]
  return saveUser(u)
  }

  func saveUser(u *User) error {
      // actual implementation
  }
```

Without overlap, a search for "save user" might miss the `ProcessUser` function because `saveUser` is at the boundary.

### Snapshot System

`meowg1k` maintains three snapshots of your workspace:

1. **`_head_`** — Files as they exist in the last commit (HEAD)
2. **`_stage_`** — Files currently staged for commit (`git add`)
3. **`_workdir_`** — Files in your working directory with uncommitted changes

**Why multiple snapshots?**

This allows you to:

- Search through historical code (HEAD) even after making changes
- Find code you're about to commit (stage) but haven't committed yet
- Search work-in-progress code (workdir)

**Snapshot priority:**

When using `ask` or `query`, results from all three snapshots are merged and sorted by similarity score. This means you'll see the most relevant results regardless of which snapshot they come from.

To search only specific snapshots:

```bash
# Only search uncommitted changes
meow query "new feature" --snapshots _workdir_

# Search staged and committed code
meow ask "How does auth work?" --snapshots _stage_,_head_
```

### Deduplication

`meowg1k` uses content-based deduplication to avoid reprocessing unchanged files:

1. Computes SHA-256 hash of each file's content
2. Checks if a document version with that hash already exists in the database
3. If yes, reuses the existing chunks and embeddings
4. If no, chunks the file and computes new embeddings

This makes incremental indexing very fast — only changed files are reprocessed.

### Vector Index Storage

Vector indices are stored as binary blobs in the SQLite database (in the `meta` table). Each snapshot has its own HNSW index:

- `idx_dump_head` — Index for HEAD snapshot
- `idx_dump_stage` — Index for stage snapshot
- `idx_dump_workdir` — Index for working directory snapshot

These indices are rebuilt from scratch on each `meow index` run to ensure consistency.

### Choosing an Embedding Model

Different embedding models have different characteristics:

| Model                    | Provider          | Dimensions | Quality   | Speed     | Privacy  |
| ------------------------ | ----------------- | ---------- | --------- | --------- | -------- |
| `text-embedding-004`     | Gemini            | 768        | High      | Fast      | Cloud    |
| `text-embedding-3-large` | OpenAI            | 3072       | Very High | Medium    | Cloud    |
| `text-embedding-3-small` | OpenAI            | 1536       | High      | Very Fast | Cloud    |
| `nomic-embed-text-v1.5`  | Local (llama.cpp) | 768        | Medium    | Slow      | Complete |
| `voyage-code-2`          | Voyage AI         | 1536       | Very High | Fast      | Cloud    |

**Guidelines:**

- **For most users**: Use Gemini's `text-embedding-004` — free, fast, and high quality
- **For privacy**: Use a local model like `nomic-embed-text-v1.5` with llama.cpp
- **For best quality**: Use OpenAI's `text-embedding-3-large` or Voyage AI's `voyage-code-2`
- **For speed**: Use OpenAI's `text-embedding-3-small` or Gemini's `text-embedding-004`

## Troubleshooting

For common issues and solutions related to RAG and indexing, see the [Troubleshooting Guide](./10-TROUBLESHOOTING.md), specifically:

- [No index found](./10-TROUBLESHOOTING.md#no-index-found-for-snapshot)
- [No results from queries](./10-TROUBLESHOOTING.md#no-results-found-from-query)
- [Slow indexing](./10-TROUBLESHOOTING.md#indexing-is-very-slow)
- [Embedding count mismatch](./10-TROUBLESHOOTING.md#embedding-count-mismatch)
- [Out of memory](./10-TROUBLESHOOTING.md#out-of-memory-during-indexing)
- [Inaccurate answers](./10-TROUBLESHOOTING.md#answers-are-not-accurate)

## Best Practices

### 1. Index Regularly

Re-run `meow index` after:

- Pulling new changes from git
- Making significant code changes
- Switching branches
- Before important `ask` sessions

### 2. Use Filters Aggressively

Exclude:

- Dependencies (`node_modules`, `vendor`, etc.)
- Build artifacts
- Generated code
- Binary files
- Test fixtures with large data

This reduces index size, speeds up indexing, and improves search quality.

### 3. Start Broad, Then Narrow

When searching:

1. Start with a general query and low `minScore`
2. Review results to understand what's in your codebase
3. Refine your query or increase `minScore` to focus

### 4. Tune Chunking for Your Codebase

- **Small files (<100 lines)**: Use smaller chunks (512 runes)
- **Large files (>1000 lines)**: Use larger chunks (1536 runes)
- **Configuration-heavy projects**: Use smaller chunks (512 runes)
- **Documentation-heavy projects**: Use larger chunks (2048 runes)

### 5. Cache Embeddings

Enable caching in your embedding profile:

```yaml
profiles:
  embeddings:
    model: "gemini-embeddings"
    cache:
      enabled: true
      ttl: "168h" # 1 week
```

This avoids recomputing embeddings for queries you've asked before.

### 6. Use Appropriate Models for Each Task

- **Indexing**: Use a fast, cost-effective embedding model
- **Answering**: Use a powerful LLM for high-quality answers

Example:

```yaml
profiles:
  embeddings:
    model: "gemini-embeddings" # Fast and free

  smart:
    model: "claude-sonnet" # Powerful for reasoning
```

### 7. Experiment with `topK` and `minScore`

- **Broad questions** (e.g., "How does the system work?") → Higher `topK` (10-20), lower `minScore` (0.5-0.6)
- **Specific questions** (e.g., "Where is the login function?") → Lower `topK` (3-5), higher `minScore` (0.7-0.8)

## Related Documentation

- [Configuration Guide](./02-CONFIGURATION.md) — Detailed configuration reference for models, profiles, and RAG settings
- [Command Reference](./03-COMMAND-REFERENCE.md) — Complete reference for all commands including flags and options
- [Examples & Recipes](./06-EXAMPLES.md) — Practical examples and workflows for RAG-based development
- [Troubleshooting Guide](./10-TROUBLESHOOTING.md) — Solutions for common issues

---

**Next:** [Examples & Recipes](./06-EXAMPLES.md)
