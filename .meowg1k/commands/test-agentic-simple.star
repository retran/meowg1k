# Simple test to verify agentic API exists and is callable

def simple_handler(ctx):
    """Simple tool that returns a fixed value"""
    return "42"

def test_agentic_simple_handler(ctx):
    """Test that ctx.llm.agentic exists and has correct signature"""
    
    # Define a simple tool
    simple_tool = meow.tool(
        name="simple",
        description="A simple test tool",
        params={},
        handler=simple_handler,
    )
    
    ctx.ui.info("Testing agentic API...")
    
    # Print session info
    ctx.ui.info("Current session ID: " + ctx.session.id())
    ctx.ui.info("Tool name: " + ctx.session.tool_name())
    
    # Call agentic (will fail if provider doesn't support tool calling, but that's expected)
    # Note: Gemini supports tool calling, so this should work
    result = ctx.llm.agentic(
        tools=[simple_tool],
        prompt="Use the simple tool to get a number",
        preset="fast",
        max_iterations=5,
    )
    ctx.ui.success("Agentic call succeeded!")
    ctx.ui.info("Result: " + result)
    
    return "success"

# Create tool and register command
test_agentic_tool = meow.tool(
    name="test-agentic-simple",
    description="Simple test for agentic API",
    handler=test_agentic_simple_handler,
)

meow.command(test_agentic_tool)
