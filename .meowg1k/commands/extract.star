# Structured Response Example
# This command demonstrates how to use structured outputs with LLM APIs

def setup():
    """Setup the extract command (no configuration needed)"""
    extract_tool = meow.tool(
        name="extract",
        description="Extract structured information from text using LLM with JSON schema",
        handler=lambda ctx: extract_structured_data(ctx),
        params={
            "text": meow.param("string", required=True, desc="Text to extract information from"),
            "schema_type": meow.param("string", default="person", desc="Type of schema: person, product, or event"),
        },
    )
    meow.command(extract_tool)

def extract_structured_data(ctx):
    text = ctx.text
    schema_type = ctx.schema_type
    
    # Define different schemas for different entity types
    schemas = {
        "person": {
            "type": "object",
            "properties": {
                "name": {"type": "string", "description": "Full name"},
                "age": {"type": "integer", "description": "Age in years"},
                "occupation": {"type": "string", "description": "Job title or profession"},
                "location": {"type": "string", "description": "City and country"},
                "email": {"type": "string", "description": "Email address if mentioned"},
            },
            "required": ["name"],
        },
        "product": {
            "type": "object",
            "properties": {
                "name": {"type": "string", "description": "Product name"},
                "price": {"type": "number", "description": "Price in USD"},
                "category": {"type": "string", "description": "Product category"},
                "features": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "List of key features",
                },
                "inStock": {"type": "boolean", "description": "Availability status"},
            },
            "required": ["name", "price"],
        },
        "event": {
            "type": "object",
            "properties": {
                "title": {"type": "string", "description": "Event title"},
                "date": {"type": "string", "description": "Event date"},
                "location": {"type": "string", "description": "Event location"},
                "attendees": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "List of attendees",
                },
                "description": {"type": "string", "description": "Event description"},
            },
            "required": ["title", "date"],
        },
    }
    
    schema = schemas.get(schema_type)
    if not schema:
        fail("Invalid schema_type '{}'. Choose: person, product, or event".format(schema_type))
    
    # Generate structured output using LLM with response schema
    prompt = "Extract structured information from the following text:\n\n" + text
    
    result = ctx.llm.chat(
        preset="fast",
        prompt=prompt,
        response_format="json_object",
        response_schema=schema,
    )
    
    # Parse result — some providers return a JSON string, others a pre-parsed dict.
    # Always normalise to a dict so downstream .get() calls work reliably.
    if type(result) == "string":
        data = ctx.json.parse(result)
    else:
        data = result
    
    # Build formatted output
    output_lines = []
    
    if schema_type == "person":
        output_lines.append("## Person Information\n")
        output_lines.append("**Name:** " + data.get("name", "N/A"))
        if data.get("age"):
            output_lines.append("**Age:** " + str(data["age"]))
        if data.get("occupation"):
            output_lines.append("**Occupation:** " + data["occupation"])
        if data.get("location"):
            output_lines.append("**Location:** " + data["location"])
        if data.get("email"):
            output_lines.append("**Email:** " + data["email"])
    
    elif schema_type == "product":
        output_lines.append("## Product Information\n")
        output_lines.append("**Name:** " + data.get("name", "N/A"))
        output_lines.append("**Price:** $" + str(data.get("price", 0)))
        if data.get("category"):
            output_lines.append("**Category:** " + data["category"])
        if data.get("features"):
            output_lines.append("\n**Features:**")
            for feature in data["features"]:
                output_lines.append("  - " + feature)
        if "inStock" in data:
            status = "Yes" if data["inStock"] else "No"
            output_lines.append("**In Stock:** " + status)
    
    elif schema_type == "event":
        output_lines.append("## Event Information\n")
        output_lines.append("**Title:** " + data.get("title", "N/A"))
        output_lines.append("**Date:** " + data.get("date", "N/A"))
        if data.get("location"):
            output_lines.append("**Location:** " + data["location"])
        if data.get("attendees"):
            output_lines.append("\n**Attendees:**")
            for attendee in data["attendees"]:
                output_lines.append("  - " + attendee)
        if data.get("description"):
            output_lines.append("\n**Description:**")
            output_lines.append(data["description"])
    
    # Add raw JSON
    output_lines.append("\n---\n**Raw JSON:**")
    output_lines.append("```json")
    output_lines.append(ctx.json.stringify(data))
    output_lines.append("```")
    
    # Join and output (ctx.output.writeline for persistent output; no TUI duplicate)
    output = "\n".join(output_lines)
    ctx.output.writeline(output)
    return output
