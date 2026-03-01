"""
Memory Library for meowg1k

This library provides utilities for managing conversation history, context,
and state across agentic executions. Use it to maintain continuity between
tool calls and reduce context window usage.

## Quick Start

```python
load("//lib/memory.star", "save_context", "recall_context", "summarize_history")

def handler(ctx):
    # Save important context
    ctx.run(save_context, key="analyzed_files", value="main.go,utils.go,types.go")
    
    # Recall later in same session
    files = ctx.run(recall_context, key="analyzed_files")
    ctx.ui.info("Previously analyzed: " + files)
    
    # Summarize history to reduce context
    summary = ctx.run(summarize_history, limit=50)
    ctx.ui.info("Session summary: " + summary)
```

## Available Tools

### Context Management
- `save_context` - Save key-value pairs to session metadata
- `recall_context` - Retrieve previously saved context
- `list_context` - List all saved context keys

### History Management
- `summarize_history` - Summarize conversation history with LLM
- `get_session_info` - Get comprehensive session information

### Helper Functions
- `remember(ctx, key, value)` - Convenience function to save context
- `recall(ctx, key, default="")` - Convenience function to recall context

### Tool Sets
- `memory_tools` - All memory management tools (5 tools)

## API Reference

### save_context

Save context or state to session metadata for later recall.

**Parameters:**
- `key` (string, required): Context key/name (must be unique)
- `value` (string, required): Context value to save

**Returns:** string - Confirmation message

**Example:**
```python
# Save analysis results
ctx.run(save_context, 
    key="error_count", 
    value="42")

# Save file list
ctx.run(save_context,
    key="processed_files",
    value="file1.go,file2.go,file3.go")

# Save JSON data
results_json = ctx.json.encode({"errors": 5, "warnings": 12})
ctx.run(save_context, key="analysis_results", value=results_json)
```

**Use Cases:**
- Track progress across multiple tool calls
- Store intermediate results
- Maintain state in agentic loops
- Cache expensive computations

**Storage:** Context is stored with "context_" prefix in session metadata

---

### recall_context

Recall previously saved context from session metadata.

**Parameters:**
- `key` (string, required): Context key to recall

**Returns:** string - Previously saved value, or empty string if not found

**Example:**
```python
# Recall simple value
error_count = ctx.run(recall_context, key="error_count")
if error_count != "":
    ctx.ui.info("Found " + error_count + " errors previously")

# Recall and parse JSON
results_json = ctx.run(recall_context, key="analysis_results")
if results_json != "":
    results = ctx.json.decode(results_json)
    ctx.ui.info("Errors: %d, Warnings: %d" % 
        (results["errors"], results["warnings"]))

# Check before using
files = ctx.run(recall_context, key="processed_files")
if files == "":
    ctx.ui.warning("No previous file list found")
else:
    file_list = files.split(",")
    ctx.ui.info("Previously processed %d files" % len(file_list))
```

**Behavior:**
- Returns empty string if key not found
- Does not raise errors for missing keys
- Context persists for entire session lifetime

---

### list_context

List all saved context keys in the current session.

**Parameters:** None

**Returns:** JSON string - Array of context key names (without "context_" prefix)

**Example:**
```python
# Save multiple contexts
ctx.run(save_context, key="task_status", value="in_progress")
ctx.run(save_context, key="files_analyzed", value="10")
ctx.run(save_context, key="errors_found", value="3")

# List all saved keys
keys_json = ctx.run(list_context)
keys = ctx.json.decode(keys_json)

ctx.ui.info("Saved context keys:")
for key in keys:
    value = ctx.run(recall_context, key=key)
    ctx.ui.info("  %s: %s" % (key, value))
```

**Use Cases:**
- Debug what's been saved
- Clean up old context
- Display session state
- Verify context availability

---

### summarize_history

Summarize session history to reduce context window usage using LLM.

**Parameters:**
- `limit` (int, default=50): Number of recent events to summarize

**Returns:** string - Concise summary (under 200 words) with:
- Key decisions made
- Important findings
- Current state/progress
- Pending tasks

**Example:**
```python
# Summarize recent activity
summary = ctx.run(summarize_history, limit=30)
ctx.ui.info("Recent activity summary:")
ctx.ui.info(summary)

# Use summary as context for next operation
load("//lib/llm.star", "llm_generate")

next_steps = ctx.run(llm_generate,
    prompt="Based on this summary, what should we do next?\n\n" + summary,
    preset="smart")
```

**Use Cases:**
- Reduce context window size in long sessions
- Create session checkpoints
- Generate progress reports
- Provide context to new operations

**Performance:** Uses "fast" preset for quick summarization

---

### get_session_info

Get comprehensive information about the current session.

**Parameters:** None

**Returns:** JSON string with session details:
- `id`: Session ID
- `tool_name`: Current tool name
- `status`: Session status
- `parent_id`: Parent session ID (or "null")
- `metadata_count`: Number of metadata entries
- `children_count`: Number of child sessions

**Example:**
```python
info_json = ctx.run(get_session_info)
info = ctx.json.decode(info_json)

ctx.ui.info("Session ID: " + info["id"])
ctx.ui.info("Tool: " + info["tool_name"])
ctx.ui.info("Status: " + info["status"])
ctx.ui.info("Metadata entries: %d" % info["metadata_count"])
ctx.ui.info("Child sessions: %d" % info["children_count"])

if info["parent_id"] != "null":
    ctx.ui.info("Parent session: " + info["parent_id"])
```

**Use Cases:**
- Debug session hierarchy
- Track nested executions
- Verify session state
- Generate execution reports

---

## Advanced Usage

### Example 1: Multi-Step Task with State

```python
load("//lib/memory.star", "save_context", "recall_context", "memory_tools")
load("//lib/file_ops.star", "list_directory", "file_reader")

def multi_step_analysis_handler(ctx):
    # Check if we've already started
    phase = ctx.run(recall_context, key="current_phase")
    
    if phase == "":
        # Phase 1: List files
        ctx.ui.info("Phase 1: Discovering files...")
        files_json = ctx.run(list_directory, path="src", pattern="*.go")
        files = ctx.json.decode(files_json)
        
        ctx.run(save_context, key="files_to_analyze", value=files_json)
        ctx.run(save_context, key="current_phase", value="discovery_complete")
        ctx.run(save_context, key="files_processed", value="0")
        
        return "Discovery complete. Found %d files." % len(files)
    
    elif phase == "discovery_complete":
        # Phase 2: Analyze files
        ctx.ui.info("Phase 2: Analyzing files...")
        files_json = ctx.run(recall_context, key="files_to_analyze")
        files = ctx.json.decode(files_json)
        
        processed_count = 0
        for file_path in files:
            content = ctx.run(file_reader, path=file_path)
            # Analyze content...
            processed_count += 1
        
        ctx.run(save_context, key="files_processed", value=str(processed_count))
        ctx.run(save_context, key="current_phase", value="analysis_complete")
        
        return "Analysis complete. Processed %d files." % processed_count
    
    else:
        # Already complete
        processed = ctx.run(recall_context, key="files_processed")
        return "Task already complete. Processed %s files." % processed
```

### Example 2: Progress Tracking

```python
load("//lib/memory.star", "save_context", "recall_context")
load("//lib/file_ops.star", "file_tools")

def progress_tracker_handler(ctx):
    total_files = 100  # Example
    
    # Initialize progress
    processed = ctx.run(recall_context, key="progress_count")
    if processed == "":
        processed = "0"
    
    processed_count = int(processed)
    
    # Process next batch
    batch_size = 10
    for i in range(batch_size):
        # Process file...
        processed_count += 1
    
    # Update progress
    ctx.run(save_context, key="progress_count", value=str(processed_count))
    
    # Calculate percentage
    percent = (processed_count * 100) / total_files
    ctx.ui.info("Progress: %d%% (%d/%d)" % (percent, processed_count, total_files))
    
    if processed_count >= total_files:
        ctx.ui.success("Complete!")
        return "All files processed"
    else:
        return "Processed %d/%d files" % (processed_count, total_files)
```

### Example 3: Error Accumulation

```python
load("//lib/memory.star", "save_context", "recall_context")

def error_collector_handler(ctx):
    # Recall existing errors
    errors_json = ctx.run(recall_context, key="accumulated_errors")
    
    if errors_json == "":
        errors = []
    else:
        errors = ctx.json.decode(errors_json)
    
    # Process and collect new errors
    new_error = {
        "file": "main.go",
        "line": 42,
        "message": "undefined variable"
    }
    
    errors.append(new_error)
    
    # Save updated error list
    ctx.run(save_context, 
        key="accumulated_errors",
        value=ctx.json.encode(errors))
    
    ctx.ui.info("Total errors found: %d" % len(errors))
    
    return ctx.json.encode(errors)
```

### Example 4: Session Checkpoints

```python
load("//lib/memory.star", "save_context", "recall_context", "summarize_history")
load("//lib/llm.star", "llm_generate")

def checkpoint_handler(ctx):
    # Create checkpoint every N operations
    checkpoint_count = ctx.run(recall_context, key="checkpoint_count")
    if checkpoint_count == "":
        checkpoint_count = "0"
    
    count = int(checkpoint_count)
    count += 1
    
    if count % 10 == 0:
        # Create checkpoint
        ctx.ui.info("Creating checkpoint %d..." % (count / 10))
        
        # Summarize recent history
        summary = ctx.run(summarize_history, limit=20)
        
        # Save checkpoint
        checkpoint_key = "checkpoint_" + str(count / 10)
        ctx.run(save_context, key=checkpoint_key, value=summary)
        
        ctx.ui.success("Checkpoint saved: " + checkpoint_key)
    
    ctx.run(save_context, key="checkpoint_count", value=str(count))
    
    return "Operation %d complete" % count
```

### Example 5: Context Expiry

```python
load("//lib/memory.star", "save_context", "recall_context")
load("//lib/time.star", "current_time")

def expiring_cache_handler(ctx):
    cache_key = ctx.params["key"]
    cache_value = ctx.params["value"]
    ttl_seconds = ctx.params.get("ttl", 3600)  # 1 hour default
    
    # Save value with timestamp
    timestamp = ctx.run(current_time, format="unix")
    
    ctx.run(save_context, key=cache_key, value=cache_value)
    ctx.run(save_context, key=cache_key + "_timestamp", value=timestamp)
    
    return "Cached with TTL: %d seconds" % ttl_seconds

def recall_with_expiry_handler(ctx):
    cache_key = ctx.params["key"]
    ttl_seconds = ctx.params.get("ttl", 3600)
    
    # Get value and timestamp
    value = ctx.run(recall_context, key=cache_key)
    timestamp_str = ctx.run(recall_context, key=cache_key + "_timestamp")
    
    if value == "" or timestamp_str == "":
        return ""  # Not found
    
    # Check expiry
    current = ctx.run(current_time, format="unix")
    age = int(current) - int(timestamp_str)
    
    if age > ttl_seconds:
        ctx.ui.warning("Cache expired (age: %d seconds)" % age)
        return ""  # Expired
    
    ctx.ui.info("Cache hit (age: %d seconds)" % age)
    return value
```

### Example 6: Hierarchical Context

```python
load("//lib/memory.star", "save_context", "recall_context", "list_context")

def hierarchical_context_handler(ctx):
    namespace = ctx.params.get("namespace", "default")
    operation = ctx.params["operation"]  # "save" or "recall"
    key = ctx.params["key"]
    
    # Create namespaced key
    namespaced_key = namespace + ":" + key
    
    if operation == "save":
        value = ctx.params["value"]
        ctx.run(save_context, key=namespaced_key, value=value)
        return "Saved to namespace: " + namespace
    
    elif operation == "recall":
        value = ctx.run(recall_context, key=namespaced_key)
        return value
    
    elif operation == "list":
        # List all keys in namespace
        all_keys_json = ctx.run(list_context)
        all_keys = ctx.json.decode(all_keys_json)
        
        namespace_keys = []
        prefix = namespace + ":"
        for k in all_keys:
            if k.startswith(prefix):
                namespace_keys.append(k[len(prefix):])
        
        return ctx.json.encode(namespace_keys)
```

### Example 7: Session Resume

```python
load("//lib/memory.star", "recall_context", "summarize_history", "get_session_info")
load("//lib/llm.star", "llm_generate")

def resume_session_handler(ctx):
    # Get session info
    info_json = ctx.run(get_session_info)
    info = ctx.json.decode(info_json)
    
    ctx.ui.info("Resuming session: " + info["id"])
    
    # Recall previous state
    last_task = ctx.run(recall_context, key="last_task")
    progress = ctx.run(recall_context, key="progress")
    
    # Summarize what happened
    history = ctx.run(summarize_history, limit=30)
    
    # Ask LLM what to do next
    resume_prompt = ("Session Info:\\n" +
                     "- Last task: " + last_task + "\\n" +
                     "- Progress: " + progress + "\\n\\n" +
                     "History:\\n" + history + "\\n\\n" +
                     "What should we do next to continue this work?")
    
    next_steps = ctx.run(llm_generate, 
        prompt=resume_prompt,
        preset="smart")
    
    ctx.ui.info("Next steps:")
    ctx.ui.info(next_steps)
    
    return next_steps
```

### Example 8: Metrics Collection

```python
load("//lib/memory.star", "save_context", "recall_context")

def collect_metrics_handler(ctx):
    # Recall existing metrics
    metrics_json = ctx.run(recall_context, key="metrics")
    
    if metrics_json == "":
        metrics = {
            "files_read": 0,
            "files_written": 0,
            "api_calls": 0,
            "errors": 0
        }
    else:
        metrics = ctx.json.decode(metrics_json)
    
    # Update metrics
    event_type = ctx.params["event"]
    if event_type == "file_read":
        metrics["files_read"] += 1
    elif event_type == "file_write":
        metrics["files_written"] += 1
    elif event_type == "api_call":
        metrics["api_calls"] += 1
    elif event_type == "error":
        metrics["errors"] += 1
    
    # Save updated metrics
    ctx.run(save_context, key="metrics", value=ctx.json.encode(metrics))
    
    # Display metrics
    ctx.ui.info("Session Metrics:")
    ctx.ui.info("  Files read: %d" % metrics["files_read"])
    ctx.ui.info("  Files written: %d" % metrics["files_written"])
    ctx.ui.info("  API calls: %d" % metrics["api_calls"])
    ctx.ui.info("  Errors: %d" % metrics["errors"])
    
    return ctx.json.encode(metrics)
```

### Example 9: Conversation Context

```python
load("//lib/memory.star", "save_context", "recall_context", "summarize_history")
load("//lib/llm.star", "llm_generate")

def context_aware_chat_handler(ctx):
    user_message = ctx.params["message"]
    
    # Recall conversation history
    history_json = ctx.run(recall_context, key="conversation_history")
    
    if history_json == "":
        history = []
    else:
        history = ctx.json.decode(history_json)
    
    # Add user message
    history.append({"role": "user", "content": user_message})
    
    # If history is too long, summarize
    if len(history) > 20:
        ctx.ui.info("Summarizing long conversation...")
        old_history = ctx.json.encode(history[:-10])
        summary = ctx.run(summarize_history, limit=50)
        
        # Keep summary + recent messages
        history = [
            {"role": "system", "content": "Previous conversation summary: " + summary}
        ] + history[-10:]
    
    # Build context for LLM
    context_text = ""
    for msg in history[:-1]:  # Exclude current message
        context_text += msg["role"] + ": " + msg["content"] + "\n\n"
    
    # Generate response
    response = ctx.run(llm_generate,
        prompt=user_message,
        system="Previous context:\n" + context_text,
        preset="smart")
    
    # Add response to history
    history.append({"role": "assistant", "content": response})
    
    # Save updated history
    ctx.run(save_context, key="conversation_history", value=ctx.json.encode(history))
    
    return response
```

### Example 10: Distributed State

```python
load("//lib/memory.star", "save_context", "recall_context", "get_session_info", "memory_tools")

def distributed_state_handler(ctx):
    # Get session info to identify this worker
    info_json = ctx.run(get_session_info)
    info = ctx.json.decode(info_json)
    worker_id = info["id"]
    
    # Register this worker
    workers_json = ctx.run(recall_context, key="active_workers")
    if workers_json == "":
        workers = []
    else:
        workers = ctx.json.decode(workers_json)
    
    if worker_id not in workers:
        workers.append(worker_id)
        ctx.run(save_context, key="active_workers", value=ctx.json.encode(workers))
    
    # Claim a task
    pending_tasks_json = ctx.run(recall_context, key="pending_tasks")
    if pending_tasks_json == "":
        return "No tasks available"
    
    pending_tasks = ctx.json.decode(pending_tasks_json)
    if len(pending_tasks) == 0:
        return "No tasks available"
    
    # Take first task
    task = pending_tasks[0]
    pending_tasks = pending_tasks[1:]
    
    # Save updated task list
    ctx.run(save_context, key="pending_tasks", value=ctx.json.encode(pending_tasks))
    
    # Mark task as in progress
    in_progress_json = ctx.run(recall_context, key="in_progress_tasks")
    if in_progress_json == "":
        in_progress = {}
    else:
        in_progress = ctx.json.decode(in_progress_json)
    
    in_progress[task] = worker_id
    ctx.run(save_context, key="in_progress_tasks", value=ctx.json.encode(in_progress))
    
    ctx.ui.info("Worker %s claimed task: %s" % (worker_id, task))
    
    return task
```

## Error Handling

### Common Errors

**1. Key not found**
```python
# Error: Using recall result without checking
value = ctx.run(recall_context, key="missing_key")
count = int(value)  # Fails if value is ""

# Solution: Always check for empty string
value = ctx.run(recall_context, key="missing_key")
if value == "":
    count = 0  # Default value
else:
    count = int(value)
```

**2. Invalid JSON in context**
```python
# Error: Storing non-JSON then trying to decode
ctx.run(save_context, key="data", value="not valid json")
data = ctx.json.decode(ctx.run(recall_context, key="data"))  # Fails

# Solution: Always encode before saving
data = {"key": "value"}
ctx.run(save_context, key="data", value=ctx.json.encode(data))
data = ctx.json.decode(ctx.run(recall_context, key="data"))  # Works
```

**3. Context key collisions**
```python
# Error: Same key used for different purposes
ctx.run(save_context, key="result", value="first result")
ctx.run(save_context, key="result", value="second result")  # Overwrites!

# Solution: Use descriptive, unique keys
ctx.run(save_context, key="parsing_result", value="first result")
ctx.run(save_context, key="analysis_result", value="second result")
```

**4. Large context values**
```python
# Error: Storing very large strings
huge_file = ctx.run(file_reader, path="10GB_file.txt")
ctx.run(save_context, key="file_content", value=huge_file)  # Memory issue

# Solution: Store metadata, not content
ctx.run(save_context, key="file_path", value="10GB_file.txt")
ctx.run(save_context, key="file_size", value="10737418240")
# Read file only when needed
```

### Error Recovery Pattern

```python
def safe_memory_handler(ctx):
    # Safe recall with default
    value = ctx.run(recall_context, key="counter")
    
    if value == "":
        # Initialize if not found
        counter = 0
        ctx.ui.info("Initializing counter")
    else:
        # Try to parse
        try:
            counter = int(value)
        except:
            ctx.ui.warning("Invalid counter value, resetting")
            counter = 0
    
    # Increment
    counter += 1
    
    # Save
    ctx.run(save_context, key="counter", value=str(counter))
    
    return "Counter: %d" % counter
```

## Performance Tips

### 1. Use Summarization for Long Sessions

```python
# Instead of keeping full history, summarize periodically
message_count = int(ctx.run(recall_context, key="message_count") or "0")

if message_count > 50:
    summary = ctx.run(summarize_history, limit=50)
    ctx.run(save_context, key="history_summary", value=summary)
    ctx.run(save_context, key="message_count", value="0")  # Reset
```

### 2. Avoid Storing Large Data

```python
# Bad - storing entire file
content = ctx.run(file_reader, path="large.txt")
ctx.run(save_context, key="file_content", value=content)

# Good - store reference
ctx.run(save_context, key="file_path", value="large.txt")
ctx.run(save_context, key="file_hash", value=compute_hash(content))
```

### 3. Batch Context Operations

```python
# Instead of multiple save calls
ctx.run(save_context, key="key1", value="value1")
ctx.run(save_context, key="key2", value="value2")
ctx.run(save_context, key="key3", value="value3")

# Bundle related data in one save
data = {"key1": "value1", "key2": "value2", "key3": "value3"}
ctx.run(save_context, key="bundle", value=ctx.json.encode(data))
```

### 4. Clean Up Old Context

```python
# Periodically clean up unused context
def cleanup_context(ctx):
    keys_json = ctx.run(list_context)
    keys = ctx.json.decode(keys_json)
    
    for key in keys:
        if key.startswith("temp_") or key.startswith("cache_"):
            # Context values can't be deleted, but you can track active ones
            # In practice, context lives for session lifetime
            pass
```

## Integration Examples

### With planning.star - Persistent Plans

```python
load("//lib/memory.star", "save_context", "recall_context")
load("//lib/planning.star", "create_plan", "execute_plan")

def persistent_planning_handler(ctx):
    # Check for existing plan
    plan_json = ctx.run(recall_context, key="current_plan")
    
    if plan_json == "":
        # Create new plan
        ctx.ui.info("Creating new plan...")
        plan_json = ctx.run(create_plan, goal=ctx.params["goal"])
        ctx.run(save_context, key="current_plan", value=plan_json)
    else:
        ctx.ui.info("Using existing plan")
    
    # Execute plan
    result = ctx.run(execute_plan, plan=plan_json, tools="[]")
    
    # Save result
    ctx.run(save_context, key="plan_result", value=result)
    
    return result
```

### With llm.star - Context-Aware Generation

```python
load("//lib/memory.star", "recall_context", "summarize_history")
load("//lib/llm.star", "llm_generate")

def context_aware_generation_handler(ctx):
    prompt = ctx.params["prompt"]
    
    # Get previous context
    history_summary = ctx.run(summarize_history, limit=30)
    previous_result = ctx.run(recall_context, key="last_result")
    
    # Build enhanced prompt
    enhanced_prompt = prompt
    if history_summary != "":
        enhanced_prompt = "Context: " + history_summary + "\n\n" + enhanced_prompt
    if previous_result != "":
        enhanced_prompt = enhanced_prompt + "\n\nPrevious result: " + previous_result
    
    # Generate with context
    result = ctx.run(llm_generate, prompt=enhanced_prompt, preset="smart")
    
    # Save for next time
    ctx.run(save_context, key="last_result", value=result)
    
    return result
```

### With file_ops.star - File Processing State

```python
load("//lib/memory.star", "save_context", "recall_context", "memory_tools")
load("//lib/file_ops.star", "list_directory", "file_reader", "file_tools")

def stateful_file_processor_handler(ctx):
    # Get list of files
    files_json = ctx.run(list_directory, path="src", pattern="*.go")
    files = ctx.json.decode(files_json)
    
    # Recall processed files
    processed_json = ctx.run(recall_context, key="processed_files")
    if processed_json == "":
        processed = []
    else:
        processed = ctx.json.decode(processed_json)
    
    # Find unprocessed files
    unprocessed = [f for f in files if f not in processed]
    
    if len(unprocessed) == 0:
        return "All files processed!"
    
    # Process next file
    next_file = unprocessed[0]
    content = ctx.run(file_reader, path=next_file)
    
    # Do processing...
    
    # Mark as processed
    processed.append(next_file)
    ctx.run(save_context, key="processed_files", value=ctx.json.encode(processed))
    
    remaining = len(unprocessed) - 1
    return "Processed: %s (%d remaining)" % (next_file, remaining)
```

## Security Considerations

1. **Don't store secrets** - Context is stored in session metadata, not encrypted
2. **Validate recalled data** - Always validate data retrieved from context
3. **Limit context size** - Don't store unbounded data that could grow indefinitely
4. **Namespace keys** - Use prefixes to avoid key collisions

```python
# Example: Safe context usage
def safe_context_handler(ctx):
    # Don't store secrets
    # BAD: ctx.run(save_context, key="api_key", value=secret_key)
    
    # Validate recalled data
    count_str = ctx.run(recall_context, key="count")
    if count_str != "":
        try:
            count = int(count_str)
            if count < 0 or count > 10000:
                count = 0  # Sanity check
        except:
            count = 0
    
    # Use namespaced keys
    ctx.run(save_context, key="myapp:user:count", value=str(count))
```

## See Also

- **planning.star** - Task planning and execution
- **llm.star** - LLM text generation
- **file_ops.star** - File operations
- **LIBRARY_INDEX.md** - Complete library reference

## Helper Functions Reference

### remember(ctx, key, value)

Convenience function to save context without using ctx.run().

```python
remember(ctx, "counter", "42")
```

### recall(ctx, key, default="")

Convenience function to recall context with default value.

```python
count = recall(ctx, "counter", "0")
```

---

**Last Updated:** 2024  
**Status:** Production Ready  
**Documentation:** Complete ✅
"""

