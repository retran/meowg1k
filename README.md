# 🐱 meowg1k

> ⚠️ **WIP: This documentation may be outdated as the project is under active development.**

> Your purr-sonal AI sidekick for coding, writing, and automating anything — right from your terminal.

<div align="center">

  ![Go](https://img.shields.io/badge/Go-1.25.1+-00ADD8?style=for-the-badge&logo=go)
  ![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg?style=for-the-badge)
  ![GitHub stars](https://img.shields.io/github/stars/retran/meowg1k?style=for-the-badge)
  ![GitHub forks](https://img.shields.io/github/forks/retran/meowg1k?style=for-the-badge)

</div>

<div align="center">

  <img src="https://github.com/retran/meow/raw/dev/assets/icon_small.png" alt="Meow Logo" width="200">

  <br>

  <strong>meowg1k — AI Programming Assistance Tool</strong>

</div>

`meowg1k` is a command-line interface that brings the power of modern LLMs (Large Language Models) into your development workflow. With a single command, you can get code explanations, refactor suggestions, or fully generated files — all without leaving your terminal.

Part of the `project meow` ecosystem, `meowg1k` is the **AI counterpart** to [`meow`](https://github.com/retran/meow), which sets up your dev environment.

---

## Table of Contents

- [Core Principles](#core-principles)
- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)
- [Verification & Security](#verification--security)
- [Contributing](#contributing)
- [License](#license)
- [Acknowledgments](#acknowledgments)

## Core Principles

These are the fundamental, non-negotiable principles that guide the architecture and development of the project. Every future decision must adhere to these rules.

---

### 1. A Composable Engine, Not an Application

This principle defines the tool's role in the software ecosystem. It is engineered to be a reliable component—a building block—that can be plugged into larger, automated systems. Its primary purpose is to be called programmatically by other tools, such as shell scripts, CI/CD runners, or custom automation pipelines.

This is the opposite of a monolithic, interactive application that a user operates manually. The design prioritizes scriptability, predictable I/O (input/output), and the ability to function as a dependable part of a larger, unattended workflow.

---

### 2. Task Execution, Not a Conversation

This principle defines the tool's interaction model. It is designed to execute discrete, well-defined tasks that have a clear beginning and end. It is fundamentally not a chatbot or a conversational agent. Its mode of operation is transactional (`Input → Process → Output`), not a continuous, back-and-forth dialogue.

This design choice is critical for ensuring predictability, testability, and suitability for automation. A conversational model introduces ambiguity and complex state management, which is undesirable in automated toolchains. By focusing on atomic tasks, the tool remains a reliable and deterministic engineering component.

---

### 3. Native Performance. Zero Dependencies

This principle dictates that the tool must be fast, efficient, and easy to deploy anywhere. It is delivered as a single, self-contained native binary. This is a deliberate architectural choice to avoid the overhead and complexity of external runtimes like the JVM, Python, or Node.js.

The direct consequence of this is frictionless deployment and high performance. The tool starts instantly, uses minimal system resources, and can be run in any environment—from a developer's laptop to a stripped-down Docker container—without any installation or configuration of dependencies.

---

### 4. Radical Independence. No Lock-in

The system is architected to be fundamentally decoupled from any specific vendor, platform, or API. This ensures the user is always in control and is never locked into a single company's ecosystem.

This means that switching between different AI providers (e.g., from a commercial cloud service to a self-hosted local model) is a simple configuration change, not a complex code refactoring. The tool is designed to be a neutral and adaptable client, preserving the user's freedom of choice.

---

### 5. Local-First Architecture

This principle mandates that the tool's core functionality must be able to operate without a mandatory internet connection. The ability to work offline is a primary design requirement, not an optional feature. This ensures the tool is reliable even with network instability.

More importantly, this architecture guarantees data privacy and user control. Sensitive information, such as proprietary source code, does not need to leave the user's machine unless they explicitly configure the tool to use a cloud-based service. The tool is designed to be fully functional on a local machine.

---

### 6. Configuration is Code

All aspects of the tool's behavior are defined in structured, version-controllable, plain-text files. The system is built on a hierarchical model, which allows a general, base configuration to be layered with more specific settings for a particular project or environment.

This approach ensures that every workflow is transparent, reproducible, and auditable. An entire setup can be committed to a Git repository, reviewed by team members, and shared, eliminating any "it works on my machine" issues. There is no hidden state or required manual setup.

---

### 7. Process Predictability and Audibility

By its nature, the output from an LLM is stochastic (non-deterministic). This principle defines how the tool behaves reliably in that context. The tool's own logic, up to the point of calling an LLM, is strictly deterministic. Given the same input and configuration, it will always generate an identical, byte-for-byte request to the model.

Any randomness in the final result originates exclusively from the AI model's own sampling process. This randomness is, in turn, explicitly controlled by the user through configuration parameters like `temperature`. This design ensures that the tool's process is completely transparent, debuggable, and auditable.

---

### 8. Intelligent Context, Not Raw Input

The primary value of this tool is not to simply pass text to an AI, but to intelligently prepare the input for the best possible result. The project is guided by the philosophy of automatically enriching a user's prompt with relevant, discoverable context.

This means the tool is committed to evolving its capabilities to analyze source code and its surrounding environment. The goal is to reduce the manual effort a user needs to expend to gather and provide the context required for the AI to produce a high-quality, relevant, and useful output.

The project's long-term vision is to achieve a deep, contextual understanding of the entire workspace by analyzing code statically and dynamically, leveraging Git history, and employing Retrieval-Augmented Generation on a local code index. This rich context will empower the tool to orchestrate smart, non-AI tools for precise code modifications.

---

### 9. Security by Design

Security is a foundational requirement, not an add-on. The system is architected under a "zero trust" model where it is designed never to store or persist user secrets, such as API keys. Secrets are only held in memory for the duration of a request and are then discarded.

This principle also extends to the integrity of the tool itself. All official releases must be cryptographically signed. This allows users to independently verify that the executable they are running is the exact one produced by the official build process and has not been altered or compromised.

---

### 10. Radically Open

The Simple Rule: Every line of code that runs in this tool, including all its dependencies, must be open source.

This is a strict commitment to total transparency, which is essential for trust and security. It means that not only is the project's own code available for inspection, but its entire software supply chain is also auditable. There are no proprietary, closed-source "black boxes" anywhere in the stack where hidden or malicious behavior could reside.

Furthermore, the project uses a permissive open-source license. This gives users the freedom to use, modify, and integrate the tool into their own work—even for commercial purposes—with minimal legal restrictions.

---

### 11. You Control the Economics

The Simple Rule: The architecture must give the user total control over operational costs, through both explicit configuration and the tool's own internal efficiency.

This principle ensures that operational expenses are a transparent and manageable parameter of the workflow, not a surprising side effect. Control is achieved in two primary ways:

1. **Direct Control via Configuration:** You have full and granular control over all factors that influence cost, such as selecting the model, provider, and usage parameters. This empowers you to make a deliberate economic trade-off for any given task.
2. **Internal Efficiency via Context Engineering:** The tool is designed to be inherently cost-effective. The principle of **Intelligent Context Engineering (#8)** directly contributes to cost control by optimizing the context to send the minimum number of tokens required for a high-quality result. This is a built-in, automatic cost-reduction mechanism.

## Features

- **Generate code** from a prompt or from stdin
- **Refactor** existing code automatically
- **Explain** code in plain language
- **Reusable tasks**: Predefine prompts and profiles in config files
- **Profile-based configuration**: Clean, hierarchical config system with smart defaults
- **Project + user configs**: Override defaults per project or globally
- **Comprehensive AI provider support**:
  - **Content Generation**: Gemini, OpenAI, OpenRouter, Anthropic Claude, llama.cpp (local), and OpenAI-compatible APIs
  - **Embeddings**: Gemini, Voyage AI (recommended by Anthropic), OpenAI/OpenRouter, and OpenAI-compatible APIs
- **Smart defaults**: Minimal configuration required - just specify the provider and API key
- **Environment-based API keys**: Secure credential management via environment variables

---

## Prerequisites

- **Go**: version 1.25.1 or newer
- **Internet connection** for cloud models (Gemini, OpenAI, OpenRouter)
- **API key** for your chosen provider:
  - Gemini: `MEOW_GEMINI_API_KEY`
  - OpenAI: `MEOW_OPENAI_API_KEY`
  - OpenRouter: `MEOW_OPENROUTER_API_KEY`
  - Anthropic: `MEOW_ANTHROPIC_API_KEY`
  - Voyage AI: `MEOW_VOYAGE_API_KEY`
  - Llama (local): `MEOW_LLAMA_API_KEY` (optional)
- For llama.cpp:
  - A running local server
  - Base URL to connect (e.g., `http://localhost:8080`)

---

## Installation

### Install with Go

```bash
go install github.com/retran/meowg1k@latest
```

### Install with Homebrew (macOS / Linux)

```bash
brew install retran/homebrew-meow-tap/meow
```

### Install with Scoop (Windows)

```powershell
scoop bucket add meow https://github.com/retran/scoop-meow-bucket.git
scoop install meow
```

### Install `.deb` package (Debian / Ubuntu / Linux Mint)

Download the `.deb` from the [Releases](https://github.com/retran/meowg1k/releases) page and run:

```bash
sudo dpkg -i meow_<version>_amd64.deb
# or for ARM64
sudo dpkg -i meow_<version>_arm64.deb
```

### Install `.rpm` package (Fedora / CentOS / openSUSE)

Download the `.rpm` from the [Releases](https://github.com/retran/meowg1k/releases) page and run:

```bash
sudo rpm -i meow-<version>-1.x86_64.rpm
# or for ARM64
sudo rpm -i meow-<version>-1.aarch64.rpm
```

> Packaging names come from our nfpm config (`package_name: meow`). Exact filenames on GitHub Releases are generated by GoReleaser and may include distro/arch suffixes as shown above.

---

## Quick Start

```bash
# Explain code using a predefined task
cat main.go | meow generate -t review

# Direct prompt with stdin
cat component.js | meow g -p "Refactor for performance"

# Use a predefined task
cat app.py | meow g -t test
```

> Docs are available via `man meow`, `man meow-generate`, `man 5 meow-config`, and `man 7 meow-security`.

---

## Configuration

`meow` uses a powerful profile-based configuration system for maximum flexibility. Configuration files are read in this order of precedence:

1. **Explicit config**: `--config /path/to/config.yaml` (overrides all others)
2. **Project config**: `./.meowg1k/config.yaml`
3. **User config**: `~/.config/meowg1k/config.yaml`

**Important**: When `--config` is specified, only that file is used, completely ignoring project and user configs.

### Profile-Based Configuration

The architecture centers around **reusable profiles** that define provider configurations. Each profile encapsulates all the settings needed to connect to a specific AI provider:

```yaml
profiles:
  # Minimal configuration - uses smart defaults
  fast:
    provider: "gemini"
    # model: gemini-2.5-flash (default)
    # apiKeyEnv: MEOW_GEMINI_API_KEY (default)
    # maxInputTokens: 8192 (default)
    # timeout: 5m (default)

  smart:
    provider: "openai"
    model: "gpt-4o"
    maxInputTokens: 32768
    timeout: "10m"

  local:
    provider: "llama"
    baseURL: "http://localhost:8080"  # Required for llama
    apiKeyEnv: "LLAMA_SERVER_KEY"     # Optional

  anthropic-pro:
    provider: "anthropic"
    model: "claude-3-5-sonnet-20241022"
    maxInputTokens: 200000

generate:
  default:
    profile: "fast"
    systemPrompt: "You are a helpful AI assistant."
  tasks:
    review:
      profile: "smart"
      userPrompt: "Perform a thorough code review."
    security:
      profile: "anthropic-pro"
      userPrompt: "Analyze for security vulnerabilities."
```

### Supported Providers

| Provider            | Environment Variable             | Default Model                           | Base URL                           | Capabilities            |
| ------------------- | -------------------------------- | --------------------------------------- | ---------------------------------- | ----------------------- |
| `gemini`            | `MEOW_GEMINI_API_KEY`            | `gemini-2.5-flash`                      | *automatic*                        | Generation + Embeddings |
| `openai`            | `MEOW_OPENAI_API_KEY`            | `gpt-5-mini`                            | `https://api.openai.com/v1`        | Generation + Embeddings |
| `openrouter`        | `MEOW_OPENROUTER_API_KEY`        | `meta-llama/llama-3.2-3b-instruct:free` | `https://openrouter.ai/api/v1`     | Generation + Embeddings |
| `anthropic`         | `MEOW_ANTHROPIC_API_KEY`         | `claude-3-5-haiku-20241022`             | *automatic*                        | Generation only         |
| `voyage`            | `MEOW_VOYAGE_API_KEY`            | `voyage-3.5`                            | `https://api.voyageai.com/v1`      | Embeddings only         |
| `llama`             | `MEOW_LLAMA_API_KEY`             | *server-defined*                        | *required*                         | Generation only         |
| `openai-compatible` | `MEOW_OPENAI_COMPATIBLE_API_KEY` | *server-defined*                        | *required*                         | Generation + Embeddings |

### Advanced Configuration Examples

```yaml
profiles:
  # Anthropic Claude for sophisticated reasoning
  anthropic-smart:
    provider: "anthropic"
    model: "claude-3-5-sonnet-20241022"
    maxInputTokens: 200000
    timeout: "10m"

  # Voyage AI for high-quality embeddings (recommended by Anthropic)
  voyage-embeddings:
    provider: "voyage"
    model: "voyage-3.5"
    # Note: Voyage is embeddings-only, cannot generate text

  # OpenRouter for accessing multiple models through one API
  openrouter-smart:
    provider: "openrouter"
    model: "anthropic/claude-3.5-sonnet"
    maxInputTokens: 200000

  # Custom OpenAI-compatible provider (e.g., Ollama, local APIs)
  custom-local:
    provider: "openai-compatible"
    baseURL: "http://localhost:11434/v1"  # Ollama default
    model: "llama3.1:8b"
    apiKeyEnv: "OLLAMA_API_KEY"  # Optional for Ollama

generate:
  default:
    profile: "anthropic-smart"
    systemPrompt: "You are an expert software engineer."

  tasks:
    security-review:
      profile: "anthropic-smart"
      userPrompt: "Perform a comprehensive security analysis of this code."

    quick-fix:
      profile: "openrouter-smart"
      userPrompt: "Suggest a quick fix for this issue."

    local-test:
      profile: "custom-local"
      userPrompt: "Write unit tests for this code."
```

---

## Examples

### 1. Using predefined tasks

```bash
# Use a predefined review task from config
cat main.go | meow generate -t review

# Use a predefined security analysis task
cat auth.go | meow generate -t security-review

# Use a quick fix task
cat buggy.py | meow g -t quick-fix
```

### 2. Direct prompts with profiles

```bash
# Quick generation with default profile
echo "Create a REST API handler" | meow generate -p "Write Go code"

# Direct prompt (uses default profile from config)
cat service.py | meow g -p "Add comprehensive error handling"

# Combine user prompt with stdin context
cat legacy_code.js | meow g -p "Modernize this code to use ES2023 features"
```

### 3. Working with different providers through profiles

Configure profiles in your config, then use them via tasks:

```yaml
# .meowg1k/config.yaml
profiles:
  fast:
    provider: "openrouter"
    model: "meta-llama/llama-3.2-3b-instruct:free"
  smart:
    provider: "anthropic"
    model: "claude-3-5-sonnet-20241022"
  local:
    provider: "llama"
    baseURL: "http://localhost:8080"

generate:
  default:
    profile: "fast"
  tasks:
    analyze:
      profile: "smart"
      userPrompt: "Analyze the architecture and suggest improvements"
    optimize:
      profile: "local"
      userPrompt: "Optimize this code for performance"
```

Then use:

```bash
# Uses fast profile (default)
cat utils.go | meow g -p "Add error handling"

# Uses smart profile via task
cat microservice.go | meow g -t analyze

# Uses local profile via task
cat algorithm.py | meow g -t optimize
```

### 4. Project-specific configuration

Create `.meowg1k/config.yaml` in your project root:

```yaml
profiles:
  project-review:
    provider: "anthropic"
    model: "claude-3-5-sonnet-20241022"
    maxInputTokens: 200000

  project-test:
    provider: "openai"
    model: "gpt-4o"

generate:
  default:
    profile: "project-review"
  tasks:
    security:
      profile: "project-review"
      userPrompt: "Review this code for security vulnerabilities"
    test:
      profile: "project-test"
      userPrompt: "Generate comprehensive unit tests"
```

Then use it:

```bash
# Uses project-review profile (default)
cat auth.go | meow g -p "Review authentication logic"

# Uses project-review profile via task
cat auth.go | meow g -t security

# Uses project-test profile via task
cat handler.go | meow g -t test
```

### 5. Working with embedding models

Configure embedding-specific profiles:

```yaml
profiles:
  voyage-embed:
    provider: "voyage"
    model: "voyage-3.5"

  gemini-embed:
    provider: "gemini"
    model: "text-embedding-004"

# Note: Current CLI focuses on generation
# Embedding functionality available via API
```

---

## Troubleshooting

- **No API key**: Set `MEOW_GEMINI_API_KEY` in your shell profile
- **Timeouts**: Increase `defaultTimeout` in config
- **Model not found**: Use supported models: `gemini-2.5-pro` or `gemini-2.5-flash`
- **Deb/RPM install issues**:
  - On `.deb` systems: `sudo apt --fix-broken install`
  - On `.rpm` systems: `sudo dnf install -y ./meow-<version>-1.x86_64.rpm`

---

## Verification & Security

All release artifacts are:

- **Signed** with [Sigstore cosign](https://docs.sigstore.dev/cosign/overview) (keyless, via GitHub OIDC)
- **Shipped with SBOM** (Software Bill of Materials) for dependency transparency

> See also `man 7 meow-security` for detailed security guidelines.

### Verify a Release

```bash
cosign verify-blob \
  --certificate-identity-regexp 'https://github.com/retran/meowg1k' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --signature meowg1k_<version>_linux_amd64.tar.gz.sig \
  meowg1k_<version>_linux_amd64.tar.gz
```

### View SBOM

Each release includes `meowg1k_<version>_sbom.spdx.json`:

```bash
cat meowg1k_<version>_sbom.spdx.json | jq
```

See [latest release](https://github.com/retran/meowg1k/releases/latest) for all files.

---

## Contributing

- Report bugs
- Suggest new features
- Improve documentation
- Add provider integrations

---

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.

---

## Acknowledgments

`meowg1k` builds on the excellent work of the open-source and AI community:

- [spf13/cobra](https://github.com/spf13/cobra) — CLI framework
- [spf13/viper](https://github.com/spf13/viper) — configuration management
- [go-task/task](https://github.com/go-task/task) — task runner
- [llama.cpp](https://github.com/ggerganov/llama.cpp) — local LLM inference
- The **Go** team and ecosystem

And of course, thanks to the broader developer community for libraries, tools, and inspiration that make this project better every day.

---

<div align="center">

**Happy coding with `project meow`! 🐱**

Made with ❤️ by Andrew Vasilyev and feline assistants Sonya Blade, Mila, and Marcus Fenix.

[Report Bug](https://github.com/retran/meowg1k/issues) · [Request Feature](https://github.com/retran/meowg1k/issues) · [Contribute](https://github.com/retran/meowg1k/pulls)

</div>
