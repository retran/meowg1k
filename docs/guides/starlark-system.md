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
load("//lib/help.star", "build_preset_desc")

def setup(preset="smart"):
    """Setup the write command with the specified preset."""
    
    # Define the tool
    tool = meow.tool(
        name="write",
        description="Generate AI-powered content",
        long_description="""
        AI-powered content generation with configurable tones and formats.
        Supports context from stdin and flexible output styles.
        """,
        handler=_handler,  # Function to execute
        parameters=[
            meow.param(
                name="prompt",
                type="string",
                description="What to generate",
                required=True,
                shorthand="p"
            ),
            meow.param(
                name="context",
                type="string",
                description="Additional context",
                required=False,
                shorthand="c"
            ),
            meow.param(
                name="tone",
                type="string",
                description="Writing tone (technical, casual, formal)",
                required=False,
                shorthand="t",
                default=""
            )
        ]
    )
    
    # Register the command
    meow.register_command(tool, preset=preset)

def _handler(ctx):
    """Execute the write command."""
    prompt = ctx.param("prompt")
    context = ctx.param("context")
    tone = ctx.param("tone")
    
    # Build the LLM request
    messages = []
    if tone:
        messages.append({"role": "system", "content": f"Write in a {tone} tone."})
    
    user_message = prompt
    if context:
        user_message += f"\n\nContext:\n{context}"
    messages.append({"role": "user", "content": user_message})
    
    # Call LLM
    response = ctx.run_llm(messages=messages)
    
    # Output result
    output.print_markdown(response)
```

### Parameter Types

```python
# String parameter
meow.param(
    name="prompt",
    type="string",
    description="User prompt",
    required=True
)

# Boolean flag
meow.param(
    name="verbose",
    type="bool",
    description="Enable verbose output",
    default=False
)

# Integer parameter
meow.param(
    name="limit",
    type="int",
    description="Maximum results",
    default=10
)

# Choice parameter (enum)
meow.param(
    name="format",
    type="string",
    description="Output format",
    choices=["json", "yaml", "text"],
    default="json"
)
```

### Handler Context

The handler function receives a **context object** (`ctx`) with access to parameters and utilities.

```python
def _handler(ctx):
    # Get parameter values
    prompt = ctx.param("prompt")
    verbose = ctx.param("verbose")
    
    # Run LLM generation
    response = ctx.run_llm(
        messages=[{"role": "user", "content": prompt}],
        temperature=0.7,
        max_tokens=2048
    )
    
    # Access configuration
    preset = ctx.preset()  # Get current preset
    
    # Output results
    output.print(response)
```

**Context Methods**:
- `ctx.param(name)` - Get parameter value
- `ctx.run_llm(messages, **kwargs)` - Execute LLM generation
- `ctx.preset()` - Get current preset configuration
- `ctx.workspace()` - Get workspace root directory

## Standard Library Modules

meowg1k provides a rich set of built-in modules accessible from Starlark scripts.

### `meow` Module

Configuration and command registration.

```python
# Define provider
meow.provider(name, type, api_key, **kwargs)

# Define model
meow.model(name, provider, model, max_input_tokens, max_output_tokens)

# Define preset
meow.preset(name, model, temperature=0.2, **kwargs)

# Create tool
tool = meow.tool(name, description, handler, parameters)

# Register command
meow.register_command(tool, preset="smart")

# Create parameter
param = meow.param(name, type, description, required, default, choices, shorthand)
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
```

### `fs` Module

Filesystem operations.

```python
# Read file
content = fs.read_file("README.md")

# Write file
fs.write_file("output.txt", "Hello, world!")

# Check existence
exists = fs.exists("config.yaml")

# List directory
files = fs.list_dir("src/")

# Get file info
info = fs.stat("data.json")  # Returns {size, mode, mtime}

# Glob patterns
matches = fs.glob("**/*.py")
```

### `git` Module

Git operations.

```python
# Get status
status = git.status()

# Get diff
diff = git.diff(ref="HEAD", path="src/")

# Show commit
commit_info = git.show("HEAD")

# Get log
log = git.log(limit=10, path="main.go")

# Get branches
branches = git.branches()

# Get current branch
branch = git.current_branch()

# Stage files
git.stage(["file1.go", "file2.go"])

# Commit changes
git.commit("feat: add new feature")

# Get HEAD hash
head = git.head_hash()
```

### `llm` Module

LLM operations (typically accessed via `ctx.run_llm()` in handlers).

```python
# Generate content
response = llm.generate(
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Explain quantum computing."}
    ],
    preset="smart",
    temperature=0.7,
    max_tokens=1024
)

# Stream response
for chunk in llm.generate_stream(messages=messages, preset="fast"):
    output.print(chunk)
```

### `shell` Module

Execute shell commands.

```python
# Run command
result = shell.exec("ls -la")  # Returns stdout

# Run with error handling
try:
    output = shell.exec("git status")
except Exception as e:
    print("Command failed:", str(e))
```

### `index` Module

RAG indexing and search.

```python
# Index files
index.index_files(
    patterns=["**/*.go", "**/*.py"],
    ignore_patterns=["vendor/**", "node_modules/**"]
)

# Search by query
results = index.search(
    query="How does authentication work?",
    limit=10,
    preset="embeddings"
)

