"""
JSON Operations Library for meowg1k

This library provides JSON parsing, formatting, and querying capabilities.
Perfect for working with API responses, configuration files, and structured data.

## Quick Start

```python
load("//lib/json.star", "json_parse", "json_query")

def handler(ctx):
    # Parse and format JSON
    compact = '{"name":"Alice","age":30}'
    formatted = ctx.run(json_parse, json=compact)
    
    # Query nested data
    data = '{"users":[{"name":"Alice","role":"admin"}]}'
    name = ctx.run(json_query, json=data, path="users.0.name")
    # Returns: "Alice"
```

## Available Tools

- `json_parse` - Parse and pretty-print JSON
- `json_query` - Query JSON with dot notation

### Tool Sets
- `json_tools` - All JSON operation tools (2 tools)

## API Reference

### json_parse

Parse and pretty-print JSON string with consistent formatting.

**Parameters:**
- `json` (string, required): JSON string to parse

**Returns:** string - Formatted JSON string

**Example:**
```python
# Format compact JSON
compact = '{"name":"Alice","age":30,"role":"admin"}'
formatted = ctx.run(json_parse, json=compact)

ctx.output.writeline(formatted)
# Output:
# {
#   "name": "Alice",
#   "age": 30,
#   "role": "admin"
# }

# Validate JSON
try:
    ctx.run(json_parse, json='{"invalid"}')
except:
    ctx.ui.error("Invalid JSON")
```

**Use Cases:**
- Validate JSON syntax
- Pretty-print JSON for readability
- Normalize JSON formatting
- Prepare JSON for display

---

### json_query

Query JSON data using dot notation paths.

**Parameters:**
- `json` (string, required): JSON string to query
- `path` (string, required): Dot-separated path (e.g., "data.users.0.name")

**Returns:** string - JSON-encoded value at path, or "null" if not found

**Example:**
```python
data = '''
{
  "users": [
    {"name": "Alice", "role": "admin", "age": 30},
    {"name": "Bob", "role": "user", "age": 25}
  ],
  "meta": {
    "total": 2,
    "version": "1.0"
  }
}
'''

# Query scalar values
name = ctx.run(json_query, json=data, path="users.0.name")
# Returns: "Alice"

age = ctx.run(json_query, json=data, path="users.1.age")
# Returns: "25"

total = ctx.run(json_query, json=data, path="meta.total")
# Returns: "2"

# Query nested objects
user = ctx.run(json_query, json=data, path="users.0")
# Returns: '{"name":"Alice","role":"admin","age":30}'

# Non-existent paths return "null"
missing = ctx.run(json_query, json=data, path="users.5.name")
# Returns: "null"
```

**Path Syntax:**
- Use `.` to separate keys: `"users.0.name"`
- Use numeric indices for arrays: `"items.0"`, `"items.1"`
- Paths are case-sensitive
- Returns `"null"` for missing paths (not an error)

## Advanced Usage

### Working with API Responses

```python
load("//lib/http.star", "http_get")
load("//lib/json.star", "json_query")

def fetch_repo_info(ctx, owner, repo):
    # Fetch GitHub repository information.
    
    url = "https://api.github.com/repos/%s/%s" % (owner, repo)
    response = ctx.run(http_get, url=url)
    
    # Extract specific fields
    name = ctx.run(json_query, json=response, path="name")
    stars = ctx.run(json_query, json=response, path="stargazers_count")
    language = ctx.run(json_query, json=response, path="language")
    
    ctx.ui.info("Repository: " + name)
    ctx.ui.info("Stars: " + stars)
    ctx.ui.info("Language: " + language)
    
    return response
```

### Configuration File Processing

```python
load("//lib/file_ops.star", "file_reader")
load("//lib/json.star", "json_parse", "json_query")

def read_config(ctx, config_file):
    # Read and validate JSON configuration.
    
    # Read file
    content = ctx.run(file_reader, path=config_file)
    
    # Validate JSON
    try:
        formatted = ctx.run(json_parse, json=content)
    except:
        ctx.ui.error("Invalid JSON in config file")
        return None
    
    # Extract specific config values
    api_key = ctx.run(json_query, json=content, path="api.key")
    endpoint = ctx.run(json_query, json=content, path="api.endpoint")
    
    if api_key == "null" or endpoint == "null":
        ctx.ui.error("Missing required config fields")
        return None
    
    return {
        "api_key": api_key,
        "endpoint": endpoint,
    }
```

### Data Transformation

```python
load("//lib/json.star", "json_query")

def transform_user_data(ctx, users_json):
    # Transform array of users to summary.
    
    # Parse using Starlark json module (not tool)
    users = ctx.json.decode(users_json)
    
    # Extract names using query tool
    names = []
    for i in range(len(users)):
        path = "users.%d.name" % i
        # Note: This example shows concept - in practice use ctx.json.decode
        # for direct access rather than repeated queries
    
    return names
```

### Nested Data Navigation

```python
load("//lib/json.star", "json_query")

def navigate_complex_json(ctx):
    # Navigate deeply nested JSON structure.
    
    data = '''
    {
      "company": {
        "departments": [
          {
            "name": "Engineering",
            "teams": [
              {"name": "Backend", "size": 10},
              {"name": "Frontend", "size": 8}
            ]
          }
        ]
      }
    }
    '''
    
    # Deep navigation
    backend_size = ctx.run(json_query, 
                          json=data, 
                          path="company.departments.0.teams.0.size")
    
    ctx.ui.info("Backend team size: " + backend_size)
```

### API Response Validation

```python
load("//lib/http.star", "http_get")
load("//lib/json.star", "json_parse", "json_query")

def validate_api_response(ctx, url, required_fields):
    # Validate API response has required fields.
    
    # Fetch data
    response = ctx.run(http_get, url=url)
    
    # Validate JSON syntax
    try:
        ctx.run(json_parse, json=response)
    except:
        ctx.ui.error("Invalid JSON response")
        return False
    
    # Check required fields
    for field in required_fields:
        value = ctx.run(json_query, json=response, path=field)
        if value == "null":
            ctx.ui.error("Missing required field: " + field)
            return False
    
    ctx.ui.success("Response validation passed")
    return True
```

### Building JSON Programmatically

```python
def build_report(ctx, data):
    # Build JSON report from data.
    
    # Use Starlark's json module to build structure
    report = {
        "timestamp": ctx.time.now(),
        "summary": {
            "total": len(data),
            "status": "complete"
        },
        "items": data
    }
    
    # Convert to formatted JSON
    json_str = ctx.json.encode(report)
    formatted = ctx.run(json_parse, json=json_str)
    
    return formatted
```

## Error Handling

JSON operations can fail due to invalid syntax or missing paths:

```python
load("//lib/json.star", "json_parse", "json_query")

def safe_json_parse(ctx, json_str):
    # Parse JSON with error handling.
    try:
        return ctx.run(json_parse, json=json_str)
    except:
        ctx.ui.error("Failed to parse JSON")
        return None

def safe_json_query(ctx, json_str, path):
    # Query JSON with validation.
    try:
        result = ctx.run(json_query, json=json_str, path=path)
        if result == "null":
            ctx.ui.warning("Path not found: " + path)
            return None
        return result
    except:
        ctx.ui.error("Failed to query JSON")
        return None
```

**Common Errors:**
- Invalid JSON syntax (parse fails)
- Malformed path syntax
- Type mismatches (accessing object key on array)

**Best Practices:**
- Always validate JSON with `json_parse` before processing
- Check for `"null"` returns from `json_query`
- Wrap operations in try/except for production code
- Use `ctx.json.decode()` for direct manipulation when not using tools

## Performance Tips

1. **Direct Access**: For simple JSON, use `ctx.json.decode()` directly instead 
   of tools:
   ```python
   # Faster for direct access
   data = ctx.json.decode(json_str)
   name = data["users"][0]["name"]
   
   # Use tool when passing to LLM or in agentic workflows
   name = ctx.run(json_query, json=json_str, path="users.0.name")
   ```

2. **Query Once**: Extract parent object and navigate programmatically rather 
   than multiple queries:
   ```python
   # Less efficient: multiple tool calls
   name = ctx.run(json_query, json=data, path="user.name")
   age = ctx.run(json_query, json=data, path="user.age")
   
   # More efficient: query once
   user = ctx.run(json_query, json=data, path="user")
   user_obj = ctx.json.decode(user)
   name = user_obj["name"]
   age = user_obj["age"]
   ```

3. **Large JSON**: For very large JSON documents, consider streaming or 
   processing in chunks rather than loading entire structure.

## Tool vs Module

**When to use json tools (this library):**
- In agentic workflows (tools passed to `ctx.llm.agentic()`)
- When LLM needs to manipulate JSON
- For consistent string-based interface

**When to use ctx.json module directly:**
- Direct programmatic access in Starlark code
- Better performance for non-agentic workflows
- Type-safe access to structures

```python
# Using tool (for agentic workflows)
result = ctx.run(json_query, json=data, path="users.0.name")

# Using module directly (for scripts)
data_obj = ctx.json.decode(data)
result = data_obj["users"][0]["name"]
```

## Integration Examples

### With HTTP Operations

```python
load("//lib/http.star", "http_get", "http_post")
load("//lib/json.star", "json_parse", "json_query")

def api_workflow(ctx):
    # Fetch data
    response = ctx.run(http_get, url="https://api.example.com/data")
    
    # Query specific field
    id = ctx.run(json_query, json=response, path="id")
    
    # Build new request
    payload = ctx.json.encode({"id": id, "action": "update"})
    
    # Post back
    ctx.run(http_post, url="https://api.example.com/update", body=payload)
```

### With File Operations

```python
load("//lib/file_ops.star", "file_reader", "file_writer")
load("//lib/json.star", "json_parse")

def format_json_file(ctx, input_path, output_path):
    # Read, format, and write JSON file.
    
    # Read
    content = ctx.run(file_reader, path=input_path)
    
    # Format
    formatted = ctx.run(json_parse, json=content)
    
    # Write
    ctx.run(file_writer, path=output_path, content=formatted)
    ctx.ui.success("Formatted JSON written to " + output_path)
```

## See Also

- [http.star](http.star) - HTTP operations for API calls
- [file_ops.star](file_ops.star) - File operations
- [API Reference](../../API_REFERENCE.md) - JSON module (ctx.json)
"""

