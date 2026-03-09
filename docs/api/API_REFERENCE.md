# meowg1k Complete API Reference

## Quick Navigation

- [Starlark API](#global-environment) - Scripting, tools, context
- [Standard Modules](#standard-modules) - fs, git, llm, shell, index, ui, etc.
- [Type Reference](#type-reference) - Complete type signatures  
- [Cookbook](#cookbook-real-world-patterns) - Real-world examples
- [Complete Example](#complete-example) - Full code review workflow

---

The meowg1k Starlark runtime provides a rich set of modules for configuration, automation, and AI workflows.
This document details the available APIs.

## Table of Contents

- [Starlark API Reference](#global-environment)
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
    - [yaml (YAML Handling)](#yaml-yaml-handling)
    - [xml (XML Handling)](#xml-xml-handling)
    - [toml (TOML Handling)](#toml-toml-handling)
    - [csv (CSV Handling)](#csv-csv-handling)
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

The Unified Tool System allows you to define tools with inputs defined as `meow.param`. These tools can be
automatically registered as CLI commands, with arguments and flags parsed and injected directly into the handler
context.

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
    ctx.output.writeline("Hello " + ctx.name)

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

#### Static Constraints

- `choices`: Limit to specific values.
- `pattern`: Regex validation.
- `min`/`max`: Numeric range.
- `min_len`/`max_len`: String length.

**Custom Validators**
You can pass a function or tool to `validator`. The validator receives a `ctx` where `ctx.value` is the input
parameter. Return `True` (pass), `False` (fail), or an error string.

**Example:**

```python
def validate_even(ctx):
    if ctx.value % 2 != 0:
        return "Must be an even number"
    return True

tool_with_validation = meow.tool(
    name="even_checker",
    handler=lambda ctx: ctx.output.writeline("Valid number!"),
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

- `command_or_tool` (string or ToolValue): Name of command to run, or a tool object
  (from `meow.tool()` or loaded from a library).
- `**kwargs`: Arguments to pass to the command (overriding defaults).
- **Returns:** The return value of the called handler on success.
- **Error Handling:** If the called handler raises an error (via `fail()` or exception), execution stops
  immediately. In Starlark, `fail()` terminates the entire script, so errors propagate naturally.

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
- **Output**: Use `output.write` or `output.writeline` for machine-readable output,
  `ctx.ui.assistant_turn()` for user-facing responses.
- **Error Handling**: Starlark has no exceptions. Operations that fail will stop execution with an error message.
  All error messages follow the pattern: `"operation failed for 'context': details"` to provide clear,
  actionable information.

### Error Handling

All meowg1k functions provide clear, contextual error messages:

```python
# Error messages include context
result = ctx.fs.read("missing.txt")
# Error: "failed to read file 'missing.txt': no such file or directory"

result = ctx.git.commit("message")
# Error: "git commit failed: nothing to commit, working tree clean"

result = ctx.llm.chat(prompt="prompt", preset="invalid")
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
- Use `output.writeline()` to log progress and debug execution flow

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

**`fs.getcwd()`** -> `string` _(Deprecated: use `cwd()`)_
Alias for `cwd()`. Deprecated — use `fs.cwd()` instead.

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

**`git.diff_file(file, target="staged")`** -> `struct`
Get diff for a specific file.

- `file`: Path to the file (required).
- `target`: "staged" (default), "HEAD", or "commit-hash".
- Returns: `{raw, file, additions, deletions}`.

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
    ctx.output.writeline("Processing: " + file)
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
ctx.output.writeline(f"Committed: {result.hash[:7]}")
ctx.output.writeline(f"Message: {result.message}")

# Add files and check count
result = ctx.git.add(["main.go", "README.md"])
ctx.output.writeline(f"Staged {result.count} files")
for file in result.files_added:
    ctx.output.writeline(f"  • {file}")

# Push with details
result = ctx.git.push()
ctx.output.writeline(f"Pushed to {result.remote}/{result.branch}")

# Checkout with confirmation
result = ctx.git.checkout("main")
ctx.output.writeline(f"Switched to: {result.target}")

# Create branch with conditional checkout
result = ctx.git.create_branch("feature/new", should_checkout=True)
if result.checked_out:
    ctx.output.writeline(f"Created and switched to: {result.name}")
else:
    ctx.output.writeline(f"Created branch: {result.name}")
```

---

### llm (Language Models)

> **v0.3.0 Breaking Change**: `llm.generate()` and `llm.agentic()` have been removed.
> Use `llm.chat()` and `llm.agent_turn()` respectively.

**`llm.chat(prompt, preset, system=None, use_session=True, stream=False, on_event=None,`**
**`response_format=None, response_schema=None)`** -> `string | dict`

Generate a single LLM response. Returns a string by default, or a parsed dict when `response_format="json_object"`.

- `prompt` (required): The user prompt.
- `preset` (required): Name of the preset to use. No default — must be provided.
- `system`: System instruction (optional).
- `use_session`: If `True` (default), previous messages from the current session are included in the context.
- `stream`: If `True`, call `GenerateContentStream` on the gateway and emit events via `on_event`.
- `on_event`: Callable invoked with each stream event dict (see Stream Events below).
  Required when `stream=True` to observe deltas; optional otherwise.
- `response_format`: `"json_object"` to parse the response as JSON and return a dict.
- `response_schema`: Reserved for structured output schemas (future use).

**Example:**

```python
def handler(ctx):
    # Non-streaming
    result = ctx.llm.chat(
        prompt="Explain Go interfaces",
        preset="smart",
        system="You are a Go expert.",
    )
    ctx.output.markdown(result)

    # Streaming with markdown rendering
    load("//lib/ui_helpers.star", "make_markdown_stream_handler")
    result = ctx.llm.chat(
        prompt="Write a detailed explanation",
        preset="smart",
        stream=True,
        on_event=make_markdown_stream_handler(ctx),
    )

    # JSON output
    data = ctx.llm.chat(
        prompt="Extract name and age from: Alice is 30 years old",
        preset="fast",
        response_format="json_object",
    )
    ctx.output.writeline("Name: " + data["name"])
```

---

**`llm.agent_turn(prompt, preset, tools, system=None, use_session=True, stream=False, on_event=None,`**
**`max_iterations=50, on_tool_error="return", response_format=None, response_schema=None)`**
**`-> string | dict`**

Run one turn of an agentic loop with native tool calling support. The LLM can call tools, receive results,
and continue processing until it provides a final text answer or `max_iterations` is reached.

- `prompt` (required): The user prompt.
- `preset` (required): Name of the preset to use. No default — must be provided.
- `tools` (required): List of tool objects created with `meow.tool()`.
- `system`: System instruction (optional).
- `use_session`: If `True` (default), previous session messages are included.
- `stream`: If `True`, stream events are emitted via `on_event`.
- `on_event`: Callable for stream events (tool call progress, text deltas, etc.).
- `max_iterations`: Maximum LLM→tool→LLM cycles before returning (default: 50).
- `on_tool_error`: Error handling strategy:
  - `"return"` (default): Return error as tool result so the LLM can see it.
  - `"abort"`: Stop immediately and return an error.
- `response_format`: `"json_object"` to parse final response as JSON.
- `response_schema`: Reserved for structured output schemas (future use).

**Example:**

```python
load("//lib/tools.star", "calculator", "file_reader")
load("//lib/ui_helpers.star", "make_agentic_stream_handler")

def handler(ctx):
    result = ctx.llm.agent_turn(
        prompt="Read config.json and calculate the sum of all numeric values",
        preset="smart",
        tools=[calculator, file_reader],
        system="You are a helpful assistant with access to file and calculation tools.",
        max_iterations=10,
        stream=True,
        on_event=make_agentic_stream_handler(ctx),
    )
    ctx.output.writeline(result)
```

---

#### Stream Events

When `stream=True`, the `on_event` callback receives dicts with the following shapes:

| `kind`            | Additional fields                                                         |
| ----------------- | ------------------------------------------------------------------------- |
| `text`            | `delta` (str) — incremental text token                                    |
| `thinking`        | `delta` (str) — reasoning token (Anthropic extended thinking, etc.)       |
| `usage`           | `usage` dict: `{prompt, completion, total}`                               |
| `done`            | `usage` dict (optional) — signals stream completion                       |
| `error`           | `error` (str), `recoverable` (bool)                                       |
| `tool_call_start` | `tool_name`, `tool_id`, `arguments` (dict)                                |
| `tool_call_end`   | `tool_name`, `tool_id`, `duration_ms` (int), `arguments` (dict)           |
| `tool_call_error` | `tool_name`, `tool_id`, `error` (str), `duration_ms` (int), `arguments`   |

Stream events are **ephemeral** — they are not stored in the session database. Only the final aggregated messages are persisted.

Use `//lib/ui_helpers.star` for pre-built handlers:

- `make_markdown_stream_handler(ctx)` — renders text deltas as markdown
- `make_plain_stream_handler(ctx)` — writes text deltas as plain text
- `make_agentic_stream_handler(ctx, abort_on_error, max_errors)` — full agent handler

---

**`llm.embed(texts, preset)`** -> `list[list[float]]`
Generate embeddings for a list of texts.

- `texts` (required): List of strings to embed.
- `preset` (required): Name of the embeddings preset. No default — must be provided.
- Returns: List of embedding vectors (each a list of floats).

---

### session (Session Management)

The session module provides access to the current execution session and its metadata. Sessions track the complete
history of tool invocations, LLM interactions, and results.

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
Returns list of `{id, tool_name, status, parent_id}`.

**`session.set_system(prompt)`** -> `None`
Store a system prompt for the current session. This prompt is used by `ctx.llm.chat()` and
`ctx.llm.agent_turn()` when no explicit `system` parameter is provided.

**`session.get_system()`** -> `string | None`
Retrieve the system prompt stored for the current session. Returns `None` if none has been set.

**`session.get_events(limit=100, offset=0)`** -> `list[dict]`
Get events (messages, tool calls, results) for this session.
Returns list of `{id, type, content, tool_call_id}`.

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
    ctx.output.writeline("Session: " + ctx.session.id())
    ctx.output.writeline("Tool: " + ctx.session.tool_name())
    
    # Store some metadata
    ctx.session.set_metadata("user_id", "123")
    
    # Call another tool (creates child session)
    ctx.run("analyze-code", path="main.go")
    
    # Check child sessions
    children = ctx.session.get_children()
    ctx.output.writeline("Created " + str(len(children)) + " child sessions")
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

The `ui` module provides terminal output widgets and interactive components. All functions are accessed through
`ctx.ui` in handler functions.

#### Conversation-Style Output

**`ui.user_turn(text)`**
Display a user message turn.

**`ui.assistant_turn()`** -> `TurnHandle`
Begin an assistant response turn. Returns a handle for managing output:

- `.step(text)` -> `StepHandle`: Add a progress step. Returns handle with:
  - `.done(text=None)`: Mark step complete
  - `.fail(text=None)`: Mark step failed
  - `.info(text)`: Add info to step
  - `.update(text)`: Update step text
- `.stream(delta, done=False)`: Stream a text delta
- `.done(summary=None)`: Complete the turn
- `.fail(summary=None)`: Fail the turn
- `.info(text)`: Add an info line
- `.warn(text)`: Add a warning line
- `.subturn(label)` -> `SubTurnHandle`: Create a sub-turn with:
  - `.step(text)` -> `StepHandle`
  - `.stream(delta, done=False)`
  - `.done(summary=None)`
  - `.fail(summary=None)`

#### Progress Indicators

**`ui.progress_bar(total, message="")`** -> `ProgressBarHandle`
Visual progress bar for batch operations. Returns handle with:

- `.inc(amount=1)`: Increment progress
- `.set(value)`: Set absolute value
- `.done(message=None)`: Finish

**`ui.progress(message, current=None, total=None)`**
Simple single-line progress text.

#### Rich Content Display

**`ui.code(content, lang="text", title=None, max_lines=0)`**
Display code with syntax highlighting (100+ languages via Chroma). Use `max_lines` to truncate.

**`ui.diff(content, title=None, max_lines=0)`**
Display git diff with colored +/- and borders. Use `max_lines` to truncate.

**`ui.tree(data, title=None)`**
Display hierarchical data as a tree with ├── └── branches.

**`ui.table(data, columns, title=None, query=None)`**
Display data in a formatted table. `columns` is **required**.

**`ui.markdown(content)`**
Render and display a markdown string.

**`ui.panel(content, title=None, style=None)`**
Display content in a bordered panel.

**`ui.banner(title, subtext=None)`**
Display a prominent title banner with borders.

**`ui.render(value, query=None)`**
Auto-render a Starlark value: strings render as markdown, lists render as a table, diffs render as diff.
`query` is used for table filtering.

**`ui.link(text, url)`** -> `string`
Create a clickable hyperlink (OSC 8 terminal escape). Falls back to `"text (url)"` in unsupported terminals.

**`ui.pager(content, title=None, show_line_numbers=False)`**
Display content in an interactive Bubble Tea viewport for scrolling through large text.

```python
ctx.ui.code(code, lang="go", title="main.go", max_lines=20)
ctx.ui.diff(diff_text, title="patch.diff", max_lines=10)
ctx.ui.tree({"dir": {"file": "value"}})
ctx.ui.table(rows, columns=["Name", "Age"], title="Users")
link = ctx.ui.link("GitHub", "https://github.com/org/repo")
ctx.ui.pager(large_log, title="Build Log")
```

#### User Interaction

**`ui.prompt(message, default="", is_sensitive=False, validate=None)`** -> `string`
Ask the user for text input with optional masking and validation.

```python
name = ctx.ui.prompt("Enter name:")
api_key = ctx.ui.prompt("API Key:", is_sensitive=True)

def validate_port(val):
    if not val.isdigit():
        return "Must be a number"
    return None

port = ctx.ui.prompt("Enter port:", default="8080", validate=validate_port)
```

**`ui.confirm(prompt, default=False)`** -> `bool`
Ask for Y/n confirmation.

**`ui.select(prompt, items, allow_multiple=False, is_fuzzy=False, limit=0, placeholder=None,`**
**`initial_query=None, allow_new=False, should_return_index=False, label_key=None,`**
**`value_key=None, meta_key=None)`** -> `string | list`
Interactive fuzzy selection menu. Multi-select supported.

```python
# Single select
file = ctx.ui.select("Pick file:", ["a.go", "b.go"])

# Multi-select (use Space to toggle)
files = ctx.ui.select("Pick files:", ["a.go", "b.go", "c.go"], allow_multiple=True)
# Returns: ["a.go", "c.go"]

if ctx.ui.confirm("Continue?", default=True):
    pass  # proceed
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

**`xml.parse(string)`** -> `any`
Parse XML string into Starlark values using the mxj library. Supports elements, attributes, text content,
and nested structures.

- Attributes are prefixed with `-` (e.g., `-id`, `-name`)
- Text content is stored with `#text` key for elements with attributes or mixed content
- Simple text-only elements return just the text value as a string
- Multiple child elements with the same name are returned as a list

**`xml.stringify(value, indent=False, root="")`** -> `string`
Convert Starlark value to XML string.

- `indent` (bool): Enable pretty-printing with 2-space indentation
- `root` (string): Root element name (required if value is not a single-key dict)

**Example:**

```python
# Parse XML with attributes and nested elements
xml_str = '''
<user id="123" role="admin">
  <name>Alice</name>
  <age>30</age>
</user>
'''
data = ctx.xml.parse(xml_str)
# Result: {"user": {"-id": "123", "-role": "admin", "name": "Alice", "age": "30"}}

# Access parsed data
user = data["user"]
print(user["-id"])      # "123" (note the - prefix for attributes)
print(user["name"])     # "Alice"

# Generate XML with indentation
output = ctx.xml.stringify(
    {"name": "Alice", "age": 30},
    indent=True,
    root="user"
)
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
    ctx.output.writeline("Stars: " + str(repo["stargazers_count"]))

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
    ctx.output.writeline("Logged in as: " + user)

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
    ctx.output.writeline("Request failed with status: " + str(response.status_code))
    return

# Or check specific status codes
if response.status_code == 404:
    ctx.output.writeline("Resource not found")
elif response.status_code >= 500:
    ctx.output.writeline("Server error occurred")
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
ctx.output.writeline(result)  # "Hello Alice, you are 30 years old"

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
    ctx.output.writeline("Template parse error")

try:
    tmpl = ctx.template.parse("{{.User.Name}}")
    result = tmpl.render({"User": None})  # Runtime error
except:
    ctx.output.writeline("Template render error")
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
        output.writeline("No staged changes found.")
        return

    # 2. Analyze with LLM
    turn = ctx.ui.assistant_turn()
    step = turn.step("Analyzing changes...")
    prompt = "Review these changes:\n" + diff.raw
    review = ctx.llm.chat(prompt=prompt, preset="coding")
    step.done()

    # 3. Output result
    ctx.output.writeline(review)

    # 4. Ask to commit
    if ctx.ui.confirm("Commit with this review?"):
        result = ctx.git.commit(message="refactor: " + review[:50])
        ctx.output.writeline("Committed as: " + result.hash[:7])

review_tool = meow.tool(
    name="review",
    handler=review_handler,
    description="Analyze staged changes"
)

meow.command(review_tool)
```

---

## Type Reference

This section provides explicit type signatures for all Starlark functions. While Starlark is dynamically
typed, understanding parameter and return types helps prevent errors.

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
    .info(text: string) -> None,      # Add info line
    .update(text: string) -> None     # Update step text
}
```

**UITurnHandle**: `object`

```python
{
    .step(text: string) -> UIStepHandle,     # Add progress step
    .stream(delta: string, done?: bool) -> None,
    .done(summary?: string) -> None,
    .fail(summary?: string) -> None,
    .info(text: string) -> None,
    .warn(text: string) -> None,
    .subturn(label: string) -> UISubTurnHandle
}
```

**UISubTurnHandle**: `object`

```python
{
    .step(text: string) -> UIStepHandle,
    .stream(delta: string, done?: bool) -> None,
    .done(summary?: string) -> None,
    .fail(summary?: string) -> None
}
```

**UIProgressBarHandle**: `object`

```python
{
    .inc(amount?: int) -> None,       # Increment progress
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
meow.tool(name: string, handler: function, params?: dict, description?: string) -> ToolValue
meow.param(type: string, **kwargs) -> ParamValue
meow.command(tool: ToolValue, name?: string) -> None
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
fs.filter(dir: string, pattern?: string, recursive?: bool) -> list[string]
fs.walk(root: string, pattern?: string) -> list[string]
fs.stat(path: string) -> struct{size: int, mtime: int, is_dir: bool, mode: int}
fs.listdir(path: string) -> list[string]
fs.chmod(path: string, mode: int) -> bool
fs.touch(path: string, mtime?: int) -> bool
```

#### git module

```python
git.glob(ref?: string, pattern?: string, ignore?: list[string]) -> list[string]
git.read(ref_path: string) -> string  # Syntax: "ref:path"
git.read(ref?: string, path: string) -> string  # Keyword syntax
git.diff(target?: string) -> GitDiffResult
git.diff_file(file: string, target?: string) -> struct{raw: string, file: string, additions: int, deletions: int}
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
llm.chat(prompt: string, preset: string, system?: string, use_session?: bool,
         stream?: bool, on_event?: callable, response_format?: string,
         response_schema?: any) -> string | dict

llm.agent_turn(prompt: string, preset: string, tools: list,
               system?: string, use_session?: bool, stream?: bool,
               on_event?: callable, max_iterations?: int,
               on_tool_error?: string, response_format?: string,
               response_schema?: any) -> string | dict

llm.embed(texts: list[string], preset: string) -> list[list[float]]
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
ui.user_turn(text: string) -> None
ui.assistant_turn() -> UITurnHandle

ui.progress_bar(total: int, message?: string) -> UIProgressBarHandle
ui.progress(message: string, current?: int, total?: int) -> None

ui.code(content: string, lang?: string, title?: string, max_lines?: int) -> None
ui.diff(content: string, title?: string, max_lines?: int) -> None
ui.markdown(content: string) -> None
ui.tree(data: dict | list, title?: string) -> None
ui.table(data: list, columns: list[string], title?: string, query?: string) -> None
ui.panel(content: string, title?: string, style?: string) -> None
ui.banner(title: string, subtext?: string) -> None
ui.render(value: any, query?: string) -> None
ui.link(text: string, url: string) -> string
ui.pager(content: string, title?: string, show_line_numbers?: bool) -> None

ui.prompt(message: string, default?: string, is_sensitive?: bool, validate?: function) -> string
ui.confirm(prompt: string, default?: bool) -> bool
ui.select(prompt: string, items: list, allow_multiple?: bool, is_fuzzy?: bool,
          limit?: int, placeholder?: string, initial_query?: string,
          allow_new?: bool, should_return_index?: bool,
          label_key?: string, value_key?: string, meta_key?: string) -> string | list[string]
```

#### json module

```python
json.parse(text: string) -> any
json.stringify(value: any, indent?: int) -> string
```

#### path module

```python
path.join(*parts: string) -> string
path.basename(path: string) -> string
path.dirname(path: string) -> string
path.ext(path: string) -> string
path.extension(path: string) -> string  # alias for ext
path.abs(path: string) -> string
path.rel(base: string, target: string) -> string
path.clean(path: string) -> string
path.stem(path: string) -> string
path.parent(path: string) -> string  # alias for dirname
path.parts(path: string) -> list[string]
```

#### crypto module

```python
crypto.sha256(data: string) -> string  # hex-encoded
crypto.md5(data: string) -> string     # hex-encoded
crypto.hmac(key: string, data: string) -> string  # SHA256 hex-encoded
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
regexp.find_all(pattern: string, text: string, limit?: int) -> list[string]
regexp.replace(pattern: string, text: string, replacement: string) -> string
regexp.split(pattern: string, text: string, limit?: int) -> list[string]
```

#### stdin module

```python
stdin.read() -> string
stdin.read_line() -> string
stdin.is_piped() -> bool
```

#### output module

```python
output.write(content: string) -> None
output.writeline(content: string) -> None
output.writef(format: string, *args) -> None
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
        ctx.output.writeline("No staged changes found")
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
    
    summary = ctx.llm.chat(prompt=prompt, preset="fast").strip()
    
    # Generate detailed body
    body_prompt = f"""Analyze this diff and list the key changes as bullet points:

{diff.raw}

Format as:
- Change 1
- Change 2
etc."""
    
    body = ctx.llm.chat(prompt=body_prompt, preset="fast").strip()
    
    # Compose message
    message = f"{commit_type}: {summary}\n\n{body}"
    
    # Show preview
    ctx.ui.code(message, lang="markdown", title="Commit Message")
    
    if ctx.ui.confirm("Use this message?", default=True):
        result = ctx.git.commit(message)
        ctx.output.writeline("Committed successfully! Hash: " + result.hash[:7])
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
    
    description = ctx.llm.chat(prompt=prompt, preset="smart")
    
    ctx.ui.markdown(description)
    
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
        ctx.output.writeline(f"File '{filename}' not found in recent history")
        return
    
    ctx.output.writeline(f"Found in {len(found_in)} commits:")
    for ref, msg in found_in[:10]:
        ctx.output.writeline(f"{ref}: {msg[:50]}")
    
    # Let user select which version to restore
    if ctx.ui.confirm("Restore file?"):
        ref = found_in[0][0]
        content = ctx.git.read(f"{ref}:{filename}")
        ctx.fs.write(filename, content)
        ctx.output.writeline(f"Restored {filename} from {ref}")
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
        allow_multiple=True
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
        
        answer = ctx.llm.chat(prompt=prompt, preset="smart")
        
        ctx.ui.markdown(answer)
        conversation.append((question, answer))
```

### 3. UI Patterns

#### Progress Bar for Long Operations

```python
def process_files_with_progress():
    """Process files with visual progress."""
    files = ctx.fs.glob("**/*.go")
    
    turn = ctx.ui.assistant_turn()
    step = turn.step(f"Processing {len(files)} files")
    pb = ctx.ui.progress_bar(len(files), message="Processing...")
    
    results = []
    for file in files:
        # Simulate work
        content = ctx.fs.read(file)
        # ... do something ...
        
        pb.inc()
        results.append(file)
    
    pb.done("Complete!")
    step.done(f"Processed {len(results)} files")
    turn.done()
```

#### Confirmation with Validation

```python
def safe_delete_files():
    """Delete files with confirmation and validation."""
    files = ctx.ui.select(
        "Select files to delete:",
        ctx.fs.glob("**/*.tmp"),
        allow_multiple=True
    )
    
    if not files:
        ctx.output.writeline("No files selected")
        return
    
    # Show what will be deleted
    ctx.output.writeline("Files to delete:")
    for f in files:
        ctx.output.writeline(f"  - {f}")
    
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
    ctx.output.writeline(f"Deleted {len(files)} files")
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
    ctx.ui.banner("Codebase Analysis")
    ctx.ui.table(
        [
            ["Total Files", str(stats["total_files"])],
            ["Total Lines", str(stats["total_lines"])],
            ["Total Functions", str(stats["total_functions"])],
            ["Largest File", stats["largest_file"][0] + " (" + str(stats["largest_file"][1]) + " lines)"],
        ],
        columns=["Metric", "Value"]
    )
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
    ctx.output.writeline(f"Backup created: {backup_path}")
    
    try:
        # Apply modification
        modified = modifier_fn(original)
        
        # Show diff
        diff = ctx.shell.exec(f"diff -u {filepath} -").stdout
        ctx.ui.diff(diff, title="Changes")
        
        if ctx.ui.confirm("Apply changes?"):
            ctx.fs.write(filepath, modified)
            ctx.fs.remove(backup_path)
            ctx.output.writeline("Changes applied")
        else:
            ctx.output.writeline("Changes discarded")
    except Exception as e:
        # Restore from backup on error
        ctx.fs.write(filepath, original)
        ctx.fs.remove(backup_path)
        ctx.output.writeline(f"Error, restored from backup: {str(e)}")
```

### 5. RAG and Code Search

#### Semantic Code Search with Context

```python
def search_and_explain(query):
    """Search code and explain findings."""
    ctx.output.writeline(f"Searching for: {query}")
    
    # Search with RAG
    results = ctx.index.search(
        query,
        top_k=5,
        min_score=0.7
    )
    
    if not results:
        ctx.output.writeline("No results found")
        return
    
    # Show results
    ctx.output.writeline(f"Found {len(results)} matches:")
    for i, result in enumerate(results):
        ctx.output.writeline(f"\n{i+1}. {result.file_path} (score: {result.score:.2f})")
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
    
    explanation = ctx.llm.chat(prompt=prompt, preset="smart")
    ctx.ui.markdown(explanation)
```

#### Build Index and Search

```python
def index_and_search():
    """Build code index and perform semantic search."""
    # Check if index needs rebuild
    if ctx.ui.confirm("Rebuild index?", default=False):
        turn = ctx.ui.assistant_turn()
        step = turn.step("Building search index")
        ctx.index.build()
        step.done("Index built")
        turn.done()
    
    # Interactive search loop
    while True:
        query = ctx.ui.prompt("Search query (empty to quit):")
        if not query:
            break
        
        results = ctx.index.search(query, top_k=3)
        
        for result in results:
            ctx.ui.table(
                [
                    ["File", result.file_path],
                    ["Lines", f"{result.start_line}-{result.end_line}"],
                    ["Score", f"{result.score:.2f}"],
                ],
                columns=["Key", "Value"]
            )
            ctx.ui.code(result.content, lang="go")
```

---
