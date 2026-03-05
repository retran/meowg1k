# Starlark Extension System Guide

This document provides a comprehensive guide to meowg1k's Starlark-based extension system, which allows users to define custom commands, configure providers/models, and extend functionality through scripting.

## Overview

meowg1k uses **Starlark** (a Python-like language) for configuration and extensibility. Starlark scripts live in `.meowg1k/` and provide:

- **Configuration**: Define providers, models, and presets
- **Custom Commands**: Create new CLI commands with AI workflows
- **Tool System**: Define parameters, validation, and help text
- **Rich Standard Library**: Access filesystem, git, LLM, shell, and more

## Directory Structure

```
.meowg1k/
├── init.star              # Main configuration file (always loaded first)
├── commands/              # User-defined commands
│   ├── write.star        # AI content generation
│   ├── commit.star       # Commit message generation
│   ├── pr.star           # PR description generation
│   ├── code.star         # Code Q&A with RAG
│   └── search.star       # Semantic code search
└── lib/                  # Shared libraries (reusable functions)
    └── help.star         # Helper functions for commands
```

## Configuration Pattern: Provider/Model/Preset

meowg1k uses a three-tier configuration system:

### 1. Provider

A **Provider** defines connection details for an LLM service.

```python
# .meowg1k/init.star
meow.provider("gemini",
    type="gemini",                              # Provider type
    api_key=env.get("MEOW_GEMINI_API_KEY")     # API key from environment
)

meow.provider("anthropic",
    type="anthropic",
    api_key=env.get("MEOW_ANTHROPIC_API_KEY")
)
```

**Supported Provider Types**:
- `anthropic` - Anthropic Claude
- `openai` - OpenAI GPT
- `gemini` - Google Gemini
- `llama` - Ollama/Llama (local)
- `voyage` - Voyage embeddings
- `openrouter` - OpenRouter

### 2. Model

A **Model** references a provider and specifies model-specific configuration.

```python
# .meowg1k/init.star
meow.model("gemini-flash",
    provider="gemini",                    # Reference to provider
    model="gemini-3-flash-preview",       # Model identifier
    max_input_tokens=1048576,             # Token limits
    max_output_tokens=65536
)

meow.model("claude-sonnet",
    provider="anthropic",
    model="claude-sonnet-4",
    max_input_tokens=200000,
    max_output_tokens=8192
)
```

### 3. Preset

A **Preset** combines a model with generation parameters (temperature, etc.).

```python
# .meowg1k/init.star
meow.preset("fast",
    model="gemini-flash",      # Reference to model
    temperature=0.2            # Generation parameters
)

meow.preset("smart",
    model="claude-sonnet",
    temperature=0.2,
    max_tokens=4096
)

meow.preset("embeddings",
    model="gemini-embeddings"  # For RAG indexing
)
```

**Why This Pattern?**
- **Separation of Concerns**: Connection details, model config, and generation params are separate
- **Reusability**: One provider can have multiple models; one model can have multiple presets
- **Flexibility**: Easy to switch between fast/smart models without changing command code

## Tool System

The **Tool System** provides a unified way to define CLI commands with parameters, validation, and automatic help generation.

### Defining a Tool

```python
# Example: Simple write command

def write_handler(ctx):
    """Execute the write command."""
    prompt = ctx.prompt
    tone = ctx.tone
    
    system = ""
    if tone:
        system = f"Write in a {tone} tone."
    
    response = ctx.llm.chat(
        prompt=prompt,
        preset="smart",
        system=system,
        stream=True,
    )
    ctx.output.writeline(response)

write_tool = meow.tool(
    name="write",
    description="Generate AI-powered content",
    handler=write_handler,
    params={
        "prompt": meow.param("string", required=True, desc="What to generate", short="p"),
        "tone":   meow.param("string", desc="Writing tone (technical, casual, formal)", short="t"),
    }
)

meow.command(write_tool)
```

### Parameter Types

```python
# String parameter
"prompt": meow.param("string", required=True, desc="User prompt")

# Boolean flag
"verbose": meow.param("bool", default=False, desc="Enable verbose output")

# Integer parameter
"limit": meow.param("int", default=10, desc="Maximum results")

# Choice parameter (enum)
"format": meow.param("string", choices=["json", "yaml", "text"], default="json", desc="Output format")
```

### Handler Context

The handler function receives a **context object** (`ctx`) where each parameter value is accessible as a direct attribute:

```python
def my_handler(ctx):
    # Parameter values accessed as ctx attributes
    prompt = ctx.prompt
    verbose = ctx.verbose
    
    # Run LLM generation
    response = ctx.llm.chat(
        prompt=prompt,
        preset="smart",
    )
    
    ctx.output.writeline(response)
```