def save_context_handler(ctx):
    """Save context to session metadata"""
    key = ctx.params["key"]
    value = ctx.params["value"]
    
    # Store as metadata
    ctx.session.set_metadata("context_" + key, value)
    return "Saved context: " + key

def recall_context_handler(ctx):
    """Recall context from session metadata"""
    key = ctx.params["key"]
    
    # Retrieve from metadata
    value = ctx.session.get_metadata("context_" + key)
    if value == None:
        return ""
    return value

def list_context_handler(ctx):
    """List all saved context keys"""
    all_metadata = ctx.session.get_all_metadata()
    
    context_keys = []
    for key in all_metadata:
        if key.startswith("context_"):
            actual_key = key[8:]  # Remove "context_" prefix
            context_keys.append(actual_key)
    
    return ctx.json.encode(context_keys)

def summarize_history_handler(ctx):
    """Summarize session history for context window management"""
    limit = ctx.params.get("limit", 50)
    
    # Get recent events
    events = ctx.session.get_events(limit=limit, offset=0)
    
    # Build history text
    history_text = ""
    for event in events:
        event_type = event.get("type", "unknown")
        content = event.get("content", "")
        
        if event_type == "user_message":
            history_text = history_text + "User: " + content + "\n\n"
        elif event_type == "assistant_message":
            history_text = history_text + "Assistant: " + content + "\n\n"
        elif event_type == "tool_result":
            history_text = history_text + "Tool result: " + content + "\n\n"
    
    if history_text == "":
        return "No history available"
    
    # Ask LLM to summarize
    system_prompt = """Summarize the conversation history concisely.
Focus on:
- Key decisions made
- Important findings
- Current state/progress
- Pending tasks

Keep it under 200 words."""
    
    summary = ctx.llm.chat(
        prompt="Summarize this conversation:\n\n" + history_text,
        system=system_prompt,
        preset="fast"
    )
    
    return summary

