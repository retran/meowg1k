// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package agentturn

import (
	"fmt"
	"strings"
)

const (
	modeList       = "list"
	modeDiff       = "diff"
	modeAdd        = "add"
	modeRun        = "run"
	modeRead       = "read"
	modeEmbeddings = "embeddings"
	modeStatus     = "status"
	modeLog        = "log"
	modeShow       = "show"
	modeComplete   = "complete"
	modeApply      = "apply"

	toolWorkspace = "workspace"
	toolSearch    = "search"
	toolGit       = "git"
	toolSummarize = "summarize"
	toolPlan      = "plan"
	toolMemory    = "memory"
	toolCommand   = "command"
	toolPatch     = "patch"
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
	case toolWorkspace:
		return td.getWorkspaceRunningMessage(args)
	case toolSearch:
		return td.getSearchRunningMessage(args)
	case toolGit:
		return td.getGitRunningMessage(args)
	case toolSummarize:
		return td.getSummarizeRunningMessage(args)
	case toolPlan:
		return td.getPlanRunningMessage(args)
	case toolMemory:
		return td.getMemoryRunningMessage(args)
	case toolCommand:
		return td.getCommandRunningMessage(args)
	case toolPatch:
		return td.getPatchRunningMessage(args)
	}
	return fmt.Sprintf("Running %s (%s)", td.Tool, td.Mode)
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
	return fmt.Sprintf("%s (%s) complete", td.Tool, td.Mode)
}

// Workspace tool messages.
func (td *ToolDescription) getWorkspaceRunningMessage(args map[string]any) string {
	var path string
	if p, ok := args["path"].(string); ok {
		path = p
	}

	switch td.Mode {
	case modeRead:
		return formatPathMsg(path, "Reading: %s", "Reading file")
	case "write":
		return formatPathMsg(path, "Writing: %s", "Writing file")
	case modeList:
		return formatPathMsg(path, "Listing: %s", "Listing directory")
	case "stat":
		return formatPathMsg(path, "Getting info: %s", "Getting file info")
	case "exists":
		return formatPathMsg(path, "Checking: %s", "Checking existence")
	case "mkdir":
		return formatPathMsg(path, "Creating: %s", "Creating directory")
	case "delete":
		return formatPathMsg(path, "Deleting: %s", "Deleting")
	case "replace":
		return formatPathMsg(path, "Replacing in: %s", "Replacing content")
	}
	return fmt.Sprintf("Running workspace (%s)", td.Mode)
}

func formatPathMsg(path, format, defaultMsg string) string {
	if path != "" {
		return fmt.Sprintf(format, path)
	}
	return defaultMsg
}

func (td *ToolDescription) getWorkspaceCompletedMessage(result *ToolResult) string {
	switch td.Mode {
	case "read":
		return "File read complete"
	case "write":
		return "File write complete"
	case "list":
		if data, ok := result.Data.([]interface{}); ok {
			return fmt.Sprintf("Listed %d items", len(data))
		}
		return "Directory listing complete"
	case "stat":
		return "File info retrieved"
	case "exists":
		return "Existence check complete"
	case "mkdir":
		return "Directory created"
	case "delete":
		return "Deletion complete"
	case "replace":
		return "Replacement complete"
	}
	return fmt.Sprintf("Workspace (%s) complete", td.Mode)
}

// Search tool messages.
func (td *ToolDescription) getSearchRunningMessage(args map[string]any) string {
	if td.Mode == modeEmbeddings {
		if searchindex, ok := args["query_text"].(string); ok && len(searchindex) > 40 {
			return fmt.Sprintf("Searching: %s", searchindex[:40])
		} else if searchindex, ok := args["query_text"].(string); ok {
			return fmt.Sprintf("Searching: %s", searchindex)
		}
		return "Searching codebase"
	}
	return fmt.Sprintf("Running search (%s)", td.Mode)
}

func (td *ToolDescription) getSearchCompletedMessage(result *ToolResult) string {
	if td.Mode == modeEmbeddings {
		if data, ok := result.Data.([]interface{}); ok {
			return fmt.Sprintf("Found %d results", len(data))
		}
		return "Search complete"
	}
	return fmt.Sprintf("Search (%s) complete", td.Mode)
}

