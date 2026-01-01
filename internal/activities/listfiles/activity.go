// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package listfiles

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	Dir string `json:"dir" mapstructure:"dir"`
}

type Output struct {
	Files []string
}

type Factory struct {
	projectStateSvc ports.ProjectStateService
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory(projectStateSvc ports.ProjectStateService) *Factory {
	return &Factory{projectStateSvc: projectStateSvc}
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		dir := strings.TrimSpace(input.Dir)
		if dir == "" {
			dir = "."
		}
		for strings.HasPrefix(dir, "./") {
			dir = strings.TrimPrefix(dir, "./")
		}
		dir = strings.TrimSuffix(dir, "/")
		if dir == "" {
			dir = "."
		}

		label := dir
		if label == "." {
			label = "(root)"
		}

		flowCtx.SendRunning(fmt.Sprintf("Listing files in %s", label))

		state, err := f.projectStateSvc.GetWorkdirState(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get workdir state: %w", err)
		}

		entriesSet := make(map[string]struct{})
		for path := range state {
			if dir == "." {
				// Direct children of the workspace root: files and directories.
				if strings.Contains(path, "/") {
					first := strings.SplitN(path, "/", 2)[0]
					if first == "" {
						continue
					}
					entriesSet[first+"/"] = struct{}{}
					continue
				}
				entriesSet[path] = struct{}{}
				continue
			}

			prefix := dir + "/"
			if !strings.HasPrefix(path, prefix) {
				continue
			}
			rel := strings.TrimPrefix(path, prefix)
			if rel == "" {
				continue
			}
			// Non-recursive: include direct files and direct child directories.
			if strings.Contains(rel, "/") {
				first := strings.SplitN(rel, "/", 2)[0]
				if first == "" {
					continue
				}
				entriesSet[prefix+first+"/"] = struct{}{}
				continue
			}
			entriesSet[path] = struct{}{}
		}

		entries := make([]string, 0, len(entriesSet))
		for e := range entriesSet {
			entries = append(entries, e)
		}
		sort.Strings(entries)

		flowCtx.SendCompleted(fmt.Sprintf("Listed %d entries in %s", len(entries), label))

		return &Output{Files: entries}, nil
	}
}
