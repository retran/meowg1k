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

// Package ensureversionsexist provides an activity to ensure document versions exist.
package ensureversionsexist

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Output contains the mapping of file paths to document version IDs.
type Output struct {
	VersionMap map[string]int64
}

// Factory creates instances of the EnsureVersionsExist activity with injected dependencies.
type Factory struct {
	indexSvc ports.IndexService
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*scanworkspacestate.Output, *Output] = (*Factory)(nil)

// NewFactory creates a new EnsureVersionsExist activity factory.
func NewFactory(indexSvc ports.IndexService) (executor.ActivityFactory[*scanworkspacestate.Output, *Output], error) {
	if indexSvc == nil {
		return nil, fmt.Errorf("ensureversionsexist.NewFactory: indexSvc cannot be nil")
	}

	return &Factory{
		indexSvc: indexSvc,
	}, nil
}

// NewActivity creates and returns the EnsureVersionsExist activity function.
func (f *Factory) NewActivity() executor.Activity[*scanworkspacestate.Output, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, workspaceState *scanworkspacestate.Output) (*Output, error) {
		executorCtx.SendRunning("Syncing document versions (embeddings)...")

		// Collect all unique files from three states into a single map
		// to avoid processing the same file multiple times
		uniqueFiles := make(map[string][]byte)

		// Helper function to merge files from a state
		mergeFiles := func(state map[string]domainindex.FileState) {
			for path, fileState := range state {
				if _, exists := uniqueFiles[path]; !exists {
					uniqueFiles[path] = fileState.Content
				}
			}
		}

		// Merge files from all three states
		mergeFiles(workspaceState.HeadState)
		mergeFiles(workspaceState.StageState)
		mergeFiles(workspaceState.WorkdirState)

		// Call the index service to ensure versions exist
		// This will compute embeddings for new/changed files
		versionMap, err := f.indexSvc.EnsureVersionsExist(uniqueFiles)
		if err != nil {
			return nil, err
		}

		executorCtx.SendCompleted("Document versions synced")
		return &Output{VersionMap: versionMap}, nil
	}
}