// Git tool messages.
func (td *ToolDescription) getGitRunningMessage(args map[string]any) string {
	switch td.Mode {
	case modeDiff:
		ref, _ := args["ref"].(string)
		path, _ := args["path"].(string)
		switch {
		case ref != "" && path != "":
			return fmt.Sprintf("Getting diff: %s (%s)", path, ref)
		case path != "":
			return fmt.Sprintf("Getting diff: %s", path)
		case ref != "":
			return fmt.Sprintf("Getting diff (%s)", ref)
		default:
			return "Getting diff"
		}
	case modeStatus:
		return "Getting status"
	case modeLog:
		if limit, ok := args["limit"].(float64); ok {
			return fmt.Sprintf("Getting log (limit: %d)", int(limit))
		}
		return "Getting log"
	case "branch":
		return "Getting branch info"
	case "current_branch":
		return "Getting current branch"
	case "stage":
		return formatFilesMsg(args, "Staging %d paths", "Staging changes")
	case "commit":
		return formatCommitMsg(args)
	case modeShow:
		if commit, ok := args["commit"].(string); ok {
			return fmt.Sprintf("Showing commit: %s", commit)
		}
		return "Showing commit"
	}
	return fmt.Sprintf("Running git (%s)", td.Mode)
}

func formatFilesMsg(args map[string]any, format, defaultMsg string) string {
	if paths, ok := args["paths"].([]interface{}); ok && len(paths) > 0 {
		return fmt.Sprintf(format, len(paths))
	}
	if files, ok := args["files"].([]interface{}); ok && len(files) > 0 {
		return fmt.Sprintf(format, len(files))
	}
	return defaultMsg
}

func formatCommitMsg(args map[string]any) string {
	if msg, ok := args["message"].(string); ok {
		if len(msg) > 30 {
			return fmt.Sprintf("Committing: %s", msg[:30])
		}
		return fmt.Sprintf("Committing: %s", msg)
	}
	return "Committing changes"
}

func (td *ToolDescription) getGitCompletedMessage(_ *ToolResult) string {
	switch td.Mode {
	case modeDiff:
		return "Diff retrieved"
	case "status":
		return "Status retrieved"
	case "log":
		return "Log retrieved"
	case "branch":
		return "Branch info retrieved"
	case "current_branch":
		return "Current branch retrieved"
	case "stage":
		return "Changes staged"
	case "commit":
		return "Changes committed"
	case "show":
		return "Commit shown"
	}
	return fmt.Sprintf("Git (%s) complete", td.Mode)
}

// Summarize tool messages.
func (td *ToolDescription) getSummarizeRunningMessage(args map[string]any) string {
	switch td.Mode {
	case "text":
		return "Summarizing text"
	case "file":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("Summarizing file: %s", path)
		}
		return "Summarizing file"
	case modeDiff:
		return "Summarizing diff"
	}
	return fmt.Sprintf("Running summarize (%s)", td.Mode)
}

func (td *ToolDescription) getSummarizeCompletedMessage(_ *ToolResult) string {
	switch td.Mode {
	case "text":
		return "Text summarized"
	case "file":
		return "File summarized"
	case modeDiff:
		return "Diff summarized"
	}
	return fmt.Sprintf("Summarize (%s) complete", td.Mode)
}

// Plan tool messages.
func (td *ToolDescription) getPlanRunningMessage(args map[string]any) string {
	switch td.Mode {
	case modeAdd:
		if text, ok := args["text"].(string); ok {
			if len(text) > 40 {
				return fmt.Sprintf("Adding to plan: %s", text[:40])
			}
			return fmt.Sprintf("Adding to plan: %s", text)
		}
		return "Adding to plan"
	case modeComplete:
		if id, ok := args["task_id"].(float64); ok {
			return fmt.Sprintf("Completing plan item: %d", int(id))
		}
		if id, ok := args["task_id"].(int); ok {
			return fmt.Sprintf("Completing plan item: %d", id)
		}
		if id, ok := args["task_id"].(string); ok {
			return fmt.Sprintf("Completing plan item: %s", id)
		}
		return "Completing plan item"
	case modeList:
		return "Listing plan"
	}
	return fmt.Sprintf("Running plan (%s)", td.Mode)
}

