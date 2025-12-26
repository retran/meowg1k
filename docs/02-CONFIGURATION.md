# Configuration Guide

`meowg1k` is designed to be highly configurable to fit any workflow. All configuration is managed through YAML files, allowing you to version control your setup.

This guide covers everything from setting your API keys to creating complex, rule-based workflows.

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

## 2. Configuration File Hierarchy

`meowg1k` builds its configuration by merging settings from multiple files. This allows for a flexible setup where you can have global defaults, project-specific settings, and command-line overrides.

The configuration is loaded and merged in the following order:

1.  **User Config (Base):** `~/.config/meowg1k/config.yaml`
    - This file is loaded first. It's the ideal place for your personal defaults, like your preferred provider, model, or global rate limits. If this file doesn't exist, it's silently ignored.

2.  **Project or Explicit Config (Merge & Override):** After loading the user config, `meowg1k` merges settings from **one** additional source:
    - **If the `--config` flag is used:** The specified file is loaded. Its settings override the user config. The project config (`.meowg1k.yaml`) is **ignored**. If the specified file is not found, the program will exit with an error.
    - **If the `--config` flag is NOT used:** The tool looks for a project config file (`.meowg1k.yaml` or `.yml`) in the root of your project. If found, its settings override the user config. If not found, it's silently ignored.

This layered approach means that settings from the project/explicit config will override any settings defined in the user config.

### 2.1. Workspace Root Detection

`meowg1k` automatically detects the workspace (project) root directory by walking up the directory tree from your current working directory. This workspace root is used for multiple purposes:

- Finding the project configuration file (`.meowg1k.yaml` or `.yml`) when `--config` flag is not used
- Setting the working context for various commands (such as `commit`, `pullrequest`, etc.)
- Determining the scope of file operations and git operations

The tool looks for the following markers in each directory, stopping at the first match:

1. `.meowg1k.yaml` — Project-specific configuration file
2. `.meowg1k.yml` — Alternative extension for project configuration
3. `.git` — Git repository root directory

The search starts from the current directory and continues upward through parent directories until one of these markers is found or the filesystem root is reached.

**Examples:**

```text
/home/user/projects/myapp/src/feature
                          └── .meowg1k.yaml  ← Found here
```

Running `meow commit` from `/home/user/projects/myapp/src/feature/subdir` will detect `/home/user/projects/myapp/src/feature/` as the workspace root.

```text
/home/user/projects/myapp/
                    └── .git/  ← Git repository root
```

If no `.meowg1k.yaml` file exists, the tool will use the `.git` directory as a marker for the project root.

If no markers are found, the current working directory is used as the workspace root.

**Explicit Workspace Root:**

You can override the automatic detection by using the `--workspace` flag:

```bash
meow commit --workspace /path/to/project
```

This is useful when:

- Working from a directory outside your project
- Testing configurations for different projects
- Running commands in CI/CD environments where automatic detection may not work as expected

> **Note:** The workspace root is detected independently of configuration loading. Even when using the `--config` flag to specify an explicit configuration file, the workspace root is still determined and used for other operations. Use `--workspace` to explicitly set the workspace root when automatic detection is not suitable.

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
    model: "claude-sonnet-4-5-20250929"
    maxInputTokens: 180000

  # A model for a local llama.cpp server
  local-dev:
    provider: "llama"
    baseURL: "http://localhost:8080" # Required for local models
    apiKeyEnv: "MY_LOCAL_API_KEY" # Optional: specify a custom env var

  # A model with rate limiting to control costs
  openai-cost-controlled:
    provider: "openai"
    model: "gpt-4o"
    rateLimit:
      requestsPerMinute: 20 # Max 20 requests per minute (0 = unlimited)
      requestsPerDay: 500 # Max 500 requests per day
      tokensPerMinute: 40000 # Max 40k tokens (input + output) per minute
