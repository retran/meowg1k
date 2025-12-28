# Command Reference

This document provides a detailed reference for all available `meowg1k` commands and their options.

## `meow init`

Initializes a new `meowg1k` project by creating a `.meowg1k.yaml` configuration file in the current directory.

### Usage

```bash
meow init [flags]
```

### Flags

- `-f, --force`: Overwrites an existing configuration file if present.

### Description

The `init` command helps you get started with `meowg1k` by creating a project-level configuration file with sensible defaults. The generated configuration uses Google Gemini as the default LLM provider.

If a `.meowg1k.yaml` file already exists in the current directory, the command will fail unless you use the `--force` flag to overwrite it.

### Examples

```bash
# Initialize a new project configuration
meow init

# Force overwrite an existing configuration
meow init --force
```

### After Initialization

After running `meow init`, you need to:

1. Get a free API key from [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Set the environment variable in your shell profile (`~/.bashrc`, `~/.zshrc`):
   ```bash
   export MEOW_GEMINI_API_KEY="your-api-key-here"
   ```
3. Restart your shell or run `source ~/.bashrc` (or `~/.zshrc`)
4. Try it out:
   ```bash
   echo "Create a hello world function" | meow write
   ```

## Global Flags

These flags can be used with any command.

- `--config <path>`: Specify a path to a configuration file. This overrides any project-level or user-level configs.
- `--workspace <path>`: Specify the workspace root directory. This overrides automatic workspace detection.
- `--silent`: Enables silent mode, which suppresses progress indicators and other non-essential output. Ideal for scripting.
- `--no-cache`: Disables LLM response caching for the current command.
- `--update-cache`: Forces a cache refresh by making a fresh request to the LLM and updating the cache entry.
- `--help`: Shows help information for any command.

## `meow do`

Runs a multi-step agent workflow (research → plan → execute → verify) to complete a task.
Each step can use tools and prompts defined in the `agent` section of your config.

### Usage

```bash
meow do "<task>"
echo "<task>" | meow do
```

### Flags

- `--profile <name>`: Override the default agent profile.
- `--system-prompt <text>`: Override the default agent system prompt.
- `-s, --snapshots <list>`: Snapshots to search for embeddings.
- `-k, --top-k <n>`: Number of top results for embeddings search.
- `--min-score <float>`: Minimum similarity score for embeddings search.

### Examples

```bash
# Run the agent on a task
meow do "Add a healthcheck endpoint and update docs"

# Provide the task via stdin
echo "Refactor the logger setup" | meow do
```

## `meow write` (aliases: `gen`, `g`)

Generates content based on a prompt and/or context provided via standard input (stdin).

### Usage

```bash
cat [file] | meow write [flags]
echo "[text]" | meow g [flags]
```

### Flags

- `-t, --task <name>`: Runs a predefined task from your configuration file.
- `-s, --system-prompt <text>`: Overrides the default system prompt.
- `-u, --user-prompt <text>`: Provides the user prompt for the generation task.

### Examples

```bash
# Run a predefined 'review' task on a file
cat main.go | meow g -t review

# Provide context from a file and a prompt via a flag
cat service.py | meow g -u "Add error handling and logging to this class"

# Use stdin for context with custom system and user prompts
echo "function add(a, b) { return a + b; }" | meow g \
  -s "You are a TypeScript expert" \
  -u "Convert this JavaScript function to TypeScript with type hints"
```

## `meow draft commit` (alias: `commit-msg`)

Generates a commit message based on staged changes or the difference between branches.

The command supports two execution strategies (configured via the `commit.strategy` field in your config file):

- **`summarize` (default)**: Uses a Map-Reduce approach, analyzing each file individually then combining summaries. Best for large commits.
- **`flat`**: Sends the entire diff directly to the model. Faster for small commits, but fails if the diff is too large.

See the [Configuration Guide](./02-CONFIGURATION.md) for details on configuring the strategy.

### Usage

```bash
meow draft commit [flags]
```

### Flags

- `-i, --intent <text>`: Provides a high-level developer intent for the commit, which helps the AI generate a more accurate message. Can also be provided via stdin.
- `--diff <mode>`: Diff source to use: `staged` (default) or `branch`.
- `-b, --base <branch>`: Base branch to compare against when `--diff branch` is set.

### Modes & Examples

#### 1. Default Mode (Staged Changes)

This is the standard mode. It analyzes files you have staged with `git add`.

```bash
# Stage your files
git add .

# Generate a commit message based on the staged changes
meow draft commit

# Provide intent to guide the AI
meow draft commit -i "Refactor user authentication to use a new JWT library"
```

#### 2. Branch Diff Mode

Use this mode when you want to generate a single commit message that summarizes all changes on your feature branch before merging.

```bash
# Generate a message for all changes on the current branch compared to 'main'
meow draft commit --diff branch --base main

# Provide intent for the branch diff
meow draft commit --diff branch --base dev -i "Implement the entire user profile feature"
```

## `meow draft pr`

Generates a Pull Request title and description based on the difference between your current branch and a base branch.

The command supports two execution strategies (configured via the `pr.strategy` field in your config file):

- **`summarize` (default)**: Uses a Map-Reduce approach, analyzing each file individually then combining summaries. Best for large PRs.
- **`flat`**: Sends the entire diff directly to the model. Faster for small PRs, but fails if the diff is too large.

See the [Configuration Guide](./02-CONFIGURATION.md) for details on configuring the strategy.

### Usage

```bash
meow draft pr [flags]
```

### Flags

- `--diff <mode>`: Diff source to use: `branch` (default) or `staged`.
- `-b, --base <branch>`: Base branch to compare against when `--diff branch` is set (e.g., `main`, `dev`, `master`).
- `-i, --intent <text>`: Provides high-level context or intent for the PR. Can also be provided via stdin.

### Examples

```bash
# Generate a PR description for changes to be merged into 'main'
meow draft pr --base main

# Provide intent to get a more focused PR description
meow draft pr -b dev -i "Add a complete Stripe payment integration"

# Pipe the intent via stdin
echo "This PR adds a new caching layer using Redis" | meow draft pr -b main
```

## `meow index`

**Aliases:** `idx`

Indexes workspace files by computing embeddings and building vector indices for semantic search and RAG-based queries.

### Usage

```bash
meow index [flags]
```

### Flags

No command-line flags. Configuration is read from the `index` section in your config file.

### Description

The `index` command processes all files in your workspace, chunks them into smaller pieces, computes embeddings using your configured embedding model, and builds HNSW vector indices for efficient similarity search.

The indexing process includes:

- Scanning workspace state (workdir, stage, HEAD)
- Deduplicating files based on content hashes to avoid reprocessing unchanged files
- Chunking new/modified files according to the configured chunker settings
- Computing embeddings in batches using the configured profile
- Saving document versions and chunks to SQLite
- Building and saving vector indices for each snapshot

This command is required before using `meow search` or `meow ask`.

### Examples

```bash
# Index your entire workspace
meow index
```

### Configuration

Configure indexing in your `.meowg1k.yaml`:

```yaml
index:
  profile: "embeddings" # Profile for computing embeddings
  chunker:
    maxRunes: 1024 # Maximum chunk size in runes
    overlapRunes: 128 # Overlap between chunks
  batchSize: 64 # Chunks per API call
```

See the [RAG and Code Search guide](./09-RAG-AND-CODE-SEARCH.md) for detailed configuration examples.

## `meow search <text>`

**Aliases:** `q`

Searches for code chunks similar to the query text using vector similarity (semantic search).

### Usage

```bash
meow search <text> [flags]
echo "<text>" | meow search [flags]
```

### Arguments

- `<text>` — The search query (can also be provided via stdin)

### Flags

- `-k, --top-k <n>` — Number of top results to return (default: 10)
- `-s, --snapshots <list>` — Snapshots to search: `_workdir_`, `_stage_`, `_head_` (default: all three)
- `--min-score <float>` — Minimum similarity score, 0.0 to 1.0 (default: 0.0, no filtering)
- `--json` — Output results in JSON format

### Description

The `query` command performs semantic search over your indexed codebase. Unlike text search that relies on exact keyword matching, semantic search uses vector similarity to understand the **meaning** of your query and match it with relevant code.

The command searches across specified snapshots (workdir, stage, head) and returns chunks with similarity scores above the minimum threshold.

### Examples

```bash
# Basic semantic search
meow search "authentication logic"

# Get more results with lower threshold
meow search "error handling" --top-k 20 --min-score 0.5

# Search only in uncommitted changes
meow search "new feature" --snapshots _workdir_

# JSON output for scripting
meow search "API endpoints" --json | jq '.[] | .FilePath'

# Search from stdin
echo "database connection" | meow search
```

### Prerequisites

You must run `meow index` at least once before using this command.

## `meow ask <question>`

**Aliases:** `a`

Asks a question about your codebase and gets an AI-generated answer using Retrieval-Augmented Generation (RAG).

### Usage

```bash
meow ask <question> [flags]
echo "<question>" | meow ask [flags]
```

### Arguments

- `<question>` — The question to ask about your codebase (can also be provided via stdin)

### Flags

- `--profile <name>` — Profile to use for answer generation (overrides config)
- `-k, --top-k <n>` — Number of top results to retrieve (0 = use config default)
- `--min-score <float>` — Minimum similarity score (0.0 = use config default)
- `-s, --snapshots <list>` — Snapshots to search (default: `_workdir_,_stage_,_head_`)
- `--show-context` — Show retrieved code context before the answer
- `--system-prompt <text>` — System prompt for answer generation (overrides config)

### Description

The `ask` command combines semantic search with LLM-powered question answering. It:

1. Performs a semantic search to find relevant code chunks
2. Retrieves the top-K most relevant chunks
3. Formats them as context
4. Sends the context + your question to an LLM
5. Returns an answer grounded in your actual codebase

This is more powerful than `query` because it synthesizes information from multiple code chunks and provides natural language explanations.

### Examples

```bash
# Ask a basic question
meow ask "How does authentication work in this project?"

# Use a more powerful model
meow ask "What's the error handling strategy?" --profile gemini-pro

# See what context the AI is using
meow ask "Explain the database layer" --show-context

# Search more thoroughly
meow ask "Where are the API routes defined?" --top-k 10 --min-score 0.5

# Ask from stdin
echo "How do I add a new API endpoint?" | meow ask
```

### Configuration

Configure the `ask` command in your `.meowg1k.yaml`:

```yaml
answer:
  profile: "gemini-pro" # Profile for generating answers
  topK: 5 # Number of chunks to retrieve
  minScore: 0.7 # Minimum similarity score
  systemPrompt: >-
    You are an expert AI assistant helping developers understand their codebase.
    Answer questions based ONLY on the provided code context.
```

### Prerequisites

You must run `meow index` at least once before using this command.

## `meow version`

Displays the application's version, build date, and commit hash.

### Usage

```bash
meow version
```

## Next Steps

Now that you're familiar with the commands, check out:

- [Code Generation and Automated Workflows](./04-GENERATION-AND-WORKFLOWS.md) — Learn about write, draft commit, and draft pr workflows
- [RAG and Code Search](./05-RAG-AND-CODE-SEARCH.md) — Semantic search and question answering
- [Examples & Recipes](./06-EXAMPLES.md) — Practical examples and complete workflows
