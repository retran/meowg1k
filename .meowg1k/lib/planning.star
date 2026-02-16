"""
Planning Library for meowg1k

This library provides utilities for task planning, decomposition, and execution
tracking in agentic workflows. Use it to break down complex goals into actionable
steps and execute them systematically.

## Quick Start

```python
load("//lib/planning.star", "create_plan", "execute_plan", "decompose_task")

def handler(ctx):
    # Create a plan from a high-level goal
    plan_json = ctx.run(create_plan, 
        goal="Analyze codebase and generate report",
        context="Focus on error handling patterns")
    
    # Execute the plan with available tools
    result = ctx.run(execute_plan, 
        plan=plan_json,
        tools=ctx.json.encode([file_reader, code_search]))
    
    ctx.ui.success("Plan executed successfully!")
```

## Available Tools

### Planning Tools
- `create_plan` - Generate detailed task plan from high-level goal
- `decompose_task` - Break complex task into smaller subtasks
- `execute_plan` - Execute task plan using agentic loop

### Helper Functions
- `plan_and_execute(ctx, goal, tools, context)` - High-level wrapper for plan + execute

### Tool Sets
- Import individual tools as needed (no pre-defined tool set)

## API Reference

### create_plan

Generate a detailed, actionable task plan from a high-level goal using LLM.

**Parameters:**
- `goal` (string, required): High-level goal to plan for
- `context` (string, default=""): Additional context for planning

**Returns:** JSON string - Array of plan steps, each with:
- `action`: What to do
- `tool`: Which tool to use (if applicable)  
- `reasoning`: Why this step is needed

**Example:**
```python
plan_json = ctx.run(create_plan,
    goal="Find and fix TypeScript errors in src/",
    context="Focus on type safety issues")

plan = ctx.json.decode(plan_json)
for step in plan:
    ctx.ui.info("Step: " + step["action"])
    ctx.ui.info("  Tool: " + step.get("tool", "N/A"))
    ctx.ui.info("  Why: " + step["reasoning"])
```

**Sample Output:**
```json
[
  {
    "action": "List all TypeScript files in src/",
    "tool": "list_directory",
    "reasoning": "Need to identify all files to check"
  },
  {
    "action": "Search for type errors",
    "tool": "code_search",
    "reasoning": "Find locations of type issues"
  },
  {
    "action": "Fix each error",
    "tool": "file_writer",
    "reasoning": "Apply corrections to files"
  }
]
```

**Performance:** Uses "smart" preset for planning (may take 2-5 seconds)

---

### execute_plan

Execute a task plan step-by-step using an agentic loop with available tools.

**Parameters:**
- `plan` (string, required): JSON-encoded plan to execute (from create_plan)
- `tools` (string, default="[]"): JSON-encoded array of tools to use

**Returns:** string - Summary of execution results

**Example:**
```python
load("//lib/file_ops.star", "file_reader", "file_writer", "search_text")

# First create a plan
plan_json = ctx.run(create_plan, goal="Refactor error handling")

# Prepare tools for execution
tools_json = ctx.json.encode([file_reader, file_writer, search_text])

# Execute the plan
result = ctx.run(execute_plan, plan=plan_json, tools=tools_json)
ctx.ui.info(result)
```

**Behavior:**
- Runs agentic loop with max 50 iterations
- Uses "smart" preset for execution
- LLM follows plan steps sequentially
- Returns summary of what was accomplished

**Performance:** 
- Execution time depends on plan complexity
- Each tool invocation adds latency
- Consider breaking very long plans into phases

---

### decompose_task

Break down a complex task into smaller, manageable subtasks with dependencies.

**Parameters:**
- `task` (string, required): Task to decompose
- `max_depth` (int, default=2): Maximum decomposition depth

**Returns:** JSON string - Task decomposition with:
- `task`: Original task
- `subtasks`: Array of subtask descriptions
- `dependencies`: Which subtasks depend on others (optional)

**Example:**
```python
result = ctx.run(decompose_task,
    task="Implement user authentication system",
    max_depth=2)

decomposition = ctx.json.decode(result)
ctx.ui.info("Task: " + decomposition["task"])

for i, subtask in enumerate(decomposition["subtasks"]):
    ctx.ui.info("  %d. %s" % (i + 1, subtask))
```

**Sample Output:**
```json
{
  "task": "Implement user authentication system",
  "subtasks": [
    "Design user database schema with credentials table",
    "Implement password hashing with bcrypt",
    "Create login endpoint with JWT token generation",
    "Add authentication middleware for protected routes",
    "Write tests for authentication flow"
  ],
  "dependencies": {
    "2": ["1"],
    "3": ["2"],
    "4": ["3"]
  }
}
```

**Use Cases:**
- Breaking down large features
- Sprint planning
- Task estimation
- Dependency analysis

---

## Advanced Usage

### Example 1: Code Review Workflow

```python
load("//lib/planning.star", "create_plan", "execute_plan")
load("//lib/git.star", "git_diff", "git_status")
load("//lib/file_ops.star", "file_reader")
load("//lib/llm.star", "llm_generate")

def review_handler(ctx):
    # Get current changes
    diff = ctx.run(git_diff)
    
    # Create review plan
    plan_json = ctx.run(create_plan,
        goal="Review code changes for quality and bugs",
        context="Current diff:\n" + diff)
    
    # Execute review with relevant tools
    tools = ctx.json.encode([file_reader, git_diff, llm_generate])
    review = ctx.run(execute_plan, plan=plan_json, tools=tools)
    
    ctx.ui.success("Review complete!")
    return review
```

### Example 2: Multi-Phase Refactoring

```python
load("//lib/planning.star", "decompose_task", "create_plan", "execute_plan")
load("//lib/file_ops.star", "file_tools")
load("//lib/code_search.star", "code_search")

def refactor_handler(ctx):
    task = "Refactor error handling across entire codebase"
    
    # Phase 1: Decompose into subtasks
    ctx.ui.info("Breaking down task...")
    decomposition_json = ctx.run(decompose_task, task=task, max_depth=3)
    decomposition = ctx.json.decode(decomposition_json)
    
    # Phase 2: Create detailed plan for each subtask
    all_plans = []
    for subtask in decomposition["subtasks"]:
        plan = ctx.run(create_plan, goal=subtask)
        all_plans.append(plan)
    
    # Phase 3: Execute each plan
    tools = ctx.json.encode(file_tools + [code_search])
    results = []
    
    for i, plan in enumerate(all_plans):
        ctx.ui.info("Executing plan %d/%d..." % (i + 1, len(all_plans)))
        result = ctx.run(execute_plan, plan=plan, tools=tools)
        results.append(result)
    
    return "\n\n".join(results)
```

### Example 3: Data Analysis Pipeline

```python
load("//lib/planning.star", "create_plan", "execute_plan")
load("//lib/file_ops.star", "list_directory", "file_reader")
load("//lib/json.star", "json_parse", "json_query")
load("//lib/math.star", "calculator")

def analyze_handler(ctx):
    # Plan data analysis
    plan_json = ctx.run(create_plan,
        goal="Analyze JSON log files and calculate statistics",
        context="Data is in logs/ directory, calculate error rates")
    
    # Execute with data tools
    tools = ctx.json.encode([
        list_directory, file_reader, 
        json_parse, json_query, calculator
    ])
    
    result = ctx.run(execute_plan, plan=plan_json, tools=tools)
    
    ctx.ui.info("Analysis results:")
    ctx.ui.info(result)
```

### Example 4: Incremental Migration

```python
load("//lib/planning.star", "decompose_task", "plan_and_execute")
load("//lib/file_ops.star", "file_tools")
load("//lib/git.star", "git_tools")

def migrate_handler(ctx):
    # Decompose migration task
    task = "Migrate from REST API to GraphQL"
    decomposition_json = ctx.run(decompose_task, task=task)
    decomposition = ctx.json.decode(decomposition_json)
    
    # Execute first subtask as proof of concept
    first_subtask = decomposition["subtasks"][0]
    ctx.ui.info("Starting with: " + first_subtask)
    
    # Use helper function for quick plan + execute
    all_tools = file_tools + git_tools
    result = plan_and_execute(ctx, first_subtask, all_tools)
    
    ctx.ui.success("First phase complete!")
    ctx.ui.info("Remaining tasks: %d" % (len(decomposition["subtasks"]) - 1))
    
    return result
```

### Example 5: Test Generation

```python
load("//lib/planning.star", "create_plan", "execute_plan")
load("//lib/file_ops.star", "file_reader", "file_writer", "list_directory")
load("//lib/code_search.star", "code_search")

def generate_tests_handler(ctx):
    target_file = ctx.params["file"]
    
    # Create test generation plan
    plan_json = ctx.run(create_plan,
        goal="Generate comprehensive unit tests for " + target_file,
        context="Use table-driven tests, aim for 80%+ coverage")
    
    # Execute with code intelligence tools
    tools = ctx.json.encode([
        file_reader, file_writer, list_directory, code_search
    ])
    
    tests = ctx.run(execute_plan, plan=plan_json, tools=tools)
    
    ctx.ui.success("Tests generated for " + target_file)
    return tests
```

### Example 6: Documentation Generation

```python
load("//lib/planning.star", "decompose_task", "create_plan", "execute_plan")
load("//lib/file_ops.star", "list_directory", "file_reader", "file_writer")
load("//lib/llm.star", "llm_generate")

def document_handler(ctx):
    # Decompose documentation task
    task_json = ctx.run(decompose_task,
        task="Create comprehensive API documentation",
        max_depth=2)
    
    task = ctx.json.decode(task_json)
    
    # Generate plan for each documentation section
    for subtask in task["subtasks"]:
        ctx.ui.info("Planning: " + subtask)
        
        plan = ctx.run(create_plan, goal=subtask)
        
        # Execute documentation generation
        tools = ctx.json.encode([
            list_directory, file_reader, file_writer, llm_generate
        ])
        
        ctx.run(execute_plan, plan=plan, tools=tools)
    
    ctx.ui.success("Documentation complete!")
```

### Example 7: Bug Hunting

```python
load("//lib/planning.star", "create_plan", "execute_plan")
load("//lib/code_search.star", "code_search")
load("//lib/file_ops.star", "file_reader", "search_text")
load("//lib/shell.star", "shell_exec")

def bug_hunt_handler(ctx):
    bug_type = ctx.params.get("type", "memory leaks")
    
    # Plan bug hunting strategy
    plan_json = ctx.run(create_plan,
        goal="Find potential " + bug_type + " in codebase",
        context="Use static analysis and pattern matching")
    
    # Execute with search and analysis tools
    tools = ctx.json.encode([
        code_search, file_reader, search_text, shell_exec
    ])
    
    findings = ctx.run(execute_plan, plan=plan_json, tools=tools)
    
    ctx.ui.info("Bug hunt complete:")
    ctx.ui.info(findings)
```

### Example 8: Performance Optimization

```python
load("//lib/planning.star", "decompose_task", "plan_and_execute")
load("//lib/file_ops.star", "file_tools")
load("//lib/shell.star", "shell_exec")
load("//lib/code_search.star", "code_search")

def optimize_handler(ctx):
    # Decompose optimization task
    decomposition_json = ctx.run(decompose_task,
        task="Optimize application performance by 50%")
    
    decomposition = ctx.json.decode(decomposition_json)
    
    # Execute optimization in phases
    tools = file_tools + [shell_exec, code_search]
    
    for i, subtask in enumerate(decomposition["subtasks"]):
        ctx.ui.info("Phase %d: %s" % (i + 1, subtask))
        
        # Plan and execute this optimization phase
        result = plan_and_execute(ctx, subtask, tools)
        
        # Show progress
        ctx.ui.success("Phase %d complete" % (i + 1))
```

### Example 9: Custom Workflow with Planning

```python
load("//lib/planning.star", "create_plan", "execute_plan")
load("//lib/memory.star", "save_context", "recall_context")

def workflow_handler(ctx):
    # Recall previous context
    previous_work = recall_context.handler(ctx)
    
    # Create plan considering previous work
    context = "Previous work: " + previous_work if previous_work else ""
    plan_json = ctx.run(create_plan,
        goal=ctx.params["goal"],
        context=context)
    
    # Save plan for later reference
    ctx.run(save_context, key="current_plan", value=plan_json)
    
    # Execute plan
    tools_json = ctx.params.get("tools", "[]")
    result = ctx.run(execute_plan, plan=plan_json, tools=tools_json)
    
    # Save results
    ctx.run(save_context, key="last_result", value=result)
    
    return result
```

### Example 10: Interactive Planning

```python
load("//lib/planning.star", "create_plan", "decompose_task")

def interactive_planner_handler(ctx):
    goal = ctx.params["goal"]
    
    # First decompose to show options
    decomposition_json = ctx.run(decompose_task, task=goal)
    decomposition = ctx.json.decode(decomposition_json)
    
    ctx.ui.info("Task breakdown:")
    for i, subtask in enumerate(decomposition["subtasks"]):
        ctx.ui.info("  %d. %s" % (i + 1, subtask))
    
    # Ask user which subtask to start with (in practice, use ui.prompt)
    ctx.ui.info("\nCreating detailed plan for first subtask...")
    
    first_subtask = decomposition["subtasks"][0]
    plan_json = ctx.run(create_plan, 
        goal=first_subtask,
        context="This is phase 1 of: " + goal)
    
    plan = ctx.json.decode(plan_json)
    
    ctx.ui.info("\nDetailed plan:")
    for i, step in enumerate(plan):
        ctx.ui.info("  Step %d: %s" % (i + 1, step["action"]))
        ctx.ui.info("    Tool: %s" % step.get("tool", "N/A"))
        ctx.ui.info("    Why: %s" % step["reasoning"])
    
    return plan_json
```

## Error Handling

### Common Errors

**1. Invalid JSON in plan**
```python
# Error: Plan format is invalid
# Solution: Always use create_plan to generate plans

# Bad - manually constructing plan
plan = '{"action": "do something"}'  # Invalid format

# Good - use create_plan
plan = ctx.run(create_plan, goal="Do something")
```

**2. Missing tools in execute_plan**
```python
# Error: Tool not found during execution
# Solution: Ensure all required tools are provided

load("//lib/file_ops.star", "file_reader")

# Bad - plan requires tools not provided
tools = ctx.json.encode([file_reader])  # Missing other needed tools
result = ctx.run(execute_plan, plan=plan_json, tools=tools)

# Good - provide comprehensive tool set
load("//lib/file_ops.star", "file_tools")
tools = ctx.json.encode(file_tools)  # All file operations available
result = ctx.run(execute_plan, plan=plan_json, tools=tools)
```

**3. Empty or vague goals**
```python
# Error: LLM produces generic or unhelpful plan
# Solution: Provide specific, actionable goals with context

# Bad - vague goal
plan = ctx.run(create_plan, goal="Fix stuff")

# Good - specific goal with context
plan = ctx.run(create_plan,
    goal="Fix TypeScript compilation errors in src/components/",
    context="Errors are related to missing type definitions")
```

**4. Plan execution timeout**
```python
# Error: Execution exceeds max iterations (50)
# Solution: Break plan into smaller phases

# Bad - trying to do too much in one plan
result = ctx.run(execute_plan,
    plan=massive_plan_json,
    tools=all_tools)  # May hit iteration limit

# Good - execute in phases
for phase_plan in phase_plans:
    result = ctx.run(execute_plan,
        plan=phase_plan,
        tools=relevant_tools)
    ctx.ui.info("Phase complete")
```

### Error Recovery Pattern

```python
def safe_execute_handler(ctx):
    plan_json = ctx.run(create_plan, goal=ctx.params["goal"])
    plan = ctx.json.decode(plan_json)
    
    # Validate plan structure
    if not plan or len(plan) == 0:
        ctx.ui.error("Failed to generate valid plan")
        return "Error: Could not create plan"
    
    # Show plan to user before executing
    ctx.ui.info("Plan has %d steps" % len(plan))
    for i, step in enumerate(plan):
        if "action" not in step:
            ctx.ui.error("Invalid step %d: missing action" % (i + 1))
            return "Error: Invalid plan format"
    
    # Execute with error handling
    tools_json = ctx.params.get("tools", "[]")
    
    try:
        result = ctx.run(execute_plan, plan=plan_json, tools=tools_json)
        ctx.ui.success("Execution complete")
        return result
    except:
        ctx.ui.error("Execution failed")
        return "Error during plan execution"
```

## Performance Tips

### 1. Cache Plans for Reuse

```python
# Instead of regenerating plans, save and reuse
load("//lib/memory.star", "save_context", "recall_context")

def cached_planning_handler(ctx):
    goal = ctx.params["goal"]
    
    # Check for cached plan
    cached_plan = ctx.run(recall_context, key="plan_" + goal)
    
    if cached_plan != "":
        ctx.ui.info("Using cached plan")
        plan_json = cached_plan
    else:
        ctx.ui.info("Generating new plan...")
        plan_json = ctx.run(create_plan, goal=goal)
        ctx.run(save_context, key="plan_" + goal, value=plan_json)
    
    return plan_json
```

### 2. Use Parallel Execution for Independent Tasks

```python
# When decomposed tasks are independent, run in parallel
decomposition_json = ctx.run(decompose_task, task="Analyze codebase")
decomposition = ctx.json.decode(decomposition_json)

# Check dependencies
deps = decomposition.get("dependencies", {})

# Tasks without dependencies can run in parallel
# (Note: Actual parallel execution depends on your orchestration)
independent_tasks = []
for i, subtask in enumerate(decomposition["subtasks"]):
    if str(i) not in deps:
        independent_tasks.append(subtask)

ctx.ui.info("Can parallelize %d tasks" % len(independent_tasks))
```

### 3. Limit Plan Complexity

```python
# For large goals, use multi-level decomposition
def hierarchical_planning_handler(ctx):
    # Level 1: High-level decomposition
    high_level = ctx.run(decompose_task, 
        task=ctx.params["goal"],
        max_depth=1)  # Keep it simple
    
    phases = ctx.json.decode(high_level)["subtasks"]
    
    # Level 2: Detailed plans per phase (not all at once)
    current_phase = phases[0]
    plan = ctx.run(create_plan, goal=current_phase)
    
    return plan  # Execute one phase at a time
```

### 4. Choose Appropriate Presets

```python
# Planning uses "smart" preset by default - it's slower but more accurate
# For simple tasks, you can use custom LLM calls with "fast" preset

def quick_plan_handler(ctx):
    # For simple, repetitive tasks use fast preset directly
    prompt = "Break down: " + ctx.params["goal"]
    
    quick_plan = ctx.llm.generate(
        prompt=prompt,
        system="List 3-5 action steps as JSON array",
        preset="fast"  # Much faster
    )
    
    return quick_plan
```

## Integration Examples

### With memory.star - Context-Aware Planning

```python
load("//lib/planning.star", "create_plan", "execute_plan")
load("//lib/memory.star", "save_context", "recall_context", "summarize_history")

def context_aware_handler(ctx):
    # Recall previous work
    previous = ctx.run(recall_context, key="last_task")
    
    # Summarize history for context
    history = ctx.run(summarize_history, limit=20)
    
    # Create plan with full context
    context = "Previous task: " + previous + "\n\nHistory: " + history
    plan = ctx.run(create_plan, 
        goal=ctx.params["goal"],
        context=context)
    
    # Execute and save results
    result = ctx.run(execute_plan, plan=plan, tools="[]")
    ctx.run(save_context, key="last_task", value=ctx.params["goal"])
    
    return result
```

### With llm.star - Custom Planning Strategies

```python
load("//lib/planning.star", "decompose_task")
load("//lib/llm.star", "llm_generate")

def custom_strategy_handler(ctx):
    # First decompose the task
    decomposition_json = ctx.run(decompose_task, task=ctx.params["task"])
    decomposition = ctx.json.decode(decomposition_json)
    
    # Use LLM to prioritize subtasks
    subtasks_text = "\n".join([
        "%d. %s" % (i + 1, t) 
        for i, t in enumerate(decomposition["subtasks"])
    ])
    
    prioritization = ctx.run(llm_generate,
        prompt="Prioritize these tasks by impact:\n" + subtasks_text,
        preset="smart")
    
    ctx.ui.info("Prioritized plan:")
    ctx.ui.info(prioritization)
    
    return prioritization
```

### With file_ops.star + git.star - Code Analysis

```python
load("//lib/planning.star", "create_plan", "execute_plan")
load("//lib/file_ops.star", "file_tools")
load("//lib/git.star", "git_tools")

def code_analysis_handler(ctx):
    # Create analysis plan
    plan = ctx.run(create_plan,
        goal="Analyze recent code changes for issues",
        context="Check for security vulnerabilities and code smells")
    
    # Execute with file and git tools
    all_tools = file_tools + git_tools
    tools_json = ctx.json.encode(all_tools)
    
    analysis = ctx.run(execute_plan, plan=plan, tools=tools_json)
    
    return analysis
```

## Security Considerations

1. **Validate tool inputs** - Plans may generate tool calls with unexpected parameters
2. **Limit tool access** - Only provide tools necessary for the task
3. **Review generated plans** - For sensitive operations, show plan before execution
4. **Avoid exposing secrets** - Don't include sensitive data in context strings

```python
# Example: Safe planning with tool restrictions
def safe_planner_handler(ctx):
    # Only allow read-only tools
    load("//lib/file_ops.star", "file_reader", "list_directory")
    load("//lib/code_search.star", "code_search")
    
    safe_tools = [file_reader, list_directory, code_search]
    # Deliberately exclude file_writer, shell_exec, etc.
    
    plan = ctx.run(create_plan, goal=ctx.params["goal"])
    result = ctx.run(execute_plan, 
        plan=plan,
        tools=ctx.json.encode(safe_tools))
    
    return result
```

## See Also

- **memory.star** - Session memory and context management
- **llm.star** - LLM text generation for custom planning
- **file_ops.star** - File operations for plan execution
- **code_search.star** - Semantic search for code-related plans
- **LIBRARY_INDEX.md** - Complete library reference

## Helper Functions Reference

### plan_and_execute(ctx, goal, tools, context="")

High-level helper that creates and executes a plan in one call.

**Parameters:**
- `ctx`: Handler context
- `goal`: High-level goal string
- `tools`: List of tool objects (not JSON-encoded)
- `context`: Optional context string

**Returns:** Execution result string

**Example:**
```python
load("//lib/planning.star", "plan_and_execute")
load("//lib/file_ops.star", "file_tools")

def quick_task_handler(ctx):
    result = plan_and_execute(
        ctx,
        goal="Find all TODO comments in code",
        tools=file_tools,
        context="Focus on high-priority TODOs"
    )
    return result
```

**Note:** This function combines create_plan and execute_plan with automatic progress reporting.

---

**Last Updated:** 2024  
**Status:** Production Ready  
**Documentation:** Complete ✅
"""

