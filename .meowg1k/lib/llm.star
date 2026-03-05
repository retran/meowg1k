"""
LLM Operations Library for meowg1k

This library provides Large Language Model (LLM) operations for text generation,
analysis, and transformation. Perfect for building agentic workflows, content
generation, and AI-powered automation.

## Quick Start

```python
load("//lib/llm.star", "llm_generate")

def handler(ctx):
    # Simple generation
    response = ctx.run(llm_generate, 
                      prompt="Explain hexagonal architecture",
                      preset="smart")
    
    ctx.output.writeline(response)
    
    # With system prompt
    summary = ctx.run(llm_generate,
                     prompt="Summarize: " + long_text,
                     system="Be concise, max 3 sentences",
                     preset="fast")
```

## Available Tools

- `llm_generate` - Generate text using LLM with custom prompts

### Tool Sets
- `llm_tools` - All LLM operation tools (1 tool)

## API Reference

### llm_generate

Generate text using a Large Language Model.

**Parameters:**
- `prompt` (string, required): User prompt
- `system` (string, optional): System prompt (default: "")
- `preset` (string, optional): LLM preset to use (default: "smart")

**Returns:** string - LLM-generated text

**Example:**
```python
# Basic generation
response = ctx.run(llm_generate,
                  prompt="What is Go's concurrency model?",
                  preset="smart")

# With system instructions
code = ctx.run(llm_generate,
              prompt="Write a binary search function",
              system="You are an expert Go programmer. Write idiomatic Go code.",
              preset="smart")

# Quick analysis with fast preset
summary = ctx.run(llm_generate,
                 prompt="Summarize: " + document,
                 system="Summarize in 2-3 sentences",
                 preset="fast")
```

**Presets:**
Presets are defined in `.meowg1k/init.star` and typically include:
- `fast` - Quick responses, lower quality (e.g., GPT-3.5, Claude Haiku, Gemini Flash)
- `smart` - Balanced quality and speed (e.g., GPT-4, Claude Sonnet)
- `best` - Highest quality (e.g., GPT-4 Turbo, Claude Opus)

Check your `init.star` for configured presets.

**Cost Consideration:** Each call consumes tokens. Use appropriate preset for 
cost/quality tradeoff.

## Advanced Usage

### Document Summarization

```python
load("//lib/llm.star", "llm_generate")
load("//lib/file_ops.star", "file_reader")

def summarize_file(ctx, file_path):
    # Summarize a document file.
    
    # Read document
    content = ctx.run(file_reader, path=file_path)
    
    # Generate summary
    summary = ctx.run(llm_generate,
        prompt="Summarize this document: " + content,
        system="Create a concise summary with key points. Use bullet points.",
        preset="smart")
    
    ctx.output.writeline("Summary of " + file_path + ":")
    ctx.output.writeline(summary)
    
    return summary
```

### Code Generation

```python
load("//lib/llm.star", "llm_generate")
load("//lib/file_ops.star", "file_writer")

def generate_code(ctx, description):
    # Generate code from description.
    
    prompt = """Generate Go code for: %s

Requirements:
- Include error handling
- Add comments
- Follow Go best practices
- Use standard library when possible
""" % description
    
    code = ctx.run(llm_generate,
                  prompt=prompt,
                  system="You are an expert Go developer.",
                  preset="smart")
    
    # Save generated code
    ctx.run(file_writer, path="generated.go", content=code)
    
    ctx.ui.success("Code generated: generated.go")
    return code
```

### Code Review

```python
load("//lib/llm.star", "llm_generate")
load("//lib/git.star", "git_diff")

def review_changes(ctx):
    # Review git changes with LLM.
    
    # Get changes
    diff = ctx.run(git_diff, staged=True)
    
    if not diff:
        ctx.ui.warning("No staged changes")
        return
    
    # Generate review
    review = ctx.run(llm_generate,
        prompt="Review this code change and provide feedback: " + diff,
        system="""You are a code reviewer. Analyze the changes and provide:
- Summary of changes
- Potential issues or bugs
- Suggestions for improvement
- Security concerns

Be constructive and specific.""",
        preset="smart")
    
    ctx.output.writeline("Code Review:")
    ctx.output.writeline(review)
```

### Commit Message Generation

```python
load("//lib/llm.star", "llm_generate")
load("//lib/git.star", "git_diff", "git_status")

def generate_commit_message(ctx):
    # Generate conventional commit message from staged changes.
    
    # Check for staged changes
    status_json = ctx.run(git_status)
    status = ctx.json.decode(status_json)
    
    if not status.get("staged", []):
        ctx.ui.error("No staged changes")
        return
    
    # Get diff
    diff = ctx.run(git_diff, staged=True)
    
    # Generate message
    message = ctx.run(llm_generate,
        prompt="Generate conventional commit message for: " + diff,
        system="""Generate a conventional commit message.

