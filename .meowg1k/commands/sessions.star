"""
Session Management Commands

Commands for viewing and managing execution sessions.

Commands:
- sessions: List all sessions with optional filters
- show-session: Display detailed information about a specific session
"""

def sessions_handler(ctx):
    """List all sessions with optional filters"""
    # Parameters are injected directly into ctx
    tool_name = ctx.tool
    status = ctx.status
    limit = ctx.limit
    
    # Query sessions
    sessions = ctx.session.list_all(tool_name=tool_name, status=status, limit=limit)
    
    if len(sessions) == 0:
        ctx.ui.info("No sessions found")
        return
    
    ctx.ui.info("Found " + str(len(sessions)) + " session(s)")
    ctx.ui.info("=" * 80)
    
    # Display sessions in a table-like format
    for session in sessions:
        session_id = session.get("id", "unknown")
        tool = session.get("tool_name", "unknown")
        sess_status = session.get("status", "unknown")
        created = session.get("created_at", "unknown")
        parent = session.get("parent_id", None)
        
        # Format output
        ctx.ui.info("")
        ctx.ui.info("Session: " + session_id)
        ctx.ui.info("  Tool:    " + tool)
        ctx.ui.info("  Status:  " + sess_status)
        ctx.ui.info("  Created: " + created)
        if parent != None:
            ctx.ui.info("  Parent:  " + parent)
        else:
            ctx.ui.info("  Parent:  <root>")
    
    ctx.ui.info("")
    ctx.ui.info("=" * 80)
    
    return "Listed " + str(len(sessions)) + " sessions"

def show_session_handler(ctx):
    """Show detailed information about a specific session"""
    session_id = ctx.id
    show_events = ctx.events
    event_limit = ctx.event_limit
    
    # If no ID provided, show current session
    if session_id == "":
        session_id = ctx.session.id()
        ctx.ui.info("Showing current session")
    
    # Get session details
    session = ctx.session.get_by_id(session_id)
    
    if session == None:
        ctx.ui.error("Session not found: " + session_id)
        return "Session not found"
    
    # Display session info
    ctx.ui.info("=" * 80)
    ctx.ui.info("SESSION DETAILS")
    ctx.ui.info("=" * 80)
    
    ctx.ui.info("")
    ctx.ui.info("ID:         " + session.get("id", "unknown"))
    ctx.ui.info("Tool:       " + session.get("tool_name", "unknown"))
    ctx.ui.info("Status:     " + session.get("status", "unknown"))
    ctx.ui.info("Created:    " + session.get("created_at", "unknown"))
    ctx.ui.info("Updated:    " + session.get("updated_at", "unknown"))
    
    parent = session.get("parent_id", None)
    if parent != None:
        ctx.ui.info("Parent:     " + parent)
    else:
        ctx.ui.info("Parent:     <root session>")
    
    # Show metadata
    ctx.ui.info("")
    ctx.ui.info("METADATA")
    ctx.ui.info("-" * 80)
    
    # Get metadata for this session (if it's the current session)
    if session_id == ctx.session.id():
        metadata = ctx.session.get_all_metadata()
        if len(metadata) == 0:
            ctx.ui.info("  (no metadata)")
        else:
            for key in metadata:
                value = metadata[key]
                # Truncate long values
                if len(value) > 100:
                    value = value[:100] + "..."
                ctx.ui.info("  " + key + ": " + value)
    else:
        ctx.ui.info("  (metadata only available for current session)")
    
    # Show child sessions
    ctx.ui.info("")
    ctx.ui.info("CHILD SESSIONS")
    ctx.ui.info("-" * 80)
    
    if session_id == ctx.session.id():
        children = ctx.session.get_children()
        if len(children) == 0:
            ctx.ui.info("  (no child sessions)")
        else:
            for child in children:
                child_id = child.get("id", "unknown")
                child_tool = child.get("tool_name", "unknown")
                child_status = child.get("status", "unknown")
                ctx.ui.info("  " + child_id + " - " + child_tool + " (" + child_status + ")")
    else:
        ctx.ui.info("  (child sessions only available for current session)")
    
    # Show events if requested
    if show_events:
        ctx.ui.info("")
        ctx.ui.info("RECENT EVENTS (last " + str(event_limit) + ")")
        ctx.ui.info("-" * 80)
        
        if session_id == ctx.session.id():
            events = ctx.session.get_events(limit=event_limit, offset=0)
            if len(events) == 0:
                ctx.ui.info("  (no events)")
            else:
                for event in events:
                    event_type = event.get("type", "unknown")
                    content = event.get("content", "")
                    created = event.get("created_at", "unknown")
                    
                    # Truncate content
                    if len(content) > 200:
                        content = content[:200] + "..."
                    
                    ctx.ui.info("")
                    ctx.ui.info("  [" + event_type + "] " + created)
                    if content != "":
                        ctx.ui.info("    " + content.replace("\n", "\n    "))
        else:
            ctx.ui.info("  (events only available for current session)")
    
    ctx.ui.info("")
    ctx.ui.info("=" * 80)
    
    return "Session details displayed"

# Tool definitions
sessions_tool = meow.tool(
    name="sessions",
    description="List all sessions with optional filters",
    params={
        "tool": meow.param("string", desc="Filter by tool name", default=""),
        "status": meow.param("string", desc="Filter by status (running, completed, failed)", default=""),
        "limit": meow.param("int", desc="Maximum number of sessions to show", default=20),
    },
    handler=sessions_handler,
)

show_session_tool = meow.tool(
    name="show-session",
    description="Show detailed information about a specific session",
    params={
        "id": meow.param("string", desc="Session ID (empty for current session)", default=""),
        "events": meow.param("bool", desc="Include recent events", default=False),
        "event_limit": meow.param("int", desc="Number of events to show", default=10),
    },
    handler=show_session_handler,
)

# Register commands
meow.command(sessions_tool)
meow.command(show_session_tool)
