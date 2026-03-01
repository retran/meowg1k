# Agentic System Guide

This guide covers the unified tool/command/agent/session system in meowg1k, including how to build autonomous agents that can plan, execute, and track complex workflows.

## Overview

meowg1k provides a powerful agentic system where:
- **Everything is a tool** - Commands, agents, and utilities all use the same tool interface
- **Sessions track execution** - Every invocation creates a session with complete history
- **Agents use tools autonomously** - LLMs can call tools iteratively to achieve goals
- **Context is preserved** - Session metadata and event history enable long-running workflows

## Architecture

### Core Concepts

1. **Tool** - A callable unit with typed parameters and a handler function
2. **Session** - An execution context that tracks events, metadata, and relationships
3. **Agentic Loop** - LLM-driven autonomous tool execution with `ctx.llm.agent_turn()`
4. **Context APIs** - Session, memory, and planning utilities for agent state management

### Session Hierarchy

```
Root Session (CLI command)
├── Child Session 1 (ctx.run or tool call)
│   ├── LLM Event (user_message)
│   ├── LLM Event (assistant_message)
│   └── Tool Event (tool_result)
├── Child Session 2 (ctx.run or tool call)
│   └── ...
└── ...
```

Every CLI command creates a root session. Every `ctx.run()` or LLM tool invocation creates a child session.

## Building Agents

### Basic Agent Structure

```python
load("//lib/tools.star", "file_reader", "code_search")

def my_agent_handler(ctx):
    """Agent handler function"""
    
    # 1. Parse inputs
    goal = ctx.params.get("goal", "")
    
    # 2. Execute agentic loop
    result = ctx.llm.agent_turn(
        prompt="Achieve this goal: " + goal,
        preset="smart",
        tools=[file_reader, code_search],
        system="You are a helpful agent...",
        max_iterations=50,
    )
    
    # 3. Return or display results
    ctx.ui.markdown(result)
    return result

# Register as tool
my_agent = meow.tool(
    name="my-agent",
    description="My custom agent",
    params={
        "goal": meow.param("string", description="Goal to achieve", required=True),
    },
    handler=my_agent_handler,
)

# Register as command
meow.command(my_agent)
```

### Agentic Loop API

The `ctx.llm.agent_turn()` method enables autonomous multi-turn LLM interactions:

```python
result = ctx.llm.agent_turn(
    prompt="User goal or query",           # Initial user prompt (required)
    preset="smart",                        # LLM preset (required, no default)
    tools=[tool1, tool2, ...],             # List of available tools (required)
    system="Agent instructions",           # System prompt defining agent behavior
    max_iterations=50,                     # Maximum number of turns
    on_tool_error="return",                # Error handling: "return" | "abort"
    stream=False,                          # Enable streaming
    on_event=None,                         # Stream event callback
)
```

**How it works:**
1. Sends initial prompt to LLM with tool schemas
2. LLM responds with text or tool calls
3. If tool calls requested, executes them and sends results back
4. Repeats until LLM provides final answer or max_iterations reached
5. All events tracked in session history

**Error Handling:**
- `return` - Return tool error message to LLM, continue loop
- `abort` - Stop execution immediately on tool error

### Tool Libraries

meowg1k provides three built-in libraries:

#### 1. Tools Library (`//lib/tools.star`)

18 general-purpose tools organized into categories:

```python
load("//lib/tools.star", 
     "file_reader", "file_writer", "file_exists", "list_directory",
     "search_text", "replace_text", 
     "shell_exec", "git_status", "git_diff",
     "calculator", "code_search", "llm_generate",
     "json_parse", "json_query",
     "current_time", "http_get", "http_post")

# Or use pre-defined tool sets
load("//lib/tools.star", "file_tools", "shell_tools", "code_tools", "all_tools")
```

**File Operations:**
- `file_reader` - Read file contents
- `file_writer` - Write file contents
- `file_exists` - Check if file exists
- `list_directory` - List files with glob patterns
- `search_text` - Grep for patterns
- `replace_text` - Replace text in files

