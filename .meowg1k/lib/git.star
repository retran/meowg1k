"""
Git Operations Library for meowg1k

This library provides Git repository operations including status checks, diff
viewing, and branch information. Perfect for commit message generation, code
review automation, and repository analysis.

## Quick Start

```python
load("//lib/git.star", "git_status", "git_diff")

def handler(ctx):
    # Get repository status
    status_json = ctx.run(git_status)
    status = ctx.json.decode(status_json)
    
    # Get unstaged changes
    diff = ctx.run(git_diff, staged=False)
    
    # Get staged changes for commit
    staged_diff = ctx.run(git_diff, staged=True)
```

## Available Tools

- `git_status` - Get current repository status
- `git_diff` - Get unified diff for staged/unstaged changes

### Tool Sets
- `git_tools` - All git operation tools (2 tools)

## API Reference

### git_status

Get current git repository status information.

**Parameters:** None

**Returns:** string - JSON object with status fields

**Example:**
```python
status_json = ctx.run(git_status)
status = ctx.json.decode(status_json)

ctx.ui.info("Branch: " + status.get("branch", "unknown"))
ctx.ui.info("Files changed: " + str(len(status.get("modified", []))))

# Check for uncommitted changes
if status.get("staged", []) or status.get("modified", []):
    ctx.ui.warning("You have uncommitted changes")
```

**Status Fields:**
- `branch` (string) - Current branch name
- `staged` (list) - List of staged file paths
- `modified` (list) - List of modified unstaged file paths
- `untracked` (list) - List of untracked file paths
- `ahead` (int) - Commits ahead of remote
- `behind` (int) - Commits behind remote

---

### git_diff

Get git diff in unified diff format.

**Parameters:**
- `staged` (bool, optional): Show staged changes only (default: False)

**Returns:** string - Unified diff text

**Example:**
```python
# Get unstaged changes
unstaged = ctx.run(git_diff, staged=False)
ctx.output.writeline("Unstaged changes:")
ctx.output.writeline(unstaged)

# Get staged changes
staged = ctx.run(git_diff, staged=True)

# Generate commit message from staged diff
if staged:
    message = ctx.run(llm_generate,
                     prompt="Generate commit message for: " + staged,
                     preset="smart")
    ctx.ui.info("Suggested commit message:")
    ctx.output.writeline(message)
```

**Diff Format:** Returns unified diff format with:
- File headers (`--- a/file`, `+++ b/file`)
- Hunk headers (`@@ -1,3 +1,4 @@`)
- Added lines (prefixed with `+`)
- Removed lines (prefixed with `-`)
- Context lines (no prefix)

## Advanced Usage

### Commit Message Generation

```python
load("//lib/git.star", "git_diff", "git_status")
load("//lib/llm.star", "llm_generate")

def generate_commit_message(ctx):
    # Generate intelligent commit message from staged changes
    
    # Check if there are staged changes
    status_json = ctx.run(git_status)
    status = ctx.json.decode(status_json)
    
    if not status.get("staged", []):
        ctx.ui.error("No staged changes to commit")
        return
    
    # Get the diff
    diff = ctx.run(git_diff, staged=True)
    
    # Generate message
    prompt = ("Analyze this git diff and generate a conventional commit message.\\n\\n" +
              "Format: <type>(<scope>): <subject>\\n\\n<body>\\n\\n" +
              "Types: feat, fix, docs, style, refactor, test, chore\\n" +
              "Keep subject under 50 chars, body wrapped at 72 chars.\\n\\nDiff:\\n" + diff)
    
    message = ctx.run(llm_generate, 
                     prompt=prompt,
                     preset="smart")
    
    ctx.ui.success("Generated commit message:")
    ctx.output.writeline(message)
    return message
```

### Pre-Commit Checks

```python
load("//lib/git.star", "git_status", "git_diff")
load("//lib/shell.star", "shell_exec")

def pre_commit_checks(ctx):
    # Run checks before committing.
    
    # Get staged files
    status_json = ctx.run(git_status)
    status = ctx.json.decode(status_json)
    staged = status.get("staged", [])
    
    if not staged:
        ctx.ui.error("No files staged")
        return False
    
    # Check if Go files are staged
    go_files = [f for f in staged if f.endswith(".go")]
    
    if go_files:
        ctx.ui.info("Running Go checks...")
        
        # Format check
        try:
            ctx.run(shell_exec, command="gofmt -l " + " ".join(go_files))
        except:
            ctx.ui.error("Go files not formatted")
            return False
        
        # Lint check
        try:
            ctx.run(shell_exec, command="golangci-lint run " + " ".join(go_files))
        except:
            ctx.ui.error("Linting issues found")
            return False
    
    ctx.ui.success("Pre-commit checks passed")
    return True
```

### Change Analysis

```python
load("//lib/git.star", "git_diff")

def analyze_changes(ctx):
    # Analyze what type of changes were made.
    
    diff = ctx.run(git_diff, staged=True)
    
    # Count changes
    added = 0
    removed = 0
    
    for line in diff.split("\\n"):
        if line.startswith("+") and not line.startswith("+++"):
            added += 1
        elif line.startswith("-") and not line.startswith("---"):
            removed += 1
    
    ctx.ui.info("Changes: +%d -%d" % (added, removed))
    
    # Categorize
    if added > removed * 2:
        ctx.ui.info("Category: Feature addition")
    elif removed > added * 2:
        ctx.ui.info("Category: Code removal/cleanup")
    else:
        ctx.ui.info("Category: Refactoring/modification")
```

### Branch Information

```python
load("//lib/git.star", "git_status")

def check_branch(ctx):
    # Check current branch and sync status.
    
    status_json = ctx.run(git_status)
    status = ctx.json.decode(status_json)
    
    branch = status.get("branch", "unknown")
    ahead = status.get("ahead", 0)
    behind = status.get("behind", 0)
    
    ctx.ui.info("Current branch: " + branch)
    
    if ahead > 0:
        ctx.ui.warning("You are %d commits ahead of remote" % ahead)
    
    if behind > 0:
        ctx.ui.warning("You are %d commits behind remote" % behind)
    
    if ahead == 0 and behind == 0:
        ctx.ui.success("Branch is in sync with remote")
```

### Review Preparation

```python
load("//lib/git.star", "git_status", "git_diff")
load("//lib/file_ops.star", "file_writer")

def prepare_review(ctx):
    # Prepare review documentation for code changes.
    
    # Get status
    status_json = ctx.run(git_status)
    status = ctx.json.decode(status_json)
    
    # Get diff
    diff = ctx.run(git_diff, staged=True)
    
    # Build review doc
    doc = "# Code Review\\n\\n"
    doc += "## Branch: %s\\n\\n" % status.get("branch", "unknown")
    doc += "## Files Changed\\n\\n"
    
    for file in status.get("staged", []):
        doc += "- " + file + "\\n"
    
    doc += "\\n## Diff\\n\\n```diff\\n"
    doc += diff
    doc += "\\n```\\n"
    
    # Save
    ctx.run(file_writer, path="REVIEW.md", content=doc)
    ctx.ui.success("Review document created: REVIEW.md")
```

## Error Handling

Git operations can fail in various scenarios:

```python
load("//lib/git.star", "git_status")

def safe_git_status(ctx):
    # Get git status with error handling.
    try:
        status_json = ctx.run(git_status)
        return ctx.json.decode(status_json)
    except:
        ctx.ui.error("Not a git repository or git command failed")
        return None
```

**Common Errors:**
- Not in a git repository
- Git not installed
- Repository in detached HEAD state
- Corrupted repository

**Best Practices:**
- Wrap git operations in try/except for robustness
- Check if in git repository before running git commands
- Handle empty diffs gracefully
- Validate status structure before accessing fields

## Performance Tips

1. **Status Caching**: `git_status` can be slow in large repos. Cache result if 
   calling multiple times.

2. **Diff Size**: Large diffs can consume memory. Consider processing in chunks 
   or limiting scope.

3. **Staged vs Unstaged**: Specify `staged=True` to avoid processing unstaged 
   changes when not needed.

## Integration Examples

### With LLM Tools

```python
load("//lib/git.star", "git_diff")
load("//lib/llm.star", "llm_generate")

def explain_changes(ctx):
    # Explain changes in plain English.
    diff = ctx.run(git_diff, staged=True)
    
    explanation = ctx.run(llm_generate,
        prompt="Explain these code changes in simple terms: " + diff,
        preset="smart")
    
    ctx.output.writeline(explanation)
```

### With File Operations

```python
load("//lib/git.star", "git_status")
load("//lib/file_ops.star", "file_reader")

def review_staged_files(ctx):
    # Review each staged file individually.
    status_json = ctx.run(git_status)
    status = ctx.json.decode(status_json)
    
    for file_path in status.get("staged", []):
        ctx.ui.info("Reviewing: " + file_path)
        content = ctx.run(file_reader, path=file_path)
        # Analyze content...
```

## See Also

- [shell.star](shell.star) - Shell command execution
- [llm.star](llm.star) - LLM text generation
- [file_ops.star](file_ops.star) - File operations
- [API Reference](../../API_REFERENCE.md) - Git module (ctx.git)
"""

# ==============================================================================
# TOOL HANDLERS
# ==============================================================================

def git_status_handler(ctx):
    """Get current git repository status."""
    status = ctx.git.status()
    return ctx.json.encode(status)

def git_diff_handler(ctx):
    """Get git diff for staged or unstaged changes."""
    staged = getattr(ctx, "staged", False)
    diff = ctx.git.diff(staged=staged)
    return diff

# ==============================================================================
# TOOL DEFINITIONS
# ==============================================================================

git_status = meow.tool(
    name="git_status",
    description="Get the current git repository status",
    params={},
    handler=git_status_handler,
)

git_diff = meow.tool(
    name="git_diff",
    description="Get git diff (staged or unstaged changes)",
    params={
        "staged": meow.param("bool", desc="Show staged changes only", default=False),
    },
    handler=git_diff_handler,
)

# Tool set
git_tools = [git_status, git_diff]
