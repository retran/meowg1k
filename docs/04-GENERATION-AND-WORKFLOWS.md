# Code Generation and Automated Workflows

This guide covers `meowg1k`'s core generation capabilities for automating development workflows. With meowg1k, you can generate code, commit messages, and pull request descriptions using AI-powered analysis and intelligent context preparation.

[Back to Documentation Index](./README.md)

## Overview

`meowg1k` provides four main workflows:

1. **`meow write`** — Generate or transform code based on prompts and context
2. **`meow draft commit`** — Automatically generate commit messages from code changes
3. **`meow draft pr`** — Generate comprehensive PR descriptions from branch diffs
4. **`meow do`** — Run a multi-step agent workflow with tool use

Unlike interactive chat tools, meowg1k is designed for **automation and scripting**. Each command follows a predictable input→process→output model, making it suitable for Git hooks, CI/CD pipelines, and custom development workflows.

## Architecture

### The Generation Pipeline

All generation commands follow a similar architecture:

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│              │     │              │     │              │
│   Context    │────▶│   Prompt     │────▶│   Generate   │
│  Gathering   │     │  Engineering │     │   with LLM   │
│              │     │              │     │              │
└──────────────┘     └──────────────┘     └──────────────┘
    Files,              System +             Output to
    diffs, etc.         User prompts          stdout
```

### Key Principles

1. **Context is King** — The tool automatically prepares and enriches context (code, diffs, metadata) before sending to the LLM
2. **Configuration as Code** — All prompts and behavior are defined in version-controlled YAML files
3. **Composable** — Combine with shell tools, Git hooks, and CI/CD for powerful workflows
4. **Predictable** — Same input + config = same API request (deterministic up to LLM sampling)

## Commands and Configuration

The generation system uses four main commands:

- **[`meow write`](./03-COMMAND-REFERENCE.md#meow-write)** — General-purpose code and text generation
- **[`meow draft commit`](./03-COMMAND-REFERENCE.md#meow-draft-commit-alias-commit-msg)** — Generate commit messages from staged changes
- **[`meow draft pr`](./03-COMMAND-REFERENCE.md#meow-draft-pr)** — Generate PR descriptions from branch diffs
- **[`meow do`](./03-COMMAND-REFERENCE.md#meow-do)** — Multi-step agent with research → plan → execute → verify

For complete command details including all flags and options, see the [Command Reference](./03-COMMAND-REFERENCE.md).

For configuration details of the `write`, `commit`, `pr`, `summarize`, and `agent` sections, see the [Configuration Guide](./02-CONFIGURATION.md#write-commit-and-pr).

## Quick Start

### Basic Code Generation

The simplest use case: pipe code into stdin and provide a prompt.

```bash
# Refactor a function
cat service.py | meow g -u "Add error handling and logging"

# Convert between languages
echo "function add(a, b) { return a + b; }" | meow g -u "Convert to TypeScript with types"

# Generate tests
cat calculator.go | meow g -u "Write comprehensive unit tests"
```

### Predefined Tasks

For recurring tasks, define them in your config to ensure consistency:

```yaml
# .meowg1k.yaml
write:
  default:
    profile: "gemini-pro"
  tasks:
    review:
      userPrompt: "Perform a code review. Check for bugs, performance issues, and best practices."

    document:
      userPrompt: "Generate comprehensive documentation comments for all public functions and types."

    refactor:
      userPrompt: "Refactor this code to improve readability and maintainability. Explain the changes."
```

Usage:

```bash
cat handler.go | meow g -t review
cat api.py | meow g -t document
```

> **Note:** For complete documentation of the `write` configuration section and all available parameters, see the [Configuration Guide](./02-CONFIGURATION.md#write-commit-and-pr).

### Automated Commit Messages

Generate commit messages automatically from staged changes:

```yaml
# .meowg1k.yaml
models:
  gemini-flash:
    provider: "gemini"
    model: "gemini-2.0-flash-exp"

profiles:
  gemini-flash:
    model: "gemini-flash"

commit:
  profile: "gemini-flash"
  strategy: "flat" # Fast for small commits
  systemPrompt: |
    Write a concise commit message in Conventional Commits format.
    Format: <type>(<scope>): <subject>

    Types: feat, fix, docs, refactor, test, chore, ci