**Shell/Git:**
- `shell_exec` - Execute shell commands
- `git_status` - Get repository status
- `git_diff` - Get diffs (staged or unstaged)

**Code/Data:**
- `code_search` - Semantic code search with embeddings
- `calculator` - Basic arithmetic operations
- `json_parse` - Parse and format JSON
- `json_query` - Query JSON with dot notation

**LLM:**
- `llm_generate` - Generate text with LLM (non-agentic)

**Utilities:**
- `current_time` - Get current time
- `http_get` / `http_post` - HTTP requests

#### 2. Planning Library (`//lib/planning.star`)

Task decomposition and planning utilities:

```python
load("//lib/planning.star", "create_plan", "decompose_task", "execute_plan")

# Generate a plan from a goal
plan_json = create_plan.handler(ctx)  # Or ctx.run("create_plan", goal="...")
plan = ctx.json.decode(plan_json)

# Break down complex tasks
decomposition = ctx.run("decompose_task", task="Complex task description")

# Or use the helper function
load("//lib/planning.star", "plan_and_execute")
result = plan_and_execute(ctx, goal="Analyze codebase", tools=[...])
```

#### 3. Memory Library (`//lib/memory.star`)

Context and state management:

```python
load("//lib/memory.star", 
     "save_context", "recall_context", "list_context",
     "summarize_history", "get_session_info",
     "remember", "recall")

# Save context for later
remember(ctx, "analyzed_files", file_list)

# Recall saved context
files = recall(ctx, "analyzed_files", default="[]")

# Summarize history to reduce context window
summary = ctx.run("summarize_history", limit=50)

# Get session metadata
info = ctx.run("get_session_info")
```

## Example Agents

### 1. Code Review Agent

`.meowg1k/commands/review-agent.star` - Comprehensive code review with context:

```bash
# Review staged changes
meow review-agent

# Review specific file
meow review-agent --file path/to/file.go

# Review all unstaged changes
meow review-agent --all
```

**Features:**
- Analyzes git diffs or specific files
- Uses semantic search to find related code
- Identifies issues across multiple dimensions (quality, bugs, performance, security)
- Provides actionable recommendations with examples
- Tracks review results in session metadata

**Key Techniques:**
- Combines `git_diff` with `code_search` for context
- Uses `file_reader` to examine related files
- Structured system prompt for consistent review format
- Session metadata stores review for later reference

### 2. Task Orchestrator Agent

`.meowg1k/commands/orchestrator-agent.star` - Autonomous task planning and execution:

```bash
# Execute complex task
meow orchestrator "analyze test coverage and suggest improvements"

# Multi-step workflow
meow orchestrator "find all TODO comments and create a report"

# Code transformation
meow orchestrator "refactor error handling to use custom error types"
```

**Features:**
- Breaks down high-level goals into actionable subtasks
- Gathers project context automatically
- Executes plan with full tool access
- Tracks progress via session metadata
- Provides comprehensive final report

**Key Techniques:**
- Uses LLM to decompose tasks into structured plans
- Stores plan and progress in session metadata (`remember`/`recall`)
- Combines planning with execution in single agentic loop
- Provides visibility into session hierarchy

## Session Management

### Context API (`ctx.session.*`)

Every agent has access to session management:

```python
# Get session info
session_id = ctx.session.id()
tool_name = ctx.session.tool_name()
status = ctx.session.status()  # "running" | "completed" | "failed"
parent_id = ctx.session.parent_id()

# Metadata (key-value storage)
ctx.session.set_metadata("key", "value")
value = ctx.session.get_metadata("key")
all_meta = ctx.session.get_all_metadata()

# Relationships
children = ctx.session.get_children()  # List of child sessions
parent = ctx.session.get_by_id(parent_id)

# Events (conversation history)
events = ctx.session.get_events(limit=50, offset=0)

# Global queries
all_sessions = ctx.session.list_all(tool_name="my-agent", status="completed", limit=100)
```

### Session Commands

Built-in commands for session inspection:

```bash
# List all sessions
meow sessions

# Filter sessions
meow sessions --tool orchestrator --status completed --limit 10

# Show detailed session info
meow show-session <session-id>

# Show current session (from within agent)
meow show-session
```

### Session Metadata Patterns

**Pattern 1: Progress Tracking**
```python
remember(ctx, "task_status", "planning")
remember(ctx, "subtasks_total", str(len(subtasks)))
remember(ctx, "subtasks_completed", "0")

# Later...
remember(ctx, "task_status", "executing")
remember(ctx, "subtasks_completed", str(completed_count))
```

**Pattern 2: Intermediate Results**
```python
remember(ctx, "files_analyzed", ctx.json.encode(file_list))
remember(ctx, "issues_found", str(issue_count))
remember(ctx, "final_report", report_text)
```

**Pattern 3: Context for Resume**
```python
# Save context for potential resume (future feature)
remember(ctx, "checkpoint", ctx.json.encode({
    "step": current_step,
    "data": processed_data,
    "next_action": "continue_from_here"
}))
```

## Best Practices

### 1. Tool Selection

**Principle:** Provide only necessary tools to reduce token usage and improve focus.

```python
# Good: Specific tool set for the task
code_analysis_tools = [file_reader, code_search, list_directory]
result = ctx.llm.agent_turn(tools=code_analysis_tools, ...)

# Avoid: All tools when only few needed
result = ctx.llm.agent_turn(tools=all_tools, ...)  # 18 tools = large schema
```

### 2. System Prompts

**Principle:** Clear, structured instructions improve agent behavior.

```python
system_prompt = """You are a [ROLE].

Your mission:
1. [Primary objective]
2. [Secondary objective]

Use the available tools to:
- [Tool usage guideline 1]
- [Tool usage guideline 2]

Guidelines:
- [Behavior guideline 1]
- [Behavior guideline 2]

Output format:
[Expected output structure]
"""
```

### 3. Error Handling

**Principle:** Choose error strategy based on task criticality.

```python
# Exploratory tasks: Continue on errors
result = ctx.llm.agent_turn(
    tools=[...],
    on_tool_error="return",  # LLM sees error, adapts strategy
    ...
)

# Critical tasks: Abort on errors
result = ctx.llm.agent_turn(
    tools=[...],
    on_tool_error="abort",   # Stop immediately on failure
    ...
)
```

### 4. Iteration Limits

**Principle:** Set appropriate limits based on task complexity.

```python
# Simple tasks: Low limit
result = ctx.llm.agent_turn(max_iterations=10, ...)

# Complex multi-step: Higher limit
result = ctx.llm.agent_turn(max_iterations=100, ...)
```

### 5. Session Metadata

**Principle:** Store structured data, not large blobs.

```python
# Good: Store metadata references
remember(ctx, "report_file", "/path/to/report.md")
remember(ctx, "issue_count", str(len(issues)))

# Avoid: Large data in metadata
remember(ctx, "full_report", gigantic_text)  # Use file_writer instead
```

## Advanced Patterns

### Pattern: Multi-Stage Agent

Break complex agents into distinct stages:

```python
def multi_stage_agent_handler(ctx):
    # Stage 1: Planning
    ctx.ui.info("Stage 1: Planning...")
    remember(ctx, "stage", "planning")
    
    plan = ctx.llm.chat(
        prompt="Create a plan for: " + goal,
        preset="smart",
        system="You are a planning expert..."
    )
    remember(ctx, "plan", plan)
    
    # Stage 2: Gathering
    ctx.ui.info("Stage 2: Gathering context...")
    remember(ctx, "stage", "gathering")
    
    context = ctx.llm.agent_turn(
        tools=[file_reader, code_search, list_directory],
        prompt="Gather context for: " + plan,
        preset="smart",
        max_iterations=20
    )
    remember(ctx, "context", context)
    
    # Stage 3: Execution
    ctx.ui.info("Stage 3: Executing...")
    remember(ctx, "stage", "executing")
    
    result = ctx.llm.agent_turn(
        tools=[file_writer, shell_exec],
        prompt="Execute plan with context: " + plan + "\n\n" + context,
        preset="smart",
        max_iterations=50
    )
    
    remember(ctx, "stage", "completed")
    return result
```

