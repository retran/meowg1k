// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package tools defines the standard tools available to the agent.
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/retran/meowg1k/internal/activities/control"
	"github.com/retran/meowg1k/internal/activities/deletefile"
	"github.com/retran/meowg1k/internal/activities/editfile"
	"github.com/retran/meowg1k/internal/activities/getdiff"
	"github.com/retran/meowg1k/internal/activities/getplan"
	"github.com/retran/meowg1k/internal/activities/gitundo"
	"github.com/retran/meowg1k/internal/activities/listfiles"
	"github.com/retran/meowg1k/internal/activities/memorize"
	"github.com/retran/meowg1k/internal/activities/movefile"
	"github.com/retran/meowg1k/internal/activities/plan"
	"github.com/retran/meowg1k/internal/activities/readfile"
	"github.com/retran/meowg1k/internal/activities/runshell"
	"github.com/retran/meowg1k/internal/activities/searchindex"
	"github.com/retran/meowg1k/internal/activities/summarize"
	"github.com/retran/meowg1k/internal/activities/tracktask"
	"github.com/retran/meowg1k/internal/activities/writefile"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/pkg/executor"
)

// ToolDependencies holds the activity factories and configuration for standard tools.
type ToolDependencies struct {
	ReadFile   executor.ActivityFactory[*readfile.Input, *readfile.Output]
	WriteFile  executor.ActivityFactory[*writefile.Input, *writefile.Output]
	EditFile   executor.ActivityFactory[*editfile.Input, *editfile.Output]
	MoveFile   executor.ActivityFactory[*movefile.Input, *movefile.Output]
	DeleteFile executor.ActivityFactory[*deletefile.Input, *deletefile.Output]
	GitUndo    executor.ActivityFactory[*gitundo.Input, *gitundo.Output]
	RunShell   executor.ActivityFactory[*runshell.Input, *runshell.Output]
	ListFiles  executor.ActivityFactory[*listfiles.Input, *listfiles.Output]
	SearchCode executor.ActivityFactory[*searchindex.Input, *searchindex.Output]
	GetDiff    executor.ActivityFactory[*getdiff.Input, *getdiff.Output]
	Memorize   executor.ActivityFactory[*memorize.Input, *memorize.Output]
	Plan       executor.ActivityFactory[*plan.Input, *plan.Output]
	GetPlan    executor.ActivityFactory[*getplan.Input, *getplan.Output]
	TrackTask  executor.ActivityFactory[*tracktask.Input, *tracktask.Output]
	Summarize  executor.ActivityFactory[*summarize.Input, *summarize.Output]
	Restart    executor.ActivityFactory[*control.RestartInput, *control.Output]

	// Configuration for defaults
	SearchSnapshots []string
	SearchTopK      int
	SearchMinScore  float32
}

func registerTool[I any, O any](
	r *Registry,
	factory executor.ActivityFactory[I, O],
	name string,
	description string,
	parameters map[string]any,
	required []string,
) {
	if factory == nil {
		return
	}

	r.Register(Tool{
		Definition: gateway.ToolDefinition{
			Name:        name,
			Description: description,
			Parameters: map[string]any{
				"type":       "object",
				"properties": parameters,
				"required":   required,
			},
		},
		Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
			var input I
			if err := BindArgs(args, &input); err != nil {
				return nil, err
			}
			act := factory.NewActivity()
			return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, name, act, input)
		},
	})
}

// RegisterStandardTools registers all standard tools to the registry.
func RegisterStandardTools(r *Registry, deps *ToolDependencies) {
	registerFileTools(r, deps)
	registerDirectoryTools(r, deps)
	registerSystemTools(r, deps)
	registerSearchTools(r, deps)
	registerGitTools(r, deps)
	registerPlanTools(r, deps)
	registerMemoryTools(r, deps)
	registerAgentTools(r, deps)
}

