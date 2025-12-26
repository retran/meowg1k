// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package agentstep

import (
	"fmt"
)

const (
	modeList = "list"
	modeDiff = "diff"
	modeAdd  = "add"
)

// ToolDescription encapsulates all data and behavior for a specific tool mode.
type ToolDescription struct {
	Parameters  map[string]any
	Tool        string
	Mode        string
	Description string
}

// GetRunningMessage returns a human-readable running status message for this tool.
func (td *ToolDescription) GetRunningMessage(args map[string]any) string {
	switch td.Tool {
	case "workspace":
		return td.getWorkspaceRunningMessage(args)
	case "search":
		return td.getSearchRunningMessage(args)
	case "git":
		return td.getGitRunningMessage(args)
	case "summarize":
		return td.getSummarizeRunningMessage(args)
	case "plan":
		return td.getPlanRunningMessage(args)
	case "memory":
		return td.getMemoryRunningMessage(args)
	case "command":
		return td.getCommandRunningMessage(args)
	case "patch":
		return td.getPatchRunningMessage(args)
	}
	return fmt.Sprintf("🔧 Running %s (%s)...", td.Tool, td.Mode)
}

// GetCompletedMessage returns a human-readable completion status message for this tool.
func (td *ToolDescription) GetCompletedMessage(result *ToolResult) string {
	switch td.Tool {
	case "workspace":
		return td.getWorkspaceCompletedMessage(result)
	case "search":
		return td.getSearchCompletedMessage(result)
	case "git":
		return td.getGitCompletedMessage(result)
	case "summarize":
		return td.getSummarizeCompletedMessage(result)
	case "plan":
		return td.getPlanCompletedMessage(result)
	case "memory":
		return td.getMemoryCompletedMessage(result)
	case "command":
		return td.getCommandCompletedMessage(result)
	case "patch":
		return td.getPatchCompletedMessage(result)
	}
	return fmt.Sprintf("🔧 %s (%s) complete", td.Tool, td.Mode)
}

// Workspace tool messages.
func (td *ToolDescription) getWorkspaceRunningMessage(args map[string]any) string {
	switch td.Mode {
	case "read":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("📖 Reading: %s", path)
		}
		return "📖 Reading file..."
	case "write":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("✏️ Writing: %s", path)
		}
		return "✏️ Writing file..."
	case "list":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("📂 Listing: %s", path)
		}
		return "📂 Listing directory..."
	case "stat":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("ℹ️ Getting info: %s", path)
		}
		return "ℹ️ Getting file info..."
	case "exists":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("🔍 Checking: %s", path)
		}
		return "🔍 Checking existence..."
	case "mkdir":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("📁 Creating: %s", path)
		}
		return "📁 Creating directory..."
	case "delete":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("🗑️ Deleting: %s", path)
		}
		return "🗑️ Deleting..."
	}
	return fmt.Sprintf("🔧 Running workspace (%s)...", td.Mode)
}

func (td *ToolDescription) getWorkspaceCompletedMessage(result *ToolResult) string {
	switch td.Mode {
	case "read":
		return "📖 File read complete"
	case "write":
		return "✏️ File write complete"
	case "list":
		if data, ok := result.Data.([]interface{}); ok {
			return fmt.Sprintf("📂 Listed %d items", len(data))
		}
		return "📂 Directory listing complete"
	case "stat":
		return "ℹ️ File info retrieved"
	case "exists":
		return "🔍 Existence check complete"
	case "mkdir":
		return "📁 Directory created"
	case "delete":
		return "🗑️ Deletion complete"
	}
	return fmt.Sprintf("🔧 Workspace (%s) complete", td.Mode)
}

// Search tool messages.
func (td *ToolDescription) getSearchRunningMessage(args map[string]any) string {
	if td.Mode == "embeddings" {
		if query, ok := args["query"].(string); ok && len(query) > 20 {
			return fmt.Sprintf("🔍 Searching: %s...", query[:20])
		} else if query, ok := args["query"].(string); ok {
			return fmt.Sprintf("🔍 Searching: %s", query)
		}
		return "🔍 Searching codebase..."
	}
	return fmt.Sprintf("🔍 Running search (%s)...", td.Mode)
}