### Pattern: Tool Chaining

Create specialized tools that call other tools:

```python
def smart_search_handler(ctx):
    """Semantic search with automatic relevance filtering"""
    query = ctx.params["query"]
    
    # First: Semantic search
    results_json = code_search.handler(ctx)
    results = ctx.json.decode(results_json)
    
    # Then: Read and filter with LLM
    filtered = []
    for result in results:
        content = ctx.fs.read(result["path"])
        # Use LLM to check relevance
        is_relevant = ctx.llm.chat(
            prompt="Is this relevant to '" + query + "'?\n\n" + content,
            preset="smart",
            system="Answer only 'yes' or 'no'"
        )
        if "yes" in is_relevant.lower():
            filtered.append(result)
    
    return ctx.json.encode(filtered)
```

### Pattern: Delegating Sub-Agents

Create agents that invoke other specialized agents:

```python
def coordinator_handler(ctx):
    """Coordinates multiple specialized agents"""
    task = ctx.params["task"]
    
    # Analyze what needs to be done
    analysis = ctx.llm.chat(
        prompt="What agents are needed for: " + task,
        preset="smart",
        system="Available: code-review, search, refactor"
    )
    
    # Invoke appropriate agents
    if "review" in analysis:
        review = ctx.run("review-agent", file="target.go")
        remember(ctx, "review_result", review)
    
    if "search" in analysis:
        findings = ctx.run("code-search", query="error handling")
        remember(ctx, "search_result", findings)
    
    # Synthesize results
    return "Coordination complete. See session metadata for results."
```

## Testing Agents

### Manual Testing

```bash
# Test with simple goals
meow orchestrator "list all Go files"

# Test with moderate complexity
meow orchestrator "find functions longer than 100 lines"

# Test with complex workflows
meow orchestrator "analyze error handling patterns and suggest improvements"
```

### Debugging

```bash
# Run with verbose session tracking
meow orchestrator "task" && meow sessions

# Examine session details
meow show-session <session-id> --events --event-limit 100

# Check metadata
meow show-session <session-id>  # Shows all metadata
```

### Testing Custom Agents

Create test commands in `.meowg1k/commands/test-my-agent.star`:

```python
def test_my_agent_handler(ctx):
    """Test my custom agent"""
    
    # Test case 1
    ctx.ui.info("Test 1: Simple goal")
    result = ctx.run("my-agent", goal="simple task")
    assert "expected" in result, "Test 1 failed"
    
    # Test case 2
    ctx.ui.info("Test 2: Complex goal")
    result = ctx.run("my-agent", goal="complex task")
    session = ctx.session.get_children()[-1]
    assert session["status"] == "completed", "Test 2 failed"
    
    ctx.ui.success("All tests passed!")

meow.tool(
    name="test-my-agent",
    description="Test my custom agent",
    params={},
    handler=test_my_agent_handler
)
```

## References

- **API_REFERENCE.md** - Complete Starlark API documentation
- **docs/agents/starlark-system.md** - Starlark extension system details
- **docs/agents/architecture.md** - Hexagonal architecture overview
- **.meowg1k/lib/tools.star** - Built-in tools reference implementation
- **.meowg1k/lib/planning.star** - Planning utilities implementation
- **.meowg1k/lib/memory.star** - Memory utilities implementation
- **.meowg1k/commands/review-agent.star** - Code review agent example
- **.meowg1k/commands/orchestrator-agent.star** - Task orchestrator example

## Next Steps

1. **Explore example agents** - Study `review-agent.star` and `orchestrator-agent.star`
2. **Build a simple agent** - Start with a focused, single-purpose agent
3. **Test with real tasks** - Use your agent on actual project work
4. **Iterate and refine** - Improve system prompts and tool selection
5. **Share your agents** - Contribute useful agents back to the project
