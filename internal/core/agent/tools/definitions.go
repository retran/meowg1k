// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/control"
	"github.com/retran/meowg1k/internal/activities/editfile"
	"github.com/retran/meowg1k/internal/activities/getdiff"
	"github.com/retran/meowg1k/internal/activities/listfiles"
	"github.com/retran/meowg1k/internal/activities/memorize"
	"github.com/retran/meowg1k/internal/activities/plan"
	"github.com/retran/meowg1k/internal/activities/readfile"
	"github.com/retran/meowg1k/internal/activities/recall"
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
	Recall     executor.ActivityFactory[*recall.Input, *recall.Output]
	Plan       executor.ActivityFactory[*plan.Input, *plan.Output]
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

	// list_files
	if deps.ListFiles != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "list_files",
				Description: "List all files in the workspace (respects gitignore).",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input listfiles.Input
				// BindArgs is safe for empty input
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
				Description: "Semantically search the codebase.",
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

	// recall_facts
	if deps.Recall != nil {
		r.Register(Tool{
			Definition: gateway.ToolDefinition{
				Name:        "recall_facts",
				Description: "Search flow memory for facts.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
					"required": []string{"query"},
				},
			},
			Handler: func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
				var input recall.Input
				if err := BindArgs(args, &input); err != nil {
					return nil, err
				}
				act := deps.Recall.NewActivity()
				return executor.ExecuteActivity(ctx, execCtx.GetExecutor(), execCtx, "recall_facts", act, &input)
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
				Description: "Trigger a restart of the entire flow with a new instruction/constraint.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"instruction": map[string]any{"type": "string", "description": "The new instruction describing the failure and what to fix"},
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