func (td *ToolDescription) getPlanCompletedMessage(result *ToolResult) string {
	switch td.Mode {
	case modeAdd:
		return "Plan item added"
	case modeComplete:
		return "Plan item completed"
	case modeList:
		if data, ok := result.Data.([]interface{}); ok {
			return fmt.Sprintf("Listed %d plan items", len(data))
		}
		return "Plan listed"
	}
	return fmt.Sprintf("Plan (%s) complete", td.Mode)
}

// Memory tool messages.
func (td *ToolDescription) getMemoryRunningMessage(args map[string]any) string {
	switch td.Mode {
	case modeAdd:
		if text, ok := args["text"].(string); ok {
			if len(text) > 40 {
				return fmt.Sprintf("Storing memory: %s", text[:40])
			}
			return fmt.Sprintf("Storing memory: %s", text)
		}
		return "Storing memory"
	case modeList:
		return "Retrieving memory"
	}
	return fmt.Sprintf("Running memory (%s)", td.Mode)
}

func (td *ToolDescription) getMemoryCompletedMessage(result *ToolResult) string {
	switch td.Mode {
	case modeAdd:
		return "Memory stored"
	case modeList:
		if data, ok := result.Data.([]interface{}); ok {
			return fmt.Sprintf("Retrieved %d memories", len(data))
		}
		return "Memory retrieved"
	}
	return fmt.Sprintf("Memory (%s) complete", td.Mode)
}

// Command tool messages.
func (td *ToolDescription) getCommandRunningMessage(args map[string]any) string {
	if td.Mode == modeRun {
		if cmd, ok := args["command"].(string); ok && len(cmd) > 40 {
			return fmt.Sprintf("Running: %s", cmd[:40])
		} else if cmd, ok := args["command"].(string); ok {
			return fmt.Sprintf("Running: %s", cmd)
		}
		return "Running command"
	}
	return fmt.Sprintf("Running command (%s)", td.Mode)
}

func (td *ToolDescription) getCommandCompletedMessage(_ *ToolResult) string {
	if td.Mode == modeRun {
		return "Command executed"
	}
	return fmt.Sprintf("Command (%s) complete", td.Mode)
}

// Patch tool messages.
func (td *ToolDescription) getPatchRunningMessage(_ map[string]any) string {
	if td.Mode == modeApply {
		return "Applying patch"
	}
	return fmt.Sprintf("Running patch (%s)", td.Mode)
}

func (td *ToolDescription) getPatchCompletedMessage(_ *ToolResult) string {
	if td.Mode == modeApply {
		return "Patch applied"
	}
	return fmt.Sprintf("Patch (%s) complete", td.Mode)
}

