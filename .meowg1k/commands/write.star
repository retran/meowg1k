# ==============================================================================
# Write Command - AI Content Generation
# ==============================================================================
"""
AI-powered content generation with configurable tones and output formats.

FEATURES:
  - Flexible Prompting: Generate content from natural language prompts
  - Tone Control: Technical, casual, formal, concise, or custom
  - Output Formats: Prose, bullets, numbered lists, markdown
  - Context Support: Include context from stdin or command line
  - LLM Presets: Choose between fast and smart models
  - Configurable: Add custom tones and formats via init.star

INSTALLATION:
  # In your .meowg1k/init.star
  load("//commands/write.star", "setup")

  setup(
      # Add/override tones (merged with built-in tones)
      tones = {
          "corporate": {
              "name": "Corporate",
              "description": "Professional B2B communication style"
          }
      },
      # Add/override formats (merged with built-in formats)
      formats = {
          "memo": {
              "name": "Memo",
              "description": "Short internal memo",
              "instructions": "Format as a brief memo with action items"
          }
      },
      # Set defaults
      default_tone = "technical",
      default_format = "prose",
      preset = "smart"
  )

TONES:
  technical    Technical/engineering communication
  casual       Friendly, conversational style
  formal       Professional, business-appropriate
  concise      Brief, to-the-point responses

FORMATS:
  prose        Flowing paragraphs (default)
  bullets      Bullet point list
  numbered     Numbered steps/list
  markdown     Rich markdown with headers

EXAMPLES:
  # Generate documentation
  meow write --prompt "Write docstring for this function" --context "$(cat handler.py)"

  # Code explanation
  git diff | meow write --prompt "Explain these changes" --tone technical

  # Content transformation
  echo "User authentication flow" | meow write --prompt "Create a detailed technical spec"

  # Quick bullet points
  meow write --prompt "Key features of REST APIs" --format bullets --tone concise

PARAMETERS:
  --prompt, -p       Main prompt for generation (required)
  --context, -c      Optional context (supports piped input from stdin)
  --tone, -t         Writing tone (default: none)
  --format, -f       Output format (default: prose)
  --preset           LLM preset: "smart" (default) or "fast"

DEPENDENCIES:
  None - standalone command
"""
# ==============================================================================

load("//lib/help.star", "build_choices_desc", "build_preset_desc")
load("//lib/ui_helpers.star", "make_markdown_stream_handler")

# =============================================================================
# Constants
# =============================================================================

_SYSTEM_PROMPT = """You are an expert AI assistant specialized in generating high-quality content.
Principles: Accuracy, Clarity, Quality, Context-awareness."""

_BUILTIN_TONES = {
    "technical": {
        "name": "Technical",
        "description": "Precise terminology, detailed explanations"
    },
    "casual": {
        "name": "Casual",
        "description": "Friendly and conversational"
    },
    "formal": {
        "name": "Formal",
        "description": "Professional business language"
    },
    "concise": {
        "name": "Concise",
        "description": "Brief and to-the-point"
    }
}

_BUILTIN_FORMATS = {
    "prose": {
        "name": "Prose",
        "description": "Flowing paragraphs",
        "instructions": "Write in flowing paragraphs."
    },
    "bullets": {
        "name": "Bullet Points",
        "description": "Unordered list with - bullets",
        "instructions": "Format as a bullet point list using - for each item."
    },
    "numbered": {
        "name": "Numbered List",
        "description": "Ordered list (1. 2. 3.)",
        "instructions": "Format as a numbered list (1. 2. 3. etc)."
    },
    "markdown": {
        "name": "Markdown",
        "description": "Rich formatting with headers, bold, code",
        "instructions": "Use rich markdown formatting with headers (##), bold, code blocks where appropriate."
    }
}

# =============================================================================
# Setup Function
# =============================================================================

def setup(tones=None, formats=None, default_tone=None, default_format=None, preset=None):
    """Configure the write command.

    Args:
        tones: Dict of custom tones to add/override. Merged with built-in tones.
        formats: Dict of custom formats to add/override. Merged with built-in formats.
        default_tone: Default tone when --tone not specified
        default_format: Default format when --format not specified
        preset: LLM preset for generation
    """
    # Build config (local, not module-level to avoid freeze issues)
    all_tones = dict(_BUILTIN_TONES)
    all_formats = dict(_BUILTIN_FORMATS)

    if tones:
        for name, tone_def in tones.items():
            all_tones[name] = tone_def

    if formats:
        for name, fmt_def in formats.items():
            all_formats[name] = fmt_def

    config_default_tone = default_tone if default_tone != None else "technical"
    config_default_format = default_format if default_format != None else "prose"
    config_preset = preset if preset != None else ""

    # Internal functions using closure
    def build_system_prompt(tone_name, format_name, custom_tone=""):
        parts = [_SYSTEM_PROMPT]
        if custom_tone:
            parts.append("Tone: {}".format(custom_tone))
        elif tone_name and tone_name in all_tones:
            t = all_tones[tone_name]
            parts.append("Tone: {}".format(t["description"]))
        if format_name and format_name in all_formats:
            f = all_formats[format_name]
            parts.append("Format: {}".format(f["instructions"]))
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

    def generate_content(ctx, prompt, context="", tone="", custom_tone="", format="prose", preset="smart"):
        if not prompt:
            if context:
                prompt = "Process this input:\n\n{}".format(context)
            else:
                ctx.ui.error("No prompt provided")
                return None

        step = ctx.ui.step("Generating Content")
        full_prompt = prompt
        if context:
            ctx.ui.info("Including {} chars of context".format(len(context)))
            full_prompt = "{}\n\n### Context:\n{}".format(prompt, context)

        system = build_system_prompt(tone, format, custom_tone)

        on_event = make_markdown_stream_handler(ctx)
        step.done()
        ctx.ui.divider()
        result = ctx.llm.chat(
            preset=preset,
            system=system,
            prompt=full_prompt,
            stream=True,
            on_event=on_event,
        )
        ctx.output.writeline(result)
        return result

    def handle_write(ctx):
        full_context = merge_context_with_stdin(ctx, ctx.context)
        return generate_content(
            ctx,
            prompt=ctx.prompt,
            context=full_context,
            tone=ctx.tone,
            custom_tone=ctx.custom_tone,
            format=ctx.format,
            preset=ctx.preset
        )

    def build_tone_desc(default_tone):
        return build_choices_desc("Tone (one of):", all_tones, default_tone,
            lambda name, t: t["description"])

    def build_format_desc(default_format):
        return build_choices_desc("Output format (one of):", all_formats, default_format,
            lambda name, f: f["description"].lower())

    # Register command
    write_command = meow.tool(
        name="write",
        description="Generate text content.",
        params={
            "prompt": meow.param("string", default="", short="p", min_len=1, desc="What to write."),
            "context": meow.param("string", default="", short="c", from_stdin=True, desc="Background information or reference material."),
            "tone": meow.param("string", default=config_default_tone, short="t", choices=all_tones, desc=build_tone_desc(config_default_tone)),
            "custom_tone": meow.param("string", default="", desc="Free-form tone instructions (overrides --tone)."),
            "format": meow.param("string", default=config_default_format, short="f", choices=all_formats, desc=build_format_desc(config_default_format)),
            "preset": meow.param("string", default=config_preset, choices=meow.presets(), desc=build_preset_desc(config_preset))
        },
        handler=handle_write
    )
    meow.command(write_command)
