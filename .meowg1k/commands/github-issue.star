# ==============================================================================
# GitHub Issue Command - Create GitHub Issues via HTTP API
# ==============================================================================
"""
Create GitHub issues directly from the command line using the GitHub REST API.

FEATURES:
  - Create Issues: Post issues to any GitHub repository
  - Labels & Assignees: Add labels and assign users
  - Template Support: Use issue templates
  - Token Auth: Authenticate via GITHUB_TOKEN env var
  - Interactive: Confirm before creating

INSTALLATION:
  # In your .meowg1k/init.star
  load("//commands/github-issue.star", "setup")
  
  setup()

EXAMPLES:
  # Create a bug report
  meow github-issue --title "Fix parser crash" --body "Parser crashes on empty input" --repo owner/repo
  
  # Create issue with labels
  meow github-issue -t "Add HTTP module" -b "Need HTTP client for API calls" -r myorg/myrepo -l bug,enhancement
  
  # Create and assign
  meow github-issue -t "Update docs" -b "API reference needs http module docs" -r me/proj -a username

PARAMETERS:
  --title, -t        Issue title (required)
  --body, -b         Issue body/description (required)
  --repo, -r         Repository (owner/name format, required)
  --labels, -l       Comma-separated labels (optional)
  --assignees, -a    Comma-separated assignees (optional)
"""

# Configuration defaults
config = {
    "api_base": "https://api.github.com",
}

def handler(ctx):
    """Create a GitHub issue via the REST API."""

    # Get parameters
    title = ctx.title
    body = ctx.body
    repo = ctx.repo
    labels_str = ctx.labels or ""
    assignees_str = ctx.assignees or ""

    # Get GitHub token from environment
    token = ctx.env.get("GITHUB_TOKEN")
    if not token:
        ctx.ui.error("GITHUB_TOKEN environment variable not set")
        ctx.ui.info("Set it with: export GITHUB_TOKEN=your_token_here")
        return

    # Parse labels and assignees
    labels = [l.strip() for l in labels_str.split(",") if l.strip()]
    assignees = [a.strip() for a in assignees_str.split(",") if a.strip()]

    # Build request body
    issue_data = {
        "title": title,
        "body": body,
    }

    if labels:
        issue_data["labels"] = labels

    if assignees:
        issue_data["assignees"] = assignees

    # Show what we're about to create
    ctx.ui.banner("Creating GitHub Issue")
    ctx.ui.info("Repository: " + repo)
    ctx.ui.info("Title: " + title)
    if labels:
        ctx.ui.info("Labels: " + ", ".join(labels))
    if assignees:
        ctx.ui.info("Assignees: " + ", ".join(assignees))

    # Confirm
    if not ctx.ui.confirm("Create this issue?", default=True):
        ctx.ui.warn("Cancelled")
        return

    # Make API request
    url = config["api_base"] + "/repos/" + repo + "/issues"

    ctx.ui.info("Sending request to GitHub...")
    response = ctx.http.post(
        url,
        json=issue_data,
        headers={
            "Authorization": "Bearer " + token,
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        }
    )

    # Handle response
    if response.ok:
        issue = response.json
        issue_url = issue.get("html_url", "")
        issue_number = issue.get("number", 0)

        ctx.ui.success("Issue #" + str(issue_number) + " created successfully!")
        ctx.ui.info("URL: " + issue_url)
    else:
        ctx.ui.error("Failed to create issue (status: " + str(response.status_code) + ")")

        # Try to parse error message
        if response.json and response.json != None:
            error_msg = response.json.get("message", "Unknown error")
            ctx.ui.error("Error: " + error_msg)
        else:
            ctx.ui.error("Response: " + response.body[:200])

def setup():
    """Register the github-issue command."""

    tool = meow.tool(
        name="github-issue",
        handler=handler,
        description="Create a GitHub issue via REST API",
        params={
            "title": meow.param("string", desc="Issue title", required=True, short="t"),
            "body": meow.param("string", desc="Issue body/description", required=True, short="b"),
            "repo": meow.param("string", desc="Repository in owner/name format", required=True, short="r"),
            "labels": meow.param("string", desc="Comma-separated labels", default="", short="l"),
            "assignees": meow.param("string", desc="Comma-separated assignees", default="", short="a"),
        },
    )

    meow.command(tool)
