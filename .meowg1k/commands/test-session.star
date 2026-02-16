"""
Test command to verify session creation and tracking.
"""

def handler(ctx):
    """Test session functionality."""
    # Check if session exists
    session_id = ctx.session.id()
    
    if session_id == None:
        ctx.output.writeline("ERROR: No session created!")
        return
    
    ctx.output.writeline("✓ Root session created: " + session_id)
    
    # Check session metadata
    tool_name = ctx.session.tool_name()
    ctx.output.writeline("✓ Tool name: " + tool_name)
    
    status = ctx.session.status()
    ctx.output.writeline("✓ Status: " + status)
    
    parent_id = ctx.session.parent_id()
    if parent_id == None:
        ctx.output.writeline("✓ Parent ID: None (root session)")
    else:
        ctx.output.writeline("✗ Parent ID should be None for root session: " + parent_id)
    
    # Test metadata operations
    ctx.session.set_metadata("test_key", "test_value")
    value = ctx.session.get_metadata("test_key")
    if value == "test_value":
        ctx.output.writeline("✓ Metadata set/get works")
    else:
        ctx.output.writeline("✗ Metadata failed")
    
    ctx.output.writeline("\n--- Testing child session via ctx.run() ---")
    
    # Call child command to test child session creation
    ctx.run("test-child-session")
    
    # Check child sessions
    children = ctx.session.get_children()
    ctx.output.writeline("\n✓ Child sessions count: " + str(len(children)))
    if len(children) > 0:
        child_info = children[0]
        ctx.output.writeline("✓ Child tool name: " + child_info["tool_name"])
        ctx.output.writeline("✓ Child status: " + child_info["status"])
    
    ctx.output.writeline("\nAll session tests passed!")

# Create tool and command
test_tool = meow.tool(
    name="test-session",
    description="Test session creation and functionality",
    handler=handler
)

meow.command(test_tool)
