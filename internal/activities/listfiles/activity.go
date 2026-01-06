// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package listfiles implements an activity for listing files in a directory.
package listfiles

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input for listing files.
type Input struct {
	Dir string `json:"dir" mapstructure:"dir"`
}

// Output defines the output of the file listing operation.
type Output struct {
	Files []string
}

// Factory builds listfiles activities.
type Factory struct {
	projectStateSvc ports.ProjectStateService
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new listfiles activity factory.
func NewFactory(projectStateSvc ports.ProjectStateService) *Factory {
	return &Factory{projectStateSvc: projectStateSvc}
}

// NewActivity creates the activity.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		dir := normalizeDir(input.Dir)
		label := getDirLabel(dir)

		flowCtx.SendRunning(fmt.Sprintf("Listing files in %s", label))

		state, err := f.projectStateSvc.GetWorkdirState(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get workdir state: %w", err)
		}

		entries := listEntries(state, dir)

		flowCtx.SendCompleted(fmt.Sprintf("Listed %d entries in %s", len(entries), label))

		return &Output{Files: entries}, nil
	}
}

func normalizeDir(dir string) string {
	dir = strings.TrimSpace(dir)
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
	return dir
}

func getDirLabel(dir string) string {
	if dir == "." {
		return "(root)"
	}
	return dir
}

func listEntries[K any](state map[string]K, dir string) []string {
	entriesSet := make(map[string]struct{})
	for path := range state {
		if dir == "." {
			collectRootEntry(entriesSet, path)
		} else {
			collectSubDirEntry(entriesSet, path, dir)
		}
	}

	entries := make([]string, 0, len(entriesSet))
	for e := range entriesSet {
		entries = append(entries, e)
	}
	sort.Strings(entries)
	return entries
}

func collectRootEntry(entriesSet map[string]struct{}, path string) {
	if strings.Contains(path, "/") {
		first := strings.SplitN(path, "/", 2)[0]
		if first != "" {
			entriesSet[first+"/"] = struct{}{}
		}
	} else {
		entriesSet[path] = struct{}{}
	}
}

func collectSubDirEntry(entriesSet map[string]struct{}, path, dir string) {
	prefix := dir + "/"
	if !strings.HasPrefix(path, prefix) {
		return
	}
	rel := strings.TrimPrefix(path, prefix)
	if rel == "" {
		return
	}
	if strings.Contains(rel, "/") {
		first := strings.SplitN(rel, "/", 2)[0]
		if first != "" {
			entriesSet[prefix+first+"/"] = struct{}{}
		}
	} else {
		entriesSet[path] = struct{}{}
	}
}
