# Test system message tracking in sessions

def handler(ctx):
    """Test that system messages are tracked separately from user messages."""
    
    # Call llm.generate with system prompt
    response = ctx.llm.generate(
        prompt="What is 2+2?",
        system="You are a helpful math tutor. Always explain your reasoning.",
        preset="fast"
    )
    
    ctx.output.print("Response: " + response)
    
    # Get events from current session
    events = ctx.session.get_events()
    
    ctx.output.print("\n--- Session Events ---")
    
    # Count event types
    system_count = 0
    user_count = 0
    assistant_count = 0
    
    for event in events:
        event_type = event.get("type")
        content = event.get("content", "")
        
        if event_type == "system":
            system_count += 1
            ctx.output.print("SYSTEM: " + content[:50] + "...")
        elif event_type == "user_message":
            user_count += 1
            ctx.output.print("USER: " + content[:50] + "...")
        elif event_type == "assistant_message":
            assistant_count += 1
            ctx.output.print("ASSISTANT: " + content[:50] + "...")
    
    ctx.output.print("\n--- Event Summary ---")
    ctx.output.print("System messages: " + str(system_count))
    ctx.output.print("User messages: " + str(user_count))
    ctx.output.print("Assistant messages: " + str(assistant_count))
    
    # Verify we have separate system and user messages
    if system_count > 0 and user_count > 0:
        ctx.output.success("✓ System messages are tracked separately!")
    else:
        ctx.output.error("✗ System messages NOT tracked separately")
        ctx.output.error("  Expected: system_count > 0 and user_count > 0")
        ctx.output.error("  Got: system_count=%d, user_count=%d" % (system_count, user_count))

# Create tool and command
test_tool = meow.tool(
    name="test-system-message",
    handler=handler,
    description="Test system message tracking in sessions"
)

meow.command(test_tool)
