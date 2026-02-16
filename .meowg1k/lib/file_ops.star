"""
File Operations Library for meowg1k

This library provides comprehensive file system operations including reading,
writing, checking existence, listing directories, and text search/replace.

## Quick Start

```python
load("//lib/file_ops.star", "file_reader", "file_writer", "list_directory")

def handler(ctx):
    # Read a file
    content = ctx.run(file_reader, path="README.md")
    
    # Write to a file
    ctx.run(file_writer, path="output.txt", content="Hello, World!")
    
    # List Go files in src directory
    files_json = ctx.run(list_directory, path="src", pattern="*.go")
    files = ctx.json.decode(files_json)
```

## Available Tools

### File Operations
- `file_reader` - Read complete file contents
- `file_writer` - Write content to file (creates/overwrites)
- `file_exists` - Check if file or directory exists
- `list_directory` - List files with glob pattern support

### Text Operations
- `search_text` - Search for patterns using grep
- `replace_text` - Find and replace text in files

### Tool Sets
- `file_tools` - All file operation tools (6 tools)

## API Reference

### file_reader

Read the complete contents of a file into memory.

**Parameters:**
- `path` (string, required): Path to file, relative to workspace root

**Returns:** string - Complete file contents

**Example:**
```python
content = ctx.run(file_reader, path="README.md")
lines = content.split("\\n")
ctx.ui.info("File has %d lines" % len(lines))
```

**Performance:** Reads entire file into memory. Use cautiously with large files.

---

### file_writer

Write content to a file, creating or overwriting as needed.

**Parameters:**
- `path` (string, required): Destination file path
- `content` (string, required): Content to write

**Returns:** string - Success message with file path

**Example:**
```python
ctx.run(file_writer, 
       path="output/results.txt",
       content="Analysis complete\\nTotal files: 42")
```

**Behavior:**
- Creates parent directories automatically
- Overwrites existing files without warning
- Returns confirmation message

---

### file_exists

Check if a file or directory exists on the filesystem.

**Parameters:**
- `path` (string, required): Path to check

**Returns:** string - "True" or "False" (as string for LLM tools)

**Example:**
```python
exists = ctx.run(file_exists, path="config.yaml")
if exists == "True":
    config = ctx.run(file_reader, path="config.yaml")
else:
    ctx.ui.warning("Config file not found")
```

**Note:** Does not follow symlinks.

---

### list_directory

List files in a directory matching a glob pattern.

**Parameters:**
- `path` (string, optional): Directory path (default: ".")
- `pattern` (string, optional): Glob pattern (default: "*")

**Returns:** string - JSON array of matching file paths

**Example:**
```python
# List all Go files in src
files_json = ctx.run(list_directory, path="src", pattern="*.go")
files = ctx.json.decode(files_json)

for file_path in files:
    ctx.ui.info("Found: " + file_path)

# List all files recursively
all_files = ctx.run(list_directory, path=".", pattern="**/*")

# List test files
tests = ctx.run(list_directory, path="internal", pattern="**/*_test.go")
```

**Glob Patterns:**
- `*` - Match any characters in filename
- `**` - Match any directories recursively
- `*.go` - All Go files in directory
- `**/*.go` - All Go files recursively
- `test_*.py` - Files starting with "test_"

---

### search_text

Search for text patterns in files using grep.

**Parameters:**
- `pattern` (string, required): Text pattern or regex to search
- `path` (string, optional): Directory to search (default: ".")

**Returns:** string - JSON array of {file, line, text} objects

**Example:**
```python
# Find TODO comments
results = ctx.run(search_text, 
                 pattern="TODO:",
                 path="src")
matches = ctx.json.decode(results)

for match in matches:
    location = "%s:%d" % (match["file"], match["line"])
    ctx.ui.info(location + " - " + match["text"])

# Find function definitions
funcs = ctx.run(search_text, 
               pattern="func \\w+\\(",
               path="internal")
```

**Performance:** Uses grep internally. Can be slow on large directories.

---

### replace_text

Replace all occurrences of text in a file.

**Parameters:**
- `path` (string, required): File to modify
- `old` (string, required): Text to find
- `new` (string, required): Replacement text

**Returns:** string - Success message

**Example:**
```python
# Update version string
ctx.run(replace_text,
       path="version.txt",
       old="1.0.0",
       new="1.0.1")

# Replace import paths
ctx.run(replace_text,
       path="main.go",
       old="github.com/old/pkg",
       new="github.com/new/pkg")
```

**Behavior:**
- Performs simple string replacement (not regex)
- Reads entire file, replaces all occurrences, writes back
- Creates backup is not created

**Note:** For regex replacement, read file with `file_reader`, use `regexp` module, 
then write with `file_writer`.

## Advanced Usage

### Processing Multiple Files

```python
load("//lib/file_ops.star", "list_directory", "file_reader", "file_writer")

def handler(ctx):
    # Find all Go files
    files_json = ctx.run(list_directory, path="internal", pattern="**/*.go")
    files = ctx.json.decode(files_json)
    
    # Process each file
    for file_path in files:
        content = ctx.run(file_reader, path=file_path)
        
        # Analyze or transform
        if "TODO" in content:
            ctx.ui.warning("TODOs in: " + file_path)
```

### Safe File Operations

```python
load("//lib/file_ops.star", "file_exists", "file_reader", "file_writer")
load("//lib/validators.star", "validators")

def safe_write(ctx, path, content):
    # Check if file exists before overwriting
    exists = ctx.run(file_exists, path=path)
    if exists == "True":
        # Create backup
        backup = path + ".backup"
        original = ctx.run(file_reader, path=path)
        ctx.run(file_writer, path=backup, content=original)
    
    # Write new content
    ctx.run(file_writer, path=path, content=content)
```

### Building File Inventories

```python
def build_inventory(ctx):
    # List all source files
    go_files = ctx.json.decode(
        ctx.run(list_directory, path=".", pattern="**/*.go"))
    
    md_files = ctx.json.decode(
        ctx.run(list_directory, path=".", pattern="**/*.md"))
    
    # Generate report
    report = "# File Inventory\\n\\n"
    report += "Go files: %d\\n" % len(go_files)
    report += "Markdown files: %d\\n" % len(md_files)
    
    ctx.run(file_writer, path="inventory.md", content=report)
```

## Error Handling

All tools follow consistent error patterns:

```python
# File not found
try:
    content = ctx.run(file_reader, path="nonexistent.txt")
except:
    ctx.ui.error("File not found")
    # Handle error
```

**Common Errors:**
- File not found (read operations)
- Permission denied (read/write operations)
- Disk full (write operations)
- Invalid path (all operations)

**Best Practices:**
- Always check `file_exists` before reading critical files
- Wrap file operations in try/except for production code
- Validate paths before passing to tools
- Be cautious with `file_writer` - it overwrites without confirmation

## Performance Tips

1. **Large Files**: `file_reader` loads entire file into memory. For very large files, 
   consider processing in chunks using `ctx.fs.read()` directly with offset/limit.

2. **Recursive Search**: `search_text` with large directories can be slow. Narrow the 
   search path when possible.

3. **Bulk Operations**: When processing many files, consider batching operations and 
   showing progress with `ctx.ui.progress()`.

4. **Glob Patterns**: More specific patterns are faster. Use `*.go` instead of `**/*` 
   when you don't need recursion.

## See Also

- [shell.star](shell.star) - Shell command execution
- [git.star](git.star) - Git operations
- [validators.star](validators.star) - Parameter validation
- [API Reference](../../API_REFERENCE.md) - File system module (ctx.fs)
"""

