"""
Reusable validation functions for command parameters.

This module provides common validators that can be used across
all meowg1k commands to ensure consistent error handling and
user-friendly error messages.

Usage:
    load(".meowg1k/lib/validators.star", "validators")
    
    validators.temperature(0.7)  # OK
    validators.temperature(5.0)  # Fails with helpful error
"""

def temperature(value, name="temperature"):
    """
    Validate LLM temperature parameter.
    
    Args:
        value: The temperature value to validate
        name: Parameter name for error messages (default: "temperature")
    
    Returns:
        The validated value
    
    Raises:
        Error if value is not a number or out of range [0.0, 2.0]
    
    Example:
        temp = validators.temperature(ctx.params.get("temperature", 0.7))
    """
    # Type check
    if type(value) not in ["int", "float"]:
        fail("%s: must be a number, got %s" % (name, type(value)))
    
    # Range check
    if value < 0.0 or value > 2.0:
        fail("%s: must be between 0.0 and 2.0, got %s" % (name, value))
    
    return float(value)

def non_empty(name, value):
    """
    Validate that a string parameter is not empty.
    
    Args:
        name: Parameter name for error messages
        value: The string value to validate
    
    Returns:
        The validated string (stripped of whitespace)
    
    Raises:
        Error if value is not a string or is empty/whitespace-only
    
    Example:
        message = validators.non_empty("message", ctx.params.get("message"))
    """
    # Type check
    if type(value) != "string":
        fail("%s: must be a string, got %s" % (name, type(value)))
    
    # Empty check
    stripped = value.strip()
    if len(stripped) == 0:
        fail("%s: cannot be empty or whitespace-only" % name)
    
    return stripped

def file_exists(path, ctx):
    """
    Validate that a file exists.
    
    Args:
        path: File path to check
        ctx: Handler context (for fs.exists access)
    
    Returns:
        The validated path
    
    Raises:
        Error if file does not exist
    
    Example:
        config_file = validators.file_exists("config.yaml", ctx)
    """
    if not ctx.fs.exists(path):
        fail("file not found: %s" % path)
    return path

def one_of(name, value, options):
    """
    Validate that a value is one of the allowed options.
    
    Args:
        name: Parameter name for error messages
        value: The value to validate
        options: List of allowed values
    
    Returns:
        The validated value
    
    Raises:
        Error if value is not in options
    
    Example:
        preset = validators.one_of("preset", "fast", ["fast", "smart", "creative"])
    """
    if value not in options:
        fail("%s: must be one of %s, got '%s'" % (name, options, value))
    return value

def positive_int(name, value):
    """
    Validate that a value is a positive integer.
    
    Args:
        name: Parameter name for error messages
        value: The value to validate
    
    Returns:
        The validated integer
    
    Raises:
        Error if value is not a positive integer
    
    Example:
        count = validators.positive_int("count", ctx.params.get("count"))
    """
    # Type check
    if type(value) != "int":
        fail("%s: must be an integer, got %s" % (name, type(value)))
    
    # Positive check
    if value <= 0:
        fail("%s: must be positive, got %d" % (name, value))
    
    return value

def port_number(value, name="port"):
    """
    Validate that a value is a valid port number (1-65535).
    
    Args:
        value: The port number to validate
        name: Parameter name for error messages (default: "port")
    
    Returns:
        The validated port number
    
    Raises:
        Error if value is not a valid port number
    
    Example:
        port = validators.port_number(ctx.params.get("port", 8080))
    """
    # Type check
    if type(value) != "int":
        fail("%s: must be an integer, got %s" % (name, type(value)))
    
    # Range check
    if value < 1 or value > 65535:
        fail("%s: must be between 1 and 65535, got %d" % (name, value))
    
    return value

def list_of_strings(name, value):
    """
    Validate that a value is a list of strings.
    
    Args:
        name: Parameter name for error messages
        value: The value to validate
    
    Returns:
        The validated list
    
    Raises:
        Error if value is not a list or contains non-strings
    
    Example:
        files = validators.list_of_strings("files", ctx.params.get("files", []))
    """
    # Type check
    if type(value) != "list":
        fail("%s: must be a list, got %s" % (name, type(value)))
    
    # Check each element
    for i, item in enumerate(value):
        if type(item) != "string":
            fail("%s[%d]: must be a string, got %s" % (name, i, type(item)))
    
    return value

# Export all validators as a struct for convenient access
validators = struct(
    temperature=temperature,
    non_empty=non_empty,
    file_exists=file_exists,
    one_of=one_of,
    positive_int=positive_int,
    port_number=port_number,
    list_of_strings=list_of_strings,
)
