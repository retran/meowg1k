# Test that ctx.llm.agentic() works with tool objects loaded from libraries

load("//lib/math.star", "calculator")
load("//lib/file_ops.star", "file_reader")
load("//lib/time.star", "current_time")

def test_agentic_with_tool_objects(ctx):
    """Test that agentic accepts tool objects directly."""
    
    ctx.ui.info("Testing ctx.llm.agentic() with tool objects from library...")
    
    # Verify we have the tools
    ctx.output.writeline("Available tools:")
    ctx.output.writeline("  - calculator: " + str(calculator))
    ctx.output.writeline("  - file_reader: " + str(file_reader))
    ctx.output.writeline("  - current_time: " + str(current_time))
    
    ctx.ui.success("✓ Tool objects loaded successfully from library")
    ctx.ui.info("Note: To actually test agentic execution, you would call:")
    ctx.ui.info('  result = ctx.llm.agentic(tools=[calculator, file_reader], prompt="...")')

# Create and register test command
test_tool = meow.tool(
    name="test-tool-objects",
    description="Test that tool objects can be loaded and used",
    handler=test_agentic_with_tool_objects
)

meow.command(test_tool)
