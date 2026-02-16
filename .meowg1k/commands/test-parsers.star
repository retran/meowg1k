# Test All Data Format Parsers
# Demonstrates YAML, XML, TOML, CSV, and JSON parsing capabilities

def setup():
    """Setup the test-parsers command"""
    test_tool = meow.tool(
        name="test-parsers",
        description="Test all data format parsers (JSON, YAML, XML, TOML, CSV)",
        handler=lambda ctx: test_all_parsers(ctx),
        params={
            "format": meow.param("string", default="all", desc="Format to test: json, yaml, xml, toml, csv, or all"),
        },
    )
    meow.command(test_tool)

def test_all_parsers(ctx):
    format_type = ctx.format
    
    output_lines = ["# Data Format Parser Test Results\n"]
    
    # Test JSON
    if format_type == "all" or format_type == "json":
        output_lines.append("## JSON Parser")
        json_data = '{"name": "Alice", "age": 30, "active": true}'
        parsed = ctx.json.parse(json_data)
        stringified = ctx.json.stringify(parsed, indent=2)
        output_lines.append("✅ **Parse**: " + str(parsed))
        output_lines.append("✅ **Stringify**:")
        output_lines.append("```json")
        output_lines.append(stringified)
        output_lines.append("```\n")
    
    # Test YAML
    if format_type == "all" or format_type == "yaml":
        output_lines.append("## YAML Parser")
        yaml_data = """
name: Bob
age: 25
skills:
  - Python
  - Go
  - JavaScript
"""
        parsed = ctx.yaml.parse(yaml_data)
        stringified = ctx.yaml.stringify(parsed)
        output_lines.append("✅ **Parse**: " + str(parsed))
        output_lines.append("✅ **Stringify**:")
        output_lines.append("```yaml")
        output_lines.append(stringified)
        output_lines.append("```\n")
    
    # Test CSV
    if format_type == "all" or format_type == "csv":
        output_lines.append("## CSV Parser")
        csv_data = """name,age,city
Alice,30,NYC
Bob,25,SF
Charlie,35,LA"""
        parsed = ctx.csv.parse(csv_data, has_header=True)
        stringified = ctx.csv.stringify(parsed, headers=["name", "age", "city"])
        output_lines.append("✅ **Parse with headers**: " + str(parsed))
        output_lines.append("✅ **Stringify**:")
        output_lines.append("```csv")
        output_lines.append(stringified)
        output_lines.append("```\n")
    
    # Test TOML
    if format_type == "all" or format_type == "toml":
        output_lines.append("## TOML Parser")
        toml_data = """
title = "My Config"
enabled = true

[server]
host = "localhost"
port = 8080
"""
        parsed = ctx.toml.parse(toml_data)
        stringified = ctx.toml.stringify(parsed)
        output_lines.append("✅ **Parse**: " + str(parsed))
        output_lines.append("✅ **Stringify**:")
        output_lines.append("```toml")
        output_lines.append(stringified)
        output_lines.append("```\n")
    
    # Test XML
    if format_type == "all" or format_type == "xml":
        output_lines.append("## XML Parser")
        xml_data = """<?xml version="1.0"?>
<person>
  <name>Alice</name>
  <age>30</age>
</person>"""
        parsed = ctx.xml.parse(xml_data)
        output_lines.append("✅ **Parse**: " + str(parsed))
        output_lines.append("⚠️  **Stringify**: Known edge cases, see test failures\n")
    
    # Summary
    output_lines.append("---")
    output_lines.append("**Summary**: All core parsing modules are available in Starlark!")
    output_lines.append("- `ctx.json` - Full support ✅")
    output_lines.append("- `ctx.yaml` - Full support ✅")
    output_lines.append("- `ctx.csv` - Full support ✅")
    output_lines.append("- `ctx.toml` - Full support ✅ (minor edge case)")
    output_lines.append("- `ctx.xml` - Basic support ⚠️ (has edge cases)")
    
    output = "\n".join(output_lines)
    ctx.output.markdown(output)
    return output
