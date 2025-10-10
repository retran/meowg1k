# Configuration Guide

`meowg1k` is designed to be highly configurable to fit any workflow. All configuration is managed through YAML files, allowing you to version control your setup.

This guide covers everything from setting your API keys to creating complex, rule-based workflows.

---

## 1. Setting API Keys

The primary way to provide API keys to `meowg1k` is through environment variables. The tool automatically looks for variables based on the provider you choose.

Set these in your shell profile (e.g., `~/.bashrc`, `~/.zshrc`):

```bash
# For Google Gemini
export MEOW_GEMINI_API_KEY="your-api-key-here"

# For OpenAI
export MEOW_OPENAI_API_KEY="sk-..."

# For Anthropic Claude
export MEOW_ANTHROPIC_API_KEY="sk-ant-..."
```

> **Note:** You can specify a custom environment variable name within a profile using the `apiKeyEnv` field if needed.

---

## 2. Configuration File Hierarchy

`meowg1k` loads configuration from up to three locations, with each subsequent location overriding the previous ones:

1. **User Config (Lowest priority):** `~/.config/meowg1k/config.yaml`

    - Use this for your personal defaults, like your preferred provider or model.

2. **Project Config:** `./.meowg1k/config.yaml`

    - This is the recommended location for project-specific settings. Commit this file to your repository to share the configuration with your team.

3. **Explicit Config (Highest priority):** `--config /path/to/your/config.yaml`

    - A path specified with the global `--config` flag will override all other configuration files.

---

## 3. Configuration File Structure

The `config.yaml` file has several top-level sections that control different aspects of the tool.

### `models`

Models are definitions of a specific LLM API endpoint, including its provider, connection details, and rate limits. You can define multiple models and reference them from different profiles.

```yaml
models:
  # A model for quick, free tasks via OpenRouter
  openrouter-llama-free:
    provider: "openrouter"
    model: "meta-llama/llama-3.1-8b-instruct:free"

  # A powerful model for complex reasoning
  claude-sonnet:
    provider: "anthropic"
    model: "claude-3-5-sonnet-20240620"
    maxInputTokens: 180000

  # A model for a local llama.cpp server
  local-dev:
    provider: "llama"
    baseURL: "http://localhost:8080" # Required for local models
    apiKeyEnv: "MY_LOCAL_API_KEY"    # Optional: specify a custom env var

  # A model with rate limiting to control costs
  openai-cost-controlled:
    provider: "openai"
    model: "gpt-4o"
    rateLimit:
      requestsPerMinute: 20  # Max 20 requests per minute (0 = unlimited)
      requestsPerDay: 500    # Max 500 requests per day
      tokensPerMinute: 40000 # Max 40k tokens (input + output) per minute
```

### `profiles`

Profiles define a reusable set of parameters for an LLM request, such as timeout or temperature. Each profile must reference a `model` defined in the `models` section. This allows you to create different behaviors (e.g., "creative" vs. "analytical") using the same underlying model.

```yaml
profiles:
  # A profile for fast, general tasks
  fast:
    model: "openrouter-llama-free"

  # A profile for complex tasks that may take longer
  smart:
    model: "claude-sonnet"
    timeout: "10m" # Increase timeout for long tasks
    temperature: 0.2

  # A profile for creative generation
  creative:
    model: "claude-sonnet"
    temperature: 0.8
    topK: 50
```

### `cache`

The top-level `cache` section allows you to configure caching for LLM responses to save time and reduce costs.

```yaml
cache:
  enabled: true
  ttl: "168h" # Cache entries expire after 1 week (7 * 24h)
```

You can also override these settings on a per-profile basis:

```yaml
profiles:
  no-cache-profile:
    model: "claude-sonnet"
    cache:
      enabled: false # Disable cache for this profile
```

### `filter`

The filter section allows you to exclude files from analysis using .gitignore-style patterns. This is an essential feature not only for ignoring noise like dependencies and build artifacts, but also for security and privacy. Use it to prevent files containing secrets or proprietary code from ever being sent to an AI provider.

```yaml
filter:
  ignore:
    # Standard noise
    - "node_modules/**"
    - "dist/**"

    # Files containing secrets
    - "**/*.pem"
    - "**/*.key"
    - ".env*"
    - "secrets.yaml"
    - "terraform.tfstate"

    # Proprietary or sensitive code directories
    - "internal/billing/proprietary_legacy_code/**"
```

### `summarize`

This section configures the "Map" phase for `commit` and `pullrequest` commands, where each file change is analyzed individually. It uses a rule-based system to apply different analysis strategies to different files.

```yaml
summarize:
  # Default settings applied when no specific rule matches
  default:
    profile: "fast"
    systemPrompt: "Summarize this code change concisely."

  # Rules are evaluated top-down; the first match wins
  rules:
    # 1. Skip documentation changes entirely
    - match: "**/*.md"
      skip: true

    # 2. Use a powerful model for critical Go files
    - match: "internal/adapters/**/*.go"
      profile: "smart"
      systemPrompt: "Analyze this Go code change, focusing on business logic and potential side effects."

    # 3. Skip generated test snapshots
    - match: "**/*.snap"
      skip: true
```