func (td *ToolDescription) getSearchCompletedMessage(result *ToolResult) string {
	if td.Mode == "embeddings" {
		if data, ok := result.Data.([]interface{}); ok {
			return fmt.Sprintf("🔍 Found %d results", len(data))
		}
		return "🔍 Search complete"
	}
	return fmt.Sprintf("🔍 Search (%s) complete", td.Mode)
}

// Git tool messages.
func (td *ToolDescription) getGitRunningMessage(_ map[string]any) string {
	switch td.Mode {
	case modeDiff:
		return "📊 Getting diff..."
	case "status":
		return "📋 Getting status..."
	case "log":
		return "📜 Getting log..."
	case "branch":
		return "🌿 Getting branch info..."
	case "current_branch":
		return "🌿 Getting current branch..."
	case "stage":
		return "📤 Staging changes..."
	case "commit":
		return "💾 Committing changes..."
	case "show":
		return "📖 Showing commit..."
	}
	return fmt.Sprintf("🌿 Running git (%s)...", td.Mode)
}

func (td *ToolDescription) getGitCompletedMessage(_ *ToolResult) string {
	switch td.Mode {
	case modeDiff:
		return "📊 Diff retrieved"
	case "status":
		return "📋 Status retrieved"
	case "log":
		return "📜 Log retrieved"
	case "branch":
		return "🌿 Branch info retrieved"
	case "current_branch":
		return "🌿 Current branch retrieved"
	case "stage":
		return "📤 Changes staged"
	case "commit":
		return "💾 Changes committed"
	case "show":
		return "📖 Commit shown"
	}
	return fmt.Sprintf("🌿 Git (%s) complete", td.Mode)
}

// Summarize tool messages.
func (td *ToolDescription) getSummarizeRunningMessage(_ map[string]any) string {
	switch td.Mode {
	case "text":
		return "🧠 Summarizing text..."
	case "file":
		return "🧠 Summarizing file..."
	case modeDiff:
		return "🧠 Summarizing diff..."
	}
	return fmt.Sprintf("🧠 Running summarize (%s)...", td.Mode)
}

func (td *ToolDescription) getSummarizeCompletedMessage(_ *ToolResult) string {
	switch td.Mode {
	case "text":
		return "🧠 Text summarized"
	case "file":
		return "🧠 File summarized"
	case modeDiff:
		return "🧠 Diff summarized"
	}
	return fmt.Sprintf("🧠 Summarize (%s) complete", td.Mode)
}

// Plan tool messages.
func (td *ToolDescription) getPlanRunningMessage(_ map[string]any) string {
	switch td.Mode {
	case modeAdd:
		return "📝 Adding to plan..."
	case "complete":
		return "✅ Completing plan item..."
	case modeList:
		return "📋 Listing plan..."
	}
	return fmt.Sprintf("📝 Running plan (%s)...", td.Mode)
}

func (td *ToolDescription) getPlanCompletedMessage(result *ToolResult) string {
	switch td.Mode {
	case modeAdd:
		return "📝 Plan item added"
	case "complete":
		return "✅ Plan item completed"
	case modeList:
		if data, ok := result.Data.([]interface{}); ok {
			return fmt.Sprintf("📋 Listed %d plan items", len(data))
		}
		return "📋 Plan listed"
	}
	return fmt.Sprintf("📝 Plan (%s) complete", td.Mode)
}

// Memory tool messages.
func (td *ToolDescription) getMemoryRunningMessage(_ map[string]any) string {
	switch td.Mode {
	case modeAdd:
		return "🧠 Storing memory..."
	case modeList:
		return "🧠 Retrieving memory..."
	}
	return fmt.Sprintf("🧠 Running memory (%s)...", td.Mode)
}