# ==============================================================================
# TOOL HANDLERS
# ==============================================================================

def file_reader_handler(ctx):
    """Read complete contents of a file."""
    path = ctx.params["path"]
    return ctx.fs.read(path)

def file_writer_handler(ctx):
    """Write content to a file, creating or overwriting."""
    path = ctx.params["path"]
    content = ctx.params["content"]
    ctx.fs.write(path, content)
    return "Successfully wrote to " + path

def file_exists_handler(ctx):
    """Check if a file or directory exists."""
    path = ctx.params["path"]
    exists = ctx.fs.exists(path)
    return str(exists)

def list_directory_handler(ctx):
    """List files in a directory matching a glob pattern."""
    path = ctx.params.get("path", ".")
    pattern = ctx.params.get("pattern", "*")
    
    files = ctx.fs.glob(path + "/" + pattern)
    return ctx.json.encode(files)

def search_text_handler(ctx):
    """Search for text pattern in files using grep."""
    pattern = ctx.params["pattern"]
    path = ctx.params.get("path", ".")
    
    results = ctx.fs.grep(pattern, path)
    return ctx.json.encode(results)

def replace_text_handler(ctx):
    """Replace all occurrences of text in a file."""
    path = ctx.params["path"]
    old_text = ctx.params["old"]
    new_text = ctx.params["new"]
    
    content = ctx.fs.read(path)
    new_content = content.replace(old_text, new_text)
    ctx.fs.write(path, new_content)
    return "Replaced text in " + path

# ==============================================================================
# TOOL DEFINITIONS
# ==============================================================================

file_reader = meow.tool(
    name="file_reader",
    description="Read the contents of a file",
    params={
        "path": meow.param("string", desc="Path to the file to read", required=True),
    },
    handler=file_reader_handler,
)

file_writer = meow.tool(
    name="file_writer",
    description="Write content to a file",
    params={
        "path": meow.param("string", desc="Path to the file to write", required=True),
        "content": meow.param("string", desc="Content to write to the file", required=True),
    },
    handler=file_writer_handler,
)

file_exists = meow.tool(
    name="file_exists",
    description="Check if a file or directory exists",
    params={
        "path": meow.param("string", desc="Path to check", required=True),
    },
    handler=file_exists_handler,
)

list_directory = meow.tool(
    name="list_directory",
    description="List files in a directory with optional glob pattern",
    params={
        "path": meow.param("string", desc="Directory path", default="."),
        "pattern": meow.param("string", desc="Glob pattern (e.g. '*.py')", default="*"),
    },
    handler=list_directory_handler,
)

search_text = meow.tool(
    name="search_text",
    description="Search for text pattern in files",
    params={
        "pattern": meow.param("string", desc="Text pattern or regex to search for", required=True),
        "path": meow.param("string", desc="Directory to search in", default="."),
    },
    handler=search_text_handler,
)

replace_text = meow.tool(
    name="replace_text",
    description="Replace text in a file",
    params={
        "path": meow.param("string", desc="Path to the file", required=True),
        "old": meow.param("string", desc="Text to replace", required=True),
        "new": meow.param("string", desc="Replacement text", required=True),
    },
    handler=replace_text_handler,
)

# ==============================================================================
# TOOL SETS
# ==============================================================================

# All file operation tools
file_tools = [file_reader, file_writer, file_exists, list_directory, search_text, replace_text]