**Context modules**:
- `ctx.<param_name>` - Parameter value (direct attribute)
- `ctx.llm` - LLM operations (`chat`, `agent_turn`, `embed`)
- `ctx.fs` - Filesystem operations
- `ctx.git` - Git operations
- `ctx.shell` - Shell execution
- `ctx.ui` - Terminal UI components
- `ctx.output` - Output writing
- `ctx.session` - Session management
- `ctx.index` - RAG indexing/search
- `ctx.path` - Path manipulation
- `ctx.json` - JSON parsing/serialization
- `ctx.crypto` - Cryptographic operations
- `ctx.regexp` - Regular expressions
- `ctx.stdin` - Standard input
- `ctx.run(tool_name, **kwargs)` - Invoke another tool

## Standard Library Modules

meowg1k provides a rich set of built-in modules accessible from Starlark scripts.

### `meow` Module

Configuration and command registration.

```python
# Define provider
meow.provider("anthropic", type="anthropic", api_key=env.get("MEOW_ANTHROPIC_API_KEY"))

# Define model
meow.model("claude-sonnet", provider="anthropic", model="claude-sonnet-4",
           max_input_tokens=200000, max_output_tokens=8192)

# Define preset
meow.preset("smart", model="claude-sonnet", temperature=0.2)

# Create tool
tool = meow.tool(name="my-cmd", description="...", handler=my_handler, params={...})

# Register command
meow.command(tool)
meow.command(tool, name="alias")  # Override name

# Create parameter
param = meow.param("string", default="value", short="v", desc="Description",
                   required=False, choices=["a", "b"])
```

### `env` Module

Environment variable access.

```python
# Get environment variable
api_key = env.get("MEOW_API_KEY")

# Get with default
port = env.get("PORT", "8080")

# Set environment variable
env.set("DEBUG", "true")

# List all environment variables
all_vars = env.list()
```

### `fs` Module

Filesystem operations (all paths relative to workspace root unless absolute).

```python
# Read file
content = ctx.fs.read("README.md")

# Write file
ctx.fs.write("output.txt", "Hello, world!")

# Check existence
exists = ctx.fs.exists("config.yaml")

# List directory
files = ctx.fs.listdir("src/")

# Get file info
info = ctx.fs.stat("data.json")  # Returns struct{size, mode, mtime, is_dir}

# Glob patterns
matches = ctx.fs.glob("**/*.go")

# Recursive walk
all_files = ctx.fs.walk("src/", pattern="*.go")

# Make directory
ctx.fs.mkdir("output/reports")

# Remove file or directory
ctx.fs.remove("tmp/file.txt")

# Current working directory
cwd = ctx.fs.cwd()
```

### `git` Module

Git operations.

```python
# Get staged/unstaged diff
diff = ctx.git.diff()                          # Unstaged
diff = ctx.git.diff(target=ctx.git.STAGED)    # Staged
diff = ctx.git.diff(target="HEAD")            # Against HEAD

# Get diff for a specific file
result = ctx.git.diff_file("main.go", target=ctx.git.STAGED)
# result: {raw, file, additions, deletions}

# Get git log
log = ctx.git.log(n=10)
log = ctx.git.log(n=5, path="main.go")

# Get status string
status = ctx.git.status()

# Get current branch name
branch = ctx.git.branch()

# Commit changes
result = ctx.git.commit("feat: add new feature")
# result: {hash, message, ...}

# Push to remote
result = ctx.git.push()
# result: {remote, branch, ...}

# Create branch
result = ctx.git.create_branch("feature/new", should_checkout=True)
# result: {name, checked_out}

# Checkout branch or ref
result = ctx.git.checkout("main")
# result: {target}

# Stage files
result = ctx.git.add(["main.go", "README.md"])
# result: {count, files_added}

# Glob files tracked by git
files = ctx.git.glob("**/*.go")

# Read file at a git ref
content = ctx.git.read("main.go", ref="HEAD")

# Get lists of file states
staged   = ctx.git.staged_files()
modified = ctx.git.modified_files()
untracked = ctx.git.untracked_files()

# Constants
ctx.git.STAGED    # "staged"
ctx.git.HEAD      # "HEAD"
ctx.git.UNSTAGED  # "unstaged"
```

### `llm` Module

LLM operations accessible via `ctx.llm.*` in handlers.

```python
# Generate content (non-streaming)
response = ctx.llm.chat(
    prompt="Explain quantum computing.",
    preset="smart",
    system="You are a helpful assistant.",
)

# Stream response
def on_event(event):
    if event["type"] == "delta":
        ctx.output.write(event["content"])

response = ctx.llm.chat(
    prompt="Explain quantum computing.",
    preset="fast",
    stream=True,
    on_event=on_event,
)

# Agentic turn with tools
response = ctx.llm.agent_turn(
    prompt="Read config.json and summarise the settings.",
    preset="smart",
    tools=[file_reader],
    max_iterations=50,
    on_tool_error="return",
)

# Generate embeddings
embeddings = ctx.llm.embed(["text 1", "text 2"], preset="embeddings")
```

