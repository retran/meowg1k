# Troubleshooting Guide

This guide helps you solve common problems you might encounter while using `meowg1k`.

---

## Installation Issues

### Problem: `meow: command not found` after installation

**Solution:**

- This usually means the installation directory is not in your system's `PATH`.
- **For `go install`:** Ensure your Go binary path is in your `PATH`. You can find it by running `go env GOBIN` or `go env GOPATH`. Add this to your shell profile (e.g., `~/.bashrc`):

```bash
  export PATH="$PATH:$(go env GOPATH)/bin"
```

- **For Homebrew:** Try running `brew doctor` to diagnose issues, or `brew unlink meow && brew link meow` to relink the binary.
- After making changes, restart your terminal or source your shell profile file.

### Problem: `.deb` or `.rpm` installation fails

**Solution:**

- **For Debian/Ubuntu:** Try to fix any broken dependencies first:

```bash
sudo apt --fix-broken install
sudo dpkg -i meow_<version>_amd64.deb
```

- **For Fedora/RHEL:** Use `dnf` to handle dependencies automatically:

```bash
sudo dnf install ./meow-<version>-1.x86_64.rpm
```

---

## API & Authentication

### Problem: `No API key found for provider` error

**Solution:**

- Ensure you have set the correct environment variable for your provider. For example:

```bash
export MEOW_GEMINI_API_KEY="your-key-here"
```

- Make sure you have reloaded your shell after setting the variable (`source ~/.bashrc`) or have opened a new terminal.
- Verify the variable is set by running `echo $MEOW_GEMINI_API_KEY`.

### Problem: `Invalid API key` or authentication errors

**Solution:**

- Double-check that you copied the API key correctly, with no extra spaces or characters.
- Verify the key format. OpenAI keys usually start with `sk-`, Anthropic keys with `sk-ant-`.
- Regenerate the API key from your provider's dashboard to ensure it's active and has the correct permissions.

### Problem: Hitting provider rate limits

**Solution:**

- Your usage is exceeding your provider's limits.
- The best solution is to configure rate limiting in your `.meowg1k/config.yaml` model definition to stay within the allowed budget.

```yaml
models:
  openai-safe:
    provider: "openai"
    model: "gpt-4o"
    rateLimit:
      requestsPerMinute: 20
      tokensPerMinute: 40000
```---

## Configuration

### Problem: `Config file not found`

**Solution:**

- Make sure a config file exists at one of the default locations: `./.meowg1k/config.yaml` or `~/.config/meowg1k/config.yaml`.
- If using the `--config` flag, verify that the path is correct.
- Check file permissions to ensure the file is readable.

### Problem: Invalid YAML syntax errors

**Solution:**

- YAML is sensitive to indentation. Ensure you are using spaces (usually 2), not tabs.
- Use an online YAML validator to check your file for syntax errors.
- Strings containing special characters (like `:`) may need to be enclosed in quotes.

---

## Command Usage

### Problem: `meow pullrequest` fails with "missing required flag: --base"

**Solution:**

- The `pullrequest` command always requires you to specify the target branch for comparison. Add the flag to your command:

```bash
meow pullrequest --base main
```

#### Pro Tip: Automate Target Branch Detection

You can create a shell function or script that automatically detects your repository's default branch:

```bash
# Add this to your ~/.bashrc or ~/.zshrc
function mpr() {
    # Try to get the default branch from origin/HEAD
    default_branch=$(git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@')
    if [ -z "$default_branch" ]; then
        # Fallback for detached HEAD or other cases
        default_branch="main"
    fi
    echo "Using base branch: $default_branch"
    meow pullrequest --base "$default_branch" "$@"
}
```

After adding this function and reloading your shell, you can simply run `mpr` instead of `meow pullrequest --base main`.
```

After adding this function and reloading your shell, you can simply run `mpr` instead of `meow pr --base main`.

### Problem: `meow commit` shows "no staged changes"

**Solution:**

- In its default mode, `meow commit` only analyzes files that have been staged with `git add`.

```bash
# Stage your files first
git add .
meow commit
```

- If you want to generate a commit for all changes on your branch, use the `--target-branch` flag instead:

```bash
meow commit --target-branch main
```

### Problem: Commands are timing out

**Solution:**

- Some AI models can take a long time to respond. You can increase the request timeout in your profile configuration (default is 5 minutes).

```yaml
profiles:
    slow-model:
    provider: "anthropic"
    model: "claude-sonnet-4-5-20250929"
    timeout: "15m" # Increase timeout to 15 minutes
```

---

## Network & Proxy Issues

### Problem: Connection refused or network errors

**Solution:**

- **For cloud providers:** Check your internet connection and firewall settings. If you are behind a corporate proxy, you may need to set environment variables:

```bash
export HTTP_PROXY="[http://proxy.example.com:8080](http://proxy.example.com:8080)"
export HTTPS_PROXY="[http://proxy.example.com:8080](http://proxy.example.com:8080)"
```

- **For local models:** Ensure your `llama.cpp` server is running and that the `baseURL` in your profile matches the server's address and port (e.g., `http://localhost:8080`).

---

## Debugging & Trace Logs

### Understanding Trace Logs

Starting from version 0.1.0, `meowg1k` automatically logs detailed trace information for every command execution to help with debugging and auditing. These logs are stored in `.meowg1k/logs/` within your workspace root.

**What gets logged:**
- **API Interactions:** Every LLM API call, including request prompts, responses, timing, and token usage
- **Execution Events:** Flow and activity lifecycle events (starting, running, completed, failed, retry attempts)
- **Application Errors:** Critical errors from internal components

**Log Location:**
```
<workspace-root>/.meowg1k/logs/
```

The workspace root is automatically detected by searching upward from your current directory for:
- `.meowg1k.yaml` or `.meowg1k.yml` config file
- `.git` directory
- Or the directory specified with `--workspace` flag

**Log Format:**
- Each command execution creates a new log file with a unique timestamp
- Format: `YYYYMMDD_HHMMSS_xxxxx.log.jsonl` (JSON Lines format)
- Each line is a valid JSON object that can be parsed independently

**Example:** Viewing recent API interactions:
```bash
# View the latest log file
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1)

# Extract all API interactions from the latest log
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | jq 'select(.log_entry_type == "api_interaction")'

# View just the prompts from the latest run
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | jq 'select(.log_entry_type == "api_interaction") | .request'

# Count execution events by status
cat .meowg1k/logs/$(ls -t .meowg1k/logs/ | head -1) | jq 'select(.log_entry_type == "execution_event") | .status' | sort | uniq -c
```

**Note:** Trace log files are excluded from version control by default (via `.gitignore`). These logs may contain sensitive information from your prompts and code, so be careful when sharing them.

---

## Getting Further Help

If your problem is not listed here, please:

1. **Check Trace Logs:** Review the trace logs in `.meowg1k/logs/` for detailed information about what went wrong.

2. **Search Existing Issues:** Check the [GitHub Issues](https://github.com/retran/meowg1k/issues) to see if someone has already reported a similar problem.

3. **Open a New Issue:** If your problem is new, please [open a detailed bug report](https://github.com/retran/meowg1k/issues/new/choose). Include the command you ran, your configuration (with secrets removed), the output you received, and relevant excerpts from your trace logs (with any sensitive data removed).
