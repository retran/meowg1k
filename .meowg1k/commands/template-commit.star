# ==============================================================================
# Template Commit Example - Demonstrates template module usage
# ==============================================================================
"""
Example command showing how to use the template module for commit messages.
This is a simplified version demonstrating template-based commit generation.

INSTALLATION:
  # In your .meowg1k/init.star
  load("//commands/template-commit.star", "setup")
  setup()

USAGE:
  meow template-commit --type feat --scope auth --desc "add OAuth2 support"

PARAMETERS:
  --type, -t         Commit type (feat, fix, docs, etc.)
  --scope, -s        Component scope (e.g., auth, ui, api)
  --desc, -d         Short description
  --body, -b         Detailed body (optional)
  --breaking         Breaking change description (optional)
"""

# Built-in conventional commit template
_COMMIT_TEMPLATE = """{{.Type}}({{.Scope}}): {{.Description}}

{{if .Body}}
{{.Body}}
{{end}}
{{if .Breaking}}
BREAKING CHANGE: {{.Breaking}}
{{end}}
{{if .FilesChanged}}
Files changed:
{{range .FilesChanged}}  - {{.}}
{{end}}
{{end}}"""

def setup():
    """Register the template-commit command."""
    
    def handler(ctx):
        """Generate commit message using templates."""
        
        # Get parameters
        commit_type = ctx.params.type
        scope = ctx.params.scope
        description = ctx.params.desc
        body = ctx.params.body or ""
        breaking = ctx.params.breaking or ""
        
        # Validate required fields
        if not commit_type:
            ctx.ui.error("Commit type is required (use --type)")
            return
        if not scope:
            ctx.ui.error("Scope is required (use --scope)")
            return
        if not description:
            ctx.ui.error("Description is required (use --desc)")
            return
        
        # Get list of changed files
        diff = ctx.git.diff(target="staged")
        files_changed = [f.path for f in diff.files] if diff and diff.files else []
        
        # Parse template
        ctx.ui.info("Generating commit message from template...")
        tmpl = ctx.template.parse(_COMMIT_TEMPLATE, name="commit")
        
        # Prepare template data
        data = {
            "Type": commit_type,
            "Scope": scope,
            "Description": description,
            "Body": body,
            "Breaking": breaking,
            "FilesChanged": files_changed,
        }
        
        # Render template
        message = tmpl.render(data)
        
        # Display result
        ctx.ui.divider("thick")
        ctx.ui.success("Generated Commit Message:")
        ctx.ui.divider("thin")
        ctx.output.writeline(message)
        ctx.ui.divider("thick")
        
        # Show metadata
        ctx.ui.info("Type: " + commit_type)
        ctx.ui.info("Scope: " + scope)
        ctx.ui.info("Files: " + str(len(files_changed)))
        
        return message
    
    tool = meow.tool(
        name="template-commit",
        handler=handler,
        description="Generate commit message using templates",
    )
    
    tool.param(
        name="type",
        type="string",
        description="Commit type (feat, fix, docs, refactor, test, chore)",
        required=True,
        short="t",
    )
    
    tool.param(
        name="scope",
        type="string",
        description="Component or module scope",
        required=True,
        short="s",
    )
    
    tool.param(
        name="desc",
        type="string",
        description="Short description of changes",
        required=True,
        short="d",
    )
    
    tool.param(
        name="body",
        type="string",
        description="Detailed explanation (optional)",
        default="",
        short="b",
    )
    
    tool.param(
        name="breaking",
        type="string",
        description="Breaking change description (optional)",
        default="",
    )
    
    meow.command(tool)