```

Usage:

```bash
git add .
meow draft commit
```

## The Map-Reduce Workflow

For large commits and PRs with many file changes, meowg1k uses a **Map-Reduce** approach to handle content that exceeds token limits.

### How It Works

```
┌─────────────────────────────────────────────────────────┐
│                    MAP PHASE                            │
│  (Analyze each file individually)                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  file1.go  ──▶ [Summarize] ──▶ "Added user auth"        │
│  file2.go  ──▶ [Summarize] ──▶ "Fixed JWT validation"   │
│  file3.go  ──▶ [Skip]      ──▶ (test file, skipped)     │
│  file4.md  ──▶ [Skip]      ──▶ (docs, skipped)          │
│                                                         │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                   REDUCE PHASE                          │
│  (Combine summaries into final message)                 │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  All Summaries ──▶ [Generate] ──▶ Final commit message  │
│                                   or PR description     │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Configuration

The `summarize` section controls the Map phase:

```yaml
summarize:
  default:
    profile: "gemini-flash"
    systemPrompt: "Summarize this code change in one sentence."

  rules:
    # Skip documentation files
    - match: "**/*.md"
      skip: true

    # Use powerful model for critical code
    - match: "internal/core/**/*.go"
      profile: "gemini-pro"
      systemPrompt: "Analyze this business logic change. Focus on correctness and side effects."

    # Skip test files
    - match: "**/*_test.go"
      skip: true
```

**Rules are evaluated top-down; the first match wins.**

