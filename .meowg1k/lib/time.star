"""
Time Operations Library for meowg1k

This library provides time-related operations including getting current timestamp
and time formatting.

## Quick Start

```python
load("//lib/time.star", "current_time")

def handler(ctx):
    # Get current time
    now = ctx.run(current_time)
    ctx.output.writeline("Current time: " + now)
    
    # With custom format
    timestamp = ctx.run(current_time, format="2006-01-02 15:04:05")
```

## Available Tools

- `current_time` - Get current system timestamp

### Tool Sets
- `time_tools` - All time operation tools (1 tool)

## API Reference

### current_time

Get current system time as formatted string.

**Parameters:**
- `format` (string, optional): Time format (default: "2006-01-02 15:04:05")

**Returns:** string - Current timestamp

**Example:**
```python
# Default format
now = ctx.run(current_time)
ctx.output.writeline(now)
# Output: "2024-02-16 14:30:45"

# Custom format (Go time format)
date = ctx.run(current_time, format="2006-01-02")
# Output: "2024-02-16"

time_only = ctx.run(current_time, format="15:04:05")
# Output: "14:30:45"
```

**Go Time Format Reference:**
```
2006      - 4-digit year
01        - 2-digit month
02        - 2-digit day
15        - 2-digit hour (24-hour)
04        - 2-digit minute
05        - 2-digit second
Mon       - Abbreviated weekday
Monday    - Full weekday
Jan       - Abbreviated month
January   - Full month
MST       - Timezone abbreviation
-0700     - Timezone offset
```

## Advanced Usage

### Timestamps in Logs

```python
load("//lib/time.star", "current_time")
load("//lib/file_ops.star", "file_writer", "file_reader", "file_exists")

def log_message(ctx, message, log_file="activity.log"):
    # Append timestamped message to log file.
    
    timestamp = ctx.run(current_time, format="2006-01-02 15:04:05")
    entry = "[%s] %s\\n" % (timestamp, message)
    
    # Append to existing log
    existing = ""
    if ctx.run(file_exists, path=log_file) == "True":
        existing = ctx.run(file_reader, path=log_file)
    
    ctx.run(file_writer, path=log_file, content=existing + entry)
```

### Build Timestamps

```python
load("//lib/time.star", "current_time")
load("//lib/shell.star", "shell_exec")

def build_with_timestamp(ctx):
    # Build binary with embedded timestamp.
    
    timestamp = ctx.run(current_time, format="2006-01-02T15:04:05")
    
    # Embed timestamp in build
    cmd = 'go build -ldflags "-X main.BuildTime=%s" cmd/meow/main.go' % timestamp
    ctx.run(shell_exec, command=cmd)
    
    ctx.ui.success("Build complete with timestamp: " + timestamp)
```

### Report Generation

```python
load("//lib/time.star", "current_time")
load("//lib/file_ops.star", "file_writer")

def generate_report(ctx, data):
    # Generate report with timestamp.
    
    timestamp = ctx.run(current_time, format="2006-01-02 15:04:05")
    
    report = "# Analysis Report\\n\\n"
    report += "Generated: %s\\n\\n" % timestamp
    report += "## Summary\\n\\n"
    report += data
    
    filename = "report-%s.md" % ctx.run(current_time, format="2006-01-02-15-04")
    ctx.run(file_writer, path=filename, content=report)
    
    ctx.ui.success("Report saved: " + filename)
```

### Session Tracking

```python
load("//lib/time.star", "current_time")

def track_session(ctx):
    # Track session start and end times.
    
    # Store start time in session
    start = ctx.run(current_time)
    ctx.session.set("start_time", start)
    
    # ... do work ...
    
    # Get duration (simplified - would need time arithmetic)
    end = ctx.run(current_time)
    ctx.ui.info("Session started: " + start)
    ctx.ui.info("Session ended: " + end)
```

### Scheduled Operations

```python
load("//lib/time.star", "current_time")

def should_run_maintenance(ctx):
    # Check if maintenance window is active.
    
    now = ctx.run(current_time, format="15")  # Get hour (0-23)
    hour = int(now)
    
    # Maintenance window: 2 AM - 4 AM
    if 2 <= hour < 4:
        ctx.ui.info("In maintenance window")
        return True
    else:
        ctx.ui.info("Outside maintenance window")
        return False
```

### Timestamped Backups

```python
load("//lib/time.star", "current_time")
load("//lib/file_ops.star", "file_reader", "file_writer")

def backup_file(ctx, source_path):
    # Create timestamped backup of file.
    
    # Read source
    content = ctx.run(file_reader, path=source_path)
    
    # Generate backup filename
    timestamp = ctx.run(current_time, format="2006-01-02-15-04-05")
    backup_path = "%s.%s.backup" % (source_path, timestamp)
    
    # Write backup
    ctx.run(file_writer, path=backup_path, content=content)
    
    ctx.ui.success("Backup created: " + backup_path)
    return backup_path
```

### API Request Tracking

```python
load("//lib/time.star", "current_time")
load("//lib/http.star", "http_get")

def fetch_with_timing(ctx, url):
    # Fetch URL and log timing.
    
    start = ctx.run(current_time, format="2006-01-02 15:04:05.000")
    ctx.ui.info("Request started: " + start)
    
    response = ctx.run(http_get, url=url)
    
    end = ctx.run(current_time, format="2006-01-02 15:04:05.000")
    ctx.ui.info("Request completed: " + end)
    
    return response
```

## Error Handling

Time operations rarely fail, but handle gracefully:

```python
load("//lib/time.star", "current_time")

def safe_timestamp(ctx):
    # Get timestamp with fallback.
    try:
        return ctx.run(current_time)
    except:
        ctx.ui.error("Failed to get timestamp")
        return "unknown-time"
```

**Best Practices:**
- Use consistent format strings throughout application
- Store format strings as constants
- Validate format strings if user-provided
- Consider timezone implications (current_time returns local time)

## Limitations

1. **No Timezone Control**: Returns local system time only
2. **No Time Arithmetic**: Cannot add/subtract time durations
3. **No Parsing**: Cannot parse time strings into structured format
4. **No Comparison**: Cannot compare times directly
5. **Format Parameter May Be Ignored**: Implementation may not support custom formats

**Workaround:** For time arithmetic and parsing, use string manipulation or 
external tools via shell commands.

## Common Formats

Here are common Go time format patterns:

```python
# ISO 8601
"2006-01-02T15:04:05Z07:00"

# RFC 3339
"2006-01-02T15:04:05Z07:00"

# Date only
"2006-01-02"

# Time only
"15:04:05"

# US format
"01/02/2006"

# Readable
"Monday, January 02, 2006 at 3:04 PM"

# Unix-friendly filename
"2006-01-02-15-04-05"

# With milliseconds
"2006-01-02 15:04:05.000"
```

## Integration Examples

### With File Operations

```python
load("//lib/time.star", "current_time")
load("//lib/file_ops.star", "file_writer")

def save_with_timestamp(ctx, content):
    timestamp = ctx.run(current_time, format="2006-01-02-15-04")
    filename = "output-%s.txt" % timestamp
    ctx.run(file_writer, path=filename, content=content)
```

### With Git Operations

```python
load("//lib/time.star", "current_time")
load("//lib/git.star", "git_status")

def status_with_timestamp(ctx):
    now = ctx.run(current_time)
    ctx.ui.info("Git status as of: " + now)
    
    status = ctx.run(git_status)
    ctx.output.writeline(status)
```

### With LLM Generation

```python
load("//lib/time.star", "current_time")
load("//lib/llm.star", "llm_generate")

def generate_with_context(ctx, prompt):
    timestamp = ctx.run(current_time)
    
    context = "Current time: %s\\n\\nUser request: %s" % (timestamp, prompt)
    
    response = ctx.run(llm_generate, prompt=context, preset="smart")
    return response
```

## Use Cases

### Audit Trails
```python
def audit_log(ctx, action, user):
    timestamp = ctx.run(current_time)
    log_entry = "%s | %s | %s" % (timestamp, user, action)
    # Save to audit log...
```

### Versioning
```python
def version_with_timestamp(ctx):
    timestamp = ctx.run(current_time, format="2006.01.02")
    version = "v" + timestamp
    return version
```

### Scheduling
```python
def is_business_hours(ctx):
    hour = int(ctx.run(current_time, format="15"))
    return 9 <= hour < 17
```

## Performance Tips

1. **Caching**: If you need the same timestamp multiple times, call once and store:
   ```python
   timestamp = ctx.run(current_time)
   # Use timestamp multiple times
   ```

2. **Format Selection**: Simpler formats may be faster to format (though difference 
   is likely negligible).

## See Also

- [file_ops.star](file_ops.star) - File operations (for timestamped files)
- [API Reference](../../API_REFERENCE.md) - Time module (ctx.time)
"""

# ==============================================================================
# TOOL HANDLERS
# ==============================================================================

def current_time_handler(ctx):
    """Get current system time."""
    format = ctx.params.get("format", "2006-01-02 15:04:05")
    now = ctx.time.now(format=format)
    return now

# ==============================================================================
# TOOL DEFINITIONS
# ==============================================================================

current_time = meow.tool(
    name="current_time",
    description="Get the current system time",
    params={
        "format": meow.param("string", desc="Time format string", default="2006-01-02 15:04:05"),
    },
    handler=current_time_handler,
)

# Tool set
time_tools = [current_time]
