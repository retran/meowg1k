# ==============================================================================
# MEOWG1K CONFIGURATION
# ==============================================================================
# Main configuration file - defines providers, models, presets, and registers commands

# ==============================================================================
# PROVIDERS
# ==============================================================================

meow.provider("gemini",
    type="gemini",
    api_key=env.get("MEOW_GEMINI_API_KEY")
)

# ==============================================================================
# MODELS
# ==============================================================================

meow.model("gemini-flash",
    provider="gemini",
    model="gemini-3-flash-preview",
    max_input_tokens=1048576,
    max_output_tokens=65536
)

meow.model("gemini-pro",
    provider="gemini",
    model="gemini-3-pro-preview",
    max_input_tokens=1048576,
    max_output_tokens=65536
)

meow.model("gemini-embeddings",
    provider="gemini",
    model="gemini-embedding-001",
    max_input_tokens=2048
)

# ==============================================================================
# PRESETS
# ==============================================================================

meow.preset("fast",
    model="gemini-flash",
    temperature=0.2
)

meow.preset("smart",
    model="gemini-pro",
    temperature=0.2
)

meow.preset("embeddings",
    model="gemini-embeddings"
)

# ==============================================================================
# REGISTER COMMANDS
# ==============================================================================
# Load and configure each command module via setup()

load("//commands/write.star", "setup")
setup(preset="smart")

load("//commands/commit.star", commit_setup = "setup")
commit_setup(preset="smart", summarize_preset="fast")

load("//commands/pr.star", pr_setup = "setup")
pr_setup(preset="smart", summarize_preset="fast")

load("//commands/code.star", code_setup = "setup")
code_setup(preset="smart")

load("//commands/search.star", search_setup = "setup")
search_setup(
    ignore_patterns = [
        ".git/**",
        "node_modules/**",
        "**/*.pyc",
        "__pycache__/**",
        ".env",
        "*.lock",
        "**/.DS_Store",
        "**/dist/**",
        "**/build/**",
        "**/*.min.js",
        "**/*.min.css",
    ]
)

# ==============================================================================
# AGENT COMMANDS
# ==============================================================================
load("//commands/review-agent.star", review_handler = "review_agent_handler")
load("//commands/orchestrator-agent.star", orch_handler = "orchestrator_handler")

# ==============================================================================
# EXAMPLE COMMANDS (Structured Outputs)
# ==============================================================================
load("//commands/extract.star", extract_setup = "setup")
extract_setup()

# ==============================================================================
# SESSION MANAGEMENT COMMANDS
# ==============================================================================
load("//commands/sessions.star", sessions_list_handler = "sessions_handler")
