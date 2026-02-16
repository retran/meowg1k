# meowg1k Complete API Reference

## Quick Navigation

- [Starlark API](#starlark-api-reference) - Scripting, tools, context
- [Standard Modules](#standard-modules) - fs, git, llm, shell, index, ui, etc.
- [Type Reference](#type-reference) - Complete type signatures  
- [Cookbook](#cookbook-real-world-patterns) - Real-world examples
- [UI Module](#complete-ui-api-documentation) - Flux Terminal widgets
- [Complete Example](#complete-example) - Full code review workflow

---

# Starlark API Reference

The meowg1k Starlark runtime provides a rich set of modules for configuration, automation, and AI workflows. This document details the available APIs.

## Table of Contents

- [Starlark API Reference](#starlark-api-reference)
  - [Table of Contents](#table-of-contents)
  - [Global Environment](#global-environment)
    - [meow Module](#meow-module)
      - [Configuration (Providers, Models, Presets)](#configuration-providers-models-presets)
    - [env Module](#env-module)
  - [Unified Tool System](#unified-tool-system)
    - [Defining Tools](#defining-tools)
    - [Parameters](#parameters)
    - [Registering Commands](#registering-commands)
    - [Validation](#validation)
  - [Handler Context](#handler-context)
    - [Attributes](#attributes)
    - [Parameter Access](#parameter-access)
    - [Methods](#methods)
  - [Best Practices](#best-practices)
  - [Standard Modules](#standard-modules)
    - [fs (Filesystem)](#fs-filesystem)
    - [git (Git Operations)](#git-git-operations)
    - [llm (Language Models)](#llm-language-models)
    - [shell (Shell Execution)](#shell-shell-execution)
    - [index (RAG / Search)](#index-rag--search)
    - [ui (User Interface)](#ui-user-interface)
    - [json (JSON Handling)](#json-json-handling)
    - [path (Path Manipulation)](#path-path-manipulation)
    - [crypto (Cryptography)](#crypto-cryptography)
    - [time (Time \& Date)](#time-time--date)
    - [regexp (Regular Expressions)](#regexp-regular-expressions)
    - [stdin (Standard Input)](#stdin-standard-input)
    - [output (Output Writing)](#output-output-writing)
  - [Complete Example](#complete-example)
    - [Code Review Workflow](#code-review-workflow)
  - [Type Reference](#type-reference)
    - [Notation](#notation)
    - [Domain Types](#domain-types)
    - [Complete Function Signatures](#complete-function-signatures)
  - [Cookbook: Real-World Patterns](#cookbook-real-world-patterns)
    - [1. Working with Git](#1-working-with-git)
    - [2. LLM Workflows](#2-llm-workflows)
    - [3. UI Patterns](#3-ui-patterns)
    - [4. File Operations](#4-file-operations)
    - [5. RAG and Code Search](#5-rag-and-code-search)

---

## Global Environment

These modules are available globally in your `init.star` scripts.

### meow Module

The `meow` module is the primary entry point for configuring the application and registering capabilities.

#### Configuration (Providers, Models, Presets)

**`meow.provider(name, type, **kwargs)`**
Registers an LLM provider.
- `name` (string): Unique identifier.
- `type` (string): Driver type (e.g., "openai", "sematic", "gemini").
- `api_key` (string, optional): API Key (usually via `env.get`).
- `base_url` (string, optional): Custom API endpoint.
- `tokenizer` (string, optional): Tokenizer Model ID.
- `retry_count` (int, optional): Number of retries (default: 3).

**`meow.model(name, **kwargs)`**
Registers a model configuration.
- `name` (string): Unique identifier.
- `provider` (string): Name of the registered provider.
- `model` (string): The upstream model ID (e.g. "gpt-4").
- `max_input_tokens`, `max_output_tokens` (int): Token limits.
- `rate_limit_rpm`, `rate_limit_tpm`, `rate_limit_rpd` (int): Rate limits.

**`meow.preset(name, **kwargs)`**
Registers a generation preset (combines model + parameters).
- `name` (string): Unique identifier.
- `model` (string): Registered model name.
- `extends` (string): Parent preset to inherit from.
- `temperature`, `top_p`, `top_k` (number): Sampling parameters.
- `frequency_penalty`, `presence_penalty` (float): Penalties.
- `max_tokens` (int): Generation token limit.
- `**kwargs`: Any other parameters are passed to the provider.

**`meow.presets()`** -> `list[string]`
Returns a list of all registered preset names.

```python
# List available presets
available = meow.presets()
# Returns: ["fast", "smart", "creative", ...]
```

---

### env Module

Utilities for accessing environment variables.

**`env.get(key, default=None)`** -> `string`
Get an environment variable. Returns `default` or `None` if not set.

**`env.set(key, value)`** -> `None`
Set an environment variable for the current process.

**`env.list()`** -> `dict`
Returns a dictionary of all environment variables.

---

## Unified Tool System

The Unified Tool System allows you to define tools with inputs defined as `meow.param`. These tools can be automatically registered as CLI commands, with arguments and flags parsed and injected directly into the handler context.

### Defining Tools

**`meow.tool(name, handler, params=None, description=None)`**
Creates a reusable tool definition.
- `name` (string): Name of the tool.
- `handler` (function): The function to execute.
- `params` (dict): Dictionary mapping parameter names to `meow.param` objects.
- `description` (string): Help text.

### Parameters

**`meow.param(type, **kwargs)`**
Defines a typed input parameter.
- `type` (string): "string", "int", "float", "bool".
- `default` (any): Default value.
- `short` (string): Short flag (e.g., "f" for `-f`).
- `desc` (string): Description for help output.
- `required` (bool): Whether the parameter is mandatory.
- `from_stdin` (bool): If true, value can be piped from stdin.
- `choices` (list): Allowed values.
- `pattern` (string): Regex pattern for validation.
- `min`, `max` (number): Range constraints.
- `min_len`, `max_len` (int): Length constraints.
- `validator` (tool or function): Custom validation logic.

### Registering Commands

**`meow.command(tool, name=None)`**
Registers a `meow.tool` as a Top-Level CLI command.
- `tool` (Tool): The tool object created via `meow.tool`.
- `name` (string, optional): Override the command name.

**Example:**
```python
def my_handler(ctx):
    ctx.ui.info("Hello " + ctx.name)

hello_tool = meow.tool(
    name="hello",
    handler=my_handler,
    params={
        "name": meow.param("string", default="World", desc="Name to greet")
    }
)

meow.command(hello_tool)
```

### Validation

Parameters support both declarative constraints and custom validation logic.

**Static Constraints**
- `choices`: Limit to specific values.
- `pattern`: Regex validation.
- `min`/`max`: Numeric range.
- `min_len`/`max_len`: String length.

**Custom Validators**
You can pass a function or tool to `validator`. The validator receives a `ctx` where `ctx.value` is the input parameter. Return `True` (pass), `False` (fail), or an error string.

**Example:**
```python
def validate_even(ctx):
    if ctx.value % 2 != 0:
        return "Must be an even number"
    return True

tool_with_validation = meow.tool(
    name="even_checker",
    handler=lambda ctx: ctx.ui.success("Valid number!"),
    params={
        "number": meow.param("int", validator=validate_even)
    }
)
```

---

## Handler Context

The `ctx` object is passed to every command handler. It provides access to all runtime modules and inputs.

### Attributes
- `ctx.workspace` (string): Absolute path to the workspace root.
- `ctx.<module>`: Access to standard modules (`fs`, `git`, `ui`, etc.).

### Parameter Access
In the Unified Tool System, parameters defined in `meow.tool` are injected directly as attributes on `ctx`.
- `ctx.my_param_name`: Value of the parameter (typed).

### Methods

**`ctx.run(command_or_tool, **kwargs)`** -> `any`
Executes another registered command or tool within the same process.
- `command_or_tool` (string or ToolValue): Name of command to run, or a tool object (from `meow.tool()` or loaded from a library).
- `**kwargs`: Arguments to pass to the command (overriding defaults).
- **Returns:** The return value of the called handler on success.
- **Error Handling:** If the called handler raises an error (via `fail()` or exception), execution stops immediately. In Starlark, `fail()` terminates the entire script, so errors propagate naturally.

**Example:**
```python
# Run by string name
result = ctx.run("analyze-code", path="main.go")

# Run by tool object
load("//lib/tools.star", "calculator")
result = ctx.run(calculator, a=10, b=5, op="add")
```

---

## Best Practices

- **Idempotency**: Ensure tools can run multiple times without side effects (e.g., check `fs.exists` before writing).
- **Secrets**: Never hardcode API keys. Use `env.get("MY_KEY")`.
- **Validation**: Use `validator` functions in `meow.param` to catch errors early.
- **Output**: Use `ctx.ui` for user-facing logs and `output.write` for machine-readable output.
- **Error Handling**: Starlark has no exceptions. Operations that fail will stop execution with an error message. All error messages follow the pattern: `"operation failed for 'context': details"` to provide clear, actionable information.

### Error Handling

All meowg1k functions provide clear, contextual error messages:

```python
# Error messages include context
result = ctx.fs.read("missing.txt")
# Error: "failed to read file 'missing.txt': no such file or directory"

result = ctx.git.commit("message")
# Error: "git commit failed: nothing to commit, working tree clean"

result = ctx.llm.generate("prompt", preset="invalid")
# Error: "failed to resolve preset 'invalid': preset not found"
```

**Error message format:**
- Includes the operation that failed
- Provides the context (file path, preset name, etc.)
- Shows the underlying error details
- Uses single quotes around user-provided values

**Debugging tips:**
- Read error messages carefully - they include file paths and context
- Check that files/directories exist before operations
- Verify preset/model names are configured in `init.star`
- Use `ctx.ui.info()` to log progress and debug execution flow

---

## Standard Modules

These modules are attached to `ctx` (e.g., `ctx.fs`, `ctx.git`).

### fs (Filesystem)

**`fs.read(path)`** -> `string`
Read file contents. Path is relative to workspace root unless absolute.

**`fs.write(path, content)`** -> `bool`
Write string content to a file. Creates directories if needed.

**`fs.exists(path)`** -> `bool`
Check if a file or directory exists.

**`fs.glob(pattern, ignore=None)`** -> `list[string]`
Find files matching a pattern (supports `**`).
- `ignore` (list[string]): Patterns to exclude.

**`fs.mkdir(path)`** -> `bool`
Create a directory recursively.

**`fs.copy(src, dst)`** -> `bool`
Copy a file.

**`fs.remove(path)`** -> `bool`
Remove a file or directory.

**`fs.cwd()`** -> `string`
Get current working directory.

**`fs.getcwd()`** -> `string` *(Deprecated: use `cwd()`)*
Alias for `cwd()` (deprecated, will be removed in v3.0).

**`fs.filter(dir, pattern="*", recursive=False)`** -> `list[string]`
Filter files in a directory by pattern.
- `dir`: Directory to search (required).
- `pattern`: Glob pattern (default: "*").
- `recursive`: Search subdirectories (default: False).
- Returns: List of matching file paths (relative to workspace).

```python
# Find all Python files in src/
py_files = ctx.fs.filter("src", pattern="*.py")

# Recursively find all JSON files in config/
json_files = ctx.fs.filter("config", pattern="*.json", recursive=True)
```

**`fs.walk(root, pattern="")`** -> `list[string]`
Recursively walk a directory tree and return all matching files.
- `root`: Directory to walk (required).
- `pattern`: Optional glob pattern to filter files (e.g., "*.go").
- Returns: Flat list of file paths (relative to workspace), excluding directories.

```python
# Find all files in project
all_files = ctx.fs.walk(".")

# Find all Go files recursively
go_files = ctx.fs.walk(".", pattern="*.go")

# Find all test files in src/
test_files = ctx.fs.walk("src", pattern="*_test.py")
```

**`fs.stat(path)`** -> `struct{size, mtime, is_dir, mode}`
Get file or directory metadata.
- `path`: File or directory path (required).
- Returns: Struct with metadata fields:
  - `size`: File size in bytes (int)
  - `mtime`: Last modified time as Unix timestamp (int)
  - `is_dir`: True if directory, False if file (bool)
  - `mode`: File permissions as integer (e.g., 0o644) (int)

```python
# Get file metadata
info = ctx.fs.stat("README.md")
print(f"Size: {info.size} bytes")
print(f"Modified: {info.mtime}")
print(f"Is directory: {info.is_dir}")
print(f"Mode: {oct(info.mode)}")

# Check if file was modified recently
if time.time() - info.mtime < 3600:
    print("File modified in last hour")
```

**`fs.listdir(path)`** -> `list[string]`
List directory contents (non-recursive, names only).
- `path`: Directory path (required).
- Returns: List of file and directory names (not full paths).

```python
# List all items in current directory
items = ctx.fs.listdir(".")
print(f"Found {len(items)} items")

# List files in src directory
src_items = ctx.fs.listdir("src")
for item in src_items:
    print(f"  - {item}")
```

**`fs.chmod(path, mode)`** -> `bool`
Change file or directory permissions.
- `path`: File or directory path (required).
- `mode`: Permission mode as integer (e.g., 0o755, 0o644) (required).
- Returns: True on success.

```python
# Make script executable
ctx.fs.chmod("scripts/deploy.sh", 0o755)

# Set file as read-only
ctx.fs.chmod("config/production.yaml", 0o444)

# Standard file permissions
ctx.fs.chmod("README.md", 0o644)
```

**`fs.touch(path, mtime=None)`** -> `bool`
Create an empty file or update its timestamp.
- `path`: File path (required).
- `mtime`: Optional Unix timestamp to set (int). If None, uses current time.
- Returns: True on success.

```python
# Create empty file
ctx.fs.touch("newfile.txt")

# Update file timestamp to now
ctx.fs.touch("existing.txt")

# Set specific timestamp (Unix epoch)
import time
one_day_ago = int(time.time()) - 86400
ctx.fs.touch("file.txt", mtime=one_day_ago)
```

---

### git (Git Operations)

**`git.diff(target="staged")`** -> `struct`
Get diff statistics and raw content.
- `target`: "staged" (default), "HEAD", or "commit-hash".
- Returns: `{raw, files, additions, deletions}`.

**`git.diff_file(file, target="staged")`** -> `string`
Get diff for a specific file.
- `file`: Path to the file (required).
- `target`: "staged" (default), "HEAD", or "commit-hash".
- Returns: Diff content for the specified file.

**`git.status()`** -> `list[string]`
Get porcelain status lines.

**`git.log(count=10)`** -> `list[struct]`
Get recent commits.
- Returns list of `{hash, author, date, message}`.

**`git.branch()`** -> `string`
Get current branch name.

**`git.commit(message)`** -> `struct{success, message, hash, output}`
Create a commit with the given message. Returns commit details.

**`git.add(paths)`** -> `struct{success, files_added, count}`
Stage files (list of strings). Returns list of files added.

**`git.push(remote=None, branch=None)`** -> `struct{success, remote, branch, output}`
Push to remote. Returns push details.

**`git.checkout(target)`** -> `struct{success, target, output}`
Checkout a branch or commit. Returns checkout details.

**`git.create_branch(name, should_checkout=False)`** -> `struct{success, name, checked_out}`
Create a new branch. Returns branch creation result.

**`git.glob(ref="HEAD", pattern="**/*", ignore=None)`** -> `list[string]`
Search for files in git history matching a glob pattern.
- `ref`: Git reference (default: "HEAD"). Also accepts "staged" or "stage".
- `pattern`: Glob pattern with `**` support (default: "**/*").
- `ignore`: List of patterns to exclude (optional).
- Returns: List of matching file paths.

```python
# Find all Go files in HEAD
go_files = ctx.git.glob(pattern="**/*.go")

# Find staged Python files, ignore tests
py_files = ctx.git.glob(ref="staged", pattern="**/*.py", ignore=["**/test_*.py"])
```

**`git.read(ref:path)`** or **`git.read(ref="HEAD", path=...)`** -> `string`
Read file contents from a specific git reference.
- Syntax 1: `git.read("HEAD:path/to/file")` - Git notation (ref:path)
- Syntax 2: `git.read(ref="HEAD", path="path/to/file")` - Explicit parameters
- `ref`: Git reference (default: "HEAD").
- `path`: File path within the repository.
- Returns: File contents as string.

```python
# Read from HEAD using git notation
content = ctx.git.read("HEAD:main.go")

# Read from specific commit
old_version = ctx.git.read(ref="abc123", path="README.md")

# Read staged version
staged = ctx.git.read("staged:config.yaml")
```

**`git.staged_files()`** -> `list[string]`
Get list of all staged files.

**`git.modified_files()`** -> `list[string]`
Get list of modified but not staged files.

**`git.untracked_files()`** -> `list[string]`
Get list of untracked files (respects .gitignore).

```python
# Get different categories of files
staged = ctx.git.staged_files()
modified = ctx.git.modified_files()
untracked = ctx.git.untracked_files()

# Process all files that need attention
for file in staged + modified:
    ctx.ui.info("Processing: " + file)
```

#### Git Constants

**`git.STAGED`** = `"staged"`  
**`git.HEAD`** = `"HEAD"`  
**`git.UNSTAGED`** = `"unstaged"`

Type-safe constants for git target parameters.

```python
# Use constants instead of strings
diff = ctx.git.diff(target=ctx.git.STAGED)
files = ctx.git.modified_files()
```

#### Git Operations with Structured Returns

All git write operations (`commit`, `push`, `add`, `checkout`, `create_branch`) now return structured data instead of booleans:

```python
# Commit with result details
result = ctx.git.commit("feat: add new feature")
ctx.ui.info(f"Committed: {result.hash[:7]}")
ctx.ui.info(f"Message: {result.message}")

# Add files and check count
result = ctx.git.add(["main.go", "README.md"])
ctx.ui.success(f"Staged {result.count} files")
for file in result.files_added:
    ctx.ui.info(f"  • {file}")

# Push with details
result = ctx.git.push()
ctx.ui.success(f"Pushed to {result.remote}/{result.branch}")

# Checkout with confirmation
result = ctx.git.checkout("main")
ctx.ui.info(f"Switched to: {result.target}")

# Create branch with conditional checkout
result = ctx.git.create_branch("feature/new", should_checkout=True)
if result.checked_out:
    ctx.ui.success(f"Created and switched to: {result.name}")
else:
    ctx.ui.info(f"Created branch: {result.name}")
```

---

### llm (Language Models)

**`llm.generate(prompt, system="", preset="smart")`** -> `string`
Generate text using a configured preset.
- `prompt`: The user prompt.
- `system`: System instruction (optional).
- `preset`: Name of the preset to use (default: "smart").

**`llm.agentic(tools, prompt, system="", preset="smart", on_tool_error="return", max_iterations=50)`** -> `string`
Run an agentic loop with native tool calling support. The LLM can autonomously call tools, receive results, and continue processing until it provides a final answer.

- `tools`: List of tool objects (created with `meow.tool()` or loaded from libraries like `//lib/tools.star`).
- `prompt`: Initial user prompt.
- `system`: System instruction (optional).
- `preset`: Name of the preset to use (default: "smart").
- `on_tool_error`: Error handling strategy:
  - `"return"`: Return error as tool result, let LLM see it (default)
  - `"retry"`: Continue loop, let LLM retry
  - `"abort"`: Stop immediately on tool error
- `max_iterations`: Maximum number of LLM→tool→LLM cycles (default: 50).

**Example:**
```python
load("//lib/tools.star", "calculator", "file_reader")

def handler(ctx):
    result = ctx.llm.agentic(
        tools=[calculator, file_reader],
        prompt="Read config.json and calculate the sum of all numeric values",
        system="You are a helpful assistant with access to file and calculation tools",
        preset="smart",
        max_iterations=10
    )
    ctx.output.writeline(result)
```

**`llm.embed(texts, preset="embeddings")`** -> `list[list[float]]`
Generate embeddings for a list of texts.
- `texts`: List of strings to embed.
- `preset`: Name of the embeddings preset to use (default: "embeddings").
- Returns: List of embedding vectors (each a list of floats).

---

### session (Session Management)

The session module provides access to the current execution session and its metadata. Sessions track the complete history of tool invocations, LLM interactions, and results.

**`session.id()`** -> `string`
Get the current session ID.

**`session.tool_name()`** -> `string`
Get the name of the tool being executed in this session.

**`session.parent_id()`** -> `string | None`
Get the parent session ID (None for root sessions).

**`session.status()`** -> `string`
Get the current session status ("running", "completed", or "failed").

**`session.set_metadata(key, value)`** -> `None`
Store metadata in the current session.

**`session.get_metadata(key)`** -> `string | None`
Retrieve metadata from the current session.

**`session.get_all_metadata()`** -> `dict[string, string]`
Get all metadata for the current session.

**`session.get_children()`** -> `list[dict]`
Get information about child sessions created by this session.
Returns list of `{id, tool_name, status, created_at, updated_at}`.

**`session.get_events(limit=0, offset=0)`** -> `list[dict]`
Get events (messages, tool calls, results) for this session.
Returns list of `{id, type, content, tool_call_id, tool_calls, obsolete, created_at}`.

**`session.mark_obsolete(event_ids)`** -> `None`
Mark events as obsolete for compaction (they will be excluded from context).

**`session.insert_summary(after_event_id, content)`** -> `None`
Insert a summary event after a specific event (for context compaction).

**`session.list_all(tool_name=None, status=None, limit=0)`** -> `list[dict]`
Global query to list sessions across the system.

**`session.get_by_id(session_id)`** -> `dict | None`
Get any session by ID (not just current session).

**Example:**
```python
def handler(ctx):
    ctx.ui.info("Session: " + ctx.session.id())
    ctx.ui.info("Tool: " + ctx.session.tool_name())
    
    # Store some metadata
    ctx.session.set_metadata("user_id", "123")
    
    # Call another tool (creates child session)
    ctx.run("analyze-code", path="main.go")
    
    # Check child sessions
    children = ctx.session.get_children()
    ctx.ui.info("Created " + str(len(children)) + " child sessions")
```

---

### shell (Shell Execution)

**`shell.exec(command)`** -> `struct`
Execute a shell command.
- Returns: `{stdout, stderr, exit_code}`.

---

### index (RAG / Search)

**`index.search(query, snapshots=None, top_k=5, min_score=0.7)`** -> `list[struct]`
Semantic search against the codebase.
- Returns list of `{file_path, content, score, start_line, end_line}`.

**`index.build()`** -> `None`
Trigger a rebuild of the vector index.

#### Index Constants

**`index.STRATEGY_FIXED`** = `"fixed"`  
**`index.STRATEGY_SEMANTIC`** = `"semantic"`  
**`index.STRATEGY_AST`** = `"ast"`

Type-safe constants for chunking strategies.

```python
# Use constants for chunking strategies
ctx.index.build(strategy=ctx.index.STRATEGY_SEMANTIC)
```

---

### ui (User Interface)

The UI module provides a comprehensive **Flux Terminal** design system for terminal output with hierarchical contexts, semantic logging, and rich content rendering.

**Important:** All UI functions must be accessed through `ctx.ui` in handler functions:
```python
def my_handler(ctx):
    ctx.ui.info("Hello!")  # ✅ Correct
    # ui.info("Hello!")    # ❌ Wrong - ui is not directly accessible
```

The UI module is available in the handler context as `ctx.ui` and provides the full Flux Terminal API (25 functions).

#### Basic Status Messages

**`ui.info(msg)`**, **`ui.success(msg)`**, **`ui.warn(msg)`**, **`ui.error(msg)`**
Print styled status messages to the console.

```python
ctx.ui.info("Processing files...")
ctx.ui.success("Build completed!")
ctx.ui.warn("Deprecated function used")
ctx.ui.error("Connection failed")
```

#### Hierarchical Contexts

**`ui.step(title, icon=None)`** -> `StepHandle`
Create a visual grouping for related operations. Returns a handle with methods:
- `.done(message=None)`: Complete successfully
- `.fail(message=None)`: Fail with error
- `.write(content)`: Output within step

```python
step = ctx.ui.step("Analyzing Code", icon="🔍")
ctx.ui.info("Found 150 files")
step.done("Complete (1.2s)")
```

#### Semantic Logging

**`ui.think(message)`**
Output dimmed agent reasoning messages.

**`ui.action(message)`**
Output cyan tool/API action messages with ⚡ icon.

```python
ctx.ui.think("Analyzing security constraints...")
ctx.ui.action("Calling GitHub API")
```

#### Rich Content

**`ui.code(content, lang="text", title=None, max_lines=0)`**
Display code with syntax highlighting (100+ languages). Use `max_lines` to truncate.

**`ui.diff(content, title=None, max_lines=0)`**
Display git diff with colored +/- and borders. Use `max_lines` to truncate.

**`ui.tree(data, title=None)`**
Display hierarchical data as a tree with ├── └── branches.

**`ui.properties(data, title=None)`**
Display aligned key-value pairs.

**`ui.link(text, url)`** -> `string`
Create clickable hyperlink (OSC 8). Falls back to "text (url)" in unsupported terminals.

**`ui.pager(content, title=None, show_line_numbers=True)`**
Display content in `less` pager if >30 lines, otherwise prints directly.

```python
ctx.ui.code(code, lang="go", title="main.go", max_lines=20)  # Truncate to 20 lines
ctx.ui.diff(diff_text, title="patch.diff", max_lines=10)
ctx.ui.tree({"dir": {"file": "value"}})
ctx.ui.properties({"Key": "Value"}, title="Config")
ctx.ui.info("See: " + ctx.ui.link("GitHub", "https://github.com/org/repo"))
ctx.ui.pager(large_log, title="Build Log")  # Opens in less
```

#### User Interaction

**`ui.prompt(text, default="", is_sensitive=False, validate=None)`** -> `string`
Ask the user for input with optional masking and validation.

```python
# Simple prompt
name = ctx.ui.prompt("Enter name:")

# Password input (masked with *)
api_key = ctx.ui.prompt("API Key:", is_sensitive=True)

# With validation
def validate_port(val):
    if not val.isdigit():
        return "Must be a number"
    return None

port = ctx.ui.prompt("Enter port:", default="8080", validate=validate_port)
```

**`ui.confirm(prompt, default=False)`** -> `bool`
Ask for Y/n confirmation.

**`ui.select(prompt, items, allow_multiple=False, ...)`** -> `string | list`
Interactive selection menu. Multi-select supported!

```python
# Single select
file = ctx.ui.select("Pick file:", ["a.go", "b.go"])

# Multi-select (use Space to toggle)
files = ctx.ui.select("Pick files:", ["a.go", "b.go", "c.go"], allow_multiple=True)
# Returns: ["a.go", "c.go"]

if ctx.ui.confirm("Continue?", default=True):
    # proceed
```

#### Layout

**`ui.divider(style="line")`**
Display horizontal divider. Styles: "line", "thick", "dotted", "empty".

```python
ctx.ui.divider("thick")  # ━━━━━━━━━━━━━━
# Or use constants
ctx.ui.divider(ctx.ui.DIVIDER_THICK)
```

#### UI Constants

**`ui.DIVIDER_THICK`** = `"thick"`  
**`ui.DIVIDER_THIN`** = `"thin"`  
**`ui.DIVIDER_DOUBLE`** = `"double"`

Type-safe constants for divider styles.

```python
# Use constants for divider styles
ctx.ui.divider(ctx.ui.DIVIDER_THICK)
ctx.ui.divider(ctx.ui.DIVIDER_THIN)
ctx.ui.divider(ctx.ui.DIVIDER_DOUBLE)
```

**`ui.progress(msg, current, total)`**
Update a progress indicator.

**`ui.banner(title, subtext=None)`**
Display a prominent title banner with borders.

```python
ctx.ui.banner("Deployment System", "Production")
```

#### Progress Indicators

**`ui.activity(message)`** -> `ActivityHandle`
Animated spinner for indeterminate operations. Returns handle with:
- `.update(msg)`: Change message
- `.success(msg)`: Complete successfully
- `.fail(msg)`: Complete with error

**`ui.progress_bar(total, message="")`** -> `ProgressBarHandle`
Visual progress bar for batch operations. Returns handle with:
- `.inc(amount=1)`: Increment progress
- `.set(value)`: Set absolute value
- `.done(msg="")`: Finish at 100%

```python
activity = ctx.ui.activity("Processing...")
activity.success("Done!")

bar = ctx.ui.progress_bar(100, "Files")
for i in range(100):
    bar.inc()
bar.done()
```

---

### json (JSON Handling)

**`json.parse(string)`** -> `any`
Parse JSON string into Starlark values.

**`json.stringify(value, indent=0)`** -> `string`
Convert Starlark value to JSON string.

**Example:**
```python
data = ctx.json.parse('{"a": 1}')
print(ctx.json.stringify(data, indent=2))
```

---

### yaml (YAML Handling)

**`yaml.parse(string)`** -> `any`
Parse YAML string into Starlark values. Supports nested structures, lists, and all YAML data types.

**`yaml.stringify(value)`** -> `string`
Convert Starlark value to YAML string.

**Example:**
```python
# Parse YAML configuration
config = ctx.yaml.parse('''
name: myapp
version: 1.0
dependencies:
  - package1
  - package2
''')

# Access parsed data
print(config["name"])  # "myapp"

# Generate YAML
output = ctx.yaml.stringify({"key": "value", "list": [1, 2, 3]})
```

---

### xml (XML Handling)

⚠️ **Note:** The XML module has known limitations with parsing and complex structures. Use with caution for simple XML documents only.

**`xml.parse(string)`** -> `any`
Parse XML string into Starlark values (limited functionality).

**`xml.stringify(value, indent=False, root="")`** -> `string`
Convert Starlark value to XML string.
- `indent` (bool): Enable pretty-printing with indentation
- `root` (string): Optional root element name to wrap the value

**Example:**
```python
# Simple XML parsing (limited support)
xml_str = '<root><item>value</item></root>'
data = ctx.xml.parse(xml_str)

# Generate XML (basic functionality)
data = {"item": "value"}
xml_output = ctx.xml.stringify(data, indent=True, root="root")
```

---

### toml (TOML Handling)

**`toml.parse(string)`** -> `any`
Parse TOML string into Starlark values. Supports tables, nested tables, arrays, and TOML data types.

**`toml.stringify(value)`** -> `string`
Convert Starlark value to TOML string.

**Example:**
```python
# Parse TOML configuration
config = ctx.toml.parse('''
[package]
name = "myapp"
version = "1.0.0"

[dependencies]
lib1 = "^1.2"
lib2 = "^2.0"
''')

# Access parsed data
print(config["package"]["name"])  # "myapp"

# Generate TOML
output = ctx.toml.stringify({
    "database": {
        "server": "localhost",
        "port": 5432
    }
})
```

---

### csv (CSV Handling)

**`csv.parse(string, headers=False, delimiter=",")`** -> `list`
Parse CSV string into Starlark list.
- `headers=False`: Returns list of lists (rows)
- `headers=True`: Returns list of dicts (first row as keys)
- `delimiter`: Field separator (default: `,`)

**`csv.stringify(value, delimiter=",")`** -> `string`
Convert Starlark list to CSV string.
- Accepts list of lists (simple rows) or list of dicts (with headers)
- `delimiter`: Field separator (default: `,`)

**Example:**
```python
# Parse CSV without headers
csv_data = ctx.csv.parse('a,b,c\n1,2,3\n4,5,6')
# Result: [["a", "b", "c"], ["1", "2", "3"], ["4", "5", "6"]]

# Parse CSV with headers
csv_data = ctx.csv.parse('name,age\nAlice,30\nBob,25', headers=True)
# Result: [{"name": "Alice", "age": "30"}, {"name": "Bob", "age": "25"}]

# Generate CSV from list of lists
output = ctx.csv.stringify([["name", "age"], ["Alice", "30"], ["Bob", "25"]])

# Generate CSV from list of dicts (auto-generates headers)
output = ctx.csv.stringify([
    {"name": "Alice", "age": 30},
    {"name": "Bob", "age": 25}
])

# Use custom delimiter (TSV)
tsv_data = ctx.csv.parse("a\tb\tc\n1\t2\t3", delimiter="\t")
```

---

### path (Path Manipulation)

**`path.join(*parts)`** -> `string`
Join path components.

**`path.dirname(path)`** -> `string`
Get directory name (parent directory).

**`path.basename(path)`** -> `string`
Get base name (final path component).

**`path.ext(path)`** -> `string`
Get file extension (including dot).

**`path.extension(path)`** -> `string`
Get file extension (alias for `ext`).

**`path.abs(path)`** -> `string`
Get absolute path.

**`path.clean(path)`** -> `string`
Clean/normalize path.

**`path.rel(base, target)`** -> `string`
Get relative path from base to target.

**`path.stem(path)`** -> `string`
Get filename without extension.

**`path.parent(path)`** -> `string`
Get parent directory (alias for `dirname`).

**`path.parts(path)`** -> `list[string]`
Split path into components.

**Example:**
```python
# Basic operations
full_path = ctx.path.join(ctx.workspace, "config.json")
name = ctx.path.basename(full_path)  # "config.json"
ext = ctx.path.ext(full_path)         # ".json"
stem = ctx.path.stem(full_path)       # "config"
parent = ctx.path.parent(full_path)   # "/path/to/workspace"

# Split path into parts
parts = ctx.path.parts("/path/to/file.txt")  # ["path", "to", "file.txt"]
```

---

### crypto (Cryptography)

**`crypto.sha256(data)`** -> `string` (Hex)
**`crypto.md5(data)`** -> `string` (Hex)
**`crypto.hmac(key, data)`** -> `string` (SHA256 Hex)

**Example:**
```python
hash = ctx.crypto.sha256("hello world")
```

---

### time (Time & Date)

**`time.now(format="")`** -> `int` or `string`
Get current time. Returns Unix timestamp (int) if no format is provided.
- Format specifiers: `%Y`, `%m`, `%d`, `%H`, `%M`, `%S`.

**`time.parse(value, format)`** -> `int`
Parse time string to Unix timestamp.

**`time.format(timestamp, format)`** -> `string`
Format a Unix timestamp.

**`time.sleep(seconds)`**
Pause execution.

**Example:**
```python
ts = ctx.time.now()
iso = ctx.time.now("%Y-%m-%dT%H:%M:%S")
ctx.time.sleep(1.5)
```

---

### regexp (Regular Expressions)

**`regexp.match(pattern, text)`** -> `bool`
**`regexp.find_all(pattern, text, limit=-1)`** -> `list[string]`
**`regexp.replace(pattern, text, replacement)`** -> `string`
**`regexp.split(pattern, text, limit=-1)`** -> `list[string]`

**Example:**
```python
if ctx.regexp.match(r"^\d+$", "123"):
    print("Is number")
```

---

### http (HTTP Client)

The `http` module provides functions for making HTTP requests to APIs and web services.

**`http.get(url, headers={}, params={}, timeout=30)`** -> `struct`
Perform an HTTP GET request.
- `url` (string): The URL to request (required).
- `headers` (dict): HTTP headers to include (optional).
- `params` (dict): Query string parameters (optional).
- `timeout` (int): Request timeout in seconds (default: 30).
- Returns: `{status_code, headers, body, json, ok}`.
  - `status_code`: HTTP status code (int).
  - `headers`: Response headers (dict).
  - `body`: Raw response body (string).
  - `json`: Parsed JSON response (struct/dict/list) or `None` if not JSON.
  - `ok`: True if status code is 2xx (bool).

**`http.post(url, body="", json=None, headers={}, timeout=30)`** -> `struct`
Perform an HTTP POST request.
- `url` (string): The URL to request (required).
- `body` (string): Raw request body (optional).
- `json` (dict/list): JSON data to send (optional, auto-sets Content-Type).
- `headers` (dict): HTTP headers to include (optional).
- `timeout` (int): Request timeout in seconds (default: 30).
- Returns: Same structure as `http.get()`.

**`http.put(url, body="", json=None, headers={}, timeout=30)`** -> `struct`
Perform an HTTP PUT request. Parameters same as `http.post()`.

**`http.delete(url, headers={}, timeout=30)`** -> `struct`
Perform an HTTP DELETE request.
- `url` (string): The URL to request (required).
- `headers` (dict): HTTP headers to include (optional).
- `timeout` (int): Request timeout in seconds (default: 30).
- Returns: Same structure as `http.get()`.

**`http.graphql(url, query, variables={}, token="", timeout=30)`** -> `struct`
Perform a GraphQL query.
- `url` (string): GraphQL endpoint URL (required).
- `query` (string): GraphQL query string (required).
- `variables` (dict): Query variables (optional).
- `token` (string): Bearer token for Authorization header (optional).
- `timeout` (int): Request timeout in seconds (default: 30).
- Returns: Same structure as `http.get()`.

**Example:**
```python
# Simple GET request
response = ctx.http.get("https://api.github.com/repos/retran/meowg1k")
if response.ok:
    repo = response.json
    ctx.ui.info("Stars: " + str(repo["stargazers_count"]))

# POST with JSON
data = {"title": "Bug report", "body": "Description"}
response = ctx.http.post(
    "https://api.github.com/repos/owner/repo/issues",
    json=data,
    headers={"Authorization": "Bearer " + token}
)

# GraphQL query
response = ctx.http.graphql(
    "https://api.github.com/graphql",
    query="query { viewer { login } }",
    token=ctx.env.get("GITHUB_TOKEN")
)
if response.ok:
    user = response.json["data"]["viewer"]["login"]
    ctx.ui.success("Logged in as: " + user)

# GET with query parameters
response = ctx.http.get(
    "https://api.example.com/search",
    params={"q": "golang", "limit": "10"},
    headers={"Accept": "application/json"}
)
```

**Error Handling:**
HTTP requests may fail due to network errors, timeouts, or invalid URLs. Always check `response.ok` or `status_code`:

```python
response = ctx.http.get("https://api.example.com/data")
if not response.ok:
    ctx.ui.error("Request failed with status: " + str(response.status_code))
    return

# Or check specific status codes
if response.status_code == 404:
    ctx.ui.warn("Resource not found")
elif response.status_code >= 500:
    ctx.ui.error("Server error occurred")
```

---

### template (Text Templates)

The `template` module provides Go's `text/template` engine for dynamic text generation with data interpolation.

**`template.parse(text, name="")`** -> `Template`
Parse a template from a string.
- `text` (string): Template text using Go template syntax (required).
- `name` (string): Template name for debugging (optional, default: "template").
- Returns: Template object with `render()` method.

**`template.load(path)`** -> `Template`
Load and parse a template from a file.
- `path` (string): Path to template file (required, relative to workspace or absolute).
- Returns: Template object with `render()` method.

**`Template.render(data)`** -> `string`
Render the template with provided data.
- `data` (dict): Data to interpolate into template (required).
- Returns: Rendered string.

**Template Syntax:**
Go templates use `{{}}` for actions:
- `{{.FieldName}}` - Access field from data
- `{{if .Condition}}...{{end}}` - Conditional blocks
- `{{range .Items}}...{{end}}` - Iterate over lists
- `{{.Nested.Field}}` - Nested field access

**Example:**
```python
# Simple template parsing and rendering
tmpl = ctx.template.parse("Hello {{.Name}}, you are {{.Age}} years old")
result = tmpl.render({"Name": "Alice", "Age": 30})
ctx.ui.info(result)  # "Hello Alice, you are 30 years old"

# Template with conditional
tmpl = ctx.template.parse("""
{{if .Active}}
User {{.Name}} is active
{{else}}
User {{.Name}} is inactive
{{end}}
""")
result = tmpl.render({"Name": "Bob", "Active": True})

# Template with iteration
tmpl = ctx.template.parse("""
Users:
{{range .Users}}
  - {{.Name}} ({{.Email}})
{{end}}
""")
users = [
    {"Name": "Alice", "Email": "alice@example.com"},
    {"Name": "Bob", "Email": "bob@example.com"},
]
result = tmpl.render({"Users": users})

# Load template from file
# templates/commit.tmpl:
# {{.Type}}({{.Scope}}): {{.Description}}
# 
# {{.Body}}
tmpl = ctx.template.load("templates/commit.tmpl")
message = tmpl.render({
    "Type": "feat",
    "Scope": "auth",
    "Description": "add OAuth2 support",
    "Body": "Implements OAuth2 with PKCE flow"
})

# Nested data access
tmpl = ctx.template.parse("{{.User.Name}} <{{.User.Email}}>")
result = tmpl.render({
    "User": {
        "Name": "Alice",
        "Email": "alice@example.com"
    }
})
```

**Common Template Patterns:**
```python
# Commit message template
commit_tmpl = ctx.template.parse("""
{{.Type}}({{.Scope}}): {{.Summary}}

{{.Details}}

{{if .BreakingChanges}}
BREAKING CHANGE: {{.BreakingChanges}}
{{end}}
{{if .Issues}}
Closes: {{range .Issues}}#{{.}} {{end}}
{{end}}
""")

# PR description template
pr_tmpl = ctx.template.parse("""
## Summary
{{.Summary}}

## Changes
{{range .Changes}}
- {{.}}
{{end}}

## Testing
{{.Testing}}
""")

# Code generation template
class_tmpl = ctx.template.parse("""
class {{.ClassName}}:
    def __init__(self{{range .Fields}}, {{.Name}}{{end}}):
        {{range .Fields}}
        self.{{.Name}} = {{.Name}}
        {{end}}
""")
```

**Error Handling:**
Template parsing and rendering errors include context:
```python
try:
    tmpl = ctx.template.parse("{{.Missing")  # Syntax error
except:
    ctx.ui.error("Template parse error")

try:
    tmpl = ctx.template.parse("{{.User.Name}}")
    result = tmpl.render({"User": None})  # Runtime error
except:
    ctx.ui.error("Template render error")
```

---

### stdin (Standard Input)

**`stdin.read()`** -> `string`
Read all of stdin.

**`stdin.read_line()`** -> `string`
Read a single line.

**`stdin.is_piped()`** -> `bool`
Check if data is being piped to the process.

---

### output (Output Writing)

**`output.write(content)`**
Write text to stdout (buffered).

**`output.writeline(content)`**
Write text to stdout with newline.

**`output.writef(format, *args)`**
Formatted write (like `printf`).

---

## Complete Example

### Code Review Workflow

This example demonstrates combining `git`, `llm`, and `ui` to create a code review assistant.

```python
def review_handler(ctx):
    # 1. Get staged changes
    diff = ctx.git.diff(target="staged")
    if not diff.raw:
        ctx.ui.warn("No staged changes found.")
        return

    # 2. Analyze with LLM
    ctx.ui.info("Analyzing changes...")
    prompt = "Review these changes:\n" + diff.raw
    review = ctx.llm.generate(prompt, preset="coding")

    # 3. Output result
    ctx.ui.success("Review Complete:")
    ctx.output.writeline(review)

    # 4. Ask to commit
    if ctx.ui.prompt("Commit with this review? (y/n)") == "y":
        result = ctx.git.commit(message="refactor: " + review[:50])
        ctx.ui.success("Committed as: " + result.hash[:7])

review_tool = meow.tool(
    name="review",
    handler=review_handler,
    description="Analyze staged changes"
)

meow.command(review_tool)
```

---


The `ui` module provides a comprehensive **Flux Terminal** design system for creating professional, hierarchical terminal interfaces. All widgets support Unicode with ASCII fallback and plain mode for piping.

**🔑 Important - Accessing UI Functions:**

All UI functions are accessed through the handler context as `ctx.ui`:

```python
def my_handler(ctx):
    # ✅ Correct - use ctx.ui
    ctx.ui.info("Starting...")
    ctx.ui.code(content, lang="python")

    # ❌ Wrong - ui is not globally available
    # ui.info("Starting...")
```

The documentation below uses `ui.` in function signatures for clarity, but **always use `ctx.ui.` in your code**.

---

## 1. Flow Control & Hierarchical Contexts

### ui.step(title, icon=None) → StepHandle

Creates a visual grouping for related operations with timing and nesting support.

**Parameters:**
- `title` (string): Step title
- `icon` (string, optional): Icon/emoji to display (e.g., "🔍", "⚡")

**Returns:** `StepHandle` with methods:
- `.done(message=None)`: Complete successfully, displays duration
- `.fail(message=None)`: Fail with error message
- `.write(content)`: Output content within the step context

**Example:**
```python
step = ctx.ui.step("Analyzing Code", icon="🔍")
ctx.ui.info("Found 150 files")
ctx.ui.info("Scanning for issues...")
step.done("Analysis complete")  # Shows: ✔ Analysis complete (2.3s)

# Nested steps
parent = ctx.ui.step("Building Project", icon="🔨")
child = ctx.ui.step("Running Tests", icon="✓")
child.done()
parent.done("Build successful")

# Failed step
risky = ctx.ui.step("Deploying", icon="🚀")
risky.fail("Connection timeout")  # Shows: ✖ Connection timeout (1.5s)
```

**Output:**
```
╭─ 🔍 Analyzing Code ──────────────────
│  ℹ Found 150 files
│  ℹ Scanning for issues...
╰─ ✔ Analysis complete (2.3s) ────────
```

---

## 2. Interactive Progress Indicators

### ui.activity(message) → ActivityHandle

Creates an animated spinner for indeterminate operations.

**Parameters:**
- `message` (string): Initial status message

**Returns:** `ActivityHandle` with methods:
- `.update(message)`: Change the status message
- `.success(message)`: Complete successfully with final message
- `.fail(message)`: Complete with error message

**Example:**
```python
activity = ctx.ui.activity("Downloading dependencies...")
# ... work in progress ...
activity.update("Almost done...")
activity.success("Downloaded 42 packages")  # Shows duration

# Or on failure:
activity.fail("Network error")
```

**Output (animated):**
```
⠋ Downloading dependencies...
⠙ Downloading dependencies...
⠹ Almost done...
✔ Downloaded 42 packages (5.2s)
```

### ui.progress_bar(total, message="Progress") → ProgressBarHandle

Creates a visual progress bar for deterministic batch operations.

**Parameters:**
- `total` (int): Total number of items/steps
- `message` (string, optional): Progress label

**Returns:** `ProgressBarHandle` with methods:
- `.inc(amount=1)`: Increment progress counter
- `.set(value)`: Set absolute progress value
- `.done(message="Complete")`: Finish at 100%

**Example:**
```python
files = ["a.go", "b.go", "c.go"]
pb = ctx.ui.progress_bar(len(files), "Processing")

for file in files:
    process(file)
    pb.inc()  # Increment by 1

pb.done("All files processed")
```

**Output:**
```
Processing [████████████░░░░░░░░] 60% (18/30)
✔ All files processed (3.1s)
```

---

## 3. Semantic Logging

Messages color-coded by semantic meaning for visual scanning.

### ui.think(message)

Outputs dimmed "thinking" messages for agent reasoning (Chain of Thought).

```python
ctx.ui.think("Analyzing security constraints...")
ctx.ui.think("Should we refactor the auth module?")
```

### ui.action(message)

Outputs cyan action messages for tool/API execution.

```python
ctx.ui.action("Reading configuration file")
ctx.ui.action("Calling GitHub API")
ctx.ui.action("Writing patch to disk")
```

### ui.info(message)

Neutral informational message.

```python
ctx.ui.info("Found 150 Go files")
```

### ui.success(message)

Green success message with checkmark.

```python
ctx.ui.success("Tests passed!")
```

### ui.warn(message)

Yellow warning message.

```python
ctx.ui.warn("Deprecated function used")
```

### ui.error(message)

Red error message with X mark.

```python
ctx.ui.error("Build failed")
```

**Output:**
```
✦ Analyzing security constraints...
⚡ Calling GitHub API
ℹ Found 150 Go files
✔ Tests passed!
⚠ Deprecated function used
✖ Build failed
```

---

## 4. Rich Content Display

### ui.code(content, lang="text", title=None, max_lines=0)

Displays code with syntax highlighting (100+ languages via Chroma).

**Parameters:**
- `content` (string): Code to display
- `lang` (string, optional): Language for highlighting (go, python, json, diff, etc.)
- `title` (string, optional): Panel title (e.g., filename)
- `max_lines` (int, optional): Truncate content to N lines (0 = no limit)

**Supported languages:** go, python, javascript, typescript, rust, java, c, cpp, bash, sql, json, yaml, xml, html, css, markdown, diff, and 90+ more.

**Example:**
```python
code = """package main

import "fmt"

func main() {
    fmt.Println("Hello, Flux!")
}"""

ctx.ui.code(code, lang="go", title="main.go")

# Truncate long files
long_code = fs.read("large_file.go")
ctx.ui.code(long_code, lang="go", title="large_file.go", max_lines=20)
```

**Output:**
```
╭── main.go ────────────────────╮
│ package main                  │
│                               │
│ import "fmt"                  │
│                               │
│ func main() {                 │
│     fmt.Println("Hello!")     │
│ }                             │
╰───────────────────────────────╯
```

**Truncation example:**
```
╭── large_file.go ─────────────╮
│ package main                 │
│ ...                          │
│ (first 20 lines)             │
⋮ [150 more lines]
╰──────────────────────────────╯
```

### ui.diff(content, title=None, max_lines=0)

Displays git diff with colored additions/deletions and borders.

**Parameters:**
- `content` (string): Unified diff text
- `title` (string, optional): Panel title (usually filename)
- `max_lines` (int, optional): Truncate content to N lines (0 = no limit)

**Example:**
```python
diff = """diff --git a/auth.go b/auth.go
--- a/auth.go
+++ b/auth.go
@@ -10,2 +10,3 @@
-token = "hardcoded"
+token = os.Getenv("TOKEN")
+// Security improvement"""

ctx.ui.diff(diff, title="auth.go")
```

**Output:**
```
╭── auth.go ────────────────────╮
│ diff --git a/auth.go b/auth.go│
│ --- a/auth.go                 │
│ +++ b/auth.go                 │
│ @@ -10,2 +10,3 @@             │
│ -token = "hardcoded"          │ (red)
│ +token = os.Getenv("TOKEN")   │ (green)
│ +// Security improvement      │ (green)
╰───────────────────────────────╯
```

### ui.tree(data, title=None)

Displays hierarchical data as a tree with branches.

**Parameters:**
- `data` (dict or list): Nested structure to visualize
- `title` (string, optional): Tree title

**Example:**
```python
structure = {
    "src": {
        "cmd": ["main.go", "config.go"],
        "internal": {
            "ui": ["theme.go", "widgets.go"],
            "core": ["engine.go"]
        }
    },
    "tests": ["unit_test.go", "integration_test.go"]
}

ctx.ui.tree(structure, title="Project Structure")
```

**Output:**
```
Project Structure
src
├── cmd
│   ├── main.go
│   └── config.go
└── internal
    ├── ui
    │   ├── theme.go
    │   └── widgets.go
    └── core
        └── engine.go
tests
├── unit_test.go
└── integration_test.go
```

### ui.properties(data, title=None)

Displays key-value pairs in an aligned, compact list.

**Parameters:**
- `data` (dict): Key-value pairs to display
- `title` (string, optional): Properties section title

**Example:**
```python
ctx.ui.properties({
    "Model": "gpt-4-turbo",
    "Temperature": "0.7",
    "Max Tokens": "2048",
    "Status": "Active",
    "Cost": "$0.03"
}, title="LLM Configuration")
```

**Output:**
```
LLM Configuration
Model:        gpt-4-turbo
Temperature:  0.7
Max Tokens:   2048
Status:       Active
Cost:         $0.03
```

### ui.table(data, columns=None, title=None)

Displays data in a formatted table.

**Parameters:**
- `data` (list): List of dicts or list of lists
- `columns` (list, optional): Column headers (required if data is list of lists)
- `title` (string, optional): Table title

**Example:**
```python
# List of dicts (columns auto-detected)
results = [
    {"file": "main.go", "lines": 150, "issues": 2},
    {"file": "auth.go", "lines": 80, "issues": 0},
    {"file": "api.go", "lines": 200, "issues": 5}
]
ctx.ui.table(results, title="Scan Results")

# List of lists (columns required)
data = [
    ["Alice", 25, "Engineer"],
    ["Bob", 30, "Designer"]
]
ctx.ui.table(data, columns=["Name", "Age", "Role"])
```

**Output:**
```
Scan Results
┌──────────┬───────┬────────┐
│ FILE     │ LINES │ ISSUES │
├──────────┼───────┼────────┤
│ main.go  │ 150   │ 2      │
│ auth.go  │ 80    │ 0      │
│ api.go   │ 200   │ 5      │
└──────────┴───────┴────────┘
```

### ui.markdown(content)

Renders Markdown with basic formatting (headers, bold, italic, lists).

**Example:**
```python
ctx.ui.markdown("""
# Analysis Complete

## Summary
- **Files Scanned**: 150
- **Issues Found**: 7
- **Critical**: 2

See [documentation](https://example.com) for details.
""")
```

### ui.panel(content, title=None)

Displays content in a bordered panel.

**Example:**
```python
ctx.ui.panel("⚠️  Important: Backup your data before proceeding!",
         title="Warning")
```

### ui.link(text, url) → string

Creates a clickable hyperlink using OSC 8 terminal escape sequences. Modern terminals (iTerm2, kitty, WezTerm, Windows Terminal) will make the text clickable.

**Parameters:**
- `text` (string): Display text for the link
- `url` (string): Target URL

**Returns:** Formatted link string (with OSC 8 codes if supported, otherwise plain text)

**Example:**
```python
# Create clickable link
link = ctx.ui.link("View PR", "https://github.com/org/repo/pull/123")
ctx.ui.info("PR created: " + link)

# Simple URL (text = url)
ctx.ui.info("Documentation: " + ctx.ui.link("https://docs.example.com", "https://docs.example.com"))
```

**Terminal support:**
- ✅ iTerm2 (macOS)
- ✅ WezTerm
- ✅ Windows Terminal
- ✅ kitty
- ✅ VS Code terminal
- ❌ Basic terminals: Falls back to "text (url)" format

### ui.pager(content, title=None, show_line_numbers=True)

Displays content in a pager (like `less`) for easy scrolling through large text. If content is short (<30 lines), displays directly.

**Parameters:**
- `content` (string): Text to display
- `title` (string, optional): Header title
- `show_line_numbers` (bool, optional): Show line numbers (default: True)

**Behavior:**
- Content ≤ 30 lines: Direct output
- Content > 30 lines: Opens in `less` pager
- No `less` available: Falls back to direct output

**Pager controls (less):**
- `Space`: Next page
- `b`: Previous page
- `q`: Quit
- `/pattern`: Search
- `G`: Go to end
- `g`: Go to start

**Example:**
```python
# View large log file
logs = fs.read("build.log")
ctx.ui.pager(logs, title="Build Logs", show_line_numbers=True)

# View command output
result = shell.run("git log --all --oneline --graph")
ctx.ui.pager(result.stdout, title="Git History", show_line_numbers=False)

# Short content (prints directly)
ctx.ui.pager("Line 1\nLine 2\nLine 3", title="Short")
```

**Output (short content):**
```
=== Short ===
   1  Line 1
   2  Line 2
   3  Line 3
```

**Output (long content):** Opens interactive `less` viewer.

---

## 5. User Interaction

### ui.prompt(message, default="", is_sensitive=False, validate=None) → string

Prompts user for text input with optional password masking and validation.

**Parameters:**
- `message` (string): Prompt message
- `default` (string, optional): Default value if user presses Enter
- `is_sensitive` (bool, optional): **Hide input with asterisks** (for passwords, API keys)
  - When `True`, input is masked and not echoed to terminal
  - Uses `golang.org/x/term` for secure input
- `validate` (function, optional): Validation callback function
  - Accepts one string parameter (the input value)
  - Returns `None` if input is valid
  - Returns error message string if invalid
  - Prompt repeats on validation failure

**Returns:** User's input as string

**Example:**
```python
# Simple prompt
name = ctx.ui.prompt("Enter your name:")

# With default
branch = ctx.ui.prompt("Branch name:", default="main")

# Password input (MASKED)
api_key = ctx.ui.prompt("API Key:", is_sensitive=True)
# User sees: API Key: *********

# With validation callback
def validate_port(value):
    if not value.isdigit():
        return "Must be a number"
    port = int(value)
    if port < 1 or port > 65535:
        return "Port must be between 1 and 65535"
    return None  # Valid

port = ctx.ui.prompt("Enter port:", default="8080", validate=validate_port)

# Combined: is_sensitive + validation
def validate_token(value):
    if len(value) < 20:
        return "Token too short (min 20 chars)"
    return None

token = ctx.ui.prompt("GitHub Token:", is_sensitive=True, validate=validate_token)
```

**Interactive Flow:**
```text
# Regular input
Enter your name: Alice
✓ Accepted

# Sensitive input (masked)
API Key: ********************
✓ Accepted

# With validation
Enter port [8080]: abc
✗ Must be a number
Enter port [8080]: 99999
✗ Port must be between 1 and 65535
Enter port [8080]: 3000
✓ Accepted
```

**Mode Support:**
- ✅ Plain: Yes (no masking in non-TTY)
- ✅ Terminal: Yes (with masking and validation)
- ✅ Unicode: N/A
- ✅ ASCII: N/A

**Security Note:** When `is_sensitive=True`, input is not stored in shell history and is masked in the terminal. However, it's still in memory as a string.

---

Asks user for Y/n confirmation.

**Parameters:**
- `prompt` (string): Question to ask
- `default` (bool, optional): Default if user presses Enter

**Returns:** `True` for yes, `False` for no

**Example:**
```python
if ctx.ui.confirm("Deploy to production?", default=False):
    deploy_to_prod()
else:
    ctx.ui.info("Deployment cancelled")

# With default=True
if ctx.ui.confirm("Continue with defaults?", default=True):
    use_defaults()
```

**Output:**
```
Deploy to production? (y/N) › y
✔ Deploying...

Continue with defaults? (Y/n) › [Enter]
✔ Using defaults
```

### ui.select(prompt, options, allow_multiple=False, is_fuzzy=True, limit=10, ...) → string | list

Displays an interactive selection menu with fuzzy search.

**Parameters:**
- `prompt` (string): Selection prompt/question
- `options` (list): List of choices (strings or dicts with label/value keys)
- `allow_multiple` (bool, optional): **Enable multi-select mode** (default: False)
  - When `True`: Use Space to toggle items, Enter to confirm
  - Returns list of selected values instead of single value
  - Shows `[x]` checkboxes for selected items
- `is_fuzzy` (bool, optional): Enable fuzzy search (default: True)
- `limit` (int, optional): Max items to show at once (default: 10)
- `placeholder` (string, optional): Search input placeholder
- `initial_query` (string, optional): Pre-fill search
- `allow_new` (bool, optional): Allow creating new value (default: False)
- `should_return_index` (bool, optional): Return index instead of value (default: False)
- `label_key` (string, optional): Key for label in dict items (default: "label")
- `value_key` (string, optional): Key for value in dict items (default: "value")
- `meta_key` (string, optional): Key for metadata in dict items (default: "meta")

**Returns:**
- Single select mode: String (selected value)
- Multi-select mode: List of strings (selected values)

**Example:**
```python
# Single select (default)
env = ctx.ui.select("Choose environment:", [
    "Development",
    "Staging",
    "Production"
])

# Multi-select mode (NEW!)
files = ctx.ui.select(
    "Select files to process:",
    ["main.go", "utils.go", "config.go", "api.go"],
    allow_multiple=True
)
# User can select multiple with Space
# Returns: ["main.go", "config.go"]

# With dict options
result = ctx.ui.select("Choose preset:", [
    {"label": "Fast (GPT-3.5)", "value": "fast"},
    {"label": "Smart (GPT-4)", "value": "smart"},
    {"label": "Balanced (GPT-4-mini)", "value": "balanced"}
])

# Multi-select with dict options
selected = ctx.ui.select(
    "Choose deployment targets:",
    [
        {"label": "🚀 Production (US-East)", "value": "prod-us"},
        {"label": "🧪 Staging", "value": "staging"},
        {"label": "💻 Development", "value": "dev"}
    ],
    allow_multiple=True
)
# Returns: ["prod-us", "staging"]
```

**Interactive UI (Single Mode):**
```text
Choose environment:
Search:
> [Development]
  [Staging]
  [Production]
Enter: select  Esc: cancel
```

**Interactive UI (Multi Mode):**
```text
Select files to process:
Search: util
> [x] utils.go
  [ ] main.go
  [ ] config.go
Enter: confirm  Space: toggle  Esc: cancel
```

**Keyboard Controls:**
- **↑/↓ or k/j**: Navigate items
- **Enter**: Confirm selection (single mode) or confirm all selected (multi mode)
- **Space**: Toggle current item (multi mode only)
- **Ctrl+A**: Select all visible items (multi mode only)
- **Ctrl+D**: Deselect all (multi mode only)
- **PgUp/PgDn**: Page navigation
- **Home/End**: Jump to first/last
- **Esc**: Cancel

**Mode Support:**
- ❌ Plain: No (requires interactive terminal)
- ✅ Terminal: Yes (full interactive mode)
- ✅ Unicode: Yes (uses box drawing)
- ✅ ASCII: Fallback available

---

## 6. Layout & Formatting

### ui.divider(style="line")

Displays a horizontal divider line across terminal width.

**Parameters:**
- `style` (string, optional): Divider style
  - `"line"`: ──────────── (default)
  - `"thick"`: ━━━━━━━━━━━
  - `"dotted"`: ············
  - `"empty"`: blank line

**Example:**
```python
ctx.ui.info("Section 1")
ctx.ui.divider("thick")
ctx.ui.info("Section 2")
ctx.ui.divider()
ctx.ui.info("Section 3")
ctx.ui.divider("dotted")
```

**Output:**
```
ℹ Section 1
━━━━━━━━━━━━━━━━━━━━━━━━━━
ℹ Section 2
──────────────────────────
ℹ Section 3
··························
```

### ui.banner(title, subtext=None)

Displays a prominent title banner with borders.

**Parameters:**
- `title` (string): Main title text
- `subtext` (string, optional): Subtitle or description

**Example:**
```python
ctx.ui.banner("MEOW AI Assistant", "AI Development Assistant")
ctx.ui.banner("Code Review Complete")
```

**Output:**
```
════════════════════════════════════
  MEOW AI Assistant
  AI Development Assistant
════════════════════════════════════
```

### ui.progress(message)

Simple progress indicator (legacy, use `ui.progress_bar()` for interactive).

**Example:**
```python
ctx.ui.progress("Processing 42 files...")
```

---

## 7. Advanced/Internal Functions

### ui.render(template, data)

Internal function for template rendering. Typically not used directly.

---

## Complete Example

```python
def analyze_codebase(ctx):
    """Complete workflow demonstrating all UI widgets"""

    # 1. Banner
    ctx.ui.banner("Code Analysis Tool")
    ctx.ui.divider("thick")

    # 2. Properties panel
    ctx.ui.properties({
        "Repository": ctx.workspace,
        "Branch": "main",
        "Files": "150"
    }, title="Configuration")

    ctx.ui.divider()

    # 3. Hierarchical steps with activity
    scan_step = ctx.ui.step("Scanning Repository", icon="🔍")

    ctx.ui.think("Determining which files to analyze...")
    ctx.ui.action("Reading .gitignore patterns")

    activity = ctx.ui.activity("Indexing files...")
    # ... indexing work ...
    activity.success("Indexed 1,420 files")

    scan_step.done("Scan complete")

    # 4. Progress bar for batch work
    analysis_step = ctx.ui.step("Analyzing Code", icon="🔬")

    files = get_files_to_analyze()
    pb = ctx.ui.progress_bar(len(files), "Processing")

    issues = []
    for file in files:
        result = analyze_file(file)
        issues.extend(result.issues)
        pb.inc()

    pb.done("Analysis complete")

    # 5. Display results with table
    if issues:
        ctx.ui.warn(f"Found {len(issues)} issues")
        ctx.ui.table(issues, title="Issues Found")
    else:
        ctx.ui.success("No issues found!")

    # 6. Show code sample if requested
    if issues:
        ctx.ui.code(issues[0].code, lang="go", title=issues[0].file)
        ctx.ui.diff(issues[0].fix, title="Proposed fix")

    # 7. Tree of affected files
    affected = group_by_directory(issues)
    ctx.ui.tree(affected, title="Affected Files")

    analysis_step.done("Analysis complete")

    # 8. Interactive confirmation
    ctx.ui.divider()
    if ctx.ui.confirm("Apply automatic fixes?", default=True):
        fix_step = ctx.ui.step("Applying Fixes", icon="🔧")

        pb2 = ctx.ui.progress_bar(len(issues), "Fixing")
        for issue in issues:
            apply_fix(issue)
            pb2.inc()
        pb2.done("All fixes applied")

        ctx.ui.success("Fixes applied successfully!")
        fix_step.done()
    else:
        ctx.ui.info("Skipping fixes")

    # 9. Summary
    ctx.ui.divider("thick")
    ctx.ui.success("✨ Code analysis complete!")

    return {"issues_found": len(issues)}
```

---

## Feature Matrix

| Feature | Plain Mode | Terminal | Unicode | ASCII |
|---------|------------|----------|---------|-------|
| step() | ✅ Text | ✅ Borders | ╭─╮╰ | +--+ |
| activity() | ✅ Static | ✅ Animated | ⠋⠙⠹ | ... |
| progress_bar() | ✅ % | ✅ Visual | █░ | #- |
| think/action | ✅ Prefix | ✅ Colored | ✦⚡ | * ! |
| code() | ✅ Fence | ✅ Highlight | ✅ | ✅ |
| diff() | ✅ +/- | ✅ Colors | ✅ | ✅ |
| tree() | ✅ Text | ✅ Lines | ├──└ | |-- |
| divider() | --- | ✅ Styled | ─━· | --- |
| banner() | === | ✅ Styled | ═══ | === |

---

## Notes

### Plain Mode
All widgets automatically degrade to plain text when:
- Output is piped (`| less`, `> file`)
- `NO_COLOR` environment variable is set
- Terminal is not detected

### Unicode Support
Widgets detect terminal Unicode capability and fallback to ASCII:
- UTF-8 locale: Full Unicode
- ASCII locale: ASCII fallback
- Auto-detected per terminal

### Thread Safety
- Activity and ProgressBar are thread-safe (use mutex)
- Other widgets should be called from single thread

### Performance
- Syntax highlighting cached per language
- Terminal width detected once per widget
- Spinner updates at 80ms intervals (12.5 FPS)


---


## UI Features

### Available Features (25 total)

### Feature Categories

#### 1. Flow Control & Context (4 features)

| Feature | Description |
|---------|-------------|
| `ui.step()` | Visual grouping with timing and nesting |
| `ui.activity()` | Animated spinner for indeterminate progress |
| `ui.progress_bar()` | Determinate progress with percentage |
| `ui.divider()` | Visual separators (line, thick, dotted, empty) |

#### 2. Semantic Logging (6 features)

| Feature | Description |
|---------|-------------|
| `ui.info()` | Informational messages (blue ℹ) |
| `ui.success()` | Success messages (green ✓) |
| `ui.warn()` | Warning messages (yellow ⚠) |
| `ui.error()` | Error messages (red ✗) |
| `ui.think()` | Agent thoughts (faint, italic) |
| `ui.action()` | Tool actions (cyan ⚡) |

#### 3. Rich Content Display (7 features)

| Feature | Description |
|---------|-------------|
| `ui.markdown()` | Markdown rendering (headers, bold, lists) |
| `ui.code()` | Syntax highlighting (100+ languages) + truncation |
| `ui.diff()` | Git diffs with colored +/- + truncation |
| `ui.table()` | Tabular data with borders |
| `ui.properties()` | Aligned key-value lists |
| `ui.tree()` | Hierarchical structures with branches |
| `ui.panel()` | Bordered content boxes |

#### 4. User Interaction (5 features)

| Feature | Description |
|---------|-------------|
| `ui.prompt()` | Text input with validation + password masking |
| `ui.confirm()` | Y/n confirmation prompts |
| `ui.select()` | Interactive selection (single + multi-select) |
| `ui.progress()` | Simple progress counter |
| `ui.render()` | Auto-render Starlark values |

#### 5. Advanced Features (3 features)

| Feature | Description |
|---------|-------------|
| `ui.link()` | OSC 8 clickable hyperlinks |
| `ui.pager()` | Interactive viewer for large content (less) |
| `ui.banner()` | ASCII art title banners | 
---

### API Access Pattern

**Important:** All UI functions are accessed through `ctx.ui` in handlers:

```python
def my_handler(ctx):
    # ✅ Correct
    ctx.ui.info("Hello!")
    ctx.ui.code(content, lang="python")
    link = ctx.ui.link("GitHub", "https://github.com/org/repo")
    
    # ❌ Wrong - ui is not globally available
    # ui.info("Hello!")
```

---

### Terminal Support

**Unicode vs ASCII:**
- Auto-detects via `LANG`, `LC_ALL`, `TERM` environment variables
- Unicode: `✓ ✗ ⚠ ℹ ⚡ ╭─╮│╰─╯ ├─└ ⋮`
- ASCII fallback: `+ x ! i > +-+|+-+ |- ...`

**Color Support:**
- Detects via `TERM_PROGRAM`, `NO_COLOR`, TTY check
- Plain mode for pipes: `--plain` flag or isatty detection

**OSC 8 Hyperlinks (ui.link):**
- ✅ iTerm2, WezTerm, Windows Terminal, kitty, VS Code
- ❌ Basic terminals: Falls back to "text (url)" format

**Interactive Pager (ui.pager):**
- Auto-detects `less` availability
- Content ≤ 30 lines: Direct output
- Content > 30 lines: Opens in less
- Preserves ANSI colors

---

## Type Reference

This section provides explicit type signatures for all Starlark functions. While Starlark is dynamically typed, understanding parameter and return types helps prevent errors.

### Notation

- `string`: Text data
- `int`: Integer numbers  
- `float`: Floating-point numbers
- `bool`: True/False values
- `list[T]`: Ordered collection of type T
- `dict[K, V]`: Key-value mapping
- `struct`: Starlark struct with named fields
- `?` suffix: Optional parameter (e.g., `target?: string`)

### Domain Types

**GitDiffResult**: `struct`
```python
{
    raw: string,        # Raw diff output
    files: list[string], # Changed file paths
    additions: int,      # Number of added lines
    deletions: int       # Number of deleted lines
}
```

**GitLogEntry**: `struct`
```python
{
    hash: string,      # Commit SHA
    author: string,    # Author name
    date: string,      # Commit date
    message: string    # Commit message
}
```

**ShellResult**: `struct`
```python
{
    stdout: string,    # Standard output
    stderr: string,    # Standard error
    exit_code: int     # Exit status
}
```

**IndexSearchResult**: `struct`
```python
{
    file_path: string,  # File containing match
    content: string,    # Matched content chunk
    score: float,       # Relevance score (0-1)
    start_line: int,    # Starting line number
    end_line: int       # Ending line number
}
```

**UIStepHandle**: `object`
```python
{
    .done(message?: string) -> None,  # Complete successfully
    .fail(message?: string) -> None,  # Fail with error
    .write(content: string) -> None   # Output within step
}
```

**UIActivityHandle**: `object`
```python
{
    .done(message?: string) -> None,  # Complete activity
    .fail(message?: string) -> None   # Fail activity
}
```

**UIProgressBarHandle**: `object`
```python
{
    .inc(delta?: int) -> None,        # Increment progress
    .set(value: int) -> None,         # Set absolute value
    .done(message?: string) -> None   # Complete progress
}
```

### Complete Function Signatures

#### meow module
```python
meow.provider(name: string, type: string, **kwargs) -> None
meow.model(name: string, **kwargs) -> None
meow.preset(name: string, **kwargs) -> None
meow.presets() -> list[string]
meow.tool(name: string, handler: function, params?: dict, description?: string) -> None
meow.param(type: string, **kwargs) -> ParamDef
meow.command(name: string, tool: Tool, **kwargs) -> None
```

#### env module
```python
env.get(key: string, default?: string) -> string | None
env.set(key: string, value: string) -> None
env.list() -> dict[string, string]
```

#### fs module
```python
fs.read(path: string) -> string
fs.write(path: string, content: string) -> bool
fs.exists(path: string) -> bool
fs.glob(pattern: string, ignore?: list[string]) -> list[string]
fs.mkdir(path: string) -> bool
fs.copy(src: string, dst: string) -> bool
fs.remove(path: string) -> bool
fs.cwd() -> string
fs.getcwd() -> string  # Deprecated: use cwd()
```

#### git module
```python
git.glob(ref?: string, pattern?: string, ignore?: list[string]) -> list[string]
git.read(ref_path: string) -> string  # Syntax: "ref:path"
git.read(ref?: string, path: string) -> string  # Keyword syntax
git.diff(target?: string) -> GitDiffResult
git.diff_file(file: string, target?: string) -> string
git.staged_files() -> list[string]
git.modified_files() -> list[string]
git.untracked_files() -> list[string]
git.log(count?: int) -> list[GitLogEntry]
git.status() -> list[string]
git.branch() -> string
git.commit(message: string) -> GitCommitResult
git.add(paths: list[string]) -> GitAddResult
git.push(remote?: string, branch?: string) -> GitPushResult
git.checkout(target: string) -> GitCheckoutResult
git.create_branch(name: string, should_checkout?: bool) -> GitCreateBranchResult
```

**Git Result Types:**
```python
# GitCommitResult
{
  success: bool,      # Always true if no error
  message: string,    # Commit message
  hash: string,       # Commit SHA hash
  output: string      # Git command output
}

# GitAddResult
{
  success: bool,      # Always true if no error
  files_added: list[string],  # Files that were staged
  count: int          # Number of files added
}

# GitPushResult
{
  success: bool,      # Always true if no error
  remote: string,     # Remote name (e.g. "origin")
  branch: string,     # Branch name
  output: string      # Git command output
}

# GitCheckoutResult
{
  success: bool,      # Always true if no error
  target: string,     # Branch/commit checked out
  output: string      # Git command output
}

# GitCreateBranchResult
{
  success: bool,      # Always true if no error
  name: string,       # Branch name
  checked_out: bool   # Whether branch was checked out
}
```

#### llm module
```python
llm.generate(prompt: string, system?: string, preset?: string) -> string
```

#### shell module
```python
shell.exec(command: string) -> ShellResult
```

#### index module
```python
index.search(query: string, snapshots?: list[string], top_k?: int, min_score?: float) -> list[IndexSearchResult]
index.build() -> None
```

#### ui module
```python
ui.info(msg: string) -> None
ui.success(msg: string) -> None
ui.warn(msg: string) -> None
ui.error(msg: string) -> None
ui.think(msg: string) -> None
ui.action(msg: string) -> None

ui.step(title: string, icon?: string) -> UIStepHandle
ui.activity(message: string) -> UIActivityHandle
ui.progress_bar(total: int, message?: string) -> UIProgressBarHandle

ui.code(content: string, lang?: string, title?: string, max_lines?: int) -> None
ui.diff(content: string, title?: string, max_lines?: int) -> None
ui.markdown(content: string, title?: string) -> None
ui.tree(data: dict | list, title?: string) -> None
ui.table(data: list[list], headers?: list[string], title?: string) -> None
ui.properties(data: dict, title?: string) -> None
ui.panel(content: string, title?: string) -> None

ui.prompt(text: string, default?: string, sensitive?: bool, validate?: function) -> string
ui.confirm(prompt: string, default?: bool) -> bool
ui.select(prompt: string, items: list[string], multi?: bool) -> string | list[string]
ui.link(text: string, url: string) -> string
ui.pager(content: string, title?: string, line_numbers?: bool) -> None

ui.divider(style?: string) -> None  # "line", "thick", "dotted", "empty"
ui.banner(text: string) -> None
ui.print(text: string) -> None
```

#### json module
```python
json.encode(value: any) -> string
json.decode(text: string) -> any
```

#### path module
```python
path.join(*parts: string) -> string
path.split(path: string) -> tuple[string, string]  # (dir, file)
path.basename(path: string) -> string
path.dirname(path: string) -> string
path.ext(path: string) -> string
path.abs(path: string) -> string
path.rel(path: string, base?: string) -> string
path.clean(path: string) -> string
path.match(pattern: string, path: string) -> bool
```

#### crypto module
```python
crypto.sha256(data: string) -> string
crypto.md5(data: string) -> string
crypto.base64_encode(data: string) -> string
crypto.base64_decode(data: string) -> string
```

#### time module
```python
time.now() -> int  # Unix timestamp
time.format(timestamp: int, layout?: string) -> string
time.parse(value: string, layout?: string) -> int
time.sleep(seconds: float) -> None
```

#### regexp module
```python
regexp.match(pattern: string, text: string) -> bool
regexp.find(pattern: string, text: string) -> string | None
regexp.find_all(pattern: string, text: string) -> list[string]
regexp.replace(pattern: string, repl: string, text: string) -> string
```

#### stdin module
```python
stdin.read() -> string
stdin.is_piped() -> bool
```

#### output module
```python
output.print(text: string) -> None
output.error(text: string) -> None
```

---

## Cookbook: Real-World Patterns

This section provides practical examples demonstrating common workflows and best practices.

### 1. Working with Git

#### Generate Commit Message from Staged Changes

```python
def generate_commit():
    """Generate conventional commit message from staged changes."""
    # Get staged changes
    diff = ctx.git.diff(target="staged")
    
    if len(diff.files) == 0:
        ctx.ui.error("No staged changes found")
        return
    
    # Determine commit type based on files
    commit_types = ["feat", "fix", "docs", "refactor", "test", "chore"]
    commit_type = ctx.ui.select(
        "Select commit type:",
        commit_types
    )
    
    # Generate summary with LLM
    prompt = f"""Analyze this git diff and write a one-line commit summary (max 50 chars):

{diff.raw}

Just return the summary, nothing else."""
    
    summary = ctx.llm.generate(prompt, preset="fast").strip()
    
    # Generate detailed body
    body_prompt = f"""Analyze this diff and list the key changes as bullet points:

{diff.raw}

Format as:
- Change 1
- Change 2
etc."""
    
    body = ctx.llm.generate(body_prompt, preset="fast").strip()
    
    # Compose message
    message = f"{commit_type}: {summary}\n\n{body}"
    
    # Show preview
    ctx.ui.code(message, lang="markdown", title="Commit Message")
    
    if ctx.ui.confirm("Use this message?", default=True):
        result = ctx.git.commit(message)
        ctx.ui.success(f"Committed successfully! Hash: {result.hash[:7]}")
```

#### Create PR Description from Branch Diff

```python
def generate_pr_description():
    """Generate PR description from branch changes."""
    current_branch = ctx.git.branch()
    base_branch = "main"  # or "dev"
    
    # Get all commits in branch
    logs = ctx.git.log(count=100)
    
    # Find where branch diverged (simplified)
    # In practice, use: git merge-base
    branch_commits = []
    for log in logs:
        if "Merge" in log.message:
            break
        branch_commits.append(log)
    
    # Get full diff from base
    diff = ctx.git.diff(target=base_branch)
    
    # Generate PR description
    prompt = f"""Create a Pull Request description from these changes:

Branch: {current_branch} -> {base_branch}
Commits ({len(branch_commits)}):
{chr(10).join([f"- {c.message}" for c in branch_commits])}

Changed files ({len(diff.files)}):
{chr(10).join([f"- {f}" for f in diff.files[:10]])}

Diff summary:
+{diff.additions} -{diff.deletions}

Generate a PR description with:
1. ## Summary (2-3 sentences)
2. ## Changes (bullet list)
3. ## Testing (how to test)
"""
    
    description = ctx.llm.generate(prompt, preset="smart")
    
    ctx.ui.markdown(description, title="PR Description")
    
    return description
```

#### Find Files in Git History

```python
def find_deleted_file():
    """Search for a file that was deleted in git history."""
    filename = ctx.ui.prompt("Enter filename to search:")
    
    # Search recent commits
    found_in = []
    for i in range(50):  # Check last 50 commits
        ref = f"HEAD~{i}"
        try:
            files = ctx.git.glob(ref=ref, pattern=f"**/{filename}")
            if files:
                log = ctx.git.log(count=1)
                found_in.append((ref, log[0].message if log else ""))
        except:
            break
    
    if not found_in:
        ctx.ui.error(f"File '{filename}' not found in recent history")
        return
    
    ctx.ui.success(f"Found in {len(found_in)} commits:")
    for ref, msg in found_in[:10]:
        ctx.ui.info(f"{ref}: {msg[:50]}")
    
    # Let user select which version to restore
    if ctx.ui.confirm("Restore file?"):
        ref = found_in[0][0]
        content = ctx.git.read(f"{ref}:{filename}")
        ctx.fs.write(filename, content)
        ctx.ui.success(f"Restored {filename} from {ref}")
```

### 2. LLM Workflows

#### Multi-Turn Conversation with Context

```python
def chat_about_code():
    """Interactive code discussion with context."""
    # Load relevant files
    files = ctx.ui.select(
        "Select files for context:",
        ctx.fs.glob("**/*.go"),
        multi=True
    )
    
    context = ""
    for file in files:
        content = ctx.fs.read(file)
        context += f"\n\n## {file}\n```go\n{content}\n```"
    
    # Conversation loop
    conversation = []
    
    while True:
        question = ctx.ui.prompt("Ask about the code (empty to quit):")
        if not question:
            break
        
        # Build prompt with full conversation history
        prompt = f"""Code context:
{context}

Conversation history:
{chr(10).join([f"Q: {q}\nA: {a}" for q, a in conversation])}

New question: {question}

Answer concisely, reference specific code when relevant."""
        
        answer = ctx.llm.generate(prompt, preset="smart")
        
        ctx.ui.markdown(answer)
        conversation.append((question, answer))
```

#### Fallback Between Models

```python
def generate_with_fallback(prompt):
    """Try multiple models with fallback on failure."""
    presets = ["fast", "smart", "creative"]
    
    for preset in presets:
        try:
            ctx.ui.info(f"Trying preset: {preset}")
            result = ctx.llm.generate(prompt, preset=preset)
            ctx.ui.success(f"Success with {preset}")
            return result
        except Exception as e:
            ctx.ui.warn(f"{preset} failed: {str(e)}")
            continue
    
    ctx.ui.error("All models failed")
    return None
```

### 3. UI Patterns

#### Progress Bar for Long Operations

```python
def process_files_with_progress():
    """Process files with visual progress."""
    files = ctx.fs.glob("**/*.go")
    
    step = ctx.ui.step(f"Processing {len(files)} files")
    pb = ctx.ui.progress_bar(len(files), message="Processing...")
    
    results = []
    for file in files:
        ctx.ui.info(f"Processing: {file}")
        
        # Simulate work
        content = ctx.fs.read(file)
        # ... do something ...
        
        pb.inc()
        results.append(file)
    
    pb.done("Complete!")
    step.done(f"Processed {len(results)} files")
```

#### Confirmation with Validation

```python
def safe_delete_files():
    """Delete files with confirmation and validation."""
    files = ctx.ui.select(
        "Select files to delete:",
        ctx.fs.glob("**/*.tmp"),
        multi=True
    )
    
    if not files:
        ctx.ui.warn("No files selected")
        return
    
    # Show what will be deleted
    ctx.ui.warn("Files to delete:")
    for f in files:
        ctx.ui.info(f"  - {f}")
    
    # Require exact confirmation
    def validate_confirmation(value):
        if value != "DELETE":
            return "Type DELETE to confirm"
        return None
    
    confirmation = ctx.ui.prompt(
        "Type DELETE to confirm:",
        validate=validate_confirmation
    )
    
    # Delete with progress
    pb = ctx.ui.progress_bar(len(files), message="Deleting...")
    for file in files:
        ctx.fs.remove(file)
        pb.inc()
    
    pb.done("All files deleted")
    ctx.ui.success(f"Deleted {len(files)} files")
```

### 4. File Operations

#### Recursive File Processing with Filters

```python
def analyze_codebase():
    """Analyze Go codebase with file filtering."""
    # Get all Go files, exclude tests and vendor
    files = ctx.fs.glob(
        "**/*.go",
        ignore=["**/*_test.go", "vendor/**"]
    )
    
    stats = {
        "total_files": len(files),
        "total_lines": 0,
        "total_functions": 0,
        "largest_file": ("", 0)
    }
    
    for file in files:
        content = ctx.fs.read(file)
        lines = len(content.split("\n"))
        stats["total_lines"] += lines
        
        # Count functions (simplified)
        funcs = content.count("func ")
        stats["total_functions"] += funcs
        
        # Track largest file
        if lines > stats["largest_file"][1]:
            stats["largest_file"] = (file, lines)
    
    # Display results
    ctx.ui.divider("thick")
    ctx.ui.info("Codebase Analysis")
    ctx.ui.divider("line")
    ctx.ui.properties(stats)
    ctx.ui.divider("thick")
```

#### Safe File Modification with Backup

```python
def safe_modify_file(filepath, modifier_fn):
    """Modify file with automatic backup."""
    # Read original
    original = ctx.fs.read(filepath)
    
    # Create backup
    backup_path = f"{filepath}.backup"
    ctx.fs.write(backup_path, original)
    ctx.ui.info(f"Backup created: {backup_path}")
    
    try:
        # Apply modification
        modified = modifier_fn(original)
        
        # Show diff
        diff = ctx.shell.exec(f"diff -u {filepath} -").stdout
        ctx.ui.diff(diff, title="Changes")
        
        if ctx.ui.confirm("Apply changes?"):
            ctx.fs.write(filepath, modified)
            ctx.fs.remove(backup_path)
            ctx.ui.success("Changes applied")
        else:
            ctx.ui.info("Changes discarded")
    except Exception as e:
        # Restore from backup on error
        ctx.fs.write(filepath, original)
        ctx.fs.remove(backup_path)
        ctx.ui.error(f"Error, restored from backup: {str(e)}")
```

### 5. RAG and Code Search

#### Semantic Code Search with Context

```python
def search_and_explain(query):
    """Search code and explain findings."""
    ctx.ui.info(f"Searching for: {query}")
    
    # Search with RAG
    results = ctx.index.search(
        query,
        top_k=5,
        min_score=0.7
    )
    
    if not results:
        ctx.ui.warn("No results found")
        return
    
    # Show results
    ctx.ui.success(f"Found {len(results)} matches:")
    for i, result in enumerate(results):
        ctx.ui.info(f"\n{i+1}. {result.file_path} (score: {result.score:.2f})")
        ctx.ui.code(
            result.content,
            lang="go",
            max_lines=10
        )
    
    # Generate explanation
    context = "\n\n".join([
        f"File: {r.file_path}\n{r.content}"
        for r in results
    ])
    
    prompt = f"""Based on this code:

{context}

Explain how '{query}' is implemented in this codebase."""
    
    explanation = ctx.llm.generate(prompt, preset="smart")
    ctx.ui.markdown(explanation, title="Explanation")
```

#### Build Index and Search

```python
def index_and_search():
    """Build code index and perform semantic search."""
    # Check if index needs rebuild
    if ctx.ui.confirm("Rebuild index?", default=False):
        step = ctx.ui.step("Building search index")
        ctx.index.build()
        step.done("Index built")
    
    # Interactive search loop
    while True:
        query = ctx.ui.prompt("Search query (empty to quit):")
        if not query:
            break
        
        results = ctx.index.search(query, top_k=3)
        
        for result in results:
            ctx.ui.properties({
                "File": result.file_path,
                "Lines": f"{result.start_line}-{result.end_line}",
                "Score": f"{result.score:.2f}"
            })
            ctx.ui.code(result.content, lang="go")
```

---

## Summary

The meowg1k UI module provides 25 widget functions for building modern CLI interfaces with hierarchical contexts, rich content display, and interactive input capabilities.