func (td *ToolDescription) getMemoryCompletedMessage(result *ToolResult) string {
	switch td.Mode {
	case modeAdd:
		return "🧠 Memory stored"
	case modeList:
		if data, ok := result.Data.([]interface{}); ok {
			return fmt.Sprintf("🧠 Retrieved %d memories", len(data))
		}
		return "🧠 Memory retrieved"
	}
	return fmt.Sprintf("🧠 Memory (%s) complete", td.Mode)
}

// Command tool messages.
func (td *ToolDescription) getCommandRunningMessage(args map[string]any) string {
	if td.Mode == "run" {
		if cmd, ok := args["command"].(string); ok && len(cmd) > 20 {
			return fmt.Sprintf("⚡ Running: %s...", cmd[:20])
		} else if cmd, ok := args["command"].(string); ok {
			return fmt.Sprintf("⚡ Running: %s", cmd)
		}
		return "⚡ Running command..."
	}
	return fmt.Sprintf("⚡ Running command (%s)...", td.Mode)
}

func (td *ToolDescription) getCommandCompletedMessage(_ *ToolResult) string {
	if td.Mode == "run" {
		return "⚡ Command executed"
	}
	return fmt.Sprintf("⚡ Command (%s) complete", td.Mode)
}

// Patch tool messages.
func (td *ToolDescription) getPatchRunningMessage(_ map[string]any) string {
	if td.Mode == "apply" {
		return "🩹 Applying patch..."
	}
	return fmt.Sprintf("🩹 Running patch (%s)...", td.Mode)
}

func (td *ToolDescription) getPatchCompletedMessage(_ *ToolResult) string {
	if td.Mode == "apply" {
		return "🩹 Patch applied"
	}
	return fmt.Sprintf("🩹 Patch (%s) complete", td.Mode)
}

