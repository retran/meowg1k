// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package templates

// GlobalInitTemplate is the template for ~/.config/meowg1k/init.star
const GlobalInitTemplate = `# ~/.config/meowg1k/init.star
# Global configuration for meowg1k

# Configure your LLM providers
# Get API keys from environment variables for security
meow.provider("openai",
    type="openai",
    api_key=env.get("OPENAI_API_KEY")
)

# Example: Anthropic
# meow.provider("anthropic",
#     type="anthropic",
#     api_key=env.get("ANTHROPIC_API_KEY")
# )

# Example: Local model (Ollama, LM Studio, etc.)
# meow.provider("local",
#     type="openai-compatible",
#     base_url="http://localhost:11434/v1",
#     api_key="ollama"
# )

# Define models
meow.model("gpt4",
    provider="openai",
    model="gpt-4"
)

meow.model("gpt4-mini",
    provider="openai",
    model="gpt-4o-mini"
)

meow.model("embed",
    provider="openai",
    model="text-embedding-3-small"
)

# Create presets (model + parameters)
meow.preset("smart",
    model="gpt4",
    temperature=0.2,
    max_tokens=4000
)

meow.preset("fast",
    model="gpt4-mini",
    temperature=0.5,
    max_tokens=2000
)

meow.preset("embed",
    model="embed"
)

# Example: Global command available in all projects
def handle_sync(ctx):
    """Sync current branch with main"""
    current = git.branch()
    
    ui.info("Syncing with main...")
    git.checkout("main")
    result = shell.exec("git pull --rebase")
    
    if result.exit_code != 0:
        ui.error("Failed to sync")
        return
    
    git.checkout(current)
    ui.success("Synced with main!")

meow.register_command(
    name="sync",
    description="Sync current branch with main",
    handler=handle_sync
)
`

// ProjectInitTemplate is the template for ./.meowg1k/init.star
const ProjectInitTemplate = `# ./.meowg1k/init.star
# Project-specific configuration

# Example: Override preset for this project
# meow.preset("smart",
#     temperature=0.3,
#     max_tokens=8000
# )

# Example: Add project-specific provider
# meow.provider("company",
#     type="openai-compatible",
#     base_url="https://llm.internal.company.com/v1",
#     api_key=env.get("COMPANY_API_KEY")
# )

# Example: Project-specific model
# meow.model("company-gpt",
#     provider="company",
#     model="gpt-4-finetuned"
# )

# Example: Commit message generator
def handle_commit(ctx):
    """Generate commit message from staged git changes"""
    diff = git.diff(target="staged")
    
    if len(diff.files) == 0:
        ui.error("No staged changes found")
        return
    
    ui.info("Analyzing {} changed file(s)...".format(len(diff.files)))
    
    # Build prompt
    files_list = ", ".join(diff.files)
    ticket = ctx.flags.get("ticket", "")
    
    prompt = "Generate a concise, conventional commit message.\n\n"
    prompt += "Files changed: {}\n".format(files_list)
    
    if ticket:
        prompt += "Ticket: {}\n".format(ticket)
    
    prompt += "\nDiff:\n{}".format(diff.raw)
    
    # Generate message
    msg = ctx.llm.chat(
        preset="smart",
        system="Generate a concise, conventional commit message following the format: <type>(<scope>): <subject>",
        prompt=prompt
    )
    
    ui.success("Generated commit message:")
    print("")
    print(msg)
    print("")
    
    # Optionally auto-commit
    if ctx.flags.get("commit"):
        git.add(diff.files)
        git.commit(message=msg)
        ui.success("Committed!")

meow.register_command(
    name="commit",
    description="Generate commit message from staged changes",
    long_description="""Generate a commit message based on staged git changes.

Examples:
  meow commit              # Generate and print
  meow commit --commit     # Generate and auto-commit
  meow commit -t PROJ-123  # Include ticket reference
""",
    handler=handle_commit,
    flags={
        "commit": meow.flag(
            type="bool",
            description="Automatically commit with generated message"
        ),
        "ticket": meow.flag(
            short="t",
            type="string",
            description="Ticket/issue ID to include in message"
        )
    }
)

# Example: Semantic code search
def handle_search(ctx):
    """Search codebase using semantic similarity"""
    query = ctx.flags.get("query", "")
    
    if not query:
        ui.error("Please provide a search query with --query")
        return
    
    top_k = ctx.flags.get("top_k", 10)
    min_score = ctx.flags.get("min_score", 0.7)
    
    ui.info("Searching for: {}".format(query))
    
    results = index.search(
        query=query,
        top_k=top_k,
        min_score=min_score
    )
    
    if len(results) == 0:
        ui.warn("No results found")
        return
    
    ui.success("Found {} result(s):".format(len(results)))
    print("")
    
    for i, result in enumerate(results):
        print("{}. {}:{}-{}".format(i+1, result.file_path, result.start_line, result.end_line))
        print("   Score: {:.2f}".format(result.score))
        print("   {}...".format(result.content[:200]))
        print("")

meow.register_command(
    name="search",
    description="Semantic code search",
    long_description="""Search codebase using semantic similarity.

First, build the index with 'meow index'.

Examples:
  meow search --query "authentication logic"
  meow search -q "error handling" --top-k 20
""",
    handler=handle_search,
    flags={
        "query": meow.flag(
            short="q",
            type="string",
            required=True,
            description="Search query"
        ),
        "top_k": meow.flag(
            short="k",
            type="int",
            default=10,
            description="Number of results to return"
        ),
        "min_score": meow.flag(
            type="float",
            default=0.7,
            description="Minimum similarity score (0.0-1.0)"
        )
    }
)
`

// ProjectCommitCommandTemplate is deprecated - use ProjectInitTemplate instead
const ProjectCommitCommandTemplate = ""

// ProjectSearchCommandTemplate is deprecated - use ProjectInitTemplate instead
const ProjectSearchCommandTemplate = ""

// GitignoreEntries are the entries to add to .gitignore
const GitignoreEntries = `
# meowg1k
.meowg1k/cache/
.meowg1k/state/
`
