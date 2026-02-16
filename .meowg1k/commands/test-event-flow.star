"""
Comprehensive test of session event tracking.
Shows the complete event flow including:
- Session creation
- Metadata storage
- Child session creation
- Event tracking
"""

def handler(ctx):
    """Test complete session event flow."""
    ctx.output.writeline("=== Session Event Flow Test ===\n")
    
    # 1. Show root session info
    session_id = ctx.session.id()
    ctx.output.writeline("1. Root Session:")
    ctx.output.writeline("   ID: " + session_id)
    ctx.output.writeline("   Tool: " + ctx.session.tool_name())
    ctx.output.writeline("   Status: " + ctx.session.status())
    ctx.output.writeline("   Parent: " + str(ctx.session.parent_id()))
    
    # 2. Set metadata
    ctx.output.writeline("\n2. Setting Metadata:")
    ctx.session.set_metadata("test_start_time", str(ctx.time.now()))
    ctx.session.set_metadata("test_description", "Comprehensive event flow test")
    ctx.session.set_metadata("test_version", "1.0")
    all_meta = ctx.session.get_all_metadata()
    ctx.output.writeline("   Metadata entries: " + str(len(all_meta)))
    for key in all_meta:
        ctx.output.writeline("   - " + key + ": " + all_meta[key])
    
    # 3. Invoke child tool
    ctx.output.writeline("\n3. Invoking Child Tool:")
    ctx.run("test-child-session")
    
    # 4. Check child sessions
    ctx.output.writeline("\n4. Child Sessions:")
    children = ctx.session.get_children()
    ctx.output.writeline("   Count: " + str(len(children)))
    for i, child in enumerate(children):
        ctx.output.writeline("   Child " + str(i + 1) + ":")
        ctx.output.writeline("     ID: " + child["id"])
        ctx.output.writeline("     Tool: " + child["tool_name"])
        ctx.output.writeline("     Status: " + child["status"])
    
    # 5. Query all sessions
    ctx.output.writeline("\n5. Global Session Query:")
    all_sessions = ctx.session.list_all(limit=5)
    ctx.output.writeline("   Recent sessions (limit 5): " + str(len(all_sessions)))
    for i, s in enumerate(all_sessions):
        ctx.output.writeline("   " + str(i + 1) + ". " + s["tool_name"] + " (" + s["status"] + ")")
    
    # 6. Get events from this session
    ctx.output.writeline("\n6. Session Events:")
    events = ctx.session.get_events()
    ctx.output.writeline("   Total events: " + str(len(events)))
    for i, event in enumerate(events):
        ctx.output.writeline("   Event " + str(i + 1) + ":")
        ctx.output.writeline("     Type: " + event["type"])
        content_preview = event["content"][:60] + "..." if len(event["content"]) > 60 else event["content"]
        ctx.output.writeline("     Content: " + content_preview)
    
    # 7. Test get_by_id
    ctx.output.writeline("\n7. Get Session By ID:")
    retrieved = ctx.session.get_by_id(session_id)
    if retrieved:
        ctx.output.writeline("   ✓ Successfully retrieved session")
        ctx.output.writeline("   Tool: " + retrieved["tool_name"])
        ctx.output.writeline("   Status: " + retrieved["status"])
    else:
        ctx.output.writeline("   ✗ Failed to retrieve session")
    
    ctx.output.writeline("\n✓ All tests completed!")

# Create tool
flow_test_tool = meow.tool(
    name="test-event-flow",
    description="Comprehensive test of session event tracking",
    handler=handler
)

meow.command(flow_test_tool)
