# Frequently Asked Questions (FAQ)

This document answers common questions about `meowg1k`.

## General

### Q: Which provider should I use?

- **For beginners:** Gemini (`gemini-2.5-flash`) offers a generous free tier and is very fast.
- **For quality:** Anthropic Claude (`claude-sonnet-4-5-20250929`) provides high-quality output.
- **For cost-effectiveness:** OpenRouter gives you access to many free and low-cost models.
- **For privacy:** Use a local `llama.cpp` server for complete data privacy.

### Q: Can I use multiple AI providers at the same time?

Yes. This is a core feature. You can define multiple presets in your `config.yaml`, each pointing to a different provider or model. Then, you can use different presets for different tasks or even for different file types. See the [Configuration Guide](./02-CONFIGURATION.md) for examples.

### Q: How much does it cost to use?

The cost depends entirely on the provider and model you choose.

- `meowg1k` itself is free and open-source.
- Many providers like Gemini and OpenRouter have free tiers.
- Using a local `llama.cpp` model is completely free, limited only by your hardware.
- To control costs with paid providers, use the rate limiting and token cap features in your presets.

## Configuration

### Q: Where should I put my configuration file?

`meowg1k` loads configuration in a layered way. Here’s how it works:

1.  **User Config (Base):** It first loads a global config from `~/.config/meowg1k/config.yaml`. This is for your personal, machine-wide settings.

2.  **Project or Explicit Config (Overrides):** Then, it merges settings from **one** of the following sources, which will override the user config:
    - **Explicit Path (Highest Priority):** A file path passed via the `--config` flag. If you use this, the project-level `.meowg1k.yaml` will be **ignored**.
    - **Project Config:** If no `--config` flag is given, it looks for a `.meowg1k.yaml` (or `.yml`) in your project's root. This is for team-shared settings.

### Q: How do I share a configuration with my team?

Create a file at `.meowg1k.yaml` in the root of your project and commit it to your Git repository. Team members can still have their own personal defaults in `~/.config/meowg1k/config.yaml`, but the project-specific settings will take precedence.

### Q: What happens if a preset isn't found in the config?

If you reference a preset that doesn't exist in your configuration file, `meowg1k` will immediately fail with a clear error message indicating that the requested preset cannot be found. This is intentional behavior to prevent unexpected fallbacks that could lead to unintended API calls or cost.

However, `meowg1k` will fall back to smart defaults for the chosen provider when a preset exists but certain optional fields are missing. For example, if you specify `provider: "gemini"` but don't provide a model, it will default to `gemini-2.5-flash`.

## Usage

### Q: Can I pipe content from stdin AND use a user prompt flag?

Yes. This is a primary workflow. The content from stdin provides the **context**, and the `-u, --user-prompt` flag provides the **instruction**.

```bash
cat file.py | meow g -u "Add type hints to this Python code"
```

### Q: How can I debug what is being sent to the AI?

`meowg1k` automatically logs detailed trace information for every command execution. These logs include:

- Complete request and response data for all LLM API calls
- Execution flow and activity status changes
- Timing and token usage information
- Any errors that occurred

**Viewing trace logs:**

```bash
# View the latest log file
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | jq '.'

# See just the API interactions
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | jq 'select(.log_entry_type == "api_interaction")'
```

The logs are stored in `.meowg1k/logs/` in your workspace root in JSON Lines format (`.jsonl`). Each command creates a new uniquely-named log file.