// Tool registry - map of tool name to map of mode to ToolDescription.
var toolRegistry = map[string]map[string]*ToolDescription{
	"workspace": {
		"list": {
			Tool:        "workspace",
			Mode:        "list",
			Description: "List workspace entries under a path.",
			Parameters: schemaObject(map[string]any{
				"path":          schemaString("Relative path to list."),
				"depth":         schemaInteger("Maximum depth to traverse."),
				"include_files": schemaBool("Whether to include files."),
				"include_dirs":  schemaBool("Whether to include directories."),
			}, nil),
		},
		"read": {
			Tool:        "workspace",
			Mode:        "read",
			Description: "Read the contents of a file.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Relative path to the file to read."),
			}, nil),
		},
		"write": {
			Tool:        "workspace",
			Mode:        "write",
			Description: "Write content to a file.",
			Parameters: schemaObject(map[string]any{
				"path":    schemaString("Relative path to the file to write."),
				"content": schemaString("Content to write to the file."),
			}, nil),
		},
		"stat": {
			Tool:        "workspace",
			Mode:        "stat",
			Description: "Get file or directory statistics.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Relative path to get statistics for."),
			}, nil),
		},
		"exists": {
			Tool:        "workspace",
			Mode:        "exists",
			Description: "Check if a path exists.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Relative path to check."),
			}, nil),
		},
		"mkdir": {
			Tool:        "workspace",
			Mode:        "mkdir",
			Description: "Create a directory.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Relative path of directory to create."),
			}, nil),
		},
		"delete": {
			Tool:        "workspace",
			Mode:        "delete",
			Description: "Delete a file or directory.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Relative path to delete."),
			}, nil),
		},
	},
	"search": {
		"embeddings": {
			Tool:        "search",
			Mode:        "embeddings",
			Description: "Search the codebase using embeddings.",
			Parameters: schemaObject(map[string]any{
				"query":     schemaString("Search query."),
				"top_k":     schemaInteger("Number of results to return."),
				"min_score": schemaNumber("Minimum similarity score."),
				"snapshots": schemaArray(schemaString("Snapshot IDs to search in.")),
			}, nil),
		},
	},
	"git": {
		"diff": {
			Tool:        "git",
			Mode:        "diff",
			Description: "Get git diff for files.",
			Parameters: schemaObject(map[string]any{
				"files": schemaArray(schemaString("File paths to get diff for.")),
			}, nil),
		},
		"status": {
			Tool:        "git",
			Mode:        "status",
			Description: "Get git status.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
		"log": {
			Tool:        "git",
			Mode:        "log",
			Description: "Get git log.",
			Parameters: schemaObject(map[string]any{
				"limit": schemaInteger("Maximum number of commits to return."),
				"files": schemaArray(schemaString("File paths to filter log by.")),
			}, nil),
		},
		"branch": {
			Tool:        "git",
			Mode:        "branch",
			Description: "Get information about branches.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
		"current_branch": {
			Tool:        "git",
			Mode:        "current_branch",
			Description: "Get the current branch name.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
		"stage": {
			Tool:        "git",
			Mode:        "stage",
			Description: "Stage files for commit.",
			Parameters: schemaObject(map[string]any{
				"files": schemaArray(schemaString("File paths to stage.")),
			}, nil),
		},
		"commit": {
			Tool:        "git",
			Mode:        "commit",
			Description: "Commit staged changes.",
			Parameters: schemaObject(map[string]any{
				"message": schemaString("Commit message."),
			}, nil),
		},
		"show": {
			Tool:        "git",
			Mode:        "show",
			Description: "Show commit details.",
			Parameters: schemaObject(map[string]any{
				"commit": schemaString("Commit hash or reference."),
			}, nil),
		},
	},
	"summarize": {
		"text": {
			Tool:        "summarize",
			Mode:        "text",
			Description: "Summarize text content.",
			Parameters: schemaObject(map[string]any{
				"text": schemaString("Text to summarize."),
			}, nil),
		},
		"file": {
			Tool:        "summarize",
			Mode:        "file",
			Description: "Summarize file content.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Path to file to summarize."),
			}, nil),
		},
		"diff": {
			Tool:        "summarize",
			Mode:        "diff",
			Description: "Summarize diff content.",
			Parameters: schemaObject(map[string]any{
				"diff": schemaString("Diff content to summarize."),
			}, nil),
		},
	},
	"plan": {
		"add": {
			Tool:        "plan",
			Mode:        "add",
			Description: "Add an item to the plan.",
			Parameters: schemaObject(map[string]any{
				"item": schemaString("Plan item to add."),
			}, nil),
		},
		"complete": {
			Tool:        "plan",
			Mode:        "complete",
			Description: "Mark a plan item as complete.",
			Parameters: schemaObject(map[string]any{
				"id": schemaString("Plan item ID to complete."),
			}, nil),
		},
		"list": {
			Tool:        "plan",
			Mode:        "list",
			Description: "List plan items.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
	},
	"memory": {
		"add": {
			Tool:        "memory",
			Mode:        "add",
			Description: "Add information to memory.",
			Parameters: schemaObject(map[string]any{
				"content": schemaString("Content to remember."),
				"key":     schemaString("Optional key for the memory item."),
			}, nil),
		},
		"list": {
			Tool:        "memory",
			Mode:        "list",
			Description: "Retrieve memories.",
			Parameters: schemaObject(map[string]any{
				"key": schemaString("Optional key to filter memories."),
			}, nil),
		},
	},
	"command": {
		"run": {
			Tool:        "command",
			Mode:        "run",
			Description: "Run a shell command.",
			Parameters: schemaObject(map[string]any{
				"command": schemaString("Command to run."),
				"cwd":     schemaString("Working directory for the command."),
			}, nil),
		},
	},
	"patch": {
		"apply": {
			Tool:        "patch",
			Mode:        "apply",
			Description: "Apply a patch to files.",
			Parameters: schemaObject(map[string]any{
				"patch": schemaString("Patch content to apply."),
			}, nil),
		},
	},
}

// Helper functions for schema creation.
//
//nolint:unparam // required parameter is used in some calls but linter might miss it or it's intended for future use
func schemaObject(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func schemaString(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

func schemaInteger(description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": description,
	}
}

func schemaNumber(description string) map[string]any {
	return map[string]any{
		"type":        "number",
		"description": description,
	}
}

func schemaBool(description string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": description,
	}
}

func schemaArray(items map[string]any) map[string]any {
	return map[string]any{
		"type":  "array",
		"items": items,
	}
}
