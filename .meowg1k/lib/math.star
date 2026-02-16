"""
Math Operations Library for meowg1k

This library provides basic arithmetic operations for calculations within
Starlark workflows.

## Quick Start

```python
load("//lib/math.star", "calculator")

def handler(ctx):
    # Addition
    result = ctx.run(calculator, operation="add", a=10, b=5)
    ctx.output.writeline("10 + 5 = " + result)
    
    # Division
    quotient = ctx.run(calculator, operation="divide", a=20, b=4)
    ctx.output.writeline("20 / 4 = " + quotient)
```

## Available Tools

- `calculator` - Perform basic arithmetic operations

### Tool Sets
- `math_tools` - All math operation tools (1 tool)

## API Reference

### calculator

Perform basic arithmetic operations on two numbers.

**Parameters:**
- `operation` (string, required): Operation to perform (add, subtract, multiply, divide, modulo)
- `a` (float, required): First operand
- `b` (float, required): Second operand

**Returns:** string - Calculation result or error message

**Example:**
```python
# Addition
result = ctx.run(calculator, operation="add", a=10, b=5)
# Returns: "15.0"

# Subtraction
result = ctx.run(calculator, operation="subtract", a=10, b=3)
# Returns: "7.0"

# Multiplication
result = ctx.run(calculator, operation="multiply", a=6, b=7)
# Returns: "42.0"

# Division
result = ctx.run(calculator, operation="divide", a=20, b=4)
# Returns: "5.0"

# Modulo (remainder)
result = ctx.run(calculator, operation="modulo", a=17, b=5)
# Returns: "2.0"

# Division by zero
result = ctx.run(calculator, operation="divide", a=10, b=0)
# Returns: "Error: Division by zero"

# Unknown operation
result = ctx.run(calculator, operation="power", a=2, b=3)
# Returns: "Error: Unknown operation 'power'. Supported: add, subtract, multiply, divide, modulo"
```

**Operations:**
- `add` - Addition (a + b)
- `subtract` - Subtraction (a - b)
- `multiply` - Multiplication (a × b)
- `divide` - Division (a ÷ b)
- `modulo` - Remainder (a mod b)

**Note:** Returns string for LLM tool compatibility. Convert to number if needed 
for further calculations.

## Advanced Usage

### Calculation Chains

```python
load("//lib/math.star", "calculator")

def calculate_compound(ctx):
    # Perform chained calculations.
    
    # (10 + 5) * 3
    sum_result = ctx.run(calculator, operation="add", a=10, b=5)
    sum_val = float(sum_result)
    
    product = ctx.run(calculator, operation="multiply", a=sum_val, b=3)
    
    ctx.ui.info("Result: " + product)
    return product
```

### Statistics Calculations

```python
load("//lib/math.star", "calculator")

def calculate_average(ctx, numbers):
    # Calculate average of a list of numbers.
    
    if not numbers:
        return "0"
    
    # Sum all numbers
    total = 0.0
    for num in numbers:
        sum_result = ctx.run(calculator, operation="add", a=total, b=num)
        total = float(sum_result)
    
    # Divide by count
    avg = ctx.run(calculator, operation="divide", a=total, b=len(numbers))
    
    return avg
```

### Percentage Calculations

```python
load("//lib/math.star", "calculator")

def calculate_percentage(ctx, part, whole):
    # Calculate what percentage 'part' is of 'whole'.
    
    # (part / whole) * 100
    ratio = ctx.run(calculator, operation="divide", a=part, b=whole)
    ratio_val = float(ratio)
    
    percentage = ctx.run(calculator, operation="multiply", a=ratio_val, b=100)
    
    return percentage
```

### Code Metrics

```python
load("//lib/math.star", "calculator")
load("//lib/file_ops.star", "list_directory", "file_reader")

def calculate_code_metrics(ctx):
    # Calculate basic code metrics.
    
    # Get all Go files
    files_json = ctx.run(list_directory, path=".", pattern="**/*.go")
    files = ctx.json.decode(files_json)
    
    total_lines = 0
    for file_path in files:
        content = ctx.run(file_reader, path=file_path)
        lines = len(content.split("\\n"))
        
        # Add to total
        sum_result = ctx.run(calculator, operation="add", a=total_lines, b=lines)
        total_lines = float(sum_result)
    
    # Calculate average lines per file
    avg = ctx.run(calculator, operation="divide", a=total_lines, b=len(files))
    
    ctx.ui.info("Total files: " + str(len(files)))
    ctx.ui.info("Total lines: " + str(int(total_lines)))
    ctx.ui.info("Average lines per file: " + avg)
```

### Test Coverage Calculation

```python
load("//lib/math.star", "calculator")

def calculate_coverage(ctx, covered_lines, total_lines):
    # Calculate test coverage percentage.
    
    if total_lines == 0:
        return "0.0"
    
    # (covered / total) * 100
    percentage = calculate_percentage(ctx, covered_lines, total_lines)
    
    coverage = float(percentage)
    
    if coverage >= 75:
        ctx.ui.success("Coverage: %.2f%%" % coverage)
    elif coverage >= 50:
        ctx.ui.warning("Coverage: %.2f%%" % coverage)
    else:
        ctx.ui.error("Coverage: %.2f%%" % coverage)
    
    return str(coverage)
```

### Rate Limiting

```python
load("//lib/math.star", "calculator")

def check_rate_limit(ctx, requests_made, time_window_seconds, max_rate):
    # Check if rate limit would be exceeded.
    
    # Calculate current rate (requests per second)
    rate = ctx.run(calculator, 
                  operation="divide", 
                  a=requests_made, 
                  b=time_window_seconds)
    
    current_rate = float(rate)
    
    if current_rate > max_rate:
        ctx.ui.warning("Rate limit exceeded: %.2f req/s" % current_rate)
        return False
    else:
        ctx.ui.success("Within rate limit: %.2f req/s" % current_rate)
        return True
```

## Error Handling

Calculator handles errors gracefully:

```python
load("//lib/math.star", "calculator")

def safe_divide(ctx, a, b):
    # Safe division with error handling.
    
    result = ctx.run(calculator, operation="divide", a=a, b=b)
    
    if result.startswith("Error:"):
        ctx.ui.error(result)
        return None
    
    return float(result)

def safe_calculate(ctx, operation, a, b):
    # Calculate with validation.
    
    # Validate operation
    valid_ops = ["add", "subtract", "multiply", "divide", "modulo"]
    if operation not in valid_ops:
        ctx.ui.error("Invalid operation: " + operation)
        return None
    
    result = ctx.run(calculator, operation=operation, a=a, b=b)
    
    if result.startswith("Error:"):
        ctx.ui.error(result)
        return None
    
    return float(result)
```

**Error Cases:**
- Division by zero returns "Error: Division by zero"
- Modulo by zero returns "Error: Division by zero"
- Unknown operation returns "Error: Unknown operation '<op>'. Supported: ..."

**Best Practices:**
- Check for "Error:" prefix in results
- Handle division by zero explicitly
- Validate operation names before calling
- Convert string results to float for further calculations

## Limitations

1. **Basic Operations Only**: No advanced math (trigonometry, logarithms, exponentiation)
2. **Two Operands**: Can only operate on two numbers at a time
3. **No Operator Precedence**: Must manually chain operations
4. **String Results**: Returns strings, must convert for numeric operations
5. **Limited Precision**: Float precision limitations apply

**Workarounds:**

For advanced math, use shell commands:
```python
load("//lib/shell.star", "shell_exec")

def advanced_math(ctx, expression):
    # Use bc, awk, or python for complex math
    result = ctx.run(shell_exec, command="python3 -c 'print(%s)'" % expression)
    return result
```

For multiple operands, chain operations:
```python
# Calculate: (a + b + c) / 3
sum1 = float(ctx.run(calculator, operation="add", a=a, b=b))
sum2 = float(ctx.run(calculator, operation="add", a=sum1, b=c))
avg = ctx.run(calculator, operation="divide", a=sum2, b=3)
```

## Performance Tips

1. **Minimize Tool Calls**: For simple Starlark calculations, use native operators:
   ```python
   # Use tool for LLM/agentic workflows
   result = ctx.run(calculator, operation="add", a=10, b=5)
   
   # Use native Starlark for scripts
   result = 10 + 5
   ```

2. **Batch Calculations**: Group related calculations to minimize tool overhead.

3. **Cache Results**: Store intermediate results rather than recalculating.

## Tool vs Native Arithmetic

**When to use calculator tool:**
- In agentic workflows (LLM needs to perform calculations)
- When tool result must be returned as string
- For consistent error handling

**When to use native Starlark:**
- Direct calculations in scripts
- Better performance needed
- Working with multiple operations

```python
# Using tool (for agentic workflows)
result = ctx.run(calculator, operation="add", a=10, b=5)

# Using native Starlark (for scripts)
result = 10 + 5
result = 10.0 / 3.0
result = 17 % 5
```

## Integration Examples

### With File Operations

```python
load("//lib/math.star", "calculator")
load("//lib/file_ops.star", "list_directory")

def count_files_by_extension(ctx, extension):
    files_json = ctx.run(list_directory, path=".", pattern="**/*" + extension)
    files = ctx.json.decode(files_json)
    
    count = len(files)
    ctx.ui.info("Found %d %s files" % (count, extension))
    return count
```

### With HTTP Operations

```python
load("//lib/math.star", "calculator")
load("//lib/http.star", "http_get")
load("//lib/json.star", "json_query")

def calculate_api_stats(ctx, url):
    response = ctx.run(http_get, url=url)
    
    total = ctx.run(json_query, json=response, path="stats.total")
    completed = ctx.run(json_query, json=response, path="stats.completed")
    
    # Calculate completion percentage
    percentage = calculate_percentage(ctx, 
                                     float(completed), 
                                     float(total))
    
    ctx.ui.info("Completion: " + percentage + "%")
```

## Use Cases

### Build Size Analysis
```python
def compare_build_sizes(ctx, size_before, size_after):
    diff = ctx.run(calculator, operation="subtract", a=size_after, b=size_before)
    percentage = calculate_percentage(ctx, float(diff), size_before)
    ctx.ui.info("Size change: " + percentage + "%")
```

### Performance Metrics
```python
def calculate_throughput(ctx, requests, seconds):
    rate = ctx.run(calculator, operation="divide", a=requests, b=seconds)
    ctx.ui.info("Throughput: " + rate + " req/s")
```

### Resource Allocation
```python
def split_resources(ctx, total, num_parts):
    per_part = ctx.run(calculator, operation="divide", a=total, b=num_parts)
    ctx.ui.info("Each part gets: " + per_part)
```

## See Also

- [shell.star](shell.star) - Shell execution for advanced math via external tools
- [API Reference](../../API_REFERENCE.md) - Starlark arithmetic operators
"""

# ==============================================================================
# TOOL HANDLERS
# ==============================================================================

def calculator_handler(ctx):
    """Perform basic arithmetic operations."""
    operation = ctx.params["operation"]
    a = ctx.params["a"]
    b = ctx.params["b"]
    
    if operation == "add":
        result = a + b
    elif operation == "subtract":
        result = a - b
    elif operation == "multiply":
        result = a * b
    elif operation == "divide":
        if b == 0:
            return "Error: Division by zero"
        result = a / b
    elif operation == "modulo":
        if b == 0:
            return "Error: Division by zero"
        result = a % b
    else:
        return "Error: Unknown operation '" + operation + "'. Supported: add, subtract, multiply, divide, modulo"
    
    return str(result)

# ==============================================================================
# TOOL DEFINITIONS
# ==============================================================================

calculator = meow.tool(
    name="calculator",
    description="Perform basic arithmetic operations",
    params={
        "operation": meow.param(
            "string",
            desc="Operation to perform",
            choices=["add", "subtract", "multiply", "divide", "modulo"],
            required=True,
        ),
        "a": meow.param("float", desc="First number", required=True),
        "b": meow.param("float", desc="Second number", required=True),
    },
    handler=calculator_handler,
)

# Tool set
math_tools = [calculator]
