# ==============================================================================
# MEOWG1K CONFIGURATION
# ==============================================================================
# Main configuration file - defines providers, models, presets, and registers commands

# ==============================================================================
# PROVIDERS
# ==============================================================================

meow.provider("copilot",
    type="github-copilot",
    app_id="Iv1.b507a08c87ecfe98",
    editor_version="Neovim/0.6.1",
    editor_plugin_version="copilot.vim/1.16.0",
    user_agent="GithubCopilot/1.155.0",
    copilot_integration_id="vscode-chat",
    openai_organization="github-copilot",
    openai_intent="conversation-panel"
)

meow.provider("gemini",
    type="gemini",
    api_key=env.get("MEOW_GEMINI_API_KEY")
)

# ==============================================================================
# MODELS
# ==============================================================================

meow.model("copilot-sonnet",
    provider="copilot",
    model="claude-sonnet-4.6",
    max_input_tokens=200000,
    max_output_tokens=64000
)

meow.model("copilot-haiku",
    provider="copilot",
    model="claude-haiku-4.5",
    max_input_tokens=200000,
    max_output_tokens=64000
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
    model="copilot-haiku",
    temperature=0.2
)

meow.preset("smart",
    model="copilot-sonnet",
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
# EXAMPLE COMMANDS (Structured Outputs)
# ==============================================================================
load("//commands/extract.star", extract_setup = "setup")
extract_setup()