func registerFileTools(r *Registry, deps *ToolDependencies) {
	// --- FILE Operations ---
	registerTool(r, deps.ReadFile, "file_read",
		"Read file content. Use `start_line` and `end_line` for large files. Path must be relative to workspace root.",
		map[string]any{
			"path":       map[string]any{"type": "string", "description": "Path relative to workspace root (e.g. 'cmd/main.go')"},
			"start_line": map[string]any{"type": "integer", "description": "1-based start line (inclusive)"},
			"end_line":   map[string]any{"type": "integer", "description": "1-based end line (inclusive)"},
		},
		[]string{"path"},
	)

	registerTool(r, deps.WriteFile, "file_write",
		"Create or overwrite a file. Content MUST be the complete file source code. Do not use placeholders.",
		map[string]any{
			"path":    map[string]any{"type": "string", "description": "Path relative to workspace root"},
			"content": map[string]any{"type": "string", "description": "Full content of the file"},
		},
		[]string{"path", "content"},
	)

	registerTool(r, deps.EditFile, "file_edit",
		"Modify an existing file by replacing a specific text block. `old_string` must match exactly one continuous block in the file. Include surrounding lines in `old_string` to ensure uniqueness. Use this for refactoring or bug fixes in large files.",
		map[string]any{
			"path":       map[string]any{"type": "string", "description": "Path relative to workspace root"},
			"old_string": map[string]any{"type": "string", "description": "The exact block of text to be replaced. Must be unique in the file."},
			"new_string": map[string]any{"type": "string", "description": "The new block of text to insert in place of old_string."},
		},
		[]string{"path", "old_string", "new_string"},
	)

	registerTool(r, deps.MoveFile, "file_move",
		"Move or rename a file or directory. Creates destination directories as needed. Uses git mv when in a git repository.",
		map[string]any{
			"source_path": map[string]any{"type": "string", "description": "Current path relative to workspace root"},
			"dest_path":   map[string]any{"type": "string", "description": "New path relative to workspace root"},
		},
		[]string{"source_path", "dest_path"},
	)

	registerTool(r, deps.DeleteFile, "file_delete",
		"Permanently delete a file. Use with caution.",
		map[string]any{
			"path": map[string]any{"type": "string", "description": "Path relative to workspace root"},
		},
		[]string{"path"},
	)

	registerTool(r, deps.GitUndo, "git_undo",
		"Discard uncommitted changes to a file (git checkout HEAD). Use this if a previous file_edit broke the code and you want to revert to the last clean state.",
		map[string]any{
			"path": map[string]any{"type": "string", "description": "Path to the file to revert (relative to workspace root)"},
		},
		[]string{"path"},
	)
}

func registerDirectoryTools(r *Registry, deps *ToolDependencies) {
	// --- DIRECTORY Operations ---
	registerTool(r, deps.ListFiles, "dir_list",
		"List direct children of a directory (non-recursive).",
		map[string]any{
			"dir": map[string]any{"type": "string", "description": "Directory relative to root"},
		},
		[]string{"dir"},
	)
}

func registerSystemTools(r *Registry, deps *ToolDependencies) {
	// --- SYSTEM/SHELL Operations ---
	if deps.RunShell != nil {
		handler := func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
			var input runshell.Input
			if err := BindArgs(args, &input); err != nil {
				return nil, err
			}
			act := deps.RunShell.NewActivity()
			return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "shell_exec", act, &input)
		}

		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "shell_exec",
				Description: "Execute a non-interactive shell command.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{"type": "string", "description": "Executable (e.g. 'go', 'ls')"},
						"args":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
					"required": []string{"command"},
				},
			},
			Handler: handler,
		})
	}
}

func registerSearchTools(r *Registry, deps *ToolDependencies) {
	// --- SEARCH Operations ---
	registerSearchTextTool(r, deps)
	registerSearchSemanticTool(r, deps)
}

func registerSearchTextTool(r *Registry, deps *ToolDependencies) {
	if deps.RunShell == nil {
		return
	}

	r.Register(Tool{
		Definition: gateway.ToolDefinition{
			Name:        "search_text",
			Description: "Search text/regex using rg (ripgrep) or grep. Use this for exact string matches. Prefer `search_semantic` for concepts.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{"type": "string", "description": "Pattern to search"},
					"path":    map[string]any{"type": "string", "description": "Search path (default '.')"},
					"fixed":   map[string]any{"type": "boolean", "description": "Treat pattern as literal string"},
					"glob":    map[string]any{"type": "string", "description": "File glob (e.g. '*.go')"},
					"max":     map[string]any{"type": "integer", "description": "Max matches"},
				},
				"required": []string{"pattern"},
			},
		},
		Handler: searchTextHandler(deps),
	})
}

func searchTextHandler(deps *ToolDependencies) func(context.Context, *executor.Context, map[string]any) (any, error) {
	return func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
		pattern, ok := args["pattern"].(string)
		if !ok || strings.TrimSpace(pattern) == "" {
			return nil, fmt.Errorf("pattern is required")
		}

		path, ok := args["path"].(string)
		if !ok || strings.TrimSpace(path) == "" {
			path = "."
		}

		fixed, ok := args["fixed"].(bool)
		if !ok {
			fixed = false
		}
		glob, ok := args["glob"].(string)
		if !ok {
			glob = ""
		}

		maxResults := resolveMaxResults(args["max"])

		bin := "grep"
		if _, err := exec.LookPath("rg"); err == nil {
			bin = "rg"
		}

		var cmdArgs []string
		if bin == "rg" {
			cmdArgs = buildRgArgs(pattern, path, glob, fixed, maxResults)
		} else {
			cmdArgs = buildGrepArgs(pattern, path, glob, fixed, maxResults)
		}

		input := &runshell.Input{Command: bin, Args: cmdArgs}
		act := deps.RunShell.NewActivity()
		return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "search_text", act, input)
	}
}

