# Examples & Recipes

This guide provides practical examples and recipes to showcase how `meowg1k` can be used to solve common development tasks. These examples build on the concepts from the [Configuration Guide](./02-CONFIGURATION.md) and the [Command Reference](./03-COMMAND-REFERENCE.md).

## 1. Basic Code Refactoring

This is the simplest use case: providing a piece of code via stdin and asking for a modification.

**Goal:** Refactor a JavaScript function for clearer control flow and basic error handling.

**Command:**

```bash
echo "function getUser(id) { return fetch('/api/users/' + id).then(res => res.json()); }" \
| meow g -u "Refactor this to improve control flow and add basic error handling"
```

**Explanation:**

- The `echo` command pipes the JavaScript code into `meow`'s standard input.
- The `-u` flag provides the user prompt, telling the AI what to do with the provided code.
- `meowg1k` combines the context (the code) and the prompt into a single request to the default LLM provider.

## 2. Using Predefined Tasks for Recurring Reviews

For tasks you perform often, like code reviews or security checks, defining a task in your config saves time and ensures consistency.

**Goal:** Run a security review on a Go source file using a predefined task.

**Configuration (`.meowg1k.yaml`):**

```yaml
models:
  claude-sonnet:
    provider: "anthropic"
    model: "claude-sonnet-4-5-20250929"

profiles:
  claude-secure:
    model: "claude-sonnet"

write:
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

- The `-t security-review` flag tells `meow` to use the complex `systemPrompt` and `userPrompt` defined under `write.tasks.security-review` in the config file.
- This allows you to encapsulate detailed, expert-level prompts into simple, reusable commands.

## 3. Fully Automated Commit Messages

This example shows the power of the `summarize` and `commit` workflow.

**Goal:** Generate a Conventional Commit message based on staged changes, using a fast model for file analysis and a more capable model for the final message to optimize cost.

**Configuration (`.meowg1k.yaml`):**

```yaml
models:
  gemini-flash:
    provider: "gemini"
    model: "gemini-2.5-flash"
  claude-sonnet:
    provider: "anthropic"
    model: "claude-sonnet-4-5-20250929"

profiles:
  gemini-flash:
    model: "gemini-flash"
  claude-sonnet:
    model: "claude-sonnet"

summarize:
  # Use the fast, cheap model to summarize each file change
  default:
    profile: "gemini-flash"
    systemPrompt: "Provide a one-sentence summary of this code change."

commit:
  # Use the more capable model to write the final commit message
  profile: "claude-sonnet"
  systemPrompt: |
    You are an expert software engineer. Based on the file summaries, write a commit message in the Conventional Commits format.
    Deduce the type and scope, write a concise subject, and add a body explaining the 'why' if the change is non-trivial.
```

**Command:**

```bash
git add .
meow draft commit -i "Fix the user login bug and refactor token handling"
```

**Explanation:**

- `meowg1k` first uses the `gemini-flash` profile to analyze each staged file change individually (the "Map" step).
- Then, it collects all these individual summaries and sends them to the `claude-sonnet` profile, guided by the powerful `commit` system prompt, to generate the final, high-quality commit message (the "Reduce" step).

## 4. Fast Commit Messages for Small Changes

For quick, small commits (single file changes or minor tweaks), the full Map-Reduce workflow can be overkill. The `flat` strategy sends the entire diff directly to the model, making it much faster.

**Goal:** Generate commit messages quickly for small, incremental changes during development.

**Configuration (`.meowg1k.yaml`):**

```yaml
models:
  gemini-flash:
    provider: "gemini"
    model: "gemini-2.5-flash"

profiles:
  gemini-flash:
    model: "gemini-flash"

commit:
  profile: "gemini-flash"
  strategy: "flat" # Skip the summarize step, send diff directly
  systemPrompt: |
    You are an expert software engineer. Write a concise commit message in Conventional Commits format based on this git diff.
    Be brief and focus on what changed and why.
```

**Command:**

```bash
# Make a small change
echo "// TODO: Add validation" >> service.go
git add service.go