For more details, see the [Debugging & Trace Logs](./10-TROUBLESHOOTING.md#debugging--trace-logs) section in the Troubleshooting Guide.

### Q: What happens if I hit my provider's rate limits?

You can configure rate limits directly in your `meowg1k` model definition to prevent this. If you do hit a provider limit, the tool will receive an error. By setting `requestsPerMinute` in your config, `meowg1k` will automatically throttle itself to stay under the limit you define.

### Q: Can I use this in CI/CD?

Absolutely. This is a core use case. Use the `--silent` flag to get clean output for scripting, and set your API keys as environment variables in your CI/CD platform's secrets management system.

### Q: Why does `meow pr` require the `--base` flag?

The command needs to know which branch to compare against to generate the list of changes. Common examples are `--base main` or `--base dev`.

While automatic detection of the target branch is a feature under consideration, it's currently explicit by design for several reasons:

- Different projects use different branching strategies (GitFlow, trunk-based, feature branches)
- The default branch might not always be the intended merge target
- Explicit flags prevent accidental comparisons against the wrong branch

You can simplify your workflow by creating shell aliases for common patterns:

```bash
# Add to your ~/.bashrc or ~/.zshrc
alias mpr='meow pr --base main'
alias mprd='meow pr --base dev'
```

Or use git's default branch detection in a script:

```bash
# Get the repository's default branch
DEFAULT_BRANCH=$(git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@')
meow pr --base "$DEFAULT_BRANCH"
```

## Security & Privacy

### Q: Can I use meowg1k completely offline?

Yes. Configure a preset to use the `llama` provider and point it to a local `llama.cpp` server running on your machine. In this setup, no internet connection is required and no data ever leaves your computer.

### Q: Is my code sent to third parties?

Only if you configure `meowg1k` to use a cloud-based provider like Gemini, OpenAI, or Anthropic. If you use a local model, your code is processed entirely on your machine. You are always in control.

### Q: How are my API keys stored?

They are not. `meowg1k` never writes your API keys to disk. It reads them from environment variables at runtime and holds them in memory only for the duration of a request.

### Q: Is it safe to use in a repository with sensitive or proprietary code?

Yes, provided you use a local model. For maximum security, the recommended approach for sensitive codebases is to use the `llama` provider pointed at a self-hosted LLM.

## RAG and Code Search

### Q: What is RAG and why should I use it?

RAG (Retrieval-Augmented Generation) combines semantic search with LLM reasoning. Instead of sending your entire codebase to an AI or relying on the AI's training data, RAG:

1. Searches your indexed codebase for relevant code chunks
2. Retrieves only the most relevant pieces
3. Sends those specific chunks to the LLM as context
4. Gets an answer grounded in your actual, current code

This approach is **faster**, **cheaper**, and more **accurate** than alternative methods.

### Q: How often should I run `meow index`?

Run `meow index` whenever your codebase changes significantly:

- After pulling changes from Git
- After switching branches (can be automated with a Git hook)
- After making substantial code changes
- Before important Q&A sessions with `meow ask`

**Pro tip:** Set up a `post-checkout` Git hook to auto-index after branch switches. See the [Integrations Guide](./07-INTEGRATIONS.md) for details.

### Q: How much disk space does the index use?

The index size depends on your codebase size and configuration. As a rough estimate:

- **Small project (100 files):** ~5-10 MB
- **Medium project (1,000 files):** ~50-100 MB
- **Large project (10,000 files):** ~500 MB - 1 GB

The index includes:

- Document content (deduplicated)
- Embeddings (typically 768 or 1536 dimensions per chunk)
- HNSW vector indices
- Metadata

**Tips to reduce size:**

- Use aggressive filters to exclude unnecessary files
- Increase `maxRunes` to create fewer, larger chunks
- Exclude test files and generated code

### Q: Can I use RAG with a local embedding model?

Yes! You can use `llama.cpp` with an embedding model for complete privacy:

```yaml
models:
  local-embeddings:
    provider: "llama"
    model: "nomic-embed-text-v1.5-GGUF"
    baseURL: "http://localhost:8080"

presets:
  embeddings:
    model: "local-embeddings"

index:
  preset: "embeddings"
  chunker:
    maxRunes: 512
    overlapRunes: 64
  batchSize: 1 # Process one at a time for local models
```

Run llama.cpp with an embedding model:

```bash
llama-server -m nomic-embed-text-v1.5.Q4_K_M.gguf --port 8080 --embeddings
```

### Q: Why are my search results not relevant?

Several possible reasons:

1. **Index is outdated:** Run `meow index` to refresh
2. **Query too vague:** Be more specific in your search terms
3. **`minScore` too high:** Lower the threshold with `--min-score 0.5`
4. **Not enough chunks retrieved:** Increase `--top-k` to get more results
5. **Filters too aggressive:** Check if your filter rules are excluding relevant files

**Debugging steps:**

```bash
# Check if files are being indexed
meow index  # Look at the output to see how many files were processed

# Try a broad search with low threshold
meow search "authentication" --top-k 20 --min-score 0.3

# Verify filters aren't excluding important files
# Check .meowg1k.yaml filter section
```

### Q: How does chunking work and why does it matter?

Chunking splits files into smaller pieces for embedding. The configuration affects search quality:

- **Large chunks** (`maxRunes: 2048`)
  - More context per result
  - Fewer total chunks (faster indexing)
  - Less precise matching
  - Better for documentation/narrative text

- **Small chunks** (`maxRunes: 512`)
  - More precise matching
  - More total chunks (slower indexing)
  - Less context per result
  - Better for finding specific functions

**Recommended starting point:** `maxRunes: 1024` with `overlapRunes: 128`

### Q: What's the difference between `meow search` and `meow ask`?

- **`meow search`**: Pure semantic search. Returns code chunks with similarity scores. Use when you want to find code, not get explanations.

- **`meow ask`**: Semantic search + LLM reasoning. Retrieves code chunks, then uses an LLM to answer your question based on that code. Use when you want explanations, summaries, or answers.

**Example:**

```bash
# Use query to find code
meow search "authentication middleware"  # Returns: auth.go lines 45-67, middleware.go lines 12-34

# Use ask to understand code
meow ask "How does authentication work?"  # Returns: "The authentication system uses JWT tokens..."
```

### Q: Can I search across multiple projects?

Currently, each project maintains its own index in the `.meowg1k/` directory. To search multiple projects:

**Option 1:** Use workspace root override:

```bash
# Search project A
meow search "authentication" --workspace /path/to/project-a

# Search project B
meow search "authentication" --workspace /path/to/project-b
```

**Option 2:** Create a script to search multiple projects:

```bash
#!/bin/bash
for project in /path/to/projects/*; do
    echo "=== Searching $project ==="
    meow search "$1" --workspace "$project"
done
```

### Q: How do I share an index with my team?

**Don't commit the index files**—they're large and contain binary data. Instead:

1. **Share the configuration:** Commit `.meowg1k.yaml` to Git
2. **Document the setup:** Add indexing instructions to your README
3. **Provide a script:** Create `scripts/setup_meow.sh`:

```bash
#!/bin/bash
echo "Installing meowg1k..."
go install github.com/retran/meowg1k@latest

echo "Indexing codebase..."
meow index

echo "Done! Try: meow ask 'How does this project work?'"
```

Each team member runs the script to build their own local index.

### Q: Can I use different embedding models for different projects?

Yes! Each project's `.meowg1k.yaml` can specify different models:

**Project A (.meowg1k.yaml):**

```yaml
index:
  preset: "gemini-embeddings" # Free, fast
```

**Project B (.meowg1k.yaml):**

```yaml
index:
  preset: "openai-embeddings" # Higher quality
```

Your `~/.config/meowg1k/config.yaml` defines the models, and each project chooses which to use.

### Q: What happens if I index with one model and switch to another?

The index becomes invalid. When you change embedding models, you must re-index:

```bash
# Changed from Gemini to OpenAI in config
meow index  # Rebuilds index with new model
```

Different models produce different embedding dimensions and values, so they're not compatible.

### Q: How do I exclude files from indexing?

Use the `filter` section in your config:

```yaml
filter:
  ignore:
    - "node_modules/**"
    - "vendor/**"
    - "**/*.test.go" # Exclude test files
    - "**/*.generated.go" # Exclude generated code
    - "dist/**"
    - ".git/**"
```

This reduces index size, speeds up indexing, and improves search quality by removing noise.