# Each result: {file_path, chunk_text, score}
for result in results:
    print(f"{result['file_path']}: {result['score']}")
```

### `ui` Module

Terminal UI components (Bubble Tea/Lip Gloss).

```python
# Prompt user for input
name = ui.prompt("Enter your name:")

# Selection menu
choice = ui.select(
    "Choose an option:",
    options=["Option A", "Option B", "Option C"]
)

# Confirmation
if ui.confirm("Are you sure?"):
    # Proceed

# Progress bar
ui.progress_start("Processing files...")
ui.progress_update(0.5)  # 50%
ui.progress_complete()

# Display table
ui.table(
    headers=["Name", "Value", "Status"],
    rows=[
        ["Item 1", "100", "✓"],
        ["Item 2", "200", "✗"]
    ]
)
```

### `output` Module

Output formatting.

```python
# Print text
output.print("Hello, world!")

# Print line
output.print_line("This is a line")

# Print markdown
output.print_markdown("# Heading\n\nParagraph with **bold**.")

# Stream markdown (for LLM responses)
output.stream_markdown(chunk, done=False)
output.stream_markdown("", done=True)  # Flush

# Flush output
output.flush()
```

### `json` Module

JSON parsing and serialization.

```python
# Parse JSON
data = json.loads('{"name": "Alice", "age": 30}')

# Serialize to JSON
json_str = json.dumps({"status": "ok", "code": 200})

# Pretty print
pretty = json.dumps(data, indent=2)
```

### `path` Module

Path manipulation.

```python
# Join paths
full_path = path.join("src", "main", "app.go")

# Get directory
dir = path.dir("/home/user/file.txt")  # Returns "/home/user"

# Get filename
name = path.base("/home/user/file.txt")  # Returns "file.txt"

# Get extension
ext = path.ext("document.pdf")  # Returns ".pdf"

# Absolute path
abs_path = path.abs("../relative/path")

# Clean path
clean = path.clean("src//./main/../main/app.go")
```

### `crypto` Module

Cryptographic operations.

```python
# SHA256 hash
hash = crypto.sha256("content")

# MD5 hash
hash = crypto.md5("content")

# Base64 encode
encoded = crypto.base64_encode("data")

# Base64 decode
decoded = crypto.base64_decode(encoded)
```

### `time` Module

Time and date operations.

```python
# Current time
now = time.now()  # Unix timestamp

# Format time
formatted = time.format(now, "2006-01-02 15:04:05")

# Parse time
timestamp = time.parse("2024-01-15 10:30:00", "2006-01-02 15:04:05")

# Sleep
time.sleep(1.5)  # Sleep for 1.5 seconds
```

### `regexp` Module

Regular expressions.

```python
# Match pattern
if regexp.match(r"^\d+$", "12345"):
    print("It's a number")

# Find matches
matches = regexp.find_all(r"\b\w+@\w+\.\w+\b", text)  # Find emails

# Replace
new_text = regexp.replace(r"\s+", " ", text)  # Normalize whitespace
```

### `stdin` Module

Standard input reading.

```python
# Read all stdin
content = stdin.read()

# Check if stdin available
if stdin.is_available():
    data = stdin.read()
```

## Complete Example: Custom Command

Here's a complete example of a custom command that explains code changes:

```python
# .meowg1k/commands/explain.star

def setup(preset="smart"):
    """Setup the explain command."""
    
    tool = meow.tool(
        name="explain",
        description="Explain code changes using Git diff",
        long_description="""
        Analyzes Git diff output and provides a detailed explanation
        of what changed and why it might matter.
        """,
        handler=_handler,
        parameters=[
            meow.param(
                name="ref",
                type="string",
                description="Git ref to diff against (default: HEAD)",
                required=False,
                default="HEAD",
                shorthand="r"
            ),
            meow.param(
                name="path",
                type="string",
                description="Limit diff to specific path",
                required=False,
                default="",
                shorthand="p"
            ),
            meow.param(
                name="detailed",
                type="bool",
                description="Provide detailed analysis",
                default=False,
                shorthand="d"
            )
        ]
    )
    
    meow.register_command(tool, preset=preset)

def _handler(ctx):
    """Execute the explain command."""
    ref = ctx.param("ref")
    path = ctx.param("path")
    detailed = ctx.param("detailed")
    
    # Get the diff
    ui.progress_start("Fetching diff...")
    diff_output = git.diff(ref=ref, path=path)
    ui.progress_complete()
    
    if not diff_output:
        output.print_line("No changes detected.")
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
    
    # Call LLM
    ui.progress_start("Analyzing changes...")
    response = ctx.run_llm(
        messages=[
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt}
        ]
    )
    ui.progress_complete()
    
    # Output the explanation
    output.print_markdown(f"# Code Changes Explanation\n\n{response}")

# Register in init.star
# load("//commands/explain.star", explain_setup = "setup")
# explain_setup(preset="smart")
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
        result = git.diff(ref="HEAD")
    except Exception as e:
        output.print_line(f"Error: {str(e)}")
        return
```

### 5. Use Progress Indicators
```python
ui.progress_start("Processing...")
# Long-running operation
ui.progress_complete()
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
    output.print_markdown(format_code_block(code, "python"))
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
