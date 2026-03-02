"""
Project Statistics Command

This command demonstrates advanced file system operations by analyzing
project statistics including file counts, sizes, and types.

Usage:
  meow stats
  meow stats --directory src/

Example:
  meow stats                         # Analyze current directory
  meow stats --directory src/        # Analyze src/ directory
"""

def handler(ctx):
    """Generate statistics about project files"""

    # Get directory from params
    target_dir = ctx.directory

    # Check if directory exists
    if not ctx.fs.exists(target_dir):
        ctx.ui.error("Directory '{}' does not exist".format(target_dir))
        return

    # Get directory metadata
    dir_info = ctx.fs.stat(target_dir)
    if not dir_info.is_dir:
        ctx.ui.error("'{}' is not a directory".format(target_dir))
        return

    ctx.ui.info("Analyzing directory: {}".format(target_dir))
    ctx.output.writeline("")

    # Glob all files in directory
    if target_dir == ".":
        all_files = ctx.fs.glob("**/*")
    else:
        all_files = ctx.fs.glob("{}/**/*".format(target_dir))

    if len(all_files) == 0:
        ctx.ui.warn("No files found in directory")
        return

    # Get current time for recent file detection
    current_time = ctx.time.now()

    # Collect statistics
    total_size = 0
    file_types = {}
    largest_files = []
    recent_files = []

    for file_path in all_files:
        stat = ctx.fs.stat(file_path)
        if stat == None or stat.is_dir:
            continue
        total_size += stat.size

        # Track file types by extension
        if "." in file_path:
            ext = "." + file_path.split(".")[-1]
        else:
            ext = "(no extension)"

        if ext in file_types:
            file_types[ext]["count"] += 1
            file_types[ext]["size"] += stat.size
        else:
            file_types[ext] = {"count": 1, "size": stat.size}

        # Track largest files (keep top 5)
        largest_files.append({"path": file_path, "size": stat.size})
        largest_files = sorted(largest_files, key=lambda x: x["size"], reverse=True)[:5]

        # Track recently modified files (last 7 days)
        if current_time - stat.mtime < 604800:  # 7 days in seconds
            recent_files.append({"path": file_path, "mtime": stat.mtime})

    # Sort recent files by modification time
    recent_files = sorted(recent_files, key=lambda x: x["mtime"], reverse=True)[:10]

    # Display summary
    ctx.output.writeline("## Summary")
    ctx.output.writeline("Total files: {}".format(len(all_files)))
    ctx.output.writeline("Total size: {}".format(format_size(total_size)))
    ctx.output.writeline("")

    # Display file types
    ctx.output.writeline("## File Types")
    sorted_types = sorted(file_types.items(), key=lambda x: x[1]["count"], reverse=True)
    for ext, info in sorted_types[:10]:  # Top 10 types
        ctx.output.writeline("  {:<20s} {:5d} files  {:>10s}".format(ext, info["count"], format_size(info["size"])))
    ctx.output.writeline("")

    # Display largest files
    ctx.output.writeline("## Largest Files")
    for item in largest_files:
        ctx.output.writeline("  {:>10s}  {}".format(format_size(item["size"]), item["path"]))
    ctx.output.writeline("")

    # Display recently modified files
    if len(recent_files) > 0:
        ctx.output.writeline("## Recently Modified (Last 7 Days)")
        for item in recent_files:
            age = current_time - item["mtime"]
            ctx.output.writeline("  {:>12s}  {}".format(format_time_ago(age), item["path"]))
        ctx.output.writeline("")

    # Directory top-level contents
    ctx.output.writeline("## Directory Contents (Top Level)")
    items = ctx.fs.listdir(target_dir)
    for item in items[:10]:  # Show first 10 items
        if target_dir != ".":
            item_path = target_dir + "/" + item
        else:
            item_path = item
        item_stat = ctx.fs.stat(item_path)
        if item_stat != None:
            item_type = "DIR " if item_stat.is_dir else "FILE"
            ctx.output.writeline("  {}  {}".format(item_type, item))
        else:
            ctx.output.writeline("  ????  {}".format(item))

def format_size(bytes):
    """Format bytes as human-readable size"""
    if bytes < 1024:
        return "{} B".format(bytes)
    elif bytes < 1024 * 1024:
        return "{:.1f} KB".format(bytes / 1024)
    elif bytes < 1024 * 1024 * 1024:
        return "{:.1f} MB".format(bytes / (1024 * 1024))
    else:
        return "{:.1f} GB".format(bytes / (1024 * 1024 * 1024))

def format_time_ago(seconds):
    """Format seconds as human-readable time ago"""
    if seconds < 60:
        return "{}s ago".format(int(seconds))
    elif seconds < 3600:
        return "{}m ago".format(int(seconds / 60))
    elif seconds < 86400:
        return "{}h ago".format(int(seconds / 3600))
    else:
        days = int(seconds / 86400)
        return "{}d ago".format(days)

# Register the command
stats_tool = meow.tool(
    name="stats",
    description="Analyze project file statistics",
    params={
        "directory": meow.param("string", desc="Directory to analyze.", default="."),
    },
    handler=handler,
)

meow.command(stats_tool)
