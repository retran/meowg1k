# Test tool invocation with ToolValue objects

# Create a simple tool to be invoked
def greet_handler(ctx):
    """Greet someone."""
    name = getattr(ctx, "name", "World")
    greeting = "Hello, " + name + "!"
    ctx.output.writeline(greeting)
    return greeting

greet_tool = meow.tool(
    name="greet-test",
    description="Greet someone by name",
    handler=greet_handler,
    params={
        "name": meow.param("string", desc="Name to greet", default="World")
    }
)

# Create another tool that will be called
def calculate_handler(ctx):
    """Perform simple calculation."""
    a = getattr(ctx, "a", 0)
    b = getattr(ctx, "b", 0)
    op = getattr(ctx, "op", "add")
    
    result = 0
    if op == "add":
        result = a + b
    elif op == "multiply":
        result = a * b
    else:
        result = 0
    
    ctx.output.writeline("Result: " + str(result))
    return result

calc_tool = meow.tool(
    name="calc-test",
    description="Perform calculation",
    handler=calculate_handler,
    params={
        "a": meow.param("int", desc="First number", default=0),
        "b": meow.param("int", desc="Second number", default=0),
        "op": meow.param("string", desc="Operation (add/multiply)", default="add")
    }
)

# Main test handler
def test_handler(ctx):
    """Test ctx.run() with both string and ToolValue."""
    
    ctx.output.writeline("=== Test 1: ctx.run() with string ===")
    result1 = ctx.run("greet-test", name="Alice")
    ctx.output.writeline("Returned: " + str(result1))
    
    ctx.output.writeline("\n=== Test 2: ctx.run() with ToolValue ===")
    result2 = ctx.run(greet_tool, name="Bob")
    ctx.output.writeline("Returned: " + str(result2))
    
    ctx.output.writeline("\n=== Test 3: ctx.run() with ToolValue and calculation ===")
    result3 = ctx.run(calc_tool, a=10, b=5, op="add")
    ctx.output.writeline("Returned: " + str(result3))
    
    ctx.output.writeline("\n=== Test 4: ctx.run() with ToolValue and multiplication ===")
    result4 = ctx.run(calc_tool, a=7, b=3, op="multiply")
    ctx.output.writeline("Returned: " + str(result4))
    
    ctx.output.writeline("\n=== All tests passed! ===")
    ctx.ui.success("✓ ctx.run() works with both strings and ToolValue objects")

# Register tools as commands
meow.command(greet_tool)
meow.command(calc_tool)

# Create and register test command
test_tool = meow.tool(
    name="test-tool-value-run",
    description="Test ctx.run() with ToolValue objects",
    handler=test_handler
)

meow.command(test_tool)