### `shell` Module

Execute shell commands.

```python
# Run command — returns {stdout, stderr, exit_code}
result = ctx.shell.exec("ls -la")
if result.exit_code == 0:
    ctx.output.writeline(result.stdout)
else:
    ctx.output.writeline("Error: " + result.stderr)
```

### `index` Module

RAG indexing and semantic search.

```python
# Index files
ctx.index.index(
    patterns=["**/*.go", "**/*.py"],
    ignore_patterns=["vendor/**", "node_modules/**"],
    preset="embeddings"
)

# Search by query
results = ctx.index.search(
    query="How does authentication work?",
    limit=10,
    preset="embeddings"
)

# Each result: {file_path, chunk_text, score}
for result in results:
    ctx.output.writeline(f"{result['file_path']}: {result['score']}")
```

### `ui` Module

Terminal UI components (Bubble Tea/Lip Gloss). All called via `ctx.ui.*` in handlers.

```python
# Prompt user for input
name = ctx.ui.prompt("Enter your name:")

# Selection menu (single or multiple)
choice = ctx.ui.select("Choose an option:", ["Option A", "Option B", "Option C"])
files  = ctx.ui.select("Select files:", ctx.fs.glob("**/*.go"), allow_multiple=True)

# Confirmation
if ctx.ui.confirm("Are you sure?"):
    # Proceed
    pass

# Progress bar
bar = ctx.ui.progress_bar(total=100, message="Processing files...")
for i in range(100):
    bar.inc()
bar.done("Done!")

# Display table
ctx.ui.table(
    [{"Name": "Item 1", "Value": "100"}, {"Name": "Item 2", "Value": "200"}],
    columns=["Name", "Value"],
    title="Items"
)

# Display code with syntax highlighting
ctx.ui.code("func main() {}", lang="go", title="main.go")

# Display diff
ctx.ui.diff(diff_text, title="changes.diff")

# Render markdown
ctx.ui.markdown("# Heading\n\nContent")

# Panel
ctx.ui.panel("Content here", title="Info")

# Pager (scrollable viewer)
ctx.ui.pager(large_text, title="Output")

# Turn/step pattern for structured output
turn = ctx.ui.assistant_turn("Analyzing...")
step = turn.step("Reading files")
step.done()
turn.done("Analysis complete")
```

### `output` Module

Write output to stdout.

```python
ctx.output.write("partial line")
ctx.output.writeline("full line with newline")
ctx.output.writef("Hello %s, you are %d years old", name, age)
```

### `json` Module

JSON parsing and serialization.

```python
# Parse JSON string
data = ctx.json.parse('{"name": "Alice", "age": 30}')

# Serialize to JSON
json_str = ctx.json.stringify({"status": "ok", "code": 200})

# Pretty print
pretty = ctx.json.stringify(data, indent=2)
```

### `path` Module

Path manipulation.

```python
# Join paths
full_path = ctx.path.join("src", "main", "app.go")

# Get directory
dir = ctx.path.dirname("/home/user/file.txt")  # "/home/user"

# Get filename
name = ctx.path.basename("/home/user/file.txt")  # "file.txt"

# Get extension
ext = ctx.path.ext("document.pdf")  # ".pdf"

# Get stem (name without extension)
stem = ctx.path.stem("document.pdf")  # "document"

# Absolute path
abs_path = ctx.path.abs("../relative/path")

# Clean path
clean = ctx.path.clean("src//./main/../main/app.go")

# Relative path
rel = ctx.path.rel("/base", "/base/sub/file.go")  # "sub/file.go"
```

### `crypto` Module

Cryptographic operations.

```python
# SHA256 hash
hash = ctx.crypto.sha256("content")

# MD5 hash
hash = ctx.crypto.md5("content")

# HMAC
mac = ctx.crypto.hmac("key", "data")
```

### `time` Module

Time and date operations.

```python
# Current time (Unix timestamp as float)
now = ctx.time.now()

# Format time
formatted = ctx.time.format(now, "2006-01-02 15:04:05")

# Parse time
timestamp = ctx.time.parse("2024-01-15 10:30:00", "2006-01-02 15:04:05")

# Sleep
ctx.time.sleep(1.5)  # Sleep for 1.5 seconds
```

### `regexp` Module

Regular expressions.

```python
# Match pattern
if ctx.regexp.match(r"^\d+$", "12345"):
    ctx.output.writeline("It's a number")

# Find all matches
matches = ctx.regexp.find_all(r"\b\w+@\w+\.\w+\b", text)

# Replace
new_text = ctx.regexp.replace(r"\s+", text, " ")  # arg order: pattern, text, replacement

# Split
parts = ctx.regexp.split(r",\s*", "a, b, c")
```