Format:
<type>(<scope>): <subject>

<body>

Types: feat, fix, docs, style, refactor, test, chore
Subject: ≤50 chars, imperative mood, no period
Body: Wrap at 72 chars, explain what and why""",
        preset="smart")
    
    ctx.ui.success("Generated commit message:")
    ctx.output.writeline(message)
    return message
```

### Documentation Generation

```python
load("//lib/llm.star", "llm_generate")
load("//lib/file_ops.star", "file_reader", "file_writer")

def generate_documentation(ctx, code_file):
    # Generate documentation for code file.
    
    # Read code
    code = ctx.run(file_reader, path=code_file)
    
    # Generate docs
    docs = ctx.run(llm_generate,
        prompt="Generate API documentation for this code: " + code,
        system="""Generate comprehensive documentation including:
- Package/module overview
- Function/method descriptions
- Parameter descriptions
- Return value descriptions
- Usage examples
- Notes and warnings

Use Markdown format.""",
        preset="smart")
    
    # Save docs
    doc_file = code_file + ".md"
    ctx.run(file_writer, path=doc_file, content=docs)
    
    ctx.ui.success("Documentation generated: " + doc_file)
```

### Refactoring Suggestions

```python
load("//lib/llm.star", "llm_generate")
load("//lib/file_ops.star", "file_reader")

def suggest_refactorings(ctx, file_path):
    # Get refactoring suggestions for code.
    
    code = ctx.run(file_reader, path=file_path)
    
    suggestions = ctx.run(llm_generate,
        prompt="Suggest refactorings for this code: " + code,
        system="""Analyze the code and suggest refactorings focusing on:
- Code duplication (DRY principle)
- Function complexity
- Naming improvements
- Design patterns
- Performance optimizations

Be specific with before/after examples.""",
        preset="smart")
    
    ctx.output.writeline("Refactoring Suggestions for " + file_path + ":")
    ctx.output.writeline(suggestions)
```

### Test Generation

```python
load("//lib/llm.star", "llm_generate")
load("//lib/file_ops.star", "file_reader", "file_writer")

def generate_tests(ctx, source_file):
    # Generate unit tests for source code.
    
    # Read source
    code = ctx.run(file_reader, path=source_file)
    
    # Generate tests
    tests = ctx.run(llm_generate,
        prompt="Generate comprehensive unit tests for: " + code,
        system="""Generate Go unit tests using testing package and testify.

Include:
- Test all exported functions
- Edge cases and error conditions
- Table-driven tests where appropriate
- Clear test names (TestFunctionName_Scenario)
- Setup and teardown when needed""",
        preset="smart")
    
    # Save tests
    test_file = source_file.replace(".go", "_test.go")
    ctx.run(file_writer, path=test_file, content=tests)
    
    ctx.ui.success("Tests generated: " + test_file)
```

### Multi-Step Analysis

```python
load("//lib/llm.star", "llm_generate")
load("//lib/code_search.star", "code_search")

def analyze_architecture(ctx, component):
    # Analyze architecture of a component.
    
    # Step 1: Find relevant code
    results_json = ctx.run(code_search, 
                          query=component + " implementation",
                          limit=5)
    results = ctx.json.decode(results_json)
    
    # Step 2: Collect code
    code_context = ""
    for match in results:
        code_context += "File: %s\\n%s\\n\\n" % (match["file"], match["chunk"])
    
    # Step 3: Analyze architecture
    analysis = ctx.run(llm_generate,
        prompt="Analyze the architecture of this component: " + code_context,
        system="""Analyze the architecture and provide:
- Component responsibilities
- Dependencies
- Design patterns used
- Strengths and weaknesses
- Improvement suggestions""",
        preset="smart")
    
    ctx.output.writeline(analysis)
```

## Error Handling

LLM operations can fail due to API issues, rate limits, or invalid input:

```python
load("//lib/llm.star", "llm_generate")

def safe_generate(ctx, prompt, preset="smart"):
    # Generate with error handling.
    try:
        response = ctx.run(llm_generate,
                          prompt=prompt,
                          preset=preset)
        return response
    except:
        ctx.ui.error("LLM generation failed")
        return None

def generate_with_fallback(ctx, prompt):
    # Generate with fallback to faster model.
    try:
        # Try best model first
        return ctx.run(llm_generate, prompt=prompt, preset="smart")
    except:
        ctx.ui.warning("Smart model failed, trying fast model")
        try:
            return ctx.run(llm_generate, prompt=prompt, preset="fast")
        except:
            ctx.ui.error("All models failed")
            return None
```

**Common Errors:**
- API rate limits exceeded
- Invalid API key or configuration
- Network errors
- Token limit exceeded (prompt too long)
- Model unavailable

**Best Practices:**
- Wrap calls in try/except for production
- Implement retry logic for transient failures
- Validate prompt length
- Handle rate limits gracefully
- Provide fallback options

## Performance Tips

1. **Preset Selection**: Use fastest preset that meets quality needs
   ```python
   # Fast preset for simple tasks
   ctx.run(llm_generate, prompt="Summarize: ...", preset="fast")
   
   # Smart preset for complex tasks
   ctx.run(llm_generate, prompt="Refactor this code: ...", preset="smart")
   ```

2. **Prompt Length**: Shorter prompts are faster and cheaper
   ```python
   # Trim context to essentials
   code_sample = full_code[:2000]  # First 2000 chars
   ```

3. **Batch Processing**: Group related tasks when possible

4. **Caching**: Cache responses for repeated queries
   ```python
   # Store in session
   cache_key = "analysis_" + file_path
   cached = ctx.session.get(cache_key)
   if cached:
       return cached
   
   result = ctx.run(llm_generate, ...)
   ctx.session.set(cache_key, result)
   ```

5. **System Prompts**: Reuse system prompts across calls

## Cost Management

LLM calls cost money. Manage costs effectively:

```python
def cost_aware_generate(ctx, prompt, max_tokens_estimate):
    # Generate with cost awareness.
    
    # Estimate cost (simplified)
    if max_tokens_estimate > 10000:
        ctx.ui.warning("Large request, estimated high cost")
        # Could prompt user for confirmation
    
    return ctx.run(llm_generate, prompt=prompt, preset="smart")
```

**Cost Tips:**
- Use `fast` preset for simple tasks
- Trim prompts to essential context
- Cache responses when possible
- Batch similar requests
- Monitor API usage

## Integration Examples

### With Code Search (RAG)

```python
load("//lib/llm.star", "llm_generate")
load("//lib/code_search.star", "code_search")

def answer_code_question(ctx, question):
    # Answer using RAG pattern.
    
    # Retrieve relevant code
    results_json = ctx.run(code_search, query=question, limit=3)
    results = ctx.json.decode(results_json)
    
    context = "\\n\\n".join([r["chunk"] for r in results])
    
    # Generate answer with context
    answer = ctx.run(llm_generate,
        prompt="Question: %s\\n\\nContext:\\n%s" % (question, context),
        preset="smart")
    
    return answer
```

### With Git Operations

```python
load("//lib/llm.star", "llm_generate")
load("//lib/git.star", "git_diff")

def explain_changes(ctx):
    diff = ctx.run(git_diff, staged=True)
    explanation = ctx.run(llm_generate,
        prompt="Explain these changes in simple terms: " + diff,
        preset="fast")
    ctx.output.writeline(explanation)
```

### With File Operations

```python
load("//lib/llm.star", "llm_generate")
load("//lib/file_ops.star", "file_reader", "file_writer")

def improve_readme(ctx):
    readme = ctx.run(file_reader, path="README.md")
    improved = ctx.run(llm_generate,
        prompt="Improve this README: " + readme,
        system="Make it more professional and comprehensive",
        preset="smart")
    ctx.run(file_writer, path="README_improved.md", content=improved)
```

## See Also

- [code_search.star](code_search.star) - Semantic code search for RAG
- [git.star](git.star) - Git operations
- [file_ops.star](file_ops.star) - File operations
- [API Reference](../../API_REFERENCE.md) - LLM module (ctx.llm)
"""

# ==============================================================================
# TOOL HANDLERS
# ==============================================================================

def llm_generate_handler(ctx):
    """Generate text using an LLM."""
    prompt = ctx.params["prompt"]
    system = ctx.params.get("system", "")
    preset = ctx.params.get("preset", "smart")
    
    result = ctx.llm.chat(prompt=prompt, system=system, preset=preset)
    return result

# ==============================================================================
# TOOL DEFINITIONS
# ==============================================================================

llm_generate = meow.tool(
    name="llm_generate",
    description="Generate text using an LLM",
    params={
        "prompt": meow.param("string", desc="User prompt", required=True),
        "system": meow.param("string", desc="System prompt", default=""),
        "preset": meow.param("string", desc="LLM preset to use", default="smart"),
    },
    handler=llm_generate_handler,
)

# Tool set
llm_tools = [llm_generate]