func resolveMaxResults(maxVal any) int {
	switch v := maxVal.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func buildRgArgs(pattern, path, glob string, fixed bool, maxResults int) []string {
	cmdArgs := []string{"--no-heading", "--line-number", "--color=never"}
	if fixed {
		cmdArgs = append(cmdArgs, "-F")
	}
	if glob != "" {
		cmdArgs = append(cmdArgs, "-g", glob)
	}
	if maxResults > 0 {
		cmdArgs = append(cmdArgs, "--max-count", fmt.Sprintf("%d", maxResults))
	}
	cmdArgs = append(cmdArgs, pattern, path)
	return cmdArgs
}

func buildGrepArgs(pattern, path, glob string, fixed bool, maxResults int) []string {
	cmdArgs := []string{"-n"}
	if fixed {
		cmdArgs = append(cmdArgs, "-F")
	}
	if glob != "" {
		cmdArgs = append(cmdArgs, "--include", glob, "-R")
	} else {
		cmdArgs = append(cmdArgs, "-r")
	}
	if maxResults > 0 {
		cmdArgs = append(cmdArgs, "-m", fmt.Sprintf("%d", maxResults))
	}
	cmdArgs = append(cmdArgs, pattern, path)
	return cmdArgs
}

func registerSearchSemanticTool(r *Registry, deps *ToolDependencies) {
	if deps.SearchCode == nil {
		return
	}

	r.Register(Tool{
		Definition: gateway.ToolDefinition{
			Name:        "search_semantic",
			Description: "Semantic code search. Use for discovery/concepts. Fallback to `search_text` if no results.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Natural language query"},
				},
				"required": []string{"query"},
			},
		},
		Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
			query, ok := args["query"].(string)
			if !ok {
				return nil, fmt.Errorf("query is required")
			}

			input := &searchindex.Input{
				QueryText:        query,
				SnapshotPriority: deps.SearchSnapshots,
				TopK:             deps.SearchTopK,
				MinScore:         deps.SearchMinScore,
			}
			if len(input.SnapshotPriority) == 0 {
				input.SnapshotPriority = []string{"_workdir_"}
			}
			if input.TopK == 0 {
				input.TopK = 10
			}

			act := deps.SearchCode.NewActivity()
			return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "search_semantic", act, input)
		},
	})
}

func registerGitTools(r *Registry, deps *ToolDependencies) {
	// --- GIT Operations ---
	registerTool(r, deps.GetDiff, "git_diff",
		"Get git diff of changes in working directory.",
		map[string]any{
			"staged": map[string]any{"type": "boolean"},
		},
		nil,
	)
}

func registerPlanTools(r *Registry, deps *ToolDependencies) {
	// --- PLAN Operations ---
	registerTool(r, deps.Plan, "plan_init",
		"Initialize a new task plan.",
		map[string]any{
			"tasks": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":          map[string]any{"type": "string"},
						"description": map[string]any{"type": "string"},
					},
					"required": []string{"id", "description"},
				},
			},
		},
		[]string{"tasks"},
	)

	if deps.GetPlan != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "plan_read",
				Description: "Get the current task plan and statuses.",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, _ map[string]any) (any, error) {
				var input getplan.Input
				act := deps.GetPlan.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "plan_read", act, &input)
			},
		})
	}

	registerTool(r, deps.TrackTask, "plan_update_task",
		"Update the status of a specific task.",
		map[string]any{
			"id":     map[string]any{"type": "string"},
			"status": map[string]any{"type": "string", "enum": []string{"pending", "done", "failed", "skipped"}},
		},
		[]string{"id", "status"},
	)
}

func registerMemoryTools(r *Registry, deps *ToolDependencies) {
	// --- MEMORY/UTIL Operations ---
	registerTool(r, deps.Memorize, "mem_store",
		"Save a critical fact to long-term memory.",
		map[string]any{
			"fact": map[string]any{"type": "string"},
		},
		[]string{"fact"},
	)
}

func registerAgentTools(r *Registry, deps *ToolDependencies) {
	if deps.Summarize != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "util_summarize",
				Description: "Summarize text, files, or diffs.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content": map[string]any{"type": "string"},
						"type":    map[string]any{"type": "string", "enum": []string{"text", "diff", "file"}},
					},
					"required": []string{"content"},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input summarize.Input
				if err := BindArgs(args, &input); err != nil {
					return nil, err
				}
				if input.Type == "" {
					input.Type = "text"
				}
				act := deps.Summarize.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "util_summarize", act, &input)
			},
		})
	}

	// --- AGENT Operations ---
	registerTool(r, deps.Restart, "agent_restart",
		"Hard reset agent flow with a NEW instruction. Use when unrecoverable.",
		map[string]any{
			"instruction": map[string]any{"type": "string", "description": "Full goal prompt for the next attempt"},
		},
		[]string{"instruction"},
	)
}
