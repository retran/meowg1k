"""
Project Statistics Command

This command demonstrates advanced file system operations by analyzing
project statistics including file counts, sizes, and types.

Usage:
  meow stats [directory]

Example:
  meow stats           # Analyze current directory
  meow stats src/      # Analyze src/ directory
"""

def run(ctx, args):
    """Generate statistics about project files"""
    
    # Get directory from args or use current directory
    target_dir = args[0] if len(args) > 0 else "."
    
    # Check if directory exists
    if not ctx.fs.exists(target_dir):
        return ctx.result(error=f"Directory '{target_dir}' does not exist")
    
    # Get directory metadata
    dir_info = ctx.fs.stat(target_dir)
    if not dir_info.is_dir:
        return ctx.result(error=f"'{target_dir}' is not a directory")
    
    ctx.ui.info(f"Analyzing directory: {target_dir}")
    ctx.ui.print("")
    
    # Walk directory to find all files
    all_files = ctx.fs.walk(target_dir)
    
    if len(all_files) == 0:
        ctx.ui.warning("No files found in directory")
        return ctx.result()
    
    # Collect statistics
    total_size = 0
    file_types = {}
    largest_files = []
    recent_files = []
    import time
    current_time = time.time()
    
    for file_path in all_files:
        try:
            stat = ctx.fs.stat(file_path)
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
        except:
            # Skip files we can't stat
            pass
    
    # Sort recent files by modification time
    recent_files = sorted(recent_files, key=lambda x: x["mtime"], reverse=True)[:10]
    
    # Display summary
    ctx.ui.print("## Summary")
    ctx.ui.print(f"Total files: {len(all_files)}")
    ctx.ui.print(f"Total size: {format_size(total_size)}")
    ctx.ui.print("")
    
    # Display file types
    ctx.ui.print("## File Types")
    sorted_types = sorted(file_types.items(), key=lambda x: x[1]["count"], reverse=True)
    for ext, info in sorted_types[:10]:  # Top 10 types
        ctx.ui.print(f"  {ext:20s} {info['count']:5d} files  {format_size(info['size']):>10s}")
    ctx.ui.print("")
    
    # Display largest files
    ctx.ui.print("## Largest Files")
    for item in largest_files:
        ctx.ui.print(f"  {format_size(item['size']):>10s}  {item['path']}")
    ctx.ui.print("")
    
    # Display recently modified files
    if len(recent_files) > 0:
        ctx.ui.print("## Recently Modified (Last 7 Days)")
        for item in recent_files:
            age = current_time - item["mtime"]
            ctx.ui.print(f"  {format_time_ago(age):>12s}  {item['path']}")
        ctx.ui.print("")
    
    # Example of other fs operations
    ctx.ui.print("## Directory Contents (Top Level)")
    items = ctx.fs.listdir(target_dir)
    for item in items[:10]:  # Show first 10 items
        item_path = f"{target_dir}/{item}" if target_dir != "." else item
        try:
            item_stat = ctx.fs.stat(item_path)
            item_type = "DIR " if item_stat.is_dir else "FILE"
            ctx.ui.print(f"  {item_type}  {item}")
        except:
            ctx.ui.print(f"  ????  {item}")
    
    return ctx.result()

def format_size(bytes):
    """Format bytes as human-readable size"""
    if bytes < 1024:
        return f"{bytes} B"
    elif bytes < 1024 * 1024:
        return f"{bytes / 1024:.1f} KB"
    elif bytes < 1024 * 1024 * 1024:
        return f"{bytes / (1024 * 1024):.1f} MB"
    else:
        return f"{bytes / (1024 * 1024 * 1024):.1f} GB"

def format_time_ago(seconds):
    """Format seconds as human-readable time ago"""
    if seconds < 60:
        return f"{int(seconds)}s ago"
    elif seconds < 3600:
        return f"{int(seconds / 60)}m ago"
    elif seconds < 86400:
        return f"{int(seconds / 3600)}h ago"
    else:
        days = int(seconds / 86400)
        return f"{days}d ago"

# Register the command
meow.tool(
    name="stats",
    description="Analyze project file statistics",
    handler=run,
)