```

### `profiles`

Profiles define a reusable set of parameters for an LLM request, such as timeout, temperature, and sampling parameters. Each profile must reference a `model` defined in the `models` section. This allows you to create different behaviors (e.g., "creative" vs. "analytical") using the same underlying model.

````yaml
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
    topP: 0.95
    topK: 50

  # A profile for deterministic code generation
  deterministic-code:
    model: "claude-sonnet"
    temperature: 0.1
    maxTokens: 2048
    seed: 42 # For reproducible results
    stop: ["```", "END"] # Stop at code blocks or END marker

  # A profile to reduce repetition
  no-repeat:
    model: "gpt-4"
    temperature: 0.5
    frequencyPenalty: 0.8 # Discourage repeating tokens
    presencePenalty: 0.6 # Encourage topic diversity

  # A profile for JSON structured output
  json-output:
    model: "gpt-4"
    responseFormat: "json_object" # Force JSON output
    temperature: 0.3

  # A profile for OpenRouter with advanced sampling
  openrouter-creative:
    model: "openrouter-model"
    temperature: 0.9
    repetitionPenalty: 1.1 # OpenRouter-specific
    minP: 0.05 # OpenRouter-specific
    topA: 0.2 # OpenRouter-specific

  # A profile for llama.cpp with Mirostat
  local-mirostat:
    model: "local-dev"
    temperature: 0.7
    mirostat: 2 # Enable Mirostat v2
    mirostatTau: 5.0 # Target entropy
    mirostatEta: 0.1 # Learning rate
    typicalP: 0.95 # Typical sampling
````

#### Profile Parameters

##### Basic Parameters

- **`model`** (required): Reference to a model defined in the `models` section.
- **`timeout`** (optional): Request timeout duration (e.g., "5m", "10m"). Defaults to 5 minutes.

##### Sampling Parameters

- **`temperature`** (optional): Controls randomness in generation (0.0-2.0 for most providers). Lower values make output more focused and deterministic, higher values make it more creative.
- **`topP`** (optional): Controls nucleus sampling (0.0-1.0). The model considers only tokens with cumulative probability up to this threshold.
- **`topK`** (optional): Limits sampling to the top K most probable tokens. Use for additional control over randomness.
- **`seed`** (optional): Sets a random seed for deterministic sampling. Use the same seed to get reproducible results.

##### Penalty Parameters

- **`frequencyPenalty`** (optional): Penalizes tokens based on their frequency in the response (-2.0 to 2.0). Positive values discourage repetition.
- **`presencePenalty`** (optional): Penalizes tokens based on their presence in the response (-2.0 to 2.0). Positive values encourage topic diversity.
- **`repetitionPenalty`** (optional): Reduces repetition by penalizing tokens from the input (0.0-2.0). Higher values make repetition less likely. Supported by OpenRouter and Llama.cpp.

##### Output Control Parameters

- **`maxTokens`** (optional): Overrides the model's default maximum output tokens for this profile.
- **`stop`** (optional): List of sequences where the model will stop generating. E.g., `["END", "STOP"]`.
- **`candidateCount`** (optional): Number of response candidates to generate. Supported by OpenAI (as `n`) and Gemini.

##### Structured Output Parameters

- **`responseFormat`** (optional): Format of the response. Values: `"text"`, `"json_object"`, `"json_schema"`. Supported by OpenAI, Gemini, and OpenRouter.
- **`responseSchema`** (optional): JSON schema for structured output. When provided with `responseFormat: "json_schema"`, the model generates output matching this schema.
- **`grammar`** (optional): Grammar constraints in GBNF format (Llama.cpp only). For constrained generation following specific grammar rules.

##### Advanced Sampling Parameters

**OpenRouter-specific:**

- **`minP`** (optional): Minimum probability threshold relative to the most likely token (0.0-1.0). If set to 0.1, only tokens at least 1/10th as probable as the best option are considered.
- **`topA`** (optional): Top-A filtering based on "sufficiently high" probabilities (0.0-1.0). Dynamic filtering mechanism similar to Top-P.

**Llama.cpp-specific:**

- **`typicalP`** (optional): Locally typical sampling parameter (0.0-1.0). Balances creativity and coherence.
- **`mirostat`** (optional): Mirostat sampling algorithm mode. Values: `0` (disabled), `1` (Mirostat v1), `2` (Mirostat v2).
- **`mirostatTau`** (optional): Target entropy for Mirostat (default 5.0). Controls the balance of coherence/creativity in Mirostat sampling.
- **`mirostatEta`** (optional): Learning rate for Mirostat (default 0.1). Controls how quickly Mirostat adjusts.

##### Logging and Analysis Parameters

- **`logProbs`** (optional): Enable returning log probabilities of output tokens (boolean). Supported by OpenAI, Gemini, and Llama.cpp.
- **`topLogProbs`** (optional): Number of top log probabilities to return per token (0-20). Only used when `logProbs` is true.
- **`logitBias`** (optional): Map of token IDs to bias values (-100 to 100) to modify likelihood of specific tokens. Supported by OpenAI and Llama.cpp.

