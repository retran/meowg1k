"""
Shell Execution Library for meowg1k

This library provides shell command execution capabilities for running system
commands, scripts, and external tools from within Starlark workflows.

## Quick Start

```python
load("//lib/shell.star", "shell_exec")

def handler(ctx):
    # Run a command
    output = ctx.run(shell_exec, command="ls -la")
    ctx.output.writeline(output)
    
    # Run tests
    test_results = ctx.run(shell_exec, command="go test ./...")
    ctx.ui.info(test_results)
```

## Available Tools

- `shell_exec` - Execute shell commands and return output

## API Reference

### shell_exec

Execute a shell command and capture its output.

**Parameters:**
- `command` (string, required): Shell command to execute

**Returns:** string - Command output (stdout + stderr combined)

**Example:**
```python
# Run tests
output = ctx.run(shell_exec, command="go test ./...")
ctx.output.writeline(output)

# Build project
build_output = ctx.run(shell_exec, command="go build -o bin/meow cmd/meow/main.go")

# List processes
ps_output = ctx.run(shell_exec, command="ps aux | grep meow")

# Chain commands with shell operators
ctx.run(shell_exec, command="mkdir -p build && cp bin/* build/")
```

**Behavior:**
- Runs in workspace root directory
- Captures both stdout and stderr
- Blocks until command completes
- Raises error if command exits with non-zero status
- Inherits environment variables from meowg1k process

## Advanced Usage

### Running Tests

```python
load("//lib/shell.star", "shell_exec")

def run_tests(ctx):
    ctx.ui.info("Running tests...")
    
    try:
        output = ctx.run(shell_exec, command="go test -v ./...")
        
        # Check for failures
        if "FAIL" in output:
            ctx.ui.error("Tests failed!")
            ctx.output.writeline(output)
            return False
        else:
            ctx.ui.success("All tests passed!")
            return True
    except:
        ctx.ui.error("Test command failed")
        return False
```

### Build Automation

```python
def build_project(ctx):
    # Clean previous builds
    ctx.run(shell_exec, command="rm -rf dist/")
    
    # Build for multiple platforms
    platforms = [
        ("linux", "amd64"),
        ("darwin", "amd64"),
        ("windows", "amd64"),
    ]
    
    for os_name, arch in platforms:
        cmd = "GOOS=%s GOARCH=%s go build -o dist/meow-%s-%s cmd/meow/main.go" % (
            os_name, arch, os_name, arch
        )
        ctx.ui.info("Building for %s/%s..." % (os_name, arch))
        ctx.run(shell_exec, command=cmd)
    
    ctx.ui.success("Build complete!")
```

### Dependency Checks

```python
def check_dependencies(ctx):
    # Check if required tools are installed.
    tools = ["go", "git", "docker", "kubectl"]
    
    missing = []
    for tool in tools:
        try:
            ctx.run(shell_exec, command="which " + tool)
        except:
            missing.append(tool)
    
    if missing:
        ctx.ui.error("Missing tools: " + ", ".join(missing))
        return False
    
    ctx.ui.success("All dependencies installed")
    return True
```

### Capture and Parse Output

```python
load("//lib/shell.star", "shell_exec")
load("//lib/json.star", "json_parse")

def get_go_version(ctx):
    # Get Go version information.
    output = ctx.run(shell_exec, command="go version")
    
    # Parse output: "go version go1.21.0 darwin/amd64"
    parts = output.split()
    if len(parts) >= 3:
        version = parts[2].replace("go", "")
        ctx.ui.info("Go version: " + version)
        return version
    
    return "unknown"
```

### Process Pipeline

```python
def analyze_codebase(ctx):
    # Run multiple analysis tools in sequence.
    
    steps = [
        ("Linting", "golangci-lint run"),
        ("Security scan", "gosec ./..."),
        ("Dependency check", "go mod verify"),
        ("Test coverage", "go test -cover ./..."),
    ]
    
    for name, cmd in steps:
        ctx.ui.info("Running: " + name)
        try:
            output = ctx.run(shell_exec, command=cmd)
            ctx.ui.success(name + " passed")
        except:
            ctx.ui.error(name + " failed")
            return False
    
    return True
```

## Error Handling

Shell commands can fail for various reasons:

```python
load("//lib/shell.star", "shell_exec")

def safe_exec(ctx, cmd):
    # Execute command with error handling.
    try:
        output = ctx.run(shell_exec, command=cmd)
        return output
    except:
        ctx.ui.error("Command failed: " + cmd)
        return None
```

**Common Errors:**
- Command not found (tool not in PATH)
- Non-zero exit code (command failure)
- Permission denied (insufficient permissions)
- Timeout (long-running commands - no built-in timeout)

**Best Practices:**
- Wrap risky commands in try/except
- Check command availability before running (use `which`)
- Validate command output before using it
- Be careful with shell operators (|, &&, ||) - they work but can hide errors

## Security Considerations

**WARNING:** Shell execution can be dangerous if not used carefully.

```python
# ❌ DANGEROUS: User input in shell command
def dangerous(ctx, user_input):
    cmd = "ls " + user_input  # Could be exploited!
    ctx.run(shell_exec, command=cmd)

# ✅ SAFE: Validate and sanitize input
def safe(ctx, user_input):
    # Validate input contains only safe characters
    if not user_input.replace("-", "").replace("_", "").replace("/", "").replace(".", "").isalnum():
        fail("Invalid input")
    
    cmd = "ls " + user_input
    ctx.run(shell_exec, command=cmd)
```

**Security Guidelines:**
- Never pass unsanitized user input to shell commands
- Avoid shell operators with user-controlled data
- Use absolute paths when possible
- Be careful with sudo/privileged commands
- Consider using ctx.fs/ctx.git modules instead of shell commands when possible

## Performance Tips

1. **No Timeout**: Commands run indefinitely. For long-running tasks, consider 
   running them in background or with timeout wrapper scripts.

2. **Blocking**: Commands block execution. For parallel tasks, consider using 
   multiple tool invocations or background processes.

3. **Output Buffering**: Large outputs are buffered in memory. For huge outputs, 
   redirect to file within the command.

4. **Shell Overhead**: Each invocation spawns a new shell. For repeated operations, 
   batch commands with `&&` or `;`.

## Alternatives

Consider using specialized modules instead of shell commands:

| Shell Command | Alternative |
|---------------|-------------|
| `cat file.txt` | `ctx.fs.read("file.txt")` |
| `git status` | `ctx.git.status()` |
| `curl https://...` | `ctx.http.get(url)` |
| `date` | `ctx.time.now()` |

Specialized modules provide:
- Better error handling
- Type safety
- No shell injection risks
- Consistent cross-platform behavior

## See Also

- [git.star](git.star) - Git-specific operations
- [file_ops.star](file_ops.star) - File system operations
- [API Reference](../../API_REFERENCE.md) - Shell module (ctx.shell)
"""

# ==============================================================================
# TOOL HANDLERS
# ==============================================================================

def shell_exec_handler(ctx):
    """Execute a shell command and return output."""
    command = ctx.params["command"]
    result = ctx.shell.exec(command)
    return result

# ==============================================================================
# TOOL DEFINITIONS
# ==============================================================================

shell_exec = meow.tool(
    name="shell_exec",
    description="Execute a shell command and return its output",
    params={
        "command": meow.param("string", desc="Shell command to execute", required=True),
    },
    handler=shell_exec_handler,
)

# Tool set
shell_tools = [shell_exec]
