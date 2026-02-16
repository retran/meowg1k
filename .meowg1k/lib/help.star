# ==============================================================================
# Help Text Formatting Library (lib/help.star)
# ==============================================================================
# Shared utilities for building consistent CLI help text descriptions.
#
# Public API:
#   - build_choices_desc(title, choices, default, get_desc=None)
#   - build_preset_desc(default_preset)
# ==============================================================================

def build_choices_desc(title, choices, default, get_desc=None):
    """Build a formatted description for enumeration parameters.

    Args:
        title: Header text (e.g., "Tone (one of):")
        choices: Dict or list of choice names
        default: Default value to mark with (default)
        get_desc: Optional function(name, value) -> description string
                  If None, uses value directly for dicts or name for lists

    Returns:
        Formatted multi-line string for help text
    """
    lines = [title]

    # Handle both dict and list
    if type(choices) == "dict":
        for name, value in choices.items():
            if get_desc:
                desc = get_desc(name, value)
            else:
                desc = value.get("description", "") if type(value) == "dict" else str(value)
            suffix = " (default)" if name == default else ""
            if desc:
                lines.append("  {}: {}{}".format(name, desc, suffix))
            else:
                lines.append("  {}{}".format(name, suffix))
    else:
        for name in choices:
            suffix = " (default)" if name == default else ""
            lines.append("  {}{}".format(name, suffix))

    return "\n".join(lines)

def build_preset_desc(default_preset):
    """Build description for preset parameter using registered presets.

    Args:
        default_preset: The default preset name to mark

    Returns:
        Formatted multi-line string listing available presets
    """
    available = meow.presets()
    if not available:
        return "LLM configuration preset."
    return build_choices_desc("LLM preset (one of):", available, default_preset)
