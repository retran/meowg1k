# Starlark Library Index

Complete reference for all meowg1k Starlark libraries. Use this index to discover available functionality and find the right library for your task.

## Quick Navigation

- [Core Tool Libraries](#core-tool-libraries) - 9 libraries with 18 tools total
- [Supporting Libraries](#supporting-libraries) - 5 utility libraries
- [Category Index](#category-index) - Find libraries by domain
- [Cross-Reference](#cross-reference) - Related libraries and integration patterns
- [Migration Guide](#migration-guide) - Upgrade from old tools.star

---

## Core Tool Libraries

These libraries provide tools for agentic workflows and command handlers.

### 1. file_ops.star - File System Operations

**Tools:** 6 tools for file I/O and directory operations

```python
load("//lib/file_ops.star", "file_reader", "file_writer", "file_exists", 
     "list_directory", "search_text", "replace_text", "file_tools")
```

**Use Cases:**
- Read/write files in workspace
- Check file existence before operations
- List files with glob patterns
- Search files with regex patterns
- Find and replace text across files

**Key Features:**
- Automatic parent directory creation
- Glob pattern support (*.go, src/**/*.py)
- Grep-based text search
- Batch text replacement

**See Also:** [shell.star](#3-shellstar---shell-command-execution), [git.star](#4-gitstar---git-operations)

---

### 2. json.star - JSON Operations

**Tools:** 2 tools for JSON parsing and querying

```python
load("//lib/json.star", "json_parse", "json_query", "json_tools")
```

**Use Cases:**
- Parse JSON responses from APIs
- Extract values with JMESPath queries
- Validate JSON structure
- Transform JSON data

**Key Features:**
- Full JSON parsing with error handling
- JMESPath query language support
- Pretty-printing for debugging
- Array/object traversal helpers

**See Also:** [http.star](#6-httpstar---http-operations), [llm.star](#9-llmstar---llm-text-generation)

---

### 3. shell.star - Shell Command Execution

**Tools:** 1 tool for running shell commands

```python
load("//lib/shell.star", "shell_exec", "shell_tools")
```

**Use Cases:**
- Run build commands (go build, npm run)
- Execute tests and linters
- Invoke command-line tools
- Capture command output

**Key Features:**
- Real-time stdout/stderr capture
- Exit code checking
- Working directory control
- Environment variable support

**Security:** Be cautious with untrusted input in commands

**See Also:** [git.star](#4-gitstar---git-operations), [file_ops.star](#1-file_opsstar---file-system-operations)

---

### 4. git.star - Git Operations

**Tools:** 2 tools for Git repository inspection

```python
load("//lib/git.star", "git_status", "git_diff", "git_tools")
```

**Use Cases:**
- Check working tree status
- Generate commit messages
- Review staged changes
- Analyze diffs for code review

**Key Features:**
- Unstaged and staged change detection
- Unified diff format parsing
- Branch and remote information
- Untracked file listing

**See Also:** [file_ops.star](#1-file_opsstar---file-system-operations), [code_search.star](#5-code_searchstar---semantic-code-search)

---

### 5. code_search.star - Semantic Code Search

**Tools:** 1 tool for RAG-based code search

```python
load("//lib/code_search.star", "code_search", "code_search_tools")
```

**Use Cases:**
- Find relevant code by semantic query
- Answer questions about codebase
- Locate implementation examples
- Discover similar code patterns

**Key Features:**
- Vector embedding-based search (HNSW)
- SQLite-backed index persistence
- Configurable relevance threshold
- Result ranking by similarity

**Performance:** First search builds index (slow), subsequent searches are fast

**See Also:** [file_ops.star](#1-file_opsstar---file-system-operations), [llm.star](#9-llmstar---llm-text-generation)

---

### 6. http.star - HTTP Operations

**Tools:** 2 tools for HTTP requests

```python
load("//lib/http.star", "http_get", "http_post", "http_tools")
```

**Use Cases:**
- Fetch data from REST APIs
- POST data to webhooks
- Download files or resources
- Integrate with external services

**Key Features:**
- Custom headers support
- JSON request/response handling
- Status code checking
- Timeout configuration

**Security:** Validate URLs and sanitize responses

**See Also:** [json.star](#2-jsonstar---json-operations)

---

### 7. time.star - Time Operations

**Tools:** 1 tool for current time

```python
load("//lib/time.star", "current_time", "time_tools")
```

**Use Cases:**
- Get current timestamp
- Generate time-based filenames
- Log timestamps
- Calculate time-based conditions

**Key Features:**
- Multiple format options (ISO8601, Unix, RFC3339)
- Timezone awareness
- Custom format strings

**See Also:** [file_ops.star](#1-file_opsstar---file-system-operations)

---

### 8. math.star - Mathematical Operations

**Tools:** 1 tool for arithmetic calculations

```python
load("//lib/math.star", "calculator", "math_tools")
```

**Use Cases:**
- Evaluate arithmetic expressions
- Calculate metrics and statistics
- Parse numeric values from text
- Perform unit conversions

**Key Features:**
- Basic operations (+, -, *, /, %)
- Parentheses support
- Float and integer arithmetic
- Error handling for invalid expressions

**See Also:** [json.star](#2-jsonstar---json-operations)

---

### 9. llm.star - LLM Text Generation

**Tools:** 1 tool for text generation

```python
load("//lib/llm.star", "llm_generate", "llm_tools")
```

**Use Cases:**
- Generate text completions
- Answer questions
- Summarize content
- Transform text

**Key Features:**
- Preset support (fast, smart, balanced)
- System prompt customization
- Streaming response support
- Token limit awareness

**Performance:** Use "fast" preset for quick responses, "smart" for complex reasoning

**See Also:** [code_search.star](#5-code_searchstar---semantic-code-search), [json.star](#2-jsonstar---json-operations)

---

## Supporting Libraries

These libraries provide utilities and helpers for building commands.

### 10. validators.star - Parameter Validation

```python
load("//lib/validators.star", "validate_path", "validate_url", "validate_json")
```

**Purpose:** Validate and sanitize tool parameters

**Use Cases:**
- Validate file paths before reading
- Check URL formats
- Verify JSON structure
- Sanitize user input

**See Also:** [file_ops.star](#1-file_opsstar---file-system-operations), [http.star](#6-httpstar---http-operations)

---

### 11. diff.star - Diff Analysis

```python
load("//lib/diff.star", "parse_diff", "analyze_changes", "format_diff")
```

**Purpose:** Parse and analyze Git diffs

**Use Cases:**
- Extract changed files from diff
- Count additions/deletions
- Identify change patterns
- Format diffs for display

**See Also:** [git.star](#4-gitstar---git-operations)

---

### 12. help.star - Help Text Formatting

```python
load("//lib/help.star", "format_help", "render_examples")
```

**Purpose:** Format help text and documentation

**Use Cases:**
- Generate command help text
- Format examples and usage
- Create formatted output
- Render markdown in terminal

**See Also:** All libraries (for help text generation)

---

### 13. planning.star - Task Planning

```python
load("//lib/planning.star", "create_plan", "execute_plan", "decompose_task")
```

**Purpose:** Plan and decompose complex tasks

**Use Cases:**
- Break down high-level goals
- Generate step-by-step plans
- Execute plans with agentic loops
- Track task progress

**See Also:** [memory.star](#14-memorystar---session-memory), [llm.star](#9-llmstar---llm-text-generation)

---

### 14. memory.star - Session Memory

```python
load("//lib/memory.star", "save_context", "recall_context", "list_context", 
     "summarize_history", "get_session_info", "memory_tools")
```

**Purpose:** Manage context and state across executions

**Use Cases:**
- Save/recall context between tool calls
- Summarize conversation history
- Track session state
- Reduce context window usage

**See Also:** [planning.star](#13-planningstar---task-planning), [llm.star](#9-llmstar---llm-text-generation)

---

## Category Index

Find libraries by problem domain.

### File & Directory Operations
- **file_ops.star** - Read, write, search files
- **validators.star** - Path validation

### Version Control
- **git.star** - Git status and diffs
- **diff.star** - Diff parsing and analysis

### Data Processing
- **json.star** - JSON parsing/querying
- **math.star** - Arithmetic calculations
- **time.star** - Time operations

### External Integration
- **http.star** - HTTP GET/POST
- **shell.star** - Shell commands

### Code Intelligence
- **code_search.star** - Semantic search (RAG)
- **llm.star** - Text generation

### Workflow & State
- **planning.star** - Task planning
- **memory.star** - Session memory

### Utilities
- **validators.star** - Input validation
- **help.star** - Help formatting

---

## Cross-Reference

### Common Integration Patterns

#### Code Review Workflow
```python
load("//lib/git.star", "git_status", "git_diff")
load("//lib/code_search.star", "code_search")
load("//lib/llm.star", "llm_generate")
load("//lib/file_ops.star", "file_reader")

# 1. Get changes
status = ctx.run(git_status)
diff = ctx.run(git_diff)

# 2. Search for related code
results = ctx.run(code_search, query="error handling patterns", limit=5)

# 3. Read affected files
content = ctx.run(file_reader, path="src/main.go")

# 4. Generate review
review = ctx.run(llm_generate, 
    prompt="Review this code change: " + diff,
    preset="smart")
```

#### Data Pipeline
```python
load("//lib/http.star", "http_get")
load("//lib/json.star", "json_parse", "json_query")
load("//lib/file_ops.star", "file_writer")
load("//lib/math.star", "calculator")

# 1. Fetch data
response = ctx.run(http_get, url="https://api.example.com/data")

# 2. Parse JSON
data = ctx.run(json_parse, json_string=response)

# 3. Query specific values
values = ctx.run(json_query, json_string=data, query="items[*].price")

# 4. Calculate total
total = ctx.run(calculator, expression="10.5 + 20.3 + 15.7")

# 5. Save results
ctx.run(file_writer, path="output.txt", content="Total: " + total)
```

#### Test Execution & Reporting
```python
load("//lib/shell.star", "shell_exec")
load("//lib/file_ops.star", "search_text", "file_reader")
load("//lib/git.star", "git_status")
load("//lib/llm.star", "llm_generate")

# 1. Run tests
result = ctx.run(shell_exec, command="go test -v ./...")

# 2. Check for failures
failures = ctx.run(search_text, pattern="FAIL:", path=".")

# 3. Get context
status = ctx.run(git_status)

# 4. Generate report
report = ctx.run(llm_generate,
    prompt="Summarize test failures: " + result,
    preset="fast")
```

#### Semantic Code Search & Documentation
```python
load("//lib/code_search.star", "code_search")
load("//lib/file_ops.star", "file_reader")
load("//lib/llm.star", "llm_generate")
load("//lib/json.star", "json_parse")

# 1. Search for relevant code
results = ctx.run(code_search, query="authentication logic", limit=10)

# 2. Parse search results
parsed = ctx.run(json_parse, json_string=results)

# 3. Read top result
content = ctx.run(file_reader, path="auth/handler.go")

# 4. Generate documentation
docs = ctx.run(llm_generate,
    prompt="Document this authentication code: " + content,
    preset="smart")
```

---

## Migration Guide

### From tools.star (Legacy)

If you have old commands using `tools.star`, here's how to migrate:

#### Tool Location Mapping

| Old (tools.star) | New Library | Import Statement |
|-----------------|-------------|------------------|
| `file_reader` | file_ops.star | `load("//lib/file_ops.star", "file_reader")` |
| `file_writer` | file_ops.star | `load("//lib/file_ops.star", "file_writer")` |
| `file_exists` | file_ops.star | `load("//lib/file_ops.star", "file_exists")` |
| `list_directory` | file_ops.star | `load("//lib/file_ops.star", "list_directory")` |
| `search_text` | file_ops.star | `load("//lib/file_ops.star", "search_text")` |
| `replace_text` | file_ops.star | `load("//lib/file_ops.star", "replace_text")` |
| `shell_exec` | shell.star | `load("//lib/shell.star", "shell_exec")` |
| `git_status` | git.star | `load("//lib/git.star", "git_status")` |
| `git_diff` | git.star | `load("//lib/git.star", "git_diff")` |
| `code_search` | code_search.star | `load("//lib/code_search.star", "code_search")` |
| `json_parse` | json.star | `load("//lib/json.star", "json_parse")` |
| `json_query` | json.star | `load("//lib/json.star", "json_query")` |
| `http_get` | http.star | `load("//lib/http.star", "http_get")` |
| `http_post` | http.star | `load("//lib/http.star", "http_post")` |
| `current_time` | time.star | `load("//lib/time.star", "current_time")` |
| `calculator` | math.star | `load("//lib/math.star", "calculator")` |
| `llm_generate` | llm.star | `load("//lib/llm.star", "llm_generate")` |

#### Migration Steps

1. **Replace the load statement:**
   ```python
   # Old
   load("//lib/tools.star", "file_reader", "shell_exec")
   
   # New
   load("//lib/file_ops.star", "file_reader")
   load("//lib/shell.star", "shell_exec")
   ```

2. **No changes to tool usage** - All tool parameters and behavior remain the same

3. **Use tool sets for convenience:**
   ```python
   # Load all file tools at once
   load("//lib/file_ops.star", "file_tools")
   
   def handler(ctx):
       # Use ctx.run() with any tool from file_tools
       ctx.run(file_tools[0], path="README.md")  # file_reader
   ```

#### Example Migration

**Before:**
```python
load("//lib/tools.star", "file_reader", "git_status", "llm_generate", "calculator")

def handler(ctx):
    content = ctx.run(file_reader, path="main.go")
    status = ctx.run(git_status)
    result = ctx.run(llm_generate, prompt="Analyze code", preset="smart")
    total = ctx.run(calculator, expression="10 + 20")
```

**After:**
```python
load("//lib/file_ops.star", "file_reader")
load("//lib/git.star", "git_status")
load("//lib/llm.star", "llm_generate")
load("//lib/math.star", "calculator")

def handler(ctx):
    content = ctx.run(file_reader, path="main.go")
    status = ctx.run(git_status)
    result = ctx.run(llm_generate, prompt="Analyze code", preset="smart")
    total = ctx.run(calculator, expression="10 + 20")
```

**Key Changes:**
- Split single load into multiple focused loads
- No changes to tool usage or parameters
- Better organization by domain

---

## Best Practices

### Library Organization

1. **Load only what you need:**
   ```python
   # Good - explicit imports
   load("//lib/file_ops.star", "file_reader", "file_writer")
   
   # Avoid - importing unused tools
   load("//lib/file_ops.star", "file_tools")  # All 6 tools
   ```

2. **Group related imports:**
   ```python
   # File operations
   load("//lib/file_ops.star", "file_reader")
   load("//lib/git.star", "git_diff")
   
   # Data processing
   load("//lib/json.star", "json_parse")
   load("//lib/math.star", "calculator")
   
   # LLM operations
   load("//lib/llm.star", "llm_generate")
   ```

3. **Use tool sets for agentic loops:**
   ```python
   load("//lib/file_ops.star", "file_tools")
   load("//lib/git.star", "git_tools")
   load("//lib/code_search.star", "code_search_tools")
   
   def handler(ctx):
       # Combine tool sets
       all_tools = file_tools + git_tools + code_search_tools
       
       # Use in agentic loop
       result = ctx.llm.agentic(
           tools=all_tools,
           prompt="Analyze the codebase",
           max_iterations=20
       )
   ```

### Error Handling

Always validate inputs and handle errors gracefully:

```python
load("//lib/file_ops.star", "file_exists", "file_reader")
load("//lib/validators.star", "validate_path")

def handler(ctx):
    path = ctx.params["path"]
    
    # Validate path
    if not validate_path(path):
        return "Error: Invalid path"
    
    # Check existence
    if ctx.run(file_exists, path=path) != "True":
        return "Error: File not found: " + path
    
    # Safe to read
    content = ctx.run(file_reader, path=path)
    return content
```

### Performance Tips

1. **Minimize file I/O** - Read files once, cache results
2. **Use appropriate presets** - "fast" for simple tasks, "smart" for complex reasoning
3. **Batch operations** - Search once, process multiple results
4. **Limit search results** - Specify reasonable `limit` values for code_search

---

## Documentation Standards

All libraries follow this documentation structure:

1. **Module Docstring** - Overview, quick start, available functions
2. **Tool Definitions** - Comprehensive parameter documentation
3. **Advanced Usage** - 5-10 real-world examples
4. **Error Handling** - Common errors and solutions
5. **Performance Tips** - Optimization guidance
6. **Integration Examples** - How to combine with other libraries
7. **See Also** - Cross-references to related libraries

---

## Contributing

When creating new libraries:

1. Follow the documentation standard (see any library in Core Tool Libraries)
2. Include comprehensive examples
3. Document error handling
4. Add performance tips
5. Cross-reference related libraries
6. Update this index

---

## See Also

- **LIBRARY_REVIEW.md** - Detailed review and improvement plan
- **API_REFERENCE.md** - Complete Starlark API documentation
- **.meowg1k/commands/** - Example command implementations
- **docs/agents/starlark-system.md** - Starlark extension system guide

---

**Last Updated:** 2024 (Post library split)  
**Total Libraries:** 14 (9 core + 5 supporting)  
**Total Tools:** 18 tools + utilities
