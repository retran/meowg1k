"""
Semantic Code Search Library for meowg1k

This library provides semantic code search capabilities using vector embeddings
and Retrieval-Augmented Generation (RAG). Search code by meaning, not just keywords.

## Quick Start

```python
load("//lib/code_search.star", "code_search")

def handler(ctx):
    # Search for authentication code
    results_json = ctx.run(code_search, 
                          query="user authentication and login",
                          limit=5)
    results = ctx.json.decode(results_json)
    
    for match in results:
        ctx.ui.info("%s (score: %.2f)" % (match["file"], match["score"]))
```

## Available Tools

- `code_search` - Semantic code search using vector embeddings

## Prerequisites

**Index Required:** Semantic search requires a pre-built vector index.

Build index before using:
```python
# In your command or script
def handler(ctx):
    # Build index first (one-time or periodic)
    ctx.ui.info("Building code index...")
    ctx.index.build()
    
    # Now search
    results = ctx.run(code_search, query="authentication logic")
```

Or build from command line:
```bash
meow index build
```

## API Reference

### code_search

Search codebase semantically using vector embeddings.

**Parameters:**
- `query` (string, required): Natural language search query
- `limit` (int, optional): Maximum results to return (default: 5)

**Returns:** string - JSON array of {file, chunk, score} objects

**Example:**
```python
# Find authentication code
results_json = ctx.run(code_search,
                      query="user authentication and login logic",
                      limit=10)
matches = ctx.json.decode(results_json)

for match in matches:
    ctx.ui.info("File: " + match["file"])
    ctx.ui.info("Score: " + str(match["score"]))
    ctx.output.writeline(match["chunk"])
    ctx.output.writeline("---")
```

**Result Fields:**
- `file` (string) - File path relative to workspace
- `chunk` (string) - Code chunk that matched
- `score` (float) - Relevance score (0.0 to 1.0, higher is better)

**How It Works:**
1. Query is converted to vector embedding
2. HNSW vector search finds similar code chunks
3. Results ranked by cosine similarity
4. Top-k results returned with metadata

## Advanced Usage

### Context-Aware Code Q&A

```python
load("//lib/code_search.star", "code_search")
load("//lib/llm.star", "llm_generate")

def answer_code_question(ctx, question):
    # Answer questions about codebase using RAG
    
    # Search for relevant code
    results_json = ctx.run(code_search, query=question, limit=5)
    results = ctx.json.decode(results_json)
    
    # Build context from results
    context = "Relevant code:\\n\\n"
    for match in results:
        context += "File: %s\\n" % match["file"]
        context += match["chunk"] + "\\n\\n"
    
    # Generate answer with context
    prompt = "Based on this code:\\n\\n" + context + "\\n\\nAnswer: " + question
    
    answer = ctx.run(llm_generate, 
                    prompt=prompt,
                    preset="smart")
    
    return answer
```

### Find Similar Code

```python
load("//lib/code_search.star", "code_search")
load("//lib/file_ops.star", "file_reader")

def find_similar_code(ctx, file_path):
    # Find code similar to a given file
    
    # Read target file
    content = ctx.run(file_reader, path=file_path)
    
    # Use file content as query (truncate if needed)
    query = content[:1000]  # First 1000 chars as sample
    
    # Search for similar code
    results_json = ctx.run(code_search, query=query, limit=10)
    results = ctx.json.decode(results_json)
    
    ctx.ui.info("Code similar to " + file_path + ":")
    for match in results:
        if match["file"] != file_path:  # Exclude self
            ctx.ui.info("- %s (%.2f)" % (match["file"], match["score"]))
```

### Documentation Generator

```python
load("//lib/code_search.star", "code_search")
load("//lib/llm.star", "llm_generate")

def generate_docs_for_topic(ctx, topic):
    # Generate documentation for a specific topic
    
    # Find relevant code
    results_json = ctx.run(code_search, 
                          query=topic + " implementation and usage",
                          limit=10)
    results = ctx.json.decode(results_json)
    
    # Collect code samples
    code_samples = []
    for match in results:
        code_samples.append({
            "file": match["file"],
            "code": match["chunk"],
        })
    
    # Generate documentation
    prompt = ("Generate documentation for: " + topic + 
              "\\n\\nBased on these code examples:\\n" + ctx.json.encode(code_samples) +
              "\\n\\nInclude:\\n- Overview\\n- Usage examples\\n- API reference\\n- Best practices")
    
    docs = ctx.run(llm_generate, prompt=prompt, preset="smart")
    return docs
```

### Code Review Assistant

```python
load("//lib/code_search.star", "code_search")
load("//lib/git.star", "git_diff")

def review_with_context(ctx):
    # Review changes with context from similar code
    
    # Get changed code
    diff = ctx.run(git_diff, staged=True)
    
    # Extract meaningful query from diff
    # (simplified - real implementation would parse diff better)
    query = "similar code patterns and implementations"
    
    # Find similar existing code
    results_json = ctx.run(code_search, query=query, limit=5)
    results = ctx.json.decode(results_json)
    
    ctx.ui.info("Similar patterns in codebase:")
    for match in results:
        ctx.ui.info("- " + match["file"])
```

### Index Management

```python
def manage_index(ctx, action):
    # Manage code search index
    
    if action == "build":
        ctx.ui.info("Building index...")
        ctx.index.build()
        ctx.ui.success("Index built successfully")
    
    elif action == "rebuild":
        ctx.ui.info("Rebuilding index...")
        # Note: No explicit delete, build replaces
        ctx.index.build()
        ctx.ui.success("Index rebuilt")
    
    elif action == "stats":
        # Test search to verify index exists
        try:
            results_json = ctx.run(code_search, query="test", limit=1)
            results = ctx.json.decode(results_json)
            ctx.ui.success("Index operational, returned %d results" % len(results))
        except:
            ctx.ui.error("Index not built or corrupted")
```

## Error Handling

Code search can fail if index is not built or corrupted:

```python
load("//lib/code_search.star", "code_search")

def safe_search(ctx, query):
    # Search with error handling
    try:
        results_json = ctx.run(code_search, query=query, limit=5)
        return ctx.json.decode(results_json)
    except:
        ctx.ui.error("Code search failed - index may not be built")
        ctx.ui.info("Run: meow index build")
        return []
```

**Common Errors:**
- Index not built (run `ctx.index.build()` first)
- Empty query string
- Corrupted index database
- Embedding service unavailable

**Best Practices:**
- Always build index before first use
- Rebuild index after major code changes
- Handle empty results gracefully
- Provide fallback to text search when semantic search fails

## Performance Tips

1. **Index Size**: Large codebases create large indexes. Index builds can take 
   several minutes for 100K+ lines of code.

2. **Query Limit**: Lower `limit` values are faster. Start with 5-10 results.

3. **Incremental Updates**: No incremental index updates - must rebuild entire 
   index. Schedule periodic rebuilds rather than on every change.

4. **Result Processing**: Chunks can be large. Truncate if only showing summaries.

5. **Caching**: Cache search results if running same query multiple times.

## How Vector Search Works

```
1. Indexing Phase (ctx.index.build()):
   - Scan codebase files
   - Chunk code into semantic units
   - Generate embeddings for each chunk
   - Build HNSW index for fast similarity search
   - Store in SQLite database

2. Query Phase (code_search):
   - Convert query to embedding vector
   - Search HNSW index for nearest neighbors
   - Retrieve matching chunks with scores
   - Return ranked results
```

**Embedding Model:** Uses configured embedding provider (e.g., Voyage, OpenAI).

**Chunking Strategy:** Code is chunked by function/class boundaries when possible, 
otherwise by token/line limits.

## Comparison: Semantic vs Text Search

| Feature | Semantic Search | Text Search (grep) |
|---------|----------------|-------------------|
| Query type | Natural language | Regex/exact text |
| Understands meaning | Yes | No |
| Requires index | Yes | No |
| Speed | Fast (with index) | Variable |
| Finds similar concepts | Yes | No |
| Finds exact matches | Sometimes | Always |

**When to use Semantic Search:**
- "How does authentication work?"
- "Find error handling code"
- "Similar database query patterns"

**When to use Text Search:**
- "Find function named `getUserByID`"
- "Find all TODO comments"
- "Find specific string literal"

## Integration Examples

### With File Operations

```python
load("//lib/code_search.star", "code_search")
load("//lib/file_ops.star", "file_reader")

def extract_function(ctx, description):
    # Find and extract function matching description
    results_json = ctx.run(code_search, query=description, limit=1)
    results = ctx.json.decode(results_json)
    
    if results:
        match = results[0]
        return match["chunk"]
    
    return None
```

### With LLM Generation

```python
load("//lib/code_search.star", "code_search")
load("//lib/llm.star", "llm_generate")

def code_qa_system(ctx, question):
    # RAG-based code Q&A system
    
    # Retrieve relevant code
    results_json = ctx.run(code_search, query=question, limit=5)
    results = ctx.json.decode(results_json)
    
    # Augment prompt with retrieved context
    context = "\\n\\n".join([r["chunk"] for r in results])
    
    # Generate answer
    answer = ctx.run(llm_generate,
        prompt="Question: %s\\n\\nContext:\\n%s" % (question, context),
        preset="smart")
    
    return answer
```

## See Also

- [file_ops.star](file_ops.star) - File operations (text search alternative)
- [llm.star](llm.star) - LLM generation for RAG workflows
- [API Reference](../../API_REFERENCE.md) - Index module (ctx.index)
"""

# ==============================================================================
# TOOL HANDLERS
# ==============================================================================

def code_search_handler(ctx):
    """Search code semantically using vector embeddings."""
    query = ctx.params["query"]
    limit = ctx.params.get("limit", 5)
    
    results = ctx.index.search(query, limit=limit)
    return ctx.json.encode(results)

# ==============================================================================
# TOOL DEFINITIONS
# ==============================================================================

code_search = meow.tool(
    name="code_search",
    description="Search code semantically using vector embeddings",
    params={
        "query": meow.param("string", desc="Search query", required=True),
        "limit": meow.param("int", desc="Maximum number of results", default=5),
    },
    handler=code_search_handler,
)

# Tool set
code_search_tools = [code_search]