### `generate`, `commit`, and `pullrequest`

These top-level sections configure the specific commands.

- **`generate`**: Define pre-set tasks for the `meow generate -t <task-name>` command.
- **`commit` / `pullrequest`**: Configure the final "Reduce" phase, where individual file summaries are combined into a single commit message or PR description.

```yaml
generate:
  default:
    profile: "smart"
    systemPrompt: "You are an expert software engineer."
  tasks:
    security-review:
      userPrompt: "Perform a comprehensive security review of this code."
    add-tests:
      profile: "smart" # Can override the default profile
      userPrompt: "Write comprehensive unit tests for this code in Go."

commit:
  profile: "smart"
  systemPrompt: |
    You are an expert software engineer reviewing code changes. Your task is to write a high-quality commit message in the Conventional Commits format based on the provided summaries of file changes.

    Follow these rules:
    1.  **Type:** Deduce the correct type (`feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `ci`).
    2.  **Scope:** If possible, identify a logical scope (e.g., `config`, `api`, `db`).
    3.  **Subject:** Write a concise, imperative-mood summary of the change (e.g., "add user login endpoint" not "added user login endpoint").
    4.  **Body (Optional):** If the change is non-trivial, add a body explaining the "why" behind the change, not just the "what". Describe the problem and the solution.
    5.  **Footer:** If applicable, add a `BREAKING CHANGE:` notice or link to issues (e.g., `Closes #42`).

pullRequest:
  profile: "smart"
  systemPrompt: |
    You are an expert software engineer tasked with writing a Pull Request description. Based on the summaries of file changes, generate a complete PR description in Markdown format.

    The output must include two parts:
    1.  **A short, descriptive Title.**
    2.  **A detailed Body** using the following template:

    ---

    ## Goal
    Describe the main purpose of this PR. What problem does it solve? Link to the relevant issue if one exists.

    ## Summary of Changes
    Provide a bullet-point list of the most important changes made in this PR.
    - Change 1...
    - Change 2...

    ## How to Test
    Provide clear, step-by-step instructions for how a reviewer can test these changes.
    1. Checkout this branch.
    2. Run `...`
    3. Check that `...` works as expected.
```

---

## 4. Supported Providers

| Provider          | `provider` value    | Default Environment Variable     |
| ----------------- | ------------------- | -------------------------------- |
| Google Gemini     | `gemini`            | `MEOW_GEMINI_API_KEY`            |
| OpenAI            | `openai`            | `MEOW_OPENAI_API_KEY`            |
| Anthropic Claude  | `anthropic`         | `MEOW_ANTHROPIC_API_KEY`         |
| OpenRouter        | `openrouter`        | `MEOW_OPENROUTER_API_KEY`        |
| Llama.cpp         | `llama`             | `MEOW_LLAMA_API_KEY` (optional)  |
| OpenAI Compatible | `openai-compatible` | `MEOW_OPENAI_COMPATIBLE_API_KEY` |
| Voyage AI         | `voyage`            | `MEOW_VOYAGE_API_KEY`            |

---

## 5. Complete Examples

### Minimal Configuration

For a quick start, you only need to define a default model and profile.

```yaml
# .meowg1k/config.yaml
models:
  default:
    provider: "gemini"
    model: "gemini-1.5-flash-latest"

profiles:
  default:
    model: "default"

# All commands will now use this profile by default.
generate:
  default:
    profile: "default"
commit:
  profile: "default"
pullRequest:
  profile: "default"
```

### Comprehensive Configuration

This example showcases multiple features working together.

```yaml
# .meowg1k/config.yaml
models:
  gemini-flash:
    provider: "gemini"
    model: "gemini-1.5-flash-latest"
    rateLimit:
      requestsPerMinute: 30
  claude-sonnet:
    provider: "anthropic"
    model: "claude-3-5-sonnet-20240620"

profiles:
  fast:
    model: "gemini-flash"
  smart:
    model: "claude-sonnet"
    timeout: "15m"

filter:
  ignore:
    - "dist/**"
    - "vendor/**"
    - "**/*_test.go" # Ignore test files in summaries

summarize:
  default:
    profile: "fast"
    systemPrompt: "Summarize the following code change."
  rules:
    - match: "internal/database/**/*.sql"
      profile: "smart"
      systemPrompt: "Analyze this SQL migration. Explain schema changes and potential data loss risks."

generate:
  default:
    profile: "smart"
  tasks:
    refactor:
      userPrompt: "Refactor this code to improve readability and performance."
    docs:
      profile: "fast"
      userPrompt: "Generate GoDoc comments for all public functions."

commit:
  profile: "smart"
  systemPrompt: "Write a Conventional Commit message based on the provided change summaries."

pullRequest:
  profile: "smart"
  systemPrompt: "Write a detailed PR description based on the provided change summaries. Include a title, a summary of changes, and potential risks."
```

---

## Next Steps

Now that you know how to configure `meowg1k`, it's time to learn about the specific commands in the [Command Reference](./03-COMMAND-REFERENCE.md).