### `stdin` Module

Standard input reading.

```python
# Check if stdin is piped
if ctx.stdin.is_piped():
    data = ctx.stdin.read()

# Read all stdin
content = ctx.stdin.read()

# Read one line
line = ctx.stdin.read_line()
```

## Complete Example: Custom Command

Here's a complete example of a custom command that explains code changes:

```python
# .meowg1k/commands/explain.star

def explain_handler(ctx):
    """Execute the explain command."""
    ref = ctx.ref
    path = ctx.path_filter
    detailed = ctx.detailed
    
    # Get the diff
    if path:
        diff_output = ctx.git.diff(target=ref, path=path)
    else:
        diff_output = ctx.git.diff(target=ref)
    
    if not diff_output:
        ctx.output.writeline("No changes detected.")
        return
    
    # Build the prompt
    system_prompt = """You are an expert code reviewer. Analyze Git diffs and explain:
    1. What changed (files, functions, logic)
    2. Why it might have changed
    3. Potential impact or risks
    """
    
    if detailed:
        system_prompt += "\n4. Suggestions for improvement\n5. Testing considerations"
    
    user_prompt = f"Explain these code changes:\n\n```diff\n{diff_output}\n```"
    
    # Call LLM with streaming
    turn = ctx.ui.assistant_turn("Analyzing changes...")
    
    def on_event(event):
        if event["type"] == "delta":
            turn.stream(event["content"])
    
    response = ctx.llm.chat(
        prompt=user_prompt,
        preset="smart",
        system=system_prompt,
        stream=True,
        on_event=on_event,
    )
    
    turn.done()

explain_tool = meow.tool(
    name="explain",
    description="Explain code changes using Git diff",
    handler=explain_handler,
    params={
        "ref":         meow.param("string", default="HEAD", short="r",
                                  desc="Git ref to diff against"),
        "path_filter": meow.param("string", default="", short="p",
                                  desc="Limit diff to specific path"),
        "detailed":    meow.param("bool", default=False, short="d",
                                  desc="Provide detailed analysis"),
    }
)

meow.command(explain_tool)
```

## Best Practices

### 1. Use Descriptive Names
```python
# Good
meow.model("claude-sonnet-fast", ...)

# Bad
meow.model("m1", ...)
```

### 2. Separate Configuration from Logic
```python
# Good: Configuration in init.star, logic in commands/
# init.star
load("//commands/write.star", "setup")
setup(preset="smart", default_tone="technical")

# Bad: Mixing config and logic
```

### 3. Provide Help Text
```python
# Always include descriptions for parameters
meow.param(
    name="format",
    type="string",
    description="Output format: json, yaml, or text",  # Clear description
    choices=["json", "yaml", "text"]
)
```

### 4. Handle Errors Gracefully
```python
def _handler(ctx):
    try:
        result = ctx.git.diff(target="HEAD")
    except Exception as e:
        ctx.output.writeline(f"Error: {str(e)}")
        return
```

### 5. Use Progress Indicators
```python
bar = ctx.ui.progress_bar(total=100, message="Processing...")
# Long-running operation
bar.done("Done!")
```

## Advanced Topics

### Custom Libraries

Create reusable functions in `.meowg1k/lib/`:

```python
# .meowg1k/lib/formatting.star

def format_code_block(code, language):
    """Format code with syntax highlighting."""
    return f"```{language}\n{code}\n```"

def format_list(items):
    """Format items as markdown list."""
    return "\n".join([f"- {item}" for item in items])
```

Use in commands:

```python
load("//lib/formatting.star", "format_code_block", "format_list")

def _handler(ctx):
    code = "print('hello')"
    ctx.ui.markdown(format_code_block(code, "python"))
```

### Conditional Provider Selection

```python
# Auto-select provider based on availability
if env.get("MEOW_ANTHROPIC_API_KEY"):
    meow.provider("anthropic", type="anthropic", api_key=env.get("MEOW_ANTHROPIC_API_KEY"))
    meow.preset("smart", model="claude-sonnet")
elif env.get("MEOW_OPENAI_API_KEY"):
    meow.provider("openai", type="openai", api_key=env.get("MEOW_OPENAI_API_KEY"))
    meow.preset("smart", model="gpt-4")
```

## Summary

meowg1k's Starlark extension system provides:
- **Provider/Model/Preset pattern** for flexible LLM configuration
- **Tool system** for defining CLI commands with validation
- **Rich standard library** for filesystem, git, LLM, UI operations
- **User extensibility** through custom commands and libraries

This architecture enables users to create powerful AI workflows while maintaining a clean separation between configuration and logic.
