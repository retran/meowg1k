/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package buildsqlsnapshots provides an activity to build SQL snapshots.
package buildsqlsnapshots

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/ensureversionsexist"
	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input contains the data needed to build SQL snapshots.
type Input struct {
	WorkspaceState *scanworkspacestate.Output
	Versions       *ensureversionsexist.Output
}

// Factory creates instances of the BuildSqlSnapshots activity with injected dependencies.
type Factory struct {
	indexSvc ports.IndexService
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, struct{}] = (*Factory)(nil)

// NewFactory creates a new BuildSqlSnapshots activity factory.
func NewFactory(indexSvc ports.IndexService) (executor.ActivityFactory[*Input, struct{}], error) {
	if indexSvc == nil {
		return nil, fmt.Errorf("buildsqlsnapshots.NewFactory: indexSvc cannot be nil")
	}

	return &Factory{
		indexSvc: indexSvc,
	}, nil
}

// NewActivity creates and returns the BuildSqlSnapshots activity function.
func (f *Factory) NewActivity() executor.Activity[*Input, struct{}] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (struct{}, error) {
		executorCtx.SendRunning("Building SQL snapshots...")

		// Helper function to prepare version map for a specific state
		prepareVersionMap := func(state map[string]domainindex.FileState) map[string]int64 {
			result := make(map[string]int64, len(state))
			for path := range state {
				if versionID, exists := input.Versions.VersionMap[path]; exists {
					result[path] = versionID
				}
			}
			return result
		}

		// Build snapshot for HEAD
		headVersions := prepareVersionMap(input.WorkspaceState.HeadState)
		if err := f.indexSvc.BuildSnapshot("_head_", headVersions); err != nil {
			return struct{}{}, fmt.Errorf("failed to build _head_ snapshot: %w", err)
		}

		// Build snapshot for staging area
		stageVersions := prepareVersionMap(input.WorkspaceState.StageState)
		if err := f.indexSvc.BuildSnapshot("_stage_", stageVersions); err != nil {
			return struct{}{}, fmt.Errorf("failed to build _stage_ snapshot: %w", err)
		}

		// Build snapshot for working directory
		workdirVersions := prepareVersionMap(input.WorkspaceState.WorkdirState)
		if err := f.indexSvc.BuildSnapshot("_workdir_", workdirVersions); err != nil {
			return struct{}{}, fmt.Errorf("failed to build _workdir_ snapshot: %w", err)
		}

		executorCtx.SendCompleted("SQL snapshots built")
		return struct{}{}, nil
	}
}
