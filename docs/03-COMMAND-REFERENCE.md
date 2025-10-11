# Command Reference

This document provides a detailed reference for all available `meowg1k` commands and their options.

---

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
   echo "Create a hello world function" | meow generate
   ```

---

## Global Flags

These flags can be used with any command.

- `--config <path>`: Specify a path to a configuration file. This overrides any project-level or user-level configs.
- `--workspace <path>`: Specify the workspace root directory. This overrides automatic workspace detection.
- `--silent`: Enables silent mode, which suppresses progress indicators and other non-essential output. Ideal for scripting.
- `--no-cache`: Disables LLM response caching for the current command.
- `--update-cache`: Forces a cache refresh by making a fresh request to the LLM and updating the cache entry.
- `--help`: Shows help information for any command.

---

## `meow generate` (aliases: `gen`, `g`)

Generates content based on a prompt and/or context provided via standard input (stdin).

### Usage

```bash
cat [file] | meow generate [flags]
echo "[text]" | meow g [flags]
````

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

---

## `meow commit` (alias: `c`)

Generates a commit message based on staged changes or the difference between branches.

### Usage

```bash
meow commit [flags]
```

### Flags

- `-i, --intent <text>`: Provides a high-level developer intent for the commit, which helps the AI generate a more accurate message. Can also be provided via stdin.
- `-t, --target-branch <name>`: Switches the command to "squash commit mode". Instead of analyzing staged changes, it analyzes the diff between the current branch and the `<name>` branch.

### Modes & Examples

#### 1. Default Mode (Staged Changes)

This is the standard mode. It analyzes files you have staged with `git add`.

```bash
# Stage your files
git add .

# Generate a commit message based on the staged changes
meow commit

# Provide intent to guide the AI
meow commit -i "Refactor user authentication to use a new JWT library"
```

#### 2. Squash Commit Mode

Use this mode when you want to generate a single commit message that summarizes all changes on your feature branch before merging.

```bash
# Generate a message for all changes on the current branch compared to 'main'
meow commit --target-branch main

# Provide intent for the squash commit
meow commit -t dev -i "Implement the entire user profile feature"
```

---

## `meow pullrequest` (aliases: `pr`)

Generates a Pull Request title and description based on the difference between your current branch and a base branch.

### Usage

```bash
meow pullrequest [flags]
```

### Flags

- `-b, --base <branch>`: **(Required)** The base branch you intend to merge into (e.g., `main`, `dev`).
- `-i, --intent <text>`: Provides a high-level developer intent for the PR, which helps the AI generate a more accurate title and description. Can also be provided via stdin.

### Examples

```bash
# Generate a PR description for changes to be merged into 'main'
meow pullrequest --base main

# Provide intent to get a more focused PR description
meow pullrequest -b dev -i "Add a complete Stripe payment integration"

# Pipe the intent via stdin
echo "This PR adds a new caching layer using Redis" | meow pullrequest -b main
```

---

## `meow version`

Displays the application's version, build date, and commit hash.

### Usage

```bash
meow version
```

## Next Steps

Now that you're familiar with the commands, check out some advanced use cases in the [Examples & Recipes guide](./04-EXAMPLES.md).