def create_plan_handler(ctx):
    """Generate a task plan using LLM"""
    goal = ctx.params["goal"]
    context = ctx.params.get("context", "")
    
    system_prompt = """You are a task planning assistant. Given a goal, break it down into specific, actionable steps.
Return your plan as a JSON array of steps, where each step has:
- "action": What to do
- "tool": Which tool to use (if applicable)
- "reasoning": Why this step is needed

Example:
[
  {"action": "Read main.go file", "tool": "file_reader", "reasoning": "Need to understand entry point"},
  {"action": "Search for API endpoints", "tool": "code_search", "reasoning": "Map out the API surface"}
]
"""
    
    prompt = "Goal: " + goal
    if context != "":
        prompt = prompt + "\n\nContext:\n" + context
    
    response = ctx.llm.generate(prompt=prompt, system=system_prompt, preset="smart")
    
    # Try to parse as JSON
    plan = ctx.json.decode(response)
    return ctx.json.encode(plan)

def execute_plan_handler(ctx):
    """Execute a plan step by step using agentic loop"""
    plan_json = ctx.params["plan"]
    tools_list = ctx.params.get("tools", [])
    
    # Parse plan
    plan = ctx.json.decode(plan_json)
    
    # Build execution prompt
    steps_text = ""
    for i, step in enumerate(plan):
        steps_text = steps_text + str(i + 1) + ". " + step.get("action", "Unknown") + "\n"
    
    system_prompt = """You are a task executor. Follow the plan step by step, using the available tools.
After completing all steps, provide a summary of what was accomplished.

Plan:
""" + steps_text
    
    prompt = "Execute the plan above step by step. Use the available tools as needed."
    
    result = ctx.llm.agentic(
        tools=tools_list,
        prompt=prompt,
        system=system_prompt,
        preset="smart",
        max_iterations=50
    )
    
    return result