# Generate a quick commit message
meow draft commit
```

**Explanation:**

- With `strategy: "flat"`, `meowg1k` bypasses the file-by-file summarization and sends the complete diff directly to the LLM.
- This is ideal for small commits where the context fits easily within the model's token limit.
- If the diff is too large (exceeds `maxInputTokens`), the command will fail with a helpful error suggesting you switch back to `strategy: "summarize"`.

**When to use `flat` vs `summarize`:**

- Use `flat` for:
  - Single file changes
  - Minor bug fixes or typos
  - Quick iterative development
- Use `summarize` (default) for:
  - Multi-file refactorings
  - Large feature implementations
  - Complex changes that benefit from per-file analysis

## 5. Advanced PR Descriptions with File-Specific Rules

This recipe demonstrates how to use advanced `summarize` rules to generate a highly accurate and structured PR description.

**Goal:** Generate a PR description, but skip documentation changes and use a more powerful model for critical business logic.

**Configuration (`.meowg1k.yaml`):**

```yaml
# ... (models and profiles defined as above) ...
filter:
  ignore:
    - "dist/**"

summarize:
  default:
    profile: "gemini-flash"
  rules:
    # Rule 1: Skip all documentation changes from the analysis
    - match: "**/*.md"
      skip: true
    # Rule 2: Use the best model for critical service logic
    - match: "internal/adapters/**/*.go"
      profile: "claude-sonnet"
      systemPrompt: "Deeply analyze this business logic change. Focus on correctness, performance, and potential side effects."

pr:
  profile: "claude-sonnet"
  systemPrompt: |
    You are an expert engineer. Write a PR description with a title and a body using this Markdown template:
    ## Goal
    (Describe the problem this PR solves)
    ## Summary of Changes
    (Create a bulleted list of key changes)
    ## How to Test
    (Provide step-by-step testing instructions)
```

> **Note:** For complete documentation of the `filter`, `summarize`, and `pr` configuration sections, see the [Configuration Guide](./02-CONFIGURATION.md).

**Command:**

```bash
meow draft pr --base main
```

**Explanation:**

- When you run `meow draft pr`, it first ignores any files in `dist/`.
- Then, it skips summarizing any changes to `.md` files.
- For changes in `internal/services/`, it uses the powerful `claude-sonnet` profile and a specialized prompt.
- All other files are summarized using the default `gemini-flash` profile.
- Finally, the collected summaries are used by the `pr` configuration to generate a well-structured Markdown description.

## 5. Using Local Models for Privacy

When working with proprietary or highly sensitive code, you can use a local LLM to ensure no data ever leaves your machine.

**Goal:** Analyze a sensitive file using a local `llama.cpp` server.

**Configuration (`.meowg1k.yaml`):**

```yaml
models:
  local:
    provider: "llama"
    baseURL: "http://localhost:8080"
    model: "llama3-8b-instruct" # The model name your server is using

profiles:
  local-secure:
    model: "local"

write:
  tasks:
    local-analysis:
      profile: "local-secure"
      userPrompt: "Analyze this code for logical errors and suggest improvements."
```

_(Ensure your `llama.cpp` server is running at `http://localhost:8080`)_

**Command:**

```bash
cat ./internal/billing/core.go | meow g -t local-analysis
```

**Explanation:**

- The `local-secure` profile directs all API traffic to your local server instead of a cloud provider.
- This allows you to leverage the power of LLMs in air-gapped or high-security environments.

## 7. RAG-Based Code Understanding

Use semantic search and RAG to understand your codebase without reading every file.

**Goal:** Quickly understand how a feature works in an unfamiliar codebase.

**Configuration (`.meowg1k.yaml`):**

```yaml
models:
  gemini-embeddings:
    provider: "gemini"
    model: "text-embedding-004"

  claude-sonnet:
    provider: "anthropic"
    model: "claude-sonnet-4-5-20250929"

profiles:
  gemini-embeddings:
    model: "gemini-embeddings"
  claude-sonnet:
    model: "claude-sonnet"

index:
  profile: "gemini-embeddings"
  chunker:
    maxRunes: 1024
    overlapRunes: 128
  batchSize: 64

answer:
  profile: "claude-sonnet"
  topK: 5
  minScore: 0.7
```

