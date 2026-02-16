"""
Child test command to verify child session creation.
"""

def handler(ctx):
    """Test child session functionality."""
    session_id = ctx.session.id()
    parent_id = ctx.session.parent_id()
    
    if session_id == None:
        ctx.output.writeline("ERROR: No child session created!")
        return
    
    ctx.output.writeline("✓ Child session created: " + session_id)
    
    if parent_id != None:
        ctx.output.writeline("✓ Parent ID: " + parent_id)
    else:
        ctx.output.writeline("✗ Parent ID is None (should have a parent)")
    
    tool_name = ctx.session.tool_name()
    ctx.output.writeline("✓ Tool name: " + tool_name)

# Create tool
child_tool = meow.tool(
    name="test-child-session",
    description="Test child session creation",
    handler=handler
)

meow.command(child_tool)