> **Note:** For complete documentation of the `summarize` configuration section, see the [Configuration Guide](./02-CONFIGURATION.md#summarize).

## Strategy: Flat vs Summarize

Both `draft commit` and `draft pr` commands support two strategies:

### `flat` Strategy (Fast)

Sends the entire diff directly to the LLM without summarization.

**Pros:**

- Faster (single API call)
- Cheaper (fewer tokens)
- Simpler (no intermediate steps)

**Cons:**

- Fails if diff exceeds token limit
- Limited to small changes

**When to use:**

- Single file changes
- Minor bug fixes
- Quick iterative development
- Small refactorings

**Configuration:**

```yaml
commit:
  profile: "gemini-flash"
  strategy: "flat"
  systemPrompt: "Write a concise commit message based on this diff."
```

### `summarize` Strategy (Robust, Default)

Uses Map-Reduce to analyze files individually, then combines summaries.

**Pros:**

- Handles large changesets
- Per-file analysis with custom rules
- More detailed understanding
- Never fails due to size

**Cons:**

- Slower (multiple API calls)
- More expensive
- Requires configuration

**When to use:**

- Multi-file changes
- Large refactorings
- Feature implementations
- Complex PRs

**Configuration:**

```yaml
commit:
  profile: "gemini-pro"
  strategy: "summarize" # Default
  systemPrompt: |
    Based on the file summaries, write a high-quality commit message.
    Follow Conventional Commits format.

summarize:
  default:
    profile: "gemini-flash"
  rules:
    - match: "**/*.md"
      skip: true
```

## Advanced Patterns

### Multi-Model Optimization

Use fast models for summarization, powerful models for final generation:

```yaml
models:
  gemini-flash:
    provider: "gemini"
    model: "gemini-2.0-flash-exp"

  claude-sonnet:
    provider: "anthropic"
    model: "claude-sonnet-4-5-20250929"

profiles:
  gemini-flash:
    model: "gemini-flash"
  claude-sonnet:
    model: "claude-sonnet"

summarize:
  default:
    profile: "gemini-flash" # Cheap model for 50 file summaries

commit:
  profile: "claude-sonnet" # Expensive model for 1 final message
  strategy: "summarize"
```

### File-Specific Prompts

Different types of files need different analysis:

```yaml
summarize:
  default:
    profile: "gemini-flash"
    systemPrompt: "Summarize this code change."

  rules:
    # Database migrations need careful review
    - match: "db/migrations/**/*.sql"
      profile: "gemini-pro"
      systemPrompt: |
        Analyze this SQL migration.
        - What schema changes are made?
        - Is this backward compatible?
        - Are there any data loss risks?

    # Configuration changes need context
    - match: "**/*.yaml"
      profile: "gemini-flash"
      systemPrompt: "Describe what configuration changed and why it matters."

    # Skip generated files entirely
    - match: "**/*.pb.go"
      skip: true
```

### Developer Intent

Provide high-level context to guide generation:

```bash
# Without intent
git add .
meow draft commit
# → "feat: update user service"

# With intent
git add .
meow draft commit -i "Fix authentication bug where JWT tokens weren't validated correctly"
# → "fix(auth): validate JWT tokens properly
#
#    Previously, expired tokens were accepted due to missing
#    validation in the middleware. This fixes CVE-2024-12345."
```

The intent is passed to the LLM alongside the code changes, allowing it to understand **why** the change was made, not just **what** changed.

## Best Practices

### 1. Start Simple, Then Optimize

Begin with minimal config:

```yaml
commit:
  profile: "default"
```

Then add complexity as needed:

```yaml
commit:
  profile: "gemini-pro"
  strategy: "summarize"

summarize:
  default:
    profile: "gemini-flash"
  rules:
    - match: "**/*.md"
      skip: true
```

### 2. Use Filters Aggressively

Exclude files that don't need analysis:

```yaml
filter:
  ignore:
    - "vendor/**"
    - "node_modules/**"
    - "**/*.pb.go" # Generated files
    - "**/*_generated.go"
    - "**/*.lock" # Lock files
```

### 3. Test Your Prompts

Iterate on system prompts to get better results:

```bash
# Test different prompts
cat file.go | meow g -s "You are a senior Go engineer" -u "Review this code"
cat file.go | meow g -s "You are a security expert" -u "Find vulnerabilities"
```

### 4. Version Control Your Config

Commit `.meowg1k.yaml` to share prompts and workflows with your team:

```bash
git add .meowg1k.yaml
git commit -m "docs: add meowg1k configuration for automated commit messages"
```

### 5. Combine with Git Hooks

Automate generation at commit time:

```bash
# .git/hooks/prepare-commit-msg
#!/bin/bash
COMMIT_MSG_FILE=$1
COMMIT_SOURCE=$2

if [ -z "$COMMIT_SOURCE" ]; then
    meow draft commit --silent > "$COMMIT_MSG_FILE"
fi
```

See the [Integrations Guide](./07-INTEGRATIONS.md) for more examples.

## Troubleshooting

For common issues and solutions related to generation workflows, see the [Troubleshooting Guide](./10-TROUBLESHOOTING.md), specifically:

- [Diff too large error](./10-TROUBLESHOOTING.md#diff-too-large-error-with-flat-strategy)
- [Generic commit messages](./10-TROUBLESHOOTING.md#commit-messages-are-too-generic)
- [Summaries being skipped](./10-TROUBLESHOOTING.md#summaries-being-skipped-unexpectedly)
- [Slow generation](./10-TROUBLESHOOTING.md#generation-is-too-slow)
- [High costs](./10-TROUBLESHOOTING.md#costs-are-too-high)

## Pro Tips

### 1. Predefined Task Library

Build a library of reusable tasks:

```yaml
write:
  tasks:
    # Code quality
    review:
      userPrompt: "Code review focusing on bugs, performance, and best practices"
    refactor:
      userPrompt: "Refactor for better readability and maintainability"
    optimize:
      userPrompt: "Optimize for performance without changing behavior"

    # Documentation
    document:
      userPrompt: "Generate comprehensive documentation comments"
    readme:
      userPrompt: "Generate a README section explaining this code"

    # Testing
    test:
      userPrompt: "Write comprehensive unit tests covering edge cases"
    fixture:
      userPrompt: "Generate realistic test fixtures"

    # Security
    security:
      systemPrompt: "You are a security expert specializing in vulnerability analysis"
      userPrompt: "Perform security audit. Check for common vulnerabilities."
```

### 2. Language-Specific Rules

Tailor analysis to language characteristics:

```yaml
summarize:
  rules:
    # Go files
    - match: "**/*.go"
      systemPrompt: "Summarize Go code changes. Note any interface changes or exported API modifications."

    # Python files
    - match: "**/*.py"
      systemPrompt: "Summarize Python changes. Note any type hint or decorator changes."

    # Frontend files
    - match: "**/*.tsx"
      systemPrompt: "Summarize React component changes. Note prop interface changes."
```

### 3. Squash Commit Messages

Generate a single message for an entire feature branch:

```bash
# All changes on feature branch vs main
meow draft commit --diff branch --base main -i "Complete user profile feature implementation"
```

### 4. PR Templates

Enforce consistent PR structure:

```yaml
pr:
  profile: "gemini-pro"
  systemPrompt: |
    Generate a PR description using this exact template:

    ## Goal
    [What problem does this solve?]

    ## Changes
    - Change 1
    - Change 2

    ## Testing
    1. Step 1
    2. Step 2

    ## Breaking Changes
    [List any breaking changes, or write "None"]
```

## Related Documentation

- [Configuration Guide](./02-CONFIGURATION.md) — Detailed configuration reference for all generation settings
- [Command Reference](./03-COMMAND-REFERENCE.md) — Complete reference for write, draft commit, and draft pr commands
- [Examples & Recipes](./06-EXAMPLES.md) — Practical examples and complete workflows
- [Integrations Guide](./07-INTEGRATIONS.md) — Git hooks, CI/CD, and editor integrations

---

**Next:** [RAG and Code Search](./05-RAG-AND-CODE-SEARCH.md)