def get_session_info_handler(ctx):
    """Get comprehensive information about current session"""
    session_id = ctx.session.id()
    tool_name = ctx.session.tool_name()
    status = ctx.session.status()
    parent_id = ctx.session.parent_id()
    
    # Get metadata
    metadata = ctx.session.get_all_metadata()
    
    # Get children
    children = ctx.session.get_children()
    
    # Build info dict
    info = {
        "id": session_id,
        "tool_name": tool_name,
        "status": status,
        "parent_id": parent_id if parent_id != None else "null",
        "metadata_count": len(metadata),
        "children_count": len(children),
    }
    
    return ctx.json.encode(info)

# Tool definitions
save_context = meow.tool(
    name="save_context",
    description="Save context/state to session metadata for later recall",
    params={
        "key": meow.param("string", description="Context key/name", required=True),
        "value": meow.param("string", description="Context value to save", required=True),
    },
    handler=save_context_handler,
)

recall_context = meow.tool(
    name="recall_context",
    description="Recall previously saved context from session metadata",
    params={
        "key": meow.param("string", description="Context key/name to recall", required=True),
    },
    handler=recall_context_handler,
)

list_context = meow.tool(
    name="list_context",
    description="List all saved context keys in current session",
    params={},
    handler=list_context_handler,
)

summarize_history = meow.tool(
    name="summarize_history",
    description="Summarize session history to reduce context window usage",
    params={
        "limit": meow.param("int", description="Number of recent events to summarize", default=50),
    },
    handler=summarize_history_handler,
)

get_session_info = meow.tool(
    name="get_session_info",
    description="Get comprehensive information about the current session",
    params={},
    handler=get_session_info_handler,
)

# Helper functions
def remember(ctx, key, value):
    """Convenience function to save context"""
    ctx.session.set_metadata("context_" + key, value)

def recall(ctx, key, default=""):
    """Convenience function to recall context"""
    value = ctx.session.get_metadata("context_" + key)
    if value == None:
        return default
    return value

# Tool set for memory management
memory_tools = [save_context, recall_context, list_context, summarize_history, get_session_info]
