# Troubleshooting Guide

This guide provides solutions to common problems organized by topic. Use the table of contents to quickly find relevant issues.

[Back to Documentation Index](./README.md)

## Table of Contents

1. [Installation Issues](#installation-issues)
2. [API & Authentication](#api--authentication)
3. [Configuration Issues](#configuration-issues)
4. [Command Usage](#command-usage)
5. [Code Generation Issues](#code-generation-issues)
6. [RAG & Indexing Issues](#rag--indexing-issues)
7. [Performance Issues](#performance-issues)
8. [Network & Proxy Issues](#network--proxy-issues)
9. [Debugging & Trace Logs](#debugging--trace-logs)
10. [Getting Further Help](#getting-further-help)

---

## Installation Issues

### `meow: command not found` after installation

**Cause:** The installation directory is not in your system's `PATH`.

**Solutions:**

**For `go install`:**

1. Find your Go binary path:

   ```bash
   go env GOPATH
   ```

2. Add to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.):

   ```bash
   export PATH="$PATH:$(go env GOPATH)/bin"
   ```

3. Reload your shell:
   ```bash
   source ~/.bashrc  # or ~/.zshrc
   ```

**For Homebrew:**

```bash
# Diagnose issues
brew doctor

# Relink the binary
brew unlink meow && brew link meow
```

**For Scoop (Windows):**

```powershell
# Ensure Scoop's bin directory is in PATH (usually automatic)
scoop reset meow
```

### Package installation fails (`.deb` / `.rpm`)

**Debian/Ubuntu:**

```bash
# Fix broken dependencies first
sudo apt --fix-broken install

# Then install
sudo dpkg -i meow_<version>_amd64.deb
```

**Fedora/RHEL:**

```bash
# Use dnf to handle dependencies automatically
sudo dnf install ./meow-<version>-1.x86_64.rpm
```

---

## API & Authentication

### `No API key found for provider`

**Cause:** Environment variable not set or not accessible.

**Solutions:**

1. Set the environment variable for your provider:

   ```bash
   # For Google Gemini
   export MEOW_GEMINI_API_KEY="your-key-here"

   # For OpenAI
   export MEOW_OPENAI_API_KEY="sk-..."

   # For Anthropic
   export MEOW_ANTHROPIC_API_KEY="sk-ant-..."
   ```

2. Add to your shell profile to persist across sessions:

   ```bash
   echo 'export MEOW_GEMINI_API_KEY="your-key"' >> ~/.bashrc
   source ~/.bashrc
   ```

3. Verify the variable is set:
   ```bash
   echo $MEOW_GEMINI_API_KEY
   ```

### `Invalid API key` or authentication errors

**Cause:** Malformed or expired API key.

**Solutions:**

1. Verify key format:
   - OpenAI: `sk-...`
   - Anthropic: `sk-ant-...`
   - Check for extra spaces or line breaks

2. Regenerate the key:
   - Visit your provider's dashboard
   - Create a new API key
   - Update your environment variable

3. Check permissions:
   - Ensure the key has access to the models you're trying to use

### Provider rate limits

**Cause:** Exceeding API request limits.

**Solutions:**

1. Configure rate limiting in your config:

   ```yaml
   models:
     safe-model:
       provider: "openai"
       model: "gpt-4o"
       rateLimit:
         requestsPerMinute: 20
         requestsPerDay: 500
         tokensPerMinute: 40000
   ```

2. Use caching to reduce requests:

   ```yaml
   cache:
     enabled: true
     ttl: "168h" # 1 week
   ```

3. Switch to a provider with higher limits or use a local model

---

## Configuration Issues

### `Config file not found`

**Cause:** Config file doesn't exist or is in wrong location.

**Solutions:**

1. Check default locations:
   - Project config: `./.meowg1k.yaml` (current directory)
   - User config: `~/.config/meowg1k/config.yaml`

2. Create a project config:

   ```bash
   meow init
   ```

3. Use `--config` flag to specify custom location:

   ```bash
   meow generate --config /path/to/config.yaml
   ```

4. Check file permissions:
   ```bash
   ls -la .meowg1k.yaml
   ```

### Invalid YAML syntax

**Cause:** Malformed YAML syntax.

**Solutions:**

1. Check indentation (use spaces, not tabs):

   ```yaml
   # ✅ Correct
   models:
     default:
       provider: "gemini"

   # ❌ Wrong (mixed indentation)
   models:
   	default:
       provider: "gemini"
   ```

2. Quote strings with special characters:

   ```yaml
   # ✅ Correct
   systemPrompt: "Use this format: <type>(<scope>): <subject>"

   # ❌ Wrong (unquoted colon)
   systemPrompt: Use this format: <type>
   ```

3. Use a YAML validator:
   - [YAML Lint](https://www.yamllint.com/)
   - Or use `yamllint` command-line tool

---

## Command Usage

### `meow pullrequest` requires `--base` flag

**Cause:** The command needs a base branch for comparison.

**Solution:**

```bash
meow pullrequest --base main
```

**Automation tip:** Create a shell function:

```bash
# Add to ~/.bashrc or ~/.zshrc
mpr() {
    local default_branch=$(git symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's@^refs/remotes/origin/@@')
    [[ -z "$default_branch" ]] && default_branch="main"
    echo "Using base branch: $default_branch"
    meow pullrequest --base "$default_branch" "$@"
}
```

Usage:

```bash
mpr  # Automatically detects default branch
```

### `meow commit` shows "no staged changes"

**Cause:** No files staged with `git add`.

**Solutions:**

**Option 1: Stage files first**

```bash
git add .
meow commit
```

**Option 2: Use target branch mode**

```bash
meow commit --target-branch main
```

This analyzes all changes on your branch vs the target.

### Commands timing out

**Cause:** Model taking too long to respond.

**Solution:** Increase timeout in profile:

```yaml
profiles:
  slow-model:
    model: "claude-sonnet"
    timeout: "15m" # Default is 5m
```

---

## Code Generation Issues

### "Diff too large" error with flat strategy

**Cause:** The diff exceeds the model's token limit.

**Solution:** Switch to `summarize` strategy:

```yaml
commit:
  strategy: "summarize" # Instead of "flat"
  profile: "smart"
```

**When to use each strategy:**

- `flat`: Small commits (1-3 files, <100 lines changed)
- `summarize`: Large commits (many files, complex changes)

### Commit messages are too generic

**Cause:** Insufficient context provided to the model.

**Solutions:**

1. **Provide developer intent:**

   ```bash
   meow commit -i "Fix memory leak in cache layer"
   ```

2. **Use summarize strategy for detailed analysis:**

   ```yaml
   commit:
     strategy: "summarize"
     profile: "smart"
   ```

3. **Improve system prompt:**

   ```yaml
   commit:
     systemPrompt: |
       Write detailed commit messages with:
       - Type and scope
       - Clear description
       - Explanation of why the change was needed
       - Any breaking changes or side effects
   ```

4. **Use a more capable model:**
   ```yaml
   commit:
     profile: "smart" # Instead of "fast"
   ```

### Summaries being skipped unexpectedly

**Cause:** Overly broad summarize rules.

**Solution:** Check rule order (rules are evaluated top-down):

```yaml
summarize:
  rules:
    # ❌ BAD: Catches everything, subsequent rules never run
    - match: "**/*"
      skip: true

    # ✅ GOOD: Specific rules first, then defaults
    - match: "**/*.md"
      skip: true

    - match: "internal/core/**/*.go"
      profile: "smart"
      systemPrompt: "Analyze business logic changes carefully"

    # General rule last
    - match: "**/*.go"
      profile: "fast"
```

---

## RAG & Indexing Issues

### "No index found for snapshot"

**Cause:** Workspace not indexed yet.

**Solution:**

```bash
meow index
```

Run this command:

- After cloning a repository
- When switching to use RAG features
- Periodically to keep index fresh

### "No results found" from query

**Cause:** Query doesn't match indexed content or threshold too high.

**Solutions:**

1. **Broaden your query:**

   ```bash
   # Too specific
   meow query "authenticationMiddleware function"

   # Better
   meow query "authentication"
   ```

2. **Lower minimum score:**

   ```bash
   meow query "error handling" --min-score 0.5
   ```

3. **Check what was indexed:**

   ```bash
   # Look at indexing output
   meow index
   ```

4. **Verify filters aren't too aggressive:**
   ```yaml
   filter:
     ignore:
       - "vendor/**"
       # Don't exclude too much
   ```

### Indexing is very slow

**Cause:** Too many files or slow embedding API.

**Solutions:**

1. **Use aggressive filtering:**

   ```yaml
   filter:
     ignore:
       - "node_modules/**"
       - "vendor/**"
       - "dist/**"
       - "**/*.min.js"
       - "**/*.pb.go"
   ```

2. **Increase batch size (for cloud APIs):**

   ```yaml
   index:
     batchSize: 100 # Default is 32
   ```

3. **Check rate limits:**

   ```yaml
   models:
     embeddings:
       provider: "gemini"
       model: "text-embedding-004"
       rateLimit:
         requestsPerMinute: 1500 # Adjust based on your plan
   ```

4. **Use local models for faster iteration:**
   ```yaml
   models:
     local-embeddings:
       provider: "llama"
       baseURL: "http://localhost:8080"
   ```

### "Embedding count mismatch"

**Cause:** Embedding API didn't return expected number of embeddings.

**Solutions:**

1. **Reduce batch size:**

   ```yaml
   index:
     batchSize: 16 # Try smaller batches
   ```

2. **Check API limits and quotas**

3. **Verify API key is valid**

4. **Check trace logs:**
   ```bash
   cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | jq 'select(.log_entry_type == "api_interaction")'
   ```

### Out of memory during indexing

**Cause:** Processing too many chunks at once.

**Solutions:**

1. **Reduce batch size:**

   ```yaml
   index:
     batchSize: 8
   ```

2. **Use smaller chunks:**

   ```yaml
   index:
     chunker:
       maxRunes: 512 # Instead of 1024
       overlapRunes: 64 # Instead of 128
   ```

3. **Index incrementally:**

   ```bash
   # Index specific directories
   meow index  # with filter:
   ```

   ```yaml
   filter:
     ignore:
       - "**/*" # Exclude all
       - "!src/**" # But include src
   ```

### Answers are not accurate

**Cause:** Retrieved context is irrelevant or insufficient.

**Solutions:**

1. **Increase retrieved context:**

   ```yaml
   ask:
     topK: 10 # Retrieve more chunks
   ```

2. **Lower similarity threshold:**

   ```yaml
   ask:
     minScore: 0.5 # Be less strict
   ```

3. **Use `--show-context` to debug:**

   ```bash
   meow ask "How does auth work?" --show-context
   ```

4. **Ask more specific questions:**

   ```bash
   # Vague
   meow ask "How does the system work?"

   # Specific
   meow ask "How is JWT validation implemented in the auth middleware?"
   ```

5. **Re-index after significant changes:**
   ```bash
   meow index
   ```

---

## Performance Issues

### Generation is too slow

**Cause:** Too many API calls or slow model.

**Solutions:**

1. **Use flat strategy for small changes:**

   ```yaml
   commit:
     strategy: "flat"
   ```

2. **Skip unnecessary files:**

   ```yaml
   summarize:
     rules:
       - match: "**/*.md"
         skip: true
       - match: "**/*_test.go"
         skip: true
   ```

3. **Use faster model:**

   ```yaml
   summarize:
     default:
       profile: "fast" # Use fast model for Map phase

   commit:
     profile: "smart" # Use good model for Reduce phase
   ```

4. **Enable caching:**
   ```yaml
   cache:
     enabled: true
     ttl: "168h"
   ```

### Costs are too high

**Cause:** Using expensive models for all operations.

**Solutions:**

1. **Multi-model strategy:**

   ```yaml
   models:
     cheap:
       provider: "gemini"
       model: "gemini-2.0-flash-exp"

     expensive:
       provider: "anthropic"
       model: "claude-sonnet-4-5-20250929"

   profiles:
     fast:
       model: "cheap"
     smart:
       model: "expensive"

   summarize:
     default:
       profile: "fast" # Cheap model for many files

   commit:
     profile: "smart" # Expensive model for final message
   ```

2. **Use flat strategy when possible:**

   ```yaml
   commit:
     strategy: "flat" # 1 API call instead of N+1
   ```

3. **Configure rate limits to cap spending:**

   ```yaml
   models:
     cost-controlled:
       provider: "openai"
       model: "gpt-4o"
       rateLimit:
         requestsPerDay: 100 # Hard cap
         tokensPerMinute: 40000
   ```

4. **Use local models:**
   ```yaml
   models:
     local:
       provider: "llama"
       baseURL: "http://localhost:8080"
   ```

---

## Network & Proxy Issues

### Connection refused or network errors

**For cloud providers:**

1. **Check internet connection**

2. **Configure proxy if behind corporate firewall:**

   ```bash
   export HTTP_PROXY="http://proxy.example.com:8080"
   export HTTPS_PROXY="http://proxy.example.com:8080"
   export NO_PROXY="localhost,127.0.0.1"
   ```

3. **Check firewall rules:**
   - Ensure access to provider domains
   - Gemini: `generativelanguage.googleapis.com`
   - OpenAI: `api.openai.com`
   - Anthropic: `api.anthropic.com`

**For local models:**

1. **Verify server is running:**

   ```bash
   curl http://localhost:8080/health
   ```

2. **Check baseURL in config:**

   ```yaml
   models:
     local:
       provider: "llama"
       baseURL: "http://localhost:8080" # Must match server
   ```

3. **Check server logs** for errors

---

## Debugging & Trace Logs

### Understanding Trace Logs

Starting from version 0.1.0, `meowg1k` automatically logs detailed trace information for every command execution.

## Debugging & Trace Logs

### Understanding Trace Logs

Starting from version 0.1.0, `meowg1k` automatically logs detailed trace information for every command execution.

**Log Location:**

```text
<workspace-root>/.meowg1k/logs/
```

The workspace root is automatically detected by searching upward from your current directory for:

- `.meowg1k.yaml` or `.meowg1k.yml` config file
- `.git` directory
- Or the directory specified with `--workspace` flag

**What Gets Logged:**

- **API Interactions:** Every LLM API call with request prompts, responses, timing, and token usage
- **Execution Events:** Flow and activity lifecycle (starting, running, completed, failed, retries)
- **Application Errors:** Critical errors from internal components

**Log Format:**

- Each execution creates a new log file: `YYYYMMDD_HHMMSS_xxxxx.log.jsonl`
- JSON Lines format (each line is a valid JSON object)
- Can be parsed with standard JSON tools

### Working with Trace Logs

**View the latest log:**

```bash
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1)
```

**Extract API interactions:**

```bash
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | \
  jq 'select(.log_entry_type == "api_interaction")'
```

**View request prompts:**

```bash
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | \
  jq 'select(.log_entry_type == "api_interaction") | .request'
```

**Count events by status:**

```bash
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | \
  jq -r 'select(.log_entry_type == "execution_event") | .status' | \
  sort | uniq -c
```

**Find errors:**

```bash
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | \
  jq 'select(.status == "failed" or .status == "error")'
```

**View token usage:**

```bash
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | \
  jq 'select(.log_entry_type == "api_interaction") |
      {model: .model, input: .input_tokens, output: .output_tokens, total: .total_tokens}'
```

### Privacy & Security

**Important:** Trace logs may contain:

- Code from your files
- Prompts and AI responses
- API request details
- File paths and names

**Best practices:**

1. Logs are automatically excluded from git (via `.gitignore`)
2. Review logs before sharing
3. Redact sensitive information when posting issues
4. Consider disabling logging in CI/CD if handling sensitive data

---

## Getting Further Help

If your problem is not listed here:

### 1. Check Trace Logs

Review logs in `.meowg1k/logs/` for detailed error information:

```bash
# View latest log
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | jq '.'

# Find errors
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | \
  jq 'select(.status == "failed")'
```

### 2. Search Existing Issues

Check if someone has reported a similar problem:

**GitHub Issues:** [https://github.com/retran/meowg1k/issues](https://github.com/retran/meowg1k/issues)

Use search with relevant keywords:

- Error messages
- Command names (`commit`, `index`, etc.)
- Provider names (`gemini`, `openai`, etc.)

### 3. Open a New Issue

If your problem is new, open a detailed bug report:

**GitHub New Issue:** [https://github.com/retran/meowg1k/issues/new/choose](https://github.com/retran/meowg1k/issues/new/choose)

**Include:**

1. **Command you ran:**

   ```bash
   meow commit --target-branch main
   ```

2. **Configuration (with secrets removed):**

   ```yaml
   models:
     default:
       provider: "gemini"
       model: "gemini-2.0-flash-exp"
   ```

3. **Error output:**

   ```text
   Error: failed to generate commit message: API rate limit exceeded
   ```

4. **Relevant log excerpts:**

   ```json
   {"log_entry_type": "api_interaction", "status": "failed", ...}
   ```

5. **Environment:**
   - OS: macOS 14.5
   - meowg1k version: `meow version`
   - Installation method: Homebrew

### 4. Community Support

- **Discussions:** [GitHub Discussions](https://github.com/retran/meowg1k/discussions) for questions and ideas
- **Documentation:** Review all guides in the [docs directory](./README.md)

---

## Quick Reference

### Most Common Issues

| Symptom             | Likely Cause        | Quick Fix                       |
| ------------------- | ------------------- | ------------------------------- |
| `command not found` | PATH not set        | Add to PATH and reload shell    |
| `No API key found`  | Missing env var     | Export `MEOW_*_API_KEY`         |
| `Invalid API key`   | Wrong/expired key   | Regenerate from provider        |
| `No staged changes` | No files staged     | Run `git add` first             |
| `--base required`   | Missing flag        | Add `--base main`               |
| `Diff too large`    | Using flat strategy | Switch to `strategy: summarize` |
| `No index found`    | Not indexed         | Run `meow index`                |
| `Config not found`  | Missing config      | Run `meow init`                 |
| Timeout             | Slow model          | Increase `timeout` in profile   |
| Rate limited        | Too many requests   | Configure `rateLimit`           |

### Diagnostic Commands

```bash
# Check version
meow version

# Validate config (run any command with --help)
meow commit --help

# View environment variables
env | grep MEOW

# Check PATH
echo $PATH | tr ':' '\n' | grep -i go

# Verify installation
which meow

# Check git status
git status

# View latest log
ls -lt .meowg1k/logs/ | head -1
```

---

[Back to Documentation Index](./README.md)
