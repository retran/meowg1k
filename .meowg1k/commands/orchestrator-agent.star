"""
Task Orchestrator Agent

This agent breaks down complex high-level tasks into actionable steps and
executes them autonomously using available tools.

Features:
- Automatic task decomposition and planning
- Intelligent tool selection
- Progress tracking via session metadata
- Memory of completed steps
- Adaptive execution with error recovery

Usage:
    meow orchestrator "analyze the codebase and generate a summary report"
    meow orchestrator "find all TODO comments and create GitHub issues"
    meow orchestrator "refactor error handling to use custom error types"
"""

load("//lib/file_ops.star", "file_reader", "file_writer", "list_directory", "search_text")
load("//lib/code_search.star", "code_search")
load("//lib/git.star", "git_status", "git_diff")
load("//lib/shell.star", "shell_exec")
load("//lib/json.star", "json_parse")
load("//lib/math.star", "calculator")
load("//lib/planning.star", "create_plan", "decompose_task")
load("//lib/memory.star", "save_context", "recall_context", "list_context", "summarize_history", "remember", "recall")

def orchestrator_handler(ctx):
    """Execute task orchestrator agent"""
    
    # Get task from arguments
    args = ctx.args
    if len(args) == 0:
        ctx.ui.error("Please provide a task description")
        ctx.ui.info("")
        ctx.ui.info("Usage: meow orchestrator \"<task description>\"")
        ctx.ui.info("")
        ctx.ui.info("Examples:")
        ctx.ui.info("  meow orchestrator \"analyze test coverage and suggest improvements\"")
        ctx.ui.info("  meow orchestrator \"find all API endpoints and document them\"")
        ctx.ui.info("  meow orchestrator \"audit error handling across the codebase\"")
        return
    
    task_description = " ".join(args)
    
    ctx.ui.info("🎯 Task Orchestrator Agent")
    ctx.ui.info("=" * 80)
    ctx.ui.info("")
    ctx.ui.info("Task: " + task_description)
    ctx.ui.info("")
    ctx.ui.info("=" * 80)
    ctx.ui.info("")
    
    # Save task to memory
    remember(ctx, "task_description", task_description)
    remember(ctx, "task_status", "planning")
    
    # Step 1: Decompose the task
    ctx.ui.info("📋 Step 1: Breaking down the task...")
    ctx.ui.info("")
    
    decomposition_prompt = """Analyze this task and break it down into specific, actionable subtasks:

Task: """ + task_description + """

Consider:
1. What information needs to be gathered?
2. What analysis or processing is required?
3. What outputs or artifacts should be created?
4. What dependencies exist between steps?

Return a JSON structure with:
{
  "subtasks": [
    {"id": 1, "description": "...", "tools_needed": ["tool1", "tool2"], "depends_on": []},
    {"id": 2, "description": "...", "tools_needed": ["tool3"], "depends_on": [1]}
  ],
  "estimated_complexity": "low|medium|high"
}"""
    
    decomposition = ctx.llm.generate(
        prompt=decomposition_prompt,
        system="You are a task planning expert. Break down tasks into clear, executable steps.",
        preset="smart"
    )
    
    ctx.ui.info("Task decomposition:")
    ctx.ui.markdown(decomposition)
    ctx.ui.info("")
    
    # Parse decomposition
    plan = ctx.json.decode(decomposition)
    subtasks = plan.get("subtasks", [])
    complexity = plan.get("estimated_complexity", "medium")
    
    remember(ctx, "plan", decomposition)
    remember(ctx, "subtask_count", str(len(subtasks)))
    remember(ctx, "completed_subtasks", "0")
    
    ctx.ui.info("📊 Planning complete:")
    ctx.ui.info("  • Total subtasks: " + str(len(subtasks)))
    ctx.ui.info("  • Estimated complexity: " + complexity)
    ctx.ui.info("")
    
    # Step 2: Gather context
    ctx.ui.info("🔍 Step 2: Gathering context...")
    ctx.ui.info("")
    
    # Get project structure
    ctx.ui.info("  • Analyzing project structure...")
    project_files = ctx.fs.glob("**/*.go")[:20]  # First 20 Go files for context
    
    # Get git status
    ctx.ui.info("  • Checking repository status...")
    status = ctx.git.status()
    
    context_summary = "Project has " + str(len(project_files)) + " Go files. "
    if status:
        status_obj = ctx.json.decode(status)
        if "branch" in status_obj:
            context_summary = context_summary + "On branch: " + status_obj["branch"] + ". "
    
    remember(ctx, "project_context", context_summary)
    ctx.ui.success("  ✓ Context gathered")
    ctx.ui.info("")
    
    # Step 3: Execute the plan
    ctx.ui.info("⚙️  Step 3: Executing plan with agentic loop...")
    ctx.ui.info("")
    
    remember(ctx, "task_status", "executing")
    
    # Build comprehensive system prompt
    system_prompt = """You are an autonomous task executor with access to various tools.

TASK: """ + task_description + """

PLAN:
""" + decomposition + """

CONTEXT:
""" + context_summary + """

Your mission:
1. Execute each subtask in the plan systematically
2. Use the available tools to gather information, analyze code, and create artifacts
3. Track your progress by saving important findings
4. Handle errors gracefully and adapt as needed
5. Provide a comprehensive final report

Guidelines:
- Use file_reader to examine code
- Use code_search for semantic queries
- Use list_directory to explore structure
- Use search_text for pattern matching
- Use file_writer to create reports or artifacts
- Use git tools to understand changes
- Save key findings and decisions as you go

After completing all subtasks, provide a detailed summary of:
- What was accomplished
- Key findings
- Any issues encountered
- Recommendations or next steps"""
    
    user_prompt = """Execute the plan above step by step. Use the available tools intelligently.

For each subtask:
1. Clearly state what you're doing
2. Use appropriate tools to gather information
3. Analyze and synthesize findings
4. Document key insights

After completing all steps, provide a comprehensive final report."""
    
    # Assemble all available tools
    all_tools = [
        file_reader, file_writer, code_search, git_status, git_diff,
        list_directory, search_text, shell_exec, json_parse, calculator,
        save_context, recall_context, list_context
    ]
    
    # Execute with agentic loop
    result = ctx.llm.agentic(
        tools=all_tools,
        prompt=user_prompt,
        system=system_prompt,
        preset="smart",
        max_iterations=100,
        on_tool_error="return"
    )
    
    # Step 4: Present results
    ctx.ui.info("")
    ctx.ui.info("=" * 80)
    ctx.ui.info("TASK ORCHESTRATOR RESULTS")
    ctx.ui.info("=" * 80)
    ctx.ui.info("")
    ctx.ui.markdown(result)
    ctx.ui.info("")
    ctx.ui.info("=" * 80)
    ctx.ui.info("")
    
    # Save results
    remember(ctx, "task_status", "completed")
    remember(ctx, "final_result", result)
    
    # Show session summary
    session_id = ctx.session.id()
    children = ctx.session.get_children()
    
    ctx.ui.success("✓ Task orchestration complete!")
    ctx.ui.info("")
    ctx.ui.info("Session details:")
    ctx.ui.info("  • Session ID: " + session_id)
    ctx.ui.info("  • Child sessions: " + str(len(children)))
    ctx.ui.info("  • Final status: completed")
    ctx.ui.info("")
    ctx.ui.info("To view full session details:")
    ctx.ui.info("  meow show-session " + session_id)

# Register the command as a tool first
orchestrator_tool = meow.tool(
    name="orchestrator",
    description="Autonomous task orchestrator that plans and executes complex workflows",
    params={},
    handler=orchestrator_handler,
)

# Register as command
meow.command(orchestrator_tool)