##### Service Parameters

- **`serviceTier`** (optional): Service tier for the request (e.g., "auto", "default"). OpenAI-specific for controlling priority capacity.
- **`user`** (optional): Unique identifier for the end-user. Used for abuse monitoring and tracking. OpenAI-specific.

##### Cache Parameters

- **`cache`** (optional): Override global cache settings for this profile (see Cache section).

**Note:** The availability and exact behavior of parameters may vary by provider:

| Parameter           | Gemini | Anthropic | OpenAI      | OpenRouter | Llama.cpp |
| ------------------- | ------ | --------- | ----------- | ---------- | --------- |
| `temperature`       | ✅     | ✅        | ✅          | ✅         | ✅        |
| `topP`              | ✅     | ✅        | ✅          | ✅         | ✅        |
| `topK`              | ✅     | ✅        | ❌          | ✅         | ✅        |
| `maxTokens`         | ✅     | ✅        | ✅          | ✅         | ✅        |
| `frequencyPenalty`  | ✅     | ❌        | ✅          | ✅         | ✅        |
| `presencePenalty`   | ✅     | ❌        | ✅          | ✅         | ✅        |
| `repetitionPenalty` | ❌     | ❌        | ❌          | ✅         | ✅        |
| `seed`              | ✅     | ❌        | ✅          | ✅         | ✅        |
| `stop`              | ✅     | ✅        | ✅          | ✅         | ✅        |
| `responseFormat`    | ✅     | ❌        | ✅          | ✅         | ❌        |
| `responseSchema`    | ✅     | ❌        | ✅          | ✅         | ❌        |
| `candidateCount`    | ✅     | ❌        | ✅ (as `n`) | ✅         | ❌        |
| `logProbs`          | ✅     | ❌        | ✅          | ✅         | ✅        |
| `topLogProbs`       | ✅     | ❌        | ✅          | ✅         | ❌        |
| `logitBias`         | ❌     | ❌        | ✅          | ✅         | ✅        |
| `serviceTier`       | ❌     | ❌        | ✅          | ❌         | ❌        |
| `user`              | ❌     | ❌        | ✅          | ❌         | ❌        |
| `minP`              | ❌     | ❌        | ❌          | ✅         | ✅        |
| `topA`              | ❌     | ❌        | ❌          | ✅         | ❌        |
| `typicalP`          | ❌     | ❌        | ❌          | ❌         | ✅        |
| `mirostat`          | ❌     | ❌        | ❌          | ❌         | ✅        |
| `mirostatTau`       | ❌     | ❌        | ❌          | ❌         | ✅        |
| `mirostatEta`       | ❌     | ❌        | ❌          | ❌         | ✅        |
| `grammar`           | ❌     | ❌        | ❌          | ❌         | ✅        |

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

#### Strategy Field

Both `commit` and `pullrequest` commands support a `strategy` field that determines how changes are processed:

- **`"summarize"` (default)**: Uses a Map-Reduce approach. Each changed file is summarized individually (the "Map" phase), then all summaries are combined to generate the final commit message or PR description (the "Reduce" phase). This strategy is ideal for large commits with many files, as it allows the model to focus on each file separately before synthesizing the overall message.

- **`"flat"`**: Sends the entire git diff directly to the model without any intermediate summarization. This is faster and more efficient for small commits (e.g., a single file or a few lines of changes), as it eliminates the overhead of the Map-Reduce pipeline. However, if the diff size exceeds the model's `maxInputTokens` limit, the command will fail with an error suggesting you switch to the `"summarize"` strategy.

**When to use each strategy:**

- Use `"flat"` for quick, small commits (< 5 files, simple changes)
- Use `"summarize"` (default) for larger changesets, complex refactorings, or when you need detailed per-file analysis

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
  strategy: "summarize" # Optional: "summarize" (default) or "flat"
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
  strategy: "summarize" # Optional: "summarize" (default) or "flat"
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

## 5. Complete Examples

### Minimal Configuration

For a quick start, you only need to define a default model and profile.