// Tool registry - map of tool name to map of mode to ToolDescription.
var toolRegistry = map[string]map[string]*ToolDescription{
	toolWorkspace: {
		modeList: {
			Tool:        toolWorkspace,
			Mode:        modeList,
			Description: "List workspace entries under a path.",
			Parameters: schemaObject(map[string]any{
				"path":          schemaString("Relative path to list."),
				"depth":         schemaInteger("Maximum depth to traverse."),
				"include_files": schemaBool("Whether to include files."),
				"include_dirs":  schemaBool("Whether to include directories."),
			}, nil),
		},
		modeRead: {
			Tool:        toolWorkspace,
			Mode:        modeRead,
			Description: "Read the contents of a file.",
			Parameters: schemaObject(map[string]any{
				"path":       schemaString("Relative path to the file to read."),
				"start_line": schemaInteger("Start line number (1-based)."),
				"end_line":   schemaInteger("End line number (1-based)."),
			}, nil),
		},
		"write": {
			Tool:        toolWorkspace,
			Mode:        "write",
			Description: "Write content to a file.",
			Parameters: schemaObject(map[string]any{
				"path":      schemaString("Relative path to the file to write."),
				"content":   schemaString("Content to write to the file."),
				"create":    schemaBool("Create the file if it does not exist."),
				"overwrite": schemaBool("Overwrite the file if it exists."),
			}, nil),
		},
		"replace": {
			Tool:        toolWorkspace,
			Mode:        "replace",
			Description: "Replace content in a file.",
			Parameters: schemaObject(map[string]any{
				"path":          schemaString("Relative path to the file."),
				"old_text":      schemaString("Text to replace."),
				"new_text":      schemaString("New text."),
				"occurrence":    schemaString("Occurrence to replace (first, last, all)."),
				"require_match": schemaBool("Require old_text to match exactly."),
			}, nil),
		},
		"stat": {
			Tool:        toolWorkspace,
			Mode:        "stat",
			Description: "Get file or directory statistics.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Relative path to get statistics for."),
			}, nil),
		},
		"exists": {
			Tool:        toolWorkspace,
			Mode:        "exists",
			Description: "Check if a path exists.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Relative path to check."),
			}, nil),
		},
		"mkdir": {
			Tool:        toolWorkspace,
			Mode:        "mkdir",
			Description: "Create a directory.",
			Parameters: schemaObject(map[string]any{
				"path":    schemaString("Relative path of directory to create."),
				"parents": schemaBool("Create parent directories if needed."),
			}, nil),
		},
		"delete": {
			Tool:        toolWorkspace,
			Mode:        "delete",
			Description: "Delete a file or directory.",
			Parameters: schemaObject(map[string]any{
				"path":      schemaString("Relative path to delete."),
				"recursive": schemaBool("Delete directories recursively."),
			}, nil),
		},
	},
	toolSearch: {
		"embeddings": {
			Tool:        toolSearch,
			Mode:        "embeddings",
			Description: "Search the codebase using embeddings.",
			Parameters: schemaObject(map[string]any{
				"query_text": schemaString("Search searchindex."),
			}, nil),
		},
	},
	toolGit: {
		modeDiff: {
			Tool:        toolGit,
			Mode:        modeDiff,
			Description: "Get a git diff for a ref and optional path.",
			Parameters: schemaObject(map[string]any{
				"ref":  schemaString("Git ref (e.g. HEAD, main, commit SHA)."),
				"path": schemaString("Optional path to limit diff."),
			}, nil),
		},
		"status": {
			Tool:        toolGit,
			Mode:        "status",
			Description: "Get git status.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
		"log": {
			Tool:        toolGit,
			Mode:        "log",
			Description: "Get git log.",
			Parameters: schemaObject(map[string]any{
				"limit": schemaInteger("Maximum number of commits to return."),
				"path":  schemaString("Optional path to filter log by."),
			}, nil),
		},
		"branch": {
			Tool:        toolGit,
			Mode:        "branch",
			Description: "Get information about branches.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
		"current_branch": {
			Tool:        toolGit,
			Mode:        "current_branch",
			Description: "Get the current branch name.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
		"stage": {
			Tool:        toolGit,
			Mode:        "stage",
			Description: "Stage paths for commit.",
			Parameters: schemaObject(map[string]any{
				"paths": schemaArray(schemaString("Paths to stage.")),
			}, nil),
		},
		"commit": {
			Tool:        toolGit,
			Mode:        "commit",
			Description: "Commit staged changes.",
			Parameters: schemaObject(map[string]any{
				"message": schemaString("Commit message."),
			}, nil),
		},
		"show": {
			Tool:        toolGit,
			Mode:        "show",
			Description: "Show commit details.",
			Parameters: schemaObject(map[string]any{
				"ref": schemaString("Commit hash or reference."),
			}, nil),
		},
	},
	toolSummarize: {
		"text": {
			Tool:        toolSummarize,
			Mode:        "text",
			Description: "Summarize text content.",
			Parameters: schemaObject(map[string]any{
				"text": schemaString("Text to summarize."),
			}, nil),
		},
		"file": {
			Tool:        toolSummarize,
			Mode:        "file",
			Description: "Summarize file content.",
			Parameters: schemaObject(map[string]any{
				"path": schemaString("Path to file to summarize."),
			}, nil),
		},
		modeDiff: {
			Tool:        toolSummarize,
			Mode:        modeDiff,
			Description: "Summarize a git diff (by ref and optional path).",
			Parameters: schemaObject(map[string]any{
				"ref":  schemaString("Git ref (e.g. HEAD)."),
				"path": schemaString("Optional path to limit diff."),
			}, nil),
		},
	},
	toolPlan: {
		modeAdd: {
			Tool:        toolPlan,
			Mode:        modeAdd,
			Description: "Add an item to the plan.",
			Parameters: schemaObject(map[string]any{
				"text": schemaString("Plan item to add."),
			}, nil),
		},
		modeComplete: {
			Tool:        toolPlan,
			Mode:        modeComplete,
			Description: "Mark a plan item as complete.",
			Parameters: schemaObject(map[string]any{
				"task_id": schemaInteger("Plan item ID to complete."),
			}, nil),
		},
		modeList: {
			Tool:        toolPlan,
			Mode:        modeList,
			Description: "List plan items.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
	},
	toolMemory: {
		modeAdd: {
			Tool:        toolMemory,
			Mode:        modeAdd,
			Description: "Add information to memory.",
			Parameters: schemaObject(map[string]any{
				"text": schemaString("Text to remember."),
			}, nil),
		},
		modeList: {
			Tool:        toolMemory,
			Mode:        modeList,
			Description: "Retrieve memories.",
			Parameters:  schemaObject(map[string]any{}, nil),
		},
	},
	toolCommand: {
		modeRun: {
			Tool:        toolCommand,
			Mode:        modeRun,
			Description: "Run a shell command.",
			Parameters: schemaObject(map[string]any{
				"command": schemaString("Command to run."),
				"workdir": schemaString("Working directory for the command."),
				"timeout": schemaString("Timeout for the command (e.g. 30s)."),
			}, nil),
		},
	},
	toolPatch: {
		modeApply: {
			Tool:        toolPatch,
			Mode:        modeApply,
			Description: "Apply a patch to files.",
			Parameters: schemaObject(map[string]any{
				"diff": schemaString("Diff content to apply."),
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

// GetToolDescription returns the tool description for the given tool and mode.
func GetToolDescription(tool, mode string) *ToolDescription {
	if modes, ok := toolRegistry[tool]; ok {
		if td, ok := modes[mode]; ok {
			return td
		}
	}
	return nil
}

// GetDetailsMessage returns a detailed message for the tool execution.
func (td *ToolDescription) GetDetailsMessage(args map[string]any, result *ToolResult) string {
	switch td.Tool {
	case toolMemory:
		return td.getMemoryDetails(args, result)
	case toolCommand:
		return td.getCommandDetails(result)
	case toolWorkspace:
		return td.getWorkspaceDetails(result)
	case toolGit:
		return td.getGitDetails(result)
	case toolSearch:
		return td.getSearchDetails(result)
	case toolPlan:
		return td.getPlanDetails(args, result)
	case toolSummarize:
		return td.getSummarizeDetails(args, result)
	case toolPatch:
		return td.getPatchDetails(args)
	}
	return ""
}

func (td *ToolDescription) getMemoryDetails(args map[string]any, result *ToolResult) string {
	switch td.Mode {
	case modeAdd:
		if text, ok := args["text"].(string); ok {
			return fmt.Sprintf("Added to memory:\n%s", text)
		}
	case modeList:
		if data, ok := result.Data.([]interface{}); ok {
			var sb strings.Builder
			sb.WriteString("Memories:\n")
			for _, item := range data {
				if m, ok := item.(map[string]any); ok {
					id := m["id"]
					text := m["text"]
					sb.WriteString(fmt.Sprintf("- %v: %v\n", id, text))
					continue
				}
				sb.WriteString(fmt.Sprintf("- %v\n", item))
			}
			return sb.String()
		}
	}
	return ""
}

func (td *ToolDescription) getGitDetails(result *ToolResult) string {
	switch td.Mode {
	case modeDiff:
		diff, _ := result.Data.(string)
		if diff == "" {
			return ""
		}
		if len(diff) > 1000 {
			return diff[:1000]
		}
		return diff
	case modeStatus:
		status, _ := result.Data.(string)
		if status == "" {
			return ""
		}
		return status
	case modeLog:
		log, _ := result.Data.(string)
		if log == "" {
			return ""
		}
		return log
	case modeShow:
		show, _ := result.Data.(string)
		if show == "" {
			return ""
		}
		return show
	case "current_branch":
		branch, _ := result.Data.(string)
		if branch == "" {
			return ""
		}
		return branch
	case "branch":
		if data, ok := result.Data.([]interface{}); ok {
			var sb strings.Builder
			for _, item := range data {
				sb.WriteString(fmt.Sprintf("%v\n", item))
			}
			return sb.String()
		}
	case "stage", "commit":
		output, _ := result.Data.(string)
		if strings.TrimSpace(output) == "" {
			return ""
		}
		return output
	}
	return ""
}

func (td *ToolDescription) getSearchDetails(result *ToolResult) string {
	if td.Mode == modeEmbeddings {
		if data, ok := result.Data.([]interface{}); ok {
			var sb strings.Builder
			for _, item := range data {
				sb.WriteString(formatSearchItem(item))
			}
			return sb.String()
		}
	}
	return ""
}

func (td *ToolDescription) getPlanDetails(args map[string]any, result *ToolResult) string {
	switch td.Mode {
	case modeList:
		if data, ok := result.Data.([]interface{}); ok {
			var sb strings.Builder
			for _, item := range data {
				if m, ok := item.(map[string]any); ok {
					id := m["id"]
					text := m["text"]
					completed := m["completed"]
					if completed == true {
						sb.WriteString(fmt.Sprintf("%v (done): %v\n", id, text))
						continue
					}
					sb.WriteString(fmt.Sprintf("%v: %v\n", id, text))
					continue
				}
				sb.WriteString(fmt.Sprintf("%v\n", item))
			}
			return sb.String()
		}
	case modeAdd:
		if text, ok := args["text"].(string); ok {
			return text
		}
	case modeComplete:
		if id, ok := args["task_id"].(float64); ok {
			return fmt.Sprintf("Completed plan item: %d", int(id))
		}
		if id, ok := args["task_id"].(int); ok {
			return fmt.Sprintf("Completed plan item: %d", id)
		}
		if id, ok := args["task_id"].(string); ok {
			return fmt.Sprintf("Completed plan item: %s", id)
		}
	}
	return ""
}

func (td *ToolDescription) getSummarizeDetails(_ map[string]any, result *ToolResult) string {
	if summary, ok := result.Data.(string); ok {
		return summary
	}
	return ""
}

func (td *ToolDescription) getPatchDetails(args map[string]any) string {
	if td.Mode == modeApply {
		if diff, ok := args["diff"].(string); ok {
			if len(diff) > 500 {
				return fmt.Sprintf("Applied patch (truncated):\n%s", diff[:500])
			}
			return fmt.Sprintf("Applied patch:\n%s", diff)
		}
	}
	return ""
}

func (td *ToolDescription) getCommandDetails(result *ToolResult) string {
	if td.Mode != modeRun {
		return ""
	}

	data, ok := result.Data.(map[string]string)
	if !ok {
		return ""
	}

	var sb strings.Builder
	if stdout, ok := data["stdout"]; ok && stdout != "" {
		sb.WriteString(stdout)
		sb.WriteString("\n")
	}
	if stderr, ok := data["stderr"]; ok && stderr != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(stderr)
		sb.WriteString("\n")
	}
	return sb.String()
}

func (td *ToolDescription) getWorkspaceDetails(result *ToolResult) string {
	switch td.Mode {
	case modeRead:
		return td.getWorkspaceReadDetails(result)
	case modeList:
		return td.getWorkspaceListDetails(result)
	}
	return ""
}

func (td *ToolDescription) getWorkspaceReadDetails(result *ToolResult) string {
	if content, ok := result.Data.(string); ok {
		if len(content) > 500 {
			return content[:500]
		}
		return content
	}
	return ""
}

func (td *ToolDescription) getWorkspaceListDetails(result *ToolResult) string {
	data, ok := result.Data.([]interface{})
	if !ok {
		return ""
	}

	var sb strings.Builder
	for i, item := range data {
		if i >= 10 {
			sb.WriteString(fmt.Sprintf("%d more\n", len(data)-10))
			break
		}
		sb.WriteString(formatWorkspaceItem(item))
	}
	return sb.String()
}

func formatWorkspaceItem(item interface{}) string {
	// Try to format item nicely if it's a map
	if m, ok := item.(map[string]any); ok {
		if path, ok := m["path"]; ok {
			return fmt.Sprintf("%v\n", path)
		}
	}
	return fmt.Sprintf("%v\n", item)
}

func formatSearchItem(item interface{}) string {
	if m, ok := item.(map[string]any); ok {
		path, _ := m["file_path"].(string)
		start, _ := m["start_line"].(float64)
		end, _ := m["end_line"].(float64)
		score, _ := m["score"].(float64)
		if path != "" {
			if start > 0 && end > 0 {
				return fmt.Sprintf("%s (lines %d-%d, score %.2f)\n", path, int(start), int(end), score)
			}
			return fmt.Sprintf("%s (score %.2f)\n", path, score)
		}
	}
	return fmt.Sprintf("%v\n", item)
}
