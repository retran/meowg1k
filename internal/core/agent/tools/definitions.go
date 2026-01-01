// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/retran/meowg1k/internal/activities/control"
	"github.com/retran/meowg1k/internal/activities/editfile"
	"github.com/retran/meowg1k/internal/activities/getdiff"
	"github.com/retran/meowg1k/internal/activities/getplan"
	"github.com/retran/meowg1k/internal/activities/listfiles"
	"github.com/retran/meowg1k/internal/activities/memorize"
	"github.com/retran/meowg1k/internal/activities/plan"
	"github.com/retran/meowg1k/internal/activities/readfile"
	"github.com/retran/meowg1k/internal/activities/runcommand"
	"github.com/retran/meowg1k/internal/activities/searchindex"
	"github.com/retran/meowg1k/internal/activities/summarize"
	"github.com/retran/meowg1k/internal/activities/tracktask"
	"github.com/retran/meowg1k/internal/activities/writefile"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/pkg/executor"
)

type ToolDependencies struct {
	ReadFile   executor.ActivityFactory[*readfile.Input, *readfile.Output]
	WriteFile  executor.ActivityFactory[*writefile.Input, *writefile.Output]
	EditFile   executor.ActivityFactory[*editfile.Input, *editfile.Output]
	RunCommand executor.ActivityFactory[*runcommand.Input, *runcommand.Output]
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

func RegisterStandardTools(r *Registry, deps ToolDependencies) {
	// read_file
	if deps.ReadFile != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "read_file",
				Description: "Read the content of a file, optionally limiting to specific lines.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":       map[string]any{"type": "string", "description": "Path to the file relative to workspace root"},
						"start_line": map[string]any{"type": "integer", "description": "1-based start line (inclusive)"},
						"end_line":   map[string]any{"type": "integer", "description": "1-based end line (inclusive)"},
					},
					"required": []string{"path"},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input readfile.Input
				if err := BindArgs(args, &input); err != nil {
					return nil, err
				}
				act := deps.ReadFile.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "read_file", act, &input)
			},
		})
	}

	// write_file
	if deps.WriteFile != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "write_file",
				Description: "Create or overwrite a file with full content.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":    map[string]any{"type": "string", "description": "Path to the file"},
						"content": map[string]any{"type": "string", "description": "Full content of the file"},
					},
					"required": []string{"path", "content"},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input writefile.Input
				if err := BindArgs(args, &input); err != nil {
					return nil, err
				}
				act := deps.WriteFile.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "write_file", act, &input)
			},
		})
	}

	// edit_file
	if deps.EditFile != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "edit_file",
				Description: "Replace a specific block of text in a file.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":       map[string]any{"type": "string"},
						"old_string": map[string]any{"type": "string", "description": "Exact text to replace (must match exactly one location)"},
						"new_string": map[string]any{"type": "string", "description": "New text to insert"},
					},
					"required": []string{"path", "old_string", "new_string"},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input editfile.Input
				if err := BindArgs(args, &input); err != nil {
					return nil, err
				}
				act := deps.EditFile.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "edit_file", act, &input)
			},
		})
	}

	// run_shell
	if deps.RunCommand != nil {
		handler := func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
			var input runcommand.Input
			if err := BindArgs(args, &input); err != nil {
				return nil, err
			}
			act := deps.RunCommand.NewActivity()
			return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "run_shell", act, &input)
		}

		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "run_shell",
				Description: "Execute a shell command.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{"type": "string", "description": "Command to run (e.g. 'go')"},
						"args":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Arguments for the command"},
					},
					"required": []string{"command"},
				},
			},
			Handler: handler,
		})
	}

	// grep_files
	// Dedicated text search tool (rg preferred, grep fallback).
	// This is intentionally narrower than run_shell.
	if deps.RunCommand != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "grep_files",
				Description: "Keyword/regex search using rg (preferred) or grep (fallback). Prefer this ONLY after search_code returns nothing useful, or when you need exact string/identifier matches.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pattern": map[string]any{"type": "string", "description": "Pattern to search for"},
						"path":    map[string]any{"type": "string", "description": "Optional path to search within (relative to workspace root). Defaults to '.'"},
						"fixed":   map[string]any{"type": "boolean", "description": "If true, treat pattern as a literal string (no regex)."},
						"glob":    map[string]any{"type": "string", "description": "Optional file glob filter (e.g. '*.go')."},
						"max":     map[string]any{"type": "integer", "description": "Optional max matches to return."},
					},
					"required": []string{"pattern"},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				pattern, _ := args["pattern"].(string)
				pattern = strings.TrimSpace(pattern)
				if pattern == "" {
					return nil, fmt.Errorf("pattern is required")
				}

				path, _ := args["path"].(string)
				path = strings.TrimSpace(path)
				if path == "" {
					path = "."
				}

				fixed, _ := args["fixed"].(bool)
				glob, _ := args["glob"].(string)
				glob = strings.TrimSpace(glob)

				max := 0
				if v, ok := args["max"].(float64); ok {
					max = int(v)
				} else if v, ok := args["max"].(int); ok {
					max = v
				}
				if max < 0 {
					max = 0
				}

				bin := "grep"
				if _, err := exec.LookPath("rg"); err == nil {
					bin = "rg"
				}

				cmdArgs := make([]string, 0, 10)
				switch bin {
				case "rg":
					cmdArgs = append(cmdArgs, "--no-heading", "--line-number")
					if fixed {
						cmdArgs = append(cmdArgs, "-F")
					}
					if glob != "" {
						cmdArgs = append(cmdArgs, "-g", glob)
					}
					if max > 0 {
						cmdArgs = append(cmdArgs, "-m", fmt.Sprintf("%d", max))
					}
					cmdArgs = append(cmdArgs, pattern, path)
				default:
					// grep -R can still recurse, so we keep it non-recursive by default
					// and rely on the caller to pass a file/glob/path if desired.
					// Use -n for line numbers; -F for fixed strings.
					cmdArgs = append(cmdArgs, "-n")
					if fixed {
						cmdArgs = append(cmdArgs, "-F")
					}
					// Best-effort: if glob is set, ask the shell-less grep to match files via --include.
					if glob != "" {
						cmdArgs = append(cmdArgs, "--include", glob)
						cmdArgs = append(cmdArgs, "-R")
					}
					if max > 0 {
						cmdArgs = append(cmdArgs, "-m", fmt.Sprintf("%d", max))
					}
					cmdArgs = append(cmdArgs, pattern, path)
				}

				input := &runcommand.Input{Command: bin, Args: cmdArgs}
				act := deps.RunCommand.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "grep_files", act, input)
			},
		})
	}

	// list_files
	if deps.ListFiles != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "list_files",
				Description: "List direct child files and directories in a directory (non-recursive, respects gitignore).",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"dir": map[string]any{
							"type":        "string",
							"description": "Directory path relative to workspace root (e.g. '.', 'internal', 'cmd'). Non-recursive listing.",
						},
					},
					"required": []string{"dir"},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input listfiles.Input
				if err := BindArgs(args, &input); err != nil {
					return nil, err
				}
				act := deps.ListFiles.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "list_files", act, &input)
			},
		})
	}

	// search_code
	if deps.SearchCode != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "search_code",
				Description: "Semantic search over indexed code/context. Prefer this FIRST for discovery; if it returns no relevant results (or results are too weak/irrelevant), fall back to grep_files.",
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
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "search_code", act, input)
			},
		})
	}

	// get_diff
	if deps.GetDiff != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "get_diff",
				Description: "Get the git diff of changes.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"staged": map[string]any{"type": "boolean", "description": "If true, show staged changes. Else show workdir changes."},
					},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input getdiff.Input
				if err := BindArgs(args, &input); err != nil {
					return nil, err
				}
				act := deps.GetDiff.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "get_diff", act, &input)
			},
		})
	}

	// memorize_fact
	if deps.Memorize != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "memorize_fact",
				Description: "Save a fact to the flow memory.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"fact": map[string]any{"type": "string", "description": "The information to remember"},
					},
					"required": []string{"fact"},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input memorize.Input
				if err := BindArgs(args, &input); err != nil {
					return nil, err
				}
				act := deps.Memorize.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "memorize_fact", act, &input)
			},
		})
	}

	// create_plan
	if deps.Plan != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "create_plan",
				Description: "Initialize the task plan.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
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
					"required": []string{"tasks"},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input plan.Input
				if err := BindArgs(args, &input); err != nil {
					return nil, err
				}
				act := deps.Plan.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "create_plan", act, &input)
			},
		})
	}

	// get_plan
	if deps.GetPlan != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "get_plan",
				Description: "Get the current task plan (tasks + statuses).",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input getplan.Input
				act := deps.GetPlan.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "get_plan", act, &input)
			},
		})
	}

	// update_task
	if deps.TrackTask != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "update_task",
				Description: "Update the status of a task.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":     map[string]any{"type": "string"},
						"status": map[string]any{"type": "string", "enum": []string{"pending", "done", "failed", "skipped"}},
					},
					"required": []string{"id", "status"},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input tracktask.Input
				if err := BindArgs(args, &input); err != nil {
					return nil, err
				}
				act := deps.TrackTask.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "update_task", act, &input)
			},
		})
	}

	// summarize
	if deps.Summarize != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "summarize",
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
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "summarize", act, &input)
			},
		})
	}

	// restart_with_instruction
	if deps.Restart != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "restart_with_instruction",
				Description: "Restart the entire flow using a new, self-contained goal prompt. Use this when the current attempt is unrecoverable. The provided instruction becomes the next attempt's full prompt, so include all necessary context, constraints, and acceptance criteria. Memory facts are preserved, but the plan/task board is reset.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"instruction": map[string]any{"type": "string", "description": "A full replacement goal prompt for the next flow attempt. Be specific and detailed: (1) brief diagnosis of what failed and why, (2) explicit constraints/requirements, (3) concrete steps or guidance, (4) clear success criteria. Do NOT assume the next attempt has access to the prior conversation beyond preserved memory facts."},
					},
					"required": []string{"instruction"},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input control.RestartInput
				if err := BindArgs(args, &input); err != nil {
					return nil, err
				}
				act := deps.Restart.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "restart_with_instruction", act, &input)
			},
		})
	}
}
