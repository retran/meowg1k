"""
Code Review Agent

This agent performs comprehensive code reviews by:
1. Analyzing git changes or specific files
2. Understanding context through semantic search
3. Identifying issues and suggesting improvements
4. Providing detailed, actionable feedback

Usage:
    meow review-agent                    # Review staged changes
    meow review-agent --file path.go     # Review specific file
    meow review-agent --all              # Review all unstaged changes
"""

load("//lib/file_ops.star", "file_reader", "list_directory", "search_text")
load("//lib/code_search.star", "code_search")
load("//lib/git.star", "git_diff", "git_status")

def review_agent_handler(ctx):
    """Execute code review agent"""
    
    # Parse arguments
    file_path = ctx.params.get("file", "")
    review_all = ctx.params.get("all", False)
    
    ctx.ui.info("🔍 Starting Code Review Agent...")
    ctx.ui.info("")
    
    # Determine what to review
    diff_content = ""
    review_context = ""
    
    if file_path != "":
        # Review specific file
        ctx.ui.info("📄 Reviewing file: " + file_path)
        
        if not ctx.fs.exists(file_path):
            ctx.ui.error("File not found: " + file_path)
            return
        
        file_content = ctx.fs.read(file_path)
        diff_content = "File: " + file_path + "\n\n" + file_content
        review_context = "single file"
    
    elif review_all:
        # Review all unstaged changes
        ctx.ui.info("📝 Reviewing all unstaged changes...")
        diff_content = ctx.git.diff(staged=False)
        review_context = "unstaged changes"
        
        if diff_content == "" or diff_content == None:
            ctx.ui.warn("No unstaged changes found")
            return
    
    else:
        # Review staged changes (default)
        ctx.ui.info("📝 Reviewing staged changes...")
        diff_content = ctx.git.diff(staged=True)
        review_context = "staged changes"
        
        if diff_content == "" or diff_content == None:
            ctx.ui.warn("No staged changes found. Use --file or --all to review specific content.")
            return
    
    ctx.ui.info("")
    ctx.ui.info("🤖 Analyzing changes with AI...")
    ctx.ui.info("")
    
    # Build the review prompt
    system_prompt = """You are an expert code reviewer with deep knowledge of software engineering best practices.

Your review should cover:
1. **Code Quality**: Readability, maintainability, naming conventions
2. **Potential Bugs**: Logic errors, edge cases, error handling
3. **Performance**: Efficiency concerns, resource management
4. **Security**: Vulnerabilities, input validation, sensitive data
5. **Design**: Architecture, patterns, SOLID principles
6. **Testing**: Test coverage, testability
7. **Documentation**: Comments, docstrings, clarity

For each issue found, provide:
- **Severity**: Critical / High / Medium / Low
- **Location**: File and line (if applicable)
- **Issue**: What's wrong
- **Recommendation**: How to fix it
- **Example**: Code snippet if helpful

Use the available tools to:
- Read related files for context
- Search for similar patterns in the codebase
- Understand project structure

Be constructive and specific. Focus on actionable feedback."""
    
    user_prompt = """Review the following """ + review_context + """ and provide detailed feedback:

```diff
""" + diff_content + """
```

Please:
1. First, understand the changes by reading any related files if needed
2. Search for similar code patterns in the codebase for consistency
3. Identify issues and improvements
4. Provide a structured review with severity levels
5. Include specific recommendations and examples

Use the available tools to gather context as needed."""
    
    # Execute agentic review
    tools = [file_reader, code_search, list_directory, search_text, git_status]
    
    result = ctx.llm.agent_turn(
        tools=tools,
        prompt=user_prompt,
        system=system_prompt,
        preset="smart",
        max_iterations=30,
        on_tool_error="return"
    )
    
    # Display the review
    ctx.ui.info("=" * 80)
    ctx.ui.info("CODE REVIEW RESULTS")
    ctx.ui.info("=" * 80)
    ctx.ui.info("")
    ctx.ui.markdown(result)
    ctx.ui.info("")
    ctx.ui.info("=" * 80)
    ctx.ui.success("✓ Review complete")
    
    # Save review to session metadata
    ctx.session.set_metadata("review_result", result)
    ctx.session.set_metadata("review_context", review_context)

# Register the command as a tool first
review_agent_tool = meow.tool(
    name="review-agent",
    description="AI-powered code review agent that analyzes changes and provides detailed feedback",
    params={
        "file": meow.param("string", description="Specific file to review", default=""),
        "all": meow.param("bool", description="Review all unstaged changes", default=False),
    },
    handler=review_agent_handler,
)

# Register as command
meow.command(review_agent_tool)
