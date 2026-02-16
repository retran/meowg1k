# Test command for agentic loop with tool calling

def test_agentic_handler(ctx):
    """Test the ctx.llm.agentic() functionality with simple tools"""
    
    # Define test tools
    calculator_tool = meow.tool(
        name="calculator",
        description="Perform basic arithmetic operations",
        params={
            "operation": meow.param(
                type="string",
                description="The operation to perform",
                choices=["add", "subtract", "multiply", "divide"],
                required=True,
            ),
            "a": meow.param(
                type="float",
                description="First number",
                required=True,
            ),
            "b": meow.param(
                type="float",
                description="Second number",
                required=True,
            ),
        },
        handler=calculator_handler,
    )
    
    get_time_tool = meow.tool(
        name="get_current_time",
        description="Get the current system time",
        params={},
        handler=get_time_handler,
    )
    
    # Test agentic loop
    ctx.ui.info("Testing agentic loop...")
    ctx.ui.info("Asking LLM to calculate (10 + 5) * 2 using calculator tool")
    
    result = ctx.llm.agentic(
        tools=[calculator_tool, get_time_tool],
        prompt="Calculate (10 + 5) * 2. First add 10 and 5, then multiply the result by 2.",
        system="You are a helpful assistant that can use tools to help answer questions.",
        preset="fast",
        on_tool_error="return",
        max_iterations=10,
    )
    
    ctx.ui.success("Agentic loop completed!")
    ctx.ui.info("Final result: " + result)
    
    return result

def calculator_handler(ctx):
    """Handler for calculator tool"""
    operation = ctx.params["operation"]
    a = ctx.params["a"]
    b = ctx.params["b"]
    
    if operation == "add":
        result = a + b
    elif operation == "subtract":
        result = a - b
    elif operation == "multiply":
        result = a * b
    elif operation == "divide":
        if b == 0:
            return "Error: Division by zero"
        result = a / b
    else:
        return "Error: Unknown operation"
    
    return str(result)

def get_time_handler(ctx):
    """Handler for get_current_time tool"""
    import time
    current_time = ctx.time.now()
    return current_time

# Register the command
meow.command(
    name="test-agentic",
    description="Test agentic loop with tool calling",
    handler=test_agentic_handler,
)
