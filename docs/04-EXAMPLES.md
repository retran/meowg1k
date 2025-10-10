# Examples & Recipes

This guide provides practical examples and recipes to showcase how `meowg1k` can be used to solve common development tasks. These examples build on the concepts from the [Configuration Guide](./02-CONFIGURATION.md) and the [Command Reference](./03-COMMAND-REFERENCE.md).

---

## 1. Basic Code Refactoring

This is the simplest use case: providing a piece of code via stdin and asking for a modification.

**Goal:** Convert a JavaScript function to use modern `async/await` syntax.

**Command:**

```bash
echo "function getUser(id) { return fetch('/api/users/' + id).then(res => res.json()); }" \
| meow g -u "Refactor this to use async/await syntax and add basic error handling"
```

**Explanation:**

- The `echo` command pipes the JavaScript code into `meowg1k`'s standard input.
- The `-u` flag provides the user prompt, telling the AI what to do with the provided code.
- `meowg1k` combines the context (the code) and the prompt into a single request to the default LLM provider.

---

## 2. Using Predefined Tasks for Recurring Reviews

For tasks you perform often, like code reviews or security checks, defining a task in your config saves time and ensures consistency.

**Goal:** Run a security review on a Go source file using a predefined task.

**Configuration (`.meowg1k/config.yaml`):**

```yaml
models:
  claude-sonnet:
    provider: "anthropic"
    model: "claude-3-5-sonnet-20240620"

profiles:
  claude-secure:
    model: "claude-sonnet"

generate:
  default:
    profile: "claude-secure"
  tasks:
    security-review:
      systemPrompt: "You are a security expert specializing in Go. Analyze the following code for common vulnerabilities."
      userPrompt: "Identify potential security issues such as SQL injection, XSS, insecure error handling, or improper use of cryptographic functions. Provide code examples for fixes."
```

**Command:**

```bash
cat ./internal/handlers/auth.go | meow g -t security-review
```

**Explanation:**

- The `-t security-review` flag tells `meowg1k` to use the complex `systemPrompt` and `userPrompt` defined under `generate.tasks.security-review` in the config file.
- This allows you to encapsulate detailed, expert-level prompts into simple, reusable commands.

---

## 3. Fully Automated Commit Messages

This example shows the power of the `summarize` and `commit` workflow.

**Goal:** Generate a Conventional Commit message based on staged changes, using a fast model for file analysis and a smart model for the final message to optimize cost.

**Configuration (`.meowg1k/config.yaml`):**

```yaml
models:
  gemini-flash:
    provider: "gemini"
    model: "gemini-1.5-flash-latest"
  claude-sonnet:
    provider: "anthropic"
    model: "claude-3-5-sonnet-20240620"

profiles:
  fast:
    model: "gemini-flash"
  smart:
    model: "claude-sonnet"

summarize:
  # Use the fast, cheap model to summarize each file change
  default:
    profile: "fast"
    systemPrompt: "Provide a one-sentence summary of this code change."

commit:
  # Use the smart, expensive model to write the final commit message
  profile: "smart"
  systemPrompt: |
    You are an expert software engineer. Based on the file summaries, write a commit message in the Conventional Commits format.
    Deduce the type and scope, write a concise subject, and add a body explaining the 'why' if the change is non-trivial.
```

**Command:**

```bash
git add .
meow commit -i "Fix the user login bug and refactor token handling"
```

**Explanation:**

- `meowg1k` first uses the `gemini-fast` profile to analyze each staged file change individually (the "Map" step).
- Then, it collects all these individual summaries and sends them to the `claude-smart` profile, guided by the powerful `commit` system prompt, to generate the final, high-quality commit message (the "Reduce" step).

---

## 4. Advanced PR Descriptions with File-Specific Rules

This recipe demonstrates how to use advanced `summarize` rules to generate a highly accurate and structured PR description.

**Goal:** Generate a PR description, but skip documentation changes and use a more powerful model for critical business logic.

**Configuration (`.meowg1k/config.yaml`):**

```yaml
# ... (models and profiles defined as above) ...
filter:
  ignore:
    - "dist/**"

summarize:
  default:
    profile: "fast"
  rules:
    # Rule 1: Skip all documentation changes from the analysis
    - match: "**/*.md"
      skip: true
    # Rule 2: Use the best model for critical service logic
    - match: "internal/adapters/**/*.go"
      profile: "smart"
      systemPrompt: "Deeply analyze this business logic change. Focus on correctness, performance, and potential side effects."

pullRequest:
  profile: "smart"
  systemPrompt: |
    You are an expert engineer. Write a PR description with a title and a body using this Markdown template:
    ## 🎯 Goal
    (Describe the problem this PR solves)
    ## 📝 Summary of Changes
    (Create a bulleted list of key changes)
    ## 🧪 How to Test
    (Provide step-by-step testing instructions)
```

**Command:**

```bash
meow pullrequest --base main
```

**Explanation:**

- When you run `meow pullrequest`, it first ignores any files in `dist/`.
- Then, it skips summarizing any changes to `.md` files.
- For changes in `internal/services/`, it uses the powerful `claude-smart` profile and a specialized prompt.
- All other files are summarized using the default `gemini-fast` profile.
- Finally, the collected summaries are used by the `pullRequest` configuration to generate a well-structured Markdown description.

---

## 5. Using Local Models for Privacy

When working with proprietary or highly sensitive code, you can use a local LLM to ensure no data ever leaves your machine.

**Goal:** Analyze a sensitive file using a local `llama.cpp` server.

**Configuration (`.meowg1k/config.yaml`):**

```yaml
models:
  local:
    provider: "llama"
    baseURL: "http://localhost:8080"
    model: "llama3-8b-instruct" # The model name your server is using

profiles:
  local-secure:
    model: "local"

generate:
  tasks:
    local-analysis:
      profile: "local-secure"
      userPrompt: "Analyze this code for logical errors and suggest improvements."
```

*(Ensure your `llama.cpp` server is running at `http://localhost:8080`)*

**Command:**

```bash
cat ./internal/billing/core.go | meow g -t local-analysis
```

**Explanation:**

- The `local-secure` profile directs all API traffic to your local server instead of a cloud provider.
- This allows you to leverage the power of LLMs in air-gapped or high-security environments.

---

## Next Steps

Explore how to integrate `meowg1k` into your daily workflow with the [Integrations Guide](./05-INTEGRATIONS.md), covering Git hooks and CI/CD pipelines.