> **Note:** This is a minimal configuration for RAG. For detailed explanations of all available options including rate limiting, caching, and advanced settings, see the [Configuration Guide](./02-CONFIGURATION.md#index) and [RAG and Code Search guide](./05-RAG-AND-CODE-SEARCH.md).

**Workflow:**

```bash
# Step 1: Index the codebase (first time only)
meow index

# Step 2: Search for relevant code
meow search "authentication middleware" --top-k 10

# Step 3: Ask high-level questions
meow ask "How does the authentication system work?"

# Step 4: Ask specific questions
meow ask "Where is the JWT token validated?"

# Step 5: Ask implementation questions
meow ask "How do I add a new protected endpoint?"
```

**Explanation:**

- First, `meow index` processes all files, computes embeddings, and builds vector indices
- `meow search` performs semantic search to find relevant code chunks
- `meow ask` retrieves relevant context and uses an LLM to provide answers
- This workflow is much faster than manually searching through files

**Tips:**

- Re-run `meow index` after pulling changes or switching branches
- Use `--show-context` with `meow ask` to see what code the AI is using
- Adjust `topK` and `minScore` based on whether you want broad or focused answers

## 8. Finding Code Without Knowing Exact Terms

Semantic search finds code by meaning, not just keywords.

**Goal:** Find all error handling code, even if it uses different patterns.

**Commands:**

```bash
# Traditional text search might miss variations
git grep "error" | grep -v "// error"  # Misses: err, failure, exception

# Semantic search finds all error-related code
meow search "error handling and validation" --top-k 20 --min-score 0.6

# Find authentication code without knowing implementation details
meow search "user authentication and authorization" --top-k 15

# Find database code regardless of ORM used
meow search "database queries and transactions"

# Find code related to a concept, not exact terms
meow search "rate limiting and throttling"
```

**Example output:**

```
Found 8 results:

=== Result 1 (Score: 0.8234) ===
File: internal/handlers/user.go (Lines 45-58)

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if err := h.validator.Validate(req); err != nil {
        h.respondError(w, http.StatusBadRequest, err.Error())
        return
    }
    // ... rest of handler
}
```

**Explanation:**

- Semantic search understands that "error handling" includes validation, error responses, and error checking
- It finds relevant code even when exact keywords don't match
- This is especially useful in large or unfamiliar codebases

## 9. Onboarding to a New Codebase

Use RAG to create an interactive learning experience.

**Goal:** Quickly get up to speed on a new project.

**Workflow:**

```bash
# 1. Index the project
meow index

# 2. Start with architecture questions
meow ask "What is the overall architecture of this project?"
meow ask "What are the main components and how do they interact?"

# 3. Understand the tech stack
meow ask "What frameworks and libraries are used?"
meow ask "How is the database layer structured?"

# 4. Learn about specific features
meow ask "How does the user registration process work?"
meow ask "How are API endpoints authenticated?"

# 5. Understand conventions
meow ask "What is the error handling pattern?"
meow ask "How are tests organized?"

# 6. Find entry points
meow ask "Where is the main application entry point?"
meow ask "How do I run this project locally?"
```

**Tips for effective onboarding:**

- Start broad, then drill down into specifics
- Use `--show-context` to discover relevant files
- Take notes on the files mentioned in answers
- Combine RAG with traditional code reading for best results

## 10. Code Review with Context

Use RAG to perform more thorough code reviews.

**Goal:** Review a PR with full context of related code.

**Configuration (`.meowg1k.yaml`):**

```yaml
write:
  default:
    profile: "gemini-pro"
  tasks:
    review-with-context:
      systemPrompt: |
        You are an expert code reviewer. Use the provided context to understand how this code
        fits into the larger system. Consider:
        - Does this follow existing patterns in the codebase?
        - Are there related functions that should be updated?
        - Does this break any existing functionality?
        - Are there edge cases based on how similar code works?
      userPrompt: "Review this code change with full context awareness."
```

**Workflow:**

```bash
# 1. Ensure index is up to date
meow index

# 2. Get context about related code
meow search "authentication middleware" --top-k 10 > context.txt

# 3. Review the changes with context
git diff HEAD~1 | cat context.txt - | meow g -t review-with-context

# 4. Or ask specific questions about the changes
git diff HEAD~1 > changes.txt
meow ask "What impact will these changes have on existing authentication flows?" < changes.txt
```

**Explanation:**

- RAG provides context about existing patterns and related code
- This helps catch issues that would be missed by reviewing changes in isolation
- The AI can identify potential breaking changes by understanding the broader system

## Next Steps

Explore how to integrate `meowg1k` into your daily workflow with the [Integrations Guide](./07-INTEGRATIONS.md), covering Git hooks and CI/CD pipelines.

For detailed guides on specific features:

- [Code Generation and Automated Workflows](./04-GENERATION-AND-WORKFLOWS.md) — Deep dive into write, draft commit, and draft pr
- [RAG and Code Search](./05-RAG-AND-CODE-SEARCH.md) — Semantic search and question answering