# ==============================================================================
# TOOL HANDLERS
# ==============================================================================

def json_parse_handler(ctx):
    """Parse and pretty-print JSON string."""
    json_str = ctx.params["json"]
    data = ctx.json.decode(json_str)
    return ctx.json.encode(data)

def json_query_handler(ctx):
    """Query JSON data using dot notation path."""
    json_str = ctx.params["json"]
    key_path = ctx.params["path"]
    
    data = ctx.json.decode(json_str)
    
    # Navigate through the path
    keys = key_path.split(".")
    current = data
    for key in keys:
        # Try as list index first
        if type(current) == "list":
            # Parse as integer
            is_int = True
            idx_val = 0
            for c in key:
                if c < "0" or c > "9":
                    is_int = False
                    break
            if is_int and key != "":
                idx_val = int(key)
                if idx_val < len(current):
                    current = current[idx_val]
                    continue
        # Try as dict key
        if type(current) == "dict":
            if key in current:
                current = current[key]
            else:
                return "null"
    
    return ctx.json.encode(current)

# ==============================================================================
# TOOL DEFINITIONS
# ==============================================================================

json_parse = meow.tool(
    name="json_parse",
    description="Parse and pretty-print JSON string",
    params={
        "json": meow.param("string", desc="JSON string to parse", required=True),
    },
    handler=json_parse_handler,
)

json_query = meow.tool(
    name="json_query",
    description="Query JSON data using dot notation (e.g. 'users.0.name')",
    params={
        "json": meow.param("string", desc="JSON string", required=True),
        "path": meow.param("string", desc="Dot-separated path (e.g. 'data.users.0.name')", required=True),
    },
    handler=json_query_handler,
)

# Tool set
json_tools = [json_parse, json_query]