```yaml
# .meowg1k.yaml
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
# .meowg1k.yaml
models:
  gemini-flash:
    provider: "gemini"
    model: "gemini-1.5-flash-latest"
    rateLimit:
      requestsPerMinute: 30
  claude-sonnet:
    provider: "anthropic"
    model: "claude-sonnet-4-5-20250929"

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

### `index`

The `index` section configures the document indexing process for RAG (Retrieval-Augmented Generation) and semantic search capabilities.

```yaml
index:
  # Profile for computing embeddings (required)
  # This profile must reference a model that supports embeddings
  profile: "embeddings"

  # Chunker configuration
  chunker:
    # Maximum number of Unicode runes per chunk
    # Larger chunks = more context but fewer chunks
    # Recommended range: 512-2048
    maxRunes: 1024

    # Number of runes to overlap between consecutive chunks
    # Overlap ensures context continuity across chunk boundaries
    # Recommended: 10-20% of maxRunes
    overlapRunes: 128

  # Number of chunks to process in one embedding API call
  # Larger batches = faster indexing but higher memory usage
  # Guidelines:
  #   - Cloud APIs (Gemini, OpenAI): 32-100
  #   - Local models (llama.cpp): 1-8
  # Default: 32
  batchSize: 64
```

**Chunking Strategy:**

The chunker splits files into smaller pieces for embedding. The goal is to balance:

- **Context preservation**: Chunks large enough to contain meaningful context
- **Semantic coherence**: Each chunk represents a cohesive concept
- **Search precision**: Smaller chunks = more precise matches

**Recommended settings:**

| Use Case      | maxRunes | overlapRunes |
| ------------- | -------- | ------------ |
| General code  | 1024     | 128          |
| Documentation | 2048     | 256          |
| Config files  | 512      | 64           |

### `ask`

The `ask` section configures the RAG-based question answering feature.

```yaml
ask:
  # Profile for generating answers (required)
  # This should reference a capable LLM model for reasoning
  profile: "smart"

  # Number of top chunks to retrieve for context
  # More chunks = more context but slower and may introduce noise
  # Recommended range: 3-10
  topK: 5

  # Minimum similarity score for retrieved chunks (0.0 to 1.0)
  # Higher values = more relevant but fewer results
  # Recommended range: 0.6-0.8
  minScore: 0.7

  # System prompt for answer generation
  # Guides the LLM's behavior when answering questions
  systemPrompt: >-
    You are an expert AI assistant helping developers understand their codebase.
    Answer questions based ONLY on the provided code context.
    Be precise, concise, and reference specific code elements when relevant.
    If the context doesn't contain enough information, clearly state that.
```

**Tuning Guidelines:**

- **Broad questions** (e.g., "How does the system work?")
  - Higher `topK` (10-20)
  - Lower `minScore` (0.5-0.6)

- **Specific questions** (e.g., "Where is the login function?")
  - Lower `topK` (3-5)
  - Higher `minScore` (0.7-0.8)

### `agent`

The `agent` section configures the multi-step agent workflow used by `meow do`.
Each step (research, plan, execute, verify) can define its own profile, system prompt,
tool allow-list, and tool modes.

```yaml
agent:
  defaults:
    profile: "smart"
    systemPrompt: >-
      You are a multi-step agent that works in four steps: research, plan, execute, verify.
      Research gathers context without changes. Plan turns findings into ordered tasks. Execute applies the changes. Verify checks outcomes and reports gaps.
      Use the memory tool to keep context between steps: call memory.list at the start of each step, and call memory.add at the end of each step to store key findings, decisions, and outputs.
  tools:
    searchDefaults:
      snapshots: ["_workdir_", "_stage_", "_head_"]
      topK: 8
      minScore: 0.6
  steps:
    research:
      profile: "fast"
      systemPrompt: "Research step: discover context and constraints without modifying files."
      tools: ["workspace", "search", "summarize", "git", "plan", "memory"]
      toolModes:
        workspace: ["list", "read", "stat", "exists"]
        search: ["embeddings"]
        summarize: ["text", "file", "diff"]
        git: ["status", "log", "show", "diff", "branch", "current_branch"]
        plan: ["list"]
        memory: ["add", "list"]
```

**Notes:**

- If a step omits `profile` or `systemPrompt`, it inherits from `defaults`.
- The `defaults.systemPrompt` text is prepended to each step prompt to act as a shared instruction block.
- The `verify` step should emit a `VerificationResult: pass|fail` line so the agent can determine whether to retry.
- `toolModes` restricts which modes a tool can use; omit to allow all modes.
- Search defaults apply to `search` tool calls and can be overridden by CLI flags.

## Next Steps

Now that you know how to configure `meowg1k`, it's time to learn about the specific commands in the [Command Reference](./03-COMMAND-REFERENCE.md).