def decompose_task_handler(ctx):
    """Decompose a complex task into subtasks"""
    task = ctx.params["task"]
    max_depth = ctx.params.get("max_depth", 2)
    
    system_prompt = """Break down the given task into smaller, manageable subtasks.
Return a JSON structure with:
- "task": Original task
- "subtasks": Array of subtask descriptions
- "dependencies": Which subtasks depend on others (optional)

Keep decomposition practical and actionable."""
    
    prompt = "Decompose this task: " + task
    
    response = ctx.llm.generate(prompt=prompt, system=system_prompt, preset="smart")
    return response

# Tool definitions
create_plan = meow.tool(
    name="create_plan",
    description="Generate a detailed task plan from a high-level goal using LLM",
    params={
        "goal": meow.param("string", description="The high-level goal to plan for", required=True),
        "context": meow.param("string", description="Additional context for planning", default=""),
    },
    handler=create_plan_handler,
)

execute_plan = meow.tool(
    name="execute_plan",
    description="Execute a task plan using agentic loop with available tools",
    params={
        "plan": meow.param("string", description="JSON-encoded plan to execute", required=True),
        "tools": meow.param("string", description="List of tools to use", default="[]"),
    },
    handler=execute_plan_handler,
)

decompose_task = meow.tool(
    name="decompose_task",
    description="Break down a complex task into smaller subtasks",
    params={
        "task": meow.param("string", description="Task to decompose", required=True),
        "max_depth": meow.param("int", description="Maximum decomposition depth", default=2),
    },
    handler=decompose_task_handler,
)

# Helper function for creating plans
def plan_and_execute(ctx, goal, tools, context=""):
    """High-level helper: plan and execute a goal in one step"""
    ctx.ui.info("Planning: " + goal)
    
    # Create plan
    plan_json = create_plan.handler(ctx)
    plan = ctx.json.decode(plan_json)
    
    ctx.ui.info("Plan created with " + str(len(plan)) + " steps")
    for i, step in enumerate(plan):
        ctx.ui.info("  " + str(i + 1) + ". " + step.get("action", "Unknown"))
    
    # Execute plan
    ctx.ui.info("Executing plan...")
    result = ctx.llm.agentic(
        tools=tools,
        prompt="Execute this plan: " + plan_json,
        system="You are a task executor. Follow the plan step by step.",
        preset="smart",
        max_iterations=50
    )
    
    return result
