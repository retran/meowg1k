"""
Test command to verify LLM event tracking in sessions.
"""

def handler(ctx):
    """Test LLM event tracking."""
    session_id = ctx.session.id()
    ctx.output.writeline("Session ID: " + session_id)
    
    # Make an LLM call - this should create user_message and assistant_message events
    ctx.output.writeline("\n--- Making LLM call ---")
    result = ctx.llm.generate(
        prompt="Say hello in exactly 3 words",
        system="You are a friendly assistant.",
        preset="fast"
    )
    ctx.output.writeline("LLM Response: " + result)
    
    # Check events
    ctx.output.writeline("\n--- Checking events ---")
    events = ctx.session.get_events()
    ctx.output.writeline("Total events: " + str(len(events)))
    
    for i, event in enumerate(events):
        ctx.output.writeline("\nEvent " + str(i + 1) + ":")
        ctx.output.writeline("  Type: " + event["type"])
        ctx.output.writeline("  Content: " + event["content"][:100] + ("..." if len(event["content"]) > 100 else ""))
    
    # Verify we have at least 2 events (user_message and assistant_message)
    if len(events) >= 2:
        ctx.output.writeline("\n✓ LLM events tracked successfully!")
    else:
        ctx.output.writeline("\n✗ Expected at least 2 events, got " + str(len(events)))

# Create tool
llm_test_tool = meow.tool(
    name="test-llm-events",
    description="Test LLM event tracking in sessions",
    handler=handler
)

meow.command(llm_test_tool)
