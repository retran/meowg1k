"""
HTTP Operations Library for meowg1k

This library provides HTTP client capabilities for making GET and POST requests
to REST APIs, webhooks, and web services.

## Quick Start

```python
load("//lib/http.star", "http_get", "http_post")

def handler(ctx):
    # GET request
    response = ctx.run(http_get, url="https://api.github.com/repos/owner/repo")
    
    # POST request
    payload = ctx.json.encode({"key": "value"})
    result = ctx.run(http_post, 
                    url="https://api.example.com/data",
                    body=payload)
```

## Available Tools

- `http_get` - Make HTTP GET requests
- `http_post` - Make HTTP POST requests

### Tool Sets
- `http_tools` - All HTTP operation tools (2 tools)

## API Reference

### http_get

Make an HTTP GET request and return response body.

**Parameters:**
- `url` (string, required): URL to request

**Returns:** string - Response body

**Example:**
```python
# Fetch JSON data
response = ctx.run(http_get, url="https://api.github.com/repos/golang/go")
data = ctx.json.decode(response)

ctx.ui.info("Repository: " + data["name"])
ctx.ui.info("Stars: " + str(data["stargazers_count"]))

# Fetch plain text
html = ctx.run(http_get, url="https://example.com")
ctx.output.writeline(html)
```

**Behavior:**
- Follows redirects automatically
- Returns response body as string
- Raises error on non-2xx status codes
- No timeout (may hang on slow servers)
- No custom headers support

---

### http_post

Make an HTTP POST request with body.

**Parameters:**
- `url` (string, required): URL to request
- `body` (string, required): Request body

**Returns:** string - Response body

**Example:**
```python
# POST JSON data
payload = ctx.json.encode({
    "name": "Alice",
    "email": "alice@example.com"
})

response = ctx.run(http_post,
                  url="https://api.example.com/users",
                  body=payload)

ctx.ui.info("Response: " + response)

# Send webhook
webhook_data = ctx.json.encode({
    "text": "Deployment complete",
    "status": "success"
})

ctx.run(http_post,
       url="https://hooks.slack.com/services/xxx",
       body=webhook_data)
```

**Behavior:**
- Content-Type not customizable (defaults to application/json or text/plain)
- Raises error on non-2xx status codes
- No timeout configuration
- No custom headers support

## Advanced Usage

### API Client Pattern

```python
load("//lib/http.star", "http_get", "http_post")
load("//lib/json.star", "json_query")

def github_api(ctx):
    # GitHub API client example.
    
    def get_repo(owner, name):
        url = "https://api.github.com/repos/%s/%s" % (owner, name)
        response = ctx.run(http_get, url=url)
        return ctx.json.decode(response)
    
    def get_issues(owner, name):
        url = "https://api.github.com/repos/%s/%s/issues" % (owner, name)
        response = ctx.run(http_get, url=url)
        return ctx.json.decode(response)
    
    # Use the API
    repo = get_repo("golang", "go")
    ctx.ui.info("Repository: " + repo["full_name"])
    
    issues = get_issues("golang", "go")
    ctx.ui.info("Open issues: " + str(len(issues)))
```

### Webhook Integration

```python
load("//lib/http.star", "http_post")
load("//lib/git.star", "git_status")

def notify_deployment(ctx, webhook_url):
    # Send deployment notification to webhook.
    
    # Get git info
    status_json = ctx.run(git_status)
    status = ctx.json.decode(status_json)
    
    branch = status.get("branch", "unknown")
    
    # Build message
    message = {
        "event": "deployment",
        "branch": branch,
        "timestamp": ctx.time.now(),
        "status": "success"
    }
    
    # Send webhook
    payload = ctx.json.encode(message)
    ctx.run(http_post, url=webhook_url, body=payload)
    
    ctx.ui.success("Webhook sent")
```

### API Polling

```python
load("//lib/http.star", "http_get")
load("//lib/json.star", "json_query")

def poll_build_status(ctx, build_url):
    # Poll CI/CD build status (simplified example).
    
    ctx.ui.info("Checking build status...")
    
    response = ctx.run(http_get, url=build_url)
    status = ctx.run(json_query, json=response, path="status")
    
    if status == "success":
        ctx.ui.success("Build passed!")
        return True
    elif status == "failure":
        ctx.ui.error("Build failed!")
        return False
    else:
        ctx.ui.info("Build status: " + status)
        return None
```

### REST API CRUD Operations

```python
load("//lib/http.star", "http_get", "http_post")

def user_management(ctx, api_base):
    # Example REST API operations.
    
    # CREATE (POST)
    new_user = ctx.json.encode({
        "name": "Alice",
        "email": "alice@example.com"
    })
    
    response = ctx.run(http_post,
                      url=api_base + "/users",
                      body=new_user)
    
    user = ctx.json.decode(response)
    user_id = user["id"]
    ctx.ui.success("Created user: " + user_id)
    
    # READ (GET)
    user_response = ctx.run(http_get,
                           url=api_base + "/users/" + user_id)
    
    user_data = ctx.json.decode(user_response)
    ctx.ui.info("User: " + user_data["name"])
    
    # LIST (GET)
    all_users = ctx.run(http_get, url=api_base + "/users")
    users = ctx.json.decode(all_users)
    ctx.ui.info("Total users: " + str(len(users)))
```

### Error Handling & Retries

```python
load("//lib/http.star", "http_get")

def fetch_with_retry(ctx, url, max_attempts=3):
    # Fetch URL with retry logic.
    
    for attempt in range(1, max_attempts + 1):
        try:
            ctx.ui.info("Attempt %d/%d..." % (attempt, max_attempts))
            response = ctx.run(http_get, url=url)
            ctx.ui.success("Request successful")
            return response
        except:
            if attempt == max_attempts:
                ctx.ui.error("All attempts failed")
                raise
            ctx.ui.warning("Attempt failed, retrying...")
    
    return None
```

### API Response Validation

```python
load("//lib/http.star", "http_get")
load("//lib/json.star", "json_parse", "json_query")

def validate_api(ctx, url, required_fields):
    # Fetch and validate API response.
    
    # Fetch
    try:
        response = ctx.run(http_get, url=url)
    except:
        ctx.ui.error("HTTP request failed")
        return False
    
    # Validate JSON
    try:
        ctx.run(json_parse, json=response)
    except:
        ctx.ui.error("Response is not valid JSON")
        return False
    
    # Validate fields
    for field in required_fields:
        value = ctx.run(json_query, json=response, path=field)
        if value == "null":
            ctx.ui.error("Missing field: " + field)
            return False
    
    ctx.ui.success("API response valid")
    return True
```

## Error Handling

HTTP requests can fail for many reasons:

```python
load("//lib/http.star", "http_get", "http_post")

def safe_http_get(ctx, url):
    # HTTP GET with error handling.
    try:
        return ctx.run(http_get, url=url)
    except:
        ctx.ui.error("Request failed: " + url)
        return None

def safe_http_post(ctx, url, body):
    # HTTP POST with error handling.
    try:
        return ctx.run(http_post, url=url, body=body)
    except:
        ctx.ui.error("POST failed: " + url)
        return None
```

**Common Errors:**
- Network unreachable
- DNS resolution failure
- Connection timeout (no timeout setting - hangs indefinitely)
- HTTP 4xx errors (client errors)
- HTTP 5xx errors (server errors)
- Invalid URL format

**Best Practices:**
- Always wrap requests in try/except
- Implement retry logic for transient failures
- Validate responses before using them
- Log failures with context
- Consider using ctx.http module directly for advanced needs

## Limitations

Current HTTP tools have several limitations:

1. **No Custom Headers**: Cannot set custom headers (Authorization, User-Agent, etc.)
2. **No Timeout**: Requests may hang indefinitely
3. **No Method Variety**: Only GET and POST (no PUT, DELETE, PATCH)
4. **No Status Code Access**: Only gets body, not status code
5. **No Content-Type Control**: Cannot customize Content-Type header
6. **No Response Headers**: Cannot access response headers

**Workaround:** For advanced HTTP needs, use `ctx.http` module directly:

```python
# Advanced usage with ctx.http module
def advanced_request(ctx):
    # This is conceptual - check API docs for exact syntax
    # response = ctx.http.request(
    #     method="PUT",
    #     url="https://api.example.com/resource",
    #     headers={"Authorization": "Bearer token"},
    #     body="data",
    #     timeout=30
    # )
    pass
```

## Security Considerations

**WARNING:** HTTP operations can expose sensitive data.

```python
# ❌ DANGEROUS: Hardcoded secrets
def bad_api_call(ctx):
    token = "secret_api_key_12345"  # Never do this!
    url = "https://api.example.com/data?token=" + token
    ctx.run(http_get, url=url)

# ✅ SAFE: Use environment variables
def safe_api_call(ctx):
    token = ctx.env.get("API_TOKEN")
    if not token:
        fail("API_TOKEN environment variable not set")
    
    # Use token in request (ideally in header, but tools don't support that)
    # For now, be cautious with URLs containing secrets
    url = "https://api.example.com/data"
    # Better: use POST with body
    payload = ctx.json.encode({"token": token, "action": "fetch"})
    response = ctx.run(http_post, url=url, body=payload)
```

**Security Guidelines:**
- Never hardcode API keys or tokens
- Use environment variables for secrets
- Prefer HTTPS over HTTP
- Validate URLs before using (prevent SSRF)
- Be careful with user-controlled URLs
- Don't log request/response bodies containing secrets

## Performance Tips

1. **No Timeout**: Requests can hang. Use with caution in production workflows.

2. **Sequential Requests**: Each request blocks. For multiple independent requests, 
   consider parallel execution patterns.

3. **Response Size**: Large responses loaded into memory. Be careful with large 
   file downloads.

4. **Caching**: Cache API responses when appropriate to reduce redundant requests.

## Use Cases

### CI/CD Integration

```python
load("//lib/http.star", "http_post")
load("//lib/git.star", "git_diff")

def trigger_deployment(ctx, deploy_url):
    # Trigger deployment with change summary.
    
    diff = ctx.run(git_diff, staged=True)
    
    payload = ctx.json.encode({
        "event": "deploy",
        "changes": diff[:500],  # First 500 chars
        "author": "meowg1k"
    })
    
    response = ctx.run(http_post, url=deploy_url, body=payload)
    ctx.ui.success("Deployment triggered")
```

### Monitoring & Alerts

```python
load("//lib/http.star", "http_post")

def send_alert(ctx, message, severity="info"):
    # Send alert to monitoring system.
    
    alert = ctx.json.encode({
        "message": message,
        "severity": severity,
        "source": "meowg1k",
        "timestamp": ctx.time.now()
    })
    
    webhook = ctx.env.get("ALERT_WEBHOOK_URL")
    ctx.run(http_post, url=webhook, body=alert)
```

## Integration Examples

### With JSON Operations

```python
load("//lib/http.star", "http_get")
load("//lib/json.star", "json_query")

def fetch_and_extract(ctx):
    response = ctx.run(http_get, url="https://api.example.com/data")
    value = ctx.run(json_query, json=response, path="nested.field")
    ctx.ui.info("Extracted: " + value)
```

### With File Operations

```python
load("//lib/http.star", "http_get")
load("//lib/file_ops.star", "file_writer")

def download_and_save(ctx, url, path):
    # Download content and save to file.
    content = ctx.run(http_get, url=url)
    ctx.run(file_writer, path=path, content=content)
    ctx.ui.success("Downloaded to " + path)
```

### With LLM Generation

```python
load("//lib/http.star", "http_get")
load("//lib/llm.star", "llm_generate")

def analyze_api_response(ctx, url):
    # Fetch API data and analyze with LLM.
    response = ctx.run(http_get, url=url)
    
    analysis = ctx.run(llm_generate,
        prompt="Analyze this API response and summarize key insights: " + response,
        preset="smart")
    
    ctx.output.writeline(analysis)
```

## See Also

- [json.star](json.star) - JSON parsing and querying
- [file_ops.star](file_ops.star) - File operations
- [API Reference](../../API_REFERENCE.md) - HTTP module (ctx.http)
"""

# ==============================================================================
# TOOL HANDLERS
# ==============================================================================

def http_get_handler(ctx):
    """Make HTTP GET request."""
    url = ctx.params["url"]
    result = ctx.http.get(url)
    return result

def http_post_handler(ctx):
    """Make HTTP POST request."""
    url = ctx.params["url"]
    body = ctx.params["body"]
    result = ctx.http.post(url, body=body)
    return result

# ==============================================================================
# TOOL DEFINITIONS
# ==============================================================================

http_get = meow.tool(
    name="http_get",
    description="Make an HTTP GET request",
    params={
        "url": meow.param("string", desc="URL to request", required=True),
    },
    handler=http_get_handler,
)

http_post = meow.tool(
    name="http_post",
    description="Make an HTTP POST request",
    params={
        "url": meow.param("string", desc="URL to request", required=True),
        "body": meow.param("string", desc="Request body", required=True),
    },
    handler=http_post_handler,
)

# Tool set
http_tools = [http_get, http_post]
