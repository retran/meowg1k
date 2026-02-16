"""
Test command to verify session database persistence.
"""

def handler(ctx):
    """Test session database functionality."""
    session_id = ctx.session.id()
    ctx.output.writeline("✓ Session ID: " + session_id)
    
    # Set some metadata
    ctx.session.set_metadata("test_key_1", "value_1")
    ctx.session.set_metadata("test_key_2", "value_2")
    ctx.output.writeline("✓ Metadata set")
    
    # Get all metadata
    all_meta = ctx.session.get_all_metadata()
    ctx.output.writeline("✓ Metadata count: " + str(len(all_meta)))
    
    # Call a child command
    ctx.output.writeline("\n--- Calling child command ---")
    ctx.run("test-child-session")
    
    # Get children
    children = ctx.session.get_children()
    ctx.output.writeline("\n✓ Child sessions: " + str(len(children)))
    
    if len(children) > 0:
        child_id = children[0]["id"]
        ctx.output.writeline("✓ Child ID: " + child_id)
        
        # Try to get the child by ID using list_all
        all_sessions = ctx.session.list_all(limit=10)
        ctx.output.writeline("✓ Total sessions in DB (limit 10): " + str(len(all_sessions)))
        
        # Find our session and child in the list
        found_parent = False
        found_child = False
        for s in all_sessions:
            if s["id"] == session_id:
                found_parent = True
                ctx.output.writeline("✓ Found parent in list_all")
            if s["id"] == child_id:
                found_child = True
                ctx.output.writeline("✓ Found child in list_all")
        
        if found_parent and found_child:
            ctx.output.writeline("\n✓ All session persistence tests passed!")
        else:
            ctx.output.writeline("\n✗ Session not found in list_all")
    else:
        ctx.output.writeline("\n✗ No child sessions found")

# Create tool
persistence_test_tool = meow.tool(
    name="test-persistence",
    description="Test session database persistence",
    handler=handler
)

meow.command(persistence_test_tool)
