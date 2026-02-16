# ==============================================================================
# Code Command - AI Code Generation
# ==============================================================================
"""
AI-powered code generation with language-specific instructions and output options.

FEATURES:
  - Multi-Language: Generate code in any programming language
  - Custom Instructions: Configure language-specific style guides
  - File Output: Write directly to files with --output
  - Context Support: Include existing code from stdin or CLI
  - LLM Presets: Choose between fast and smart models
  - Configurable: Add language instructions via init.star

INSTALLATION:
  # In your .meowg1k/init.star
  load("//commands/code.star", "setup")

  setup(
      # Add language-specific instructions
      languages = {
          "python": {
              "name": "Python",
              "instructions": "Use type hints, Google-style docstrings, follow PEP 8"
          },
          "go": {
              "name": "Go",
              "instructions": "Follow Go conventions, use short variable names"
          }
      },
      # Set defaults
      default_lang = "python",
      preset = "smart"
  )

EXAMPLES:
  # Generate Python HTTP client
  meow code -p "HTTP client with retry logic and exponential backoff" -l python

  # Generate Go REST handler
  meow code -p "REST handler for user CRUD operations" -l go -o handlers/user.go

  # Refactor existing code
  cat legacy.py | meow code -p "Modernize to Python 3.10+ syntax" -l python

  # Generate from description
  meow code -p "CLI tool that converts CSV to JSON" -l python -o csv2json.py

PARAMETERS:
  --prompt, -p       What code to generate (required)
  --lang, -l         Target programming language
  --output, -o       Output file path (optional, displays if not set)
  --context, -c      Existing code or context (supports piped input from stdin)
  --preset           LLM preset: "smart" (default) or "fast"

DEPENDENCIES:
  None - standalone command
"""
# ==============================================================================

load("//lib/help.star", "build_choices_desc", "build_preset_desc")

# =============================================================================
# Constants
# =============================================================================

_SYSTEM_PROMPT = """You are an expert software engineer specialized in writing clean, production-ready code.
Principles:
- Write clear, readable, maintainable code
- Follow language idioms and best practices
- Include appropriate error handling
- Add comments for complex logic only
- Prefer simple solutions over clever ones

Output only the code, no explanations unless specifically asked."""

# No built-in languages - user configures their preferred style guides
_BUILTIN_LANGUAGES = {}

# =============================================================================
# Setup Function
# =============================================================================

def setup(languages=None, default_lang=None, preset=None):
    """Configure the code command.

    Args:
        languages: Dict of language instructions to add. Keys are language names,
                   values are dicts with 'name', 'instructions', and optional 'framework'.
        default_lang: Default programming language when --lang not specified
        preset: LLM preset for generation
    """
    all_languages = dict(_BUILTIN_LANGUAGES)
    if languages:
        for name, lang_def in languages.items():
            all_languages[name] = lang_def

    config_default_lang = default_lang if default_lang != None else ""
    config_preset = preset if preset != None else ""

    def build_system_prompt(lang_name, custom_style=""):
        parts = [_SYSTEM_PROMPT]
        if lang_name:
            parts.append("Language: {}".format(lang_name))
            if lang_name in all_languages:
                lang_config = all_languages[lang_name]
                if lang_config.get("instructions"):
                    parts.append("Style: {}".format(lang_config["instructions"]))
                if lang_config.get("framework"):
                    parts.append("Framework: {}".format(lang_config["framework"]))
        if custom_style:
            parts.append("Custom Style: {}".format(custom_style))
        return "\n\n".join(parts)

    def merge_context_with_stdin(ctx, context):
        full_context = context or ""
        if ctx.stdin.is_piped():
            stdin_content = ctx.stdin.read().strip()
            if stdin_content:
                if full_context:
                    full_context = full_context + "\n\n" + stdin_content
                else:
                    full_context = stdin_content
        return full_context

    def generate_code(ctx, prompt, lang="", custom_style="", context="", output="", preset="smart"):
        if not prompt:
            ctx.ui.error("No prompt provided")
            return None

        step = ctx.ui.step("Generating Code")
        full_prompt = prompt
        if lang:
            ctx.ui.info("Target language: {}".format(lang))
            full_prompt = "Generate {} code:\n\n{}".format(lang, prompt)

        if context:
            ctx.ui.info("Including {} chars of context".format(len(context)))
            full_prompt = "{}\n\n### Existing Code/Context:\n```\n{}\n```".format(full_prompt, context)

        system = build_system_prompt(lang, custom_style)
        activity = ctx.ui.activity("Generating...")

        result = ctx.llm.generate(
            preset=preset,
            system=system,
            prompt=full_prompt
        )

        activity.success("Generated")
        step.done()

        if output:
            write_step = ctx.ui.step("Writing File")
            ctx.fs.write(output, result)
            write_step.done()
            ctx.ui.success("Wrote to {}".format(output))
        else:
            ctx.ui.divider()
            ctx.output.code(result, lang=lang or "text")

        return result

    def handle_code(ctx):
        full_context = merge_context_with_stdin(ctx, ctx.context)
        return generate_code(
            ctx,
            prompt=ctx.prompt,
            lang=ctx.lang,
            custom_style=ctx.custom_style,
            context=full_context,
            output=ctx.output,
            preset=ctx.preset
        )

    def build_lang_desc(default_lang):
        if not all_languages:
            return "Programming language (e.g. python, go, rust)."
        def get_lang_desc(name, l):
            desc = l.get("name", name)
            if l.get("framework"):
                desc = desc + " (" + l["framework"] + ")"
            return desc
        return build_choices_desc("Language (one of):", all_languages, default_lang, get_lang_desc)

    code_command = meow.tool(
        name="code",
        description="Generate source code.",
        params={
            "prompt": meow.param("string", default="", short="p", min_len=1, desc="What to generate."),
            "lang": meow.param("string", default=config_default_lang, short="l", desc=build_lang_desc(config_default_lang)),
            "custom_style": meow.param("string", default="", short="s", desc="Coding style instructions."),
            "output": meow.param("string", default="", short="o", desc="Write result to file."),
            "context": meow.param("string", default="", short="c", from_stdin=True, desc="Existing code or reference."),
            "preset": meow.param("string", default=config_preset, choices=meow.presets(), desc=build_preset_desc(config_preset))
        },
        handler=handle_code
    )
    meow.command(code_command)
