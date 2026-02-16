"""
Test command to demonstrate agentic loop with built-in tools.

This command showcases how to use the built-in tools library in an agentic loop.
"""

load("//lib/math.star", "calculator")
load("//lib/time.star", "current_time")
load("//lib/file_ops.star", "file_tools")
load("//lib/code_search.star", "code_search")
load("//lib/file_ops.star", "file_reader", "search_text", "list_directory")

# Create code_tools set for backward compatibility
code_tools = [file_reader, search_text, code_search, list_directory]

def test_agentic_tools_handler(ctx):
    """Test agentic loop with built-in tools"""
    
    ctx.ui.info("Testing agentic loop with built-in tools library...")
    ctx.ui.info("=" * 60)
    
    # Test 1: Simple calculator test
    ctx.ui.info("\nTest 1: Calculator tool")
    ctx.ui.info("Asking LLM to calculate (15 + 25) * 2")
    
    # Note: This will fail without API key, but demonstrates the pattern
    result = ctx.llm.agentic(
        tools=[calculator],
        prompt="Calculate (15 + 25) * 2. First add 15 and 25, then multiply the result by 2. Show your work.",
        system="You are a helpful calculator assistant. Use the calculator tool to perform arithmetic operations. Always show your calculations step by step.",
        preset="fast",
        max_iterations=10,
    )
    
    ctx.ui.success("Calculator test completed!")
    ctx.ui.info("Result: " + result)
    
    # Test 2: Multiple tools test
    ctx.ui.info("\nTest 2: File operations with code search")
    ctx.ui.info("Asking LLM to list Go files and explain the project structure")
    
    result2 = ctx.llm.agentic(
        tools=code_tools,  # file_reader, search_text, code_search, list_directory
        prompt="List all .go files in the current directory and its subdirectories. Then read one of the main files and describe what this project does.",
        system="You are a code analyst. Use the tools available to explore the codebase.",
        preset="fast",
        max_iterations=15,
    )
    
    ctx.ui.success("Code analysis test completed!")
    ctx.ui.info("Result: " + result2)
    
    return "All agentic tool tests completed!"

# Create tool and register command
test_agentic_tools_tool = meow.tool(
    name="test-agentic-tools",
    description="Test agentic loop with built-in tools library",
    handler=test_agentic_tools_handler,
)

meow.command(test_agentic_tools_tool)
