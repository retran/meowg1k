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

// Package finalizesnapshots provides an activity to finalize snapshots by linking document versions.
package finalizesnapshots

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input contains the data needed to finalize snapshots.
type Input struct {
	ScanResult       *scanworkspacestate.Output
	ExistingVersions map[string]int64 // contentHash -> version_id (from deduplication)
	NewVersions      map[string]int64 // contentHash -> version_id (from pipeline)
}

// Factory creates instances of the FinalizeSnapshots activity with injected dependencies.
type Factory struct {
	snapshotRepo ports.SnapshotRepository
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, struct{}] = (*Factory)(nil)

// NewFactory creates a new FinalizeSnapshots activity factory.
func NewFactory(snapshotRepo ports.SnapshotRepository) (executor.ActivityFactory[*Input, struct{}], error) {
	if snapshotRepo == nil {
		return nil, fmt.Errorf("finalizesnapshots.NewFactory: snapshotRepo cannot be nil")
	}

	return &Factory{
		snapshotRepo: snapshotRepo,
	}, nil
}

// NewActivity creates and returns the FinalizeSnapshots activity function.
func (f *Factory) NewActivity() executor.Activity[*Input, struct{}] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (struct{}, error) {
		executorCtx.SendRunning("Finalizing snapshots...")

		// Step 1: Merge ExistingVersions and NewVersions into a complete version map
		allVersions := make(map[string]int64) // contentHash -> version_id
		for contentHash, versionID := range input.ExistingVersions {
			allVersions[contentHash] = versionID
		}
		for contentHash, versionID := range input.NewVersions {
			allVersions[contentHash] = versionID
		}

		executorCtx.SendRunning(fmt.Sprintf("Total versions available: %d", len(allVersions)))

		// Step 2: Build version maps for each snapshot based on workspace state
		headVersions := make([]int64, 0, len(input.ScanResult.HeadState))
		stageVersions := make([]int64, 0, len(input.ScanResult.StageState))
		workdirVersions := make([]int64, 0, len(input.ScanResult.WorkdirState))

		// Map HEAD state files to version IDs
		for _, fileState := range input.ScanResult.HeadState {
			if versionID, exists := allVersions[fileState.ContentHash]; exists {
				headVersions = append(headVersions, versionID)
			} else {
				return struct{}{}, fmt.Errorf("no version found for content hash %s in HEAD state", fileState.ContentHash)
			}
		}

		// Map Stage state files to version IDs
		for _, fileState := range input.ScanResult.StageState {
			if versionID, exists := allVersions[fileState.ContentHash]; exists {
				stageVersions = append(stageVersions, versionID)
			} else {
				return struct{}{}, fmt.Errorf("no version found for content hash %s in Stage state", fileState.ContentHash)
			}
		}

		// Map Workdir state files to version IDs
		for _, fileState := range input.ScanResult.WorkdirState {
			if versionID, exists := allVersions[fileState.ContentHash]; exists {
				workdirVersions = append(workdirVersions, versionID)
			} else {
				return struct{}{}, fmt.Errorf("no version found for content hash %s in Workdir state", fileState.ContentHash)
			}
		}

		// Step 3: Clear existing snapshot links
		executorCtx.SendRunning("Clearing existing snapshot links...")
		if err := f.snapshotRepo.ClearSnapshotLinks(ctx, "_head_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to clear _head_ snapshot links: %w", err)
		}
		if err := f.snapshotRepo.ClearSnapshotLinks(ctx, "_stage_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to clear _stage_ snapshot links: %w", err)
		}
		if err := f.snapshotRepo.ClearSnapshotLinks(ctx, "_workdir_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to clear _workdir_ snapshot links: %w", err)
		}

		// Step 4: Link versions to snapshots
		executorCtx.SendRunning(fmt.Sprintf("Linking %d versions to _head_ snapshot...", len(headVersions)))
		for _, versionID := range headVersions {
			if err := f.snapshotRepo.LinkVersionToSnapshot(ctx, "_head_", versionID); err != nil {
				return struct{}{}, fmt.Errorf("failed to link version %d to _head_ snapshot: %w", versionID, err)
			}
		}

		executorCtx.SendRunning(fmt.Sprintf("Linking %d versions to _stage_ snapshot...", len(stageVersions)))
		for _, versionID := range stageVersions {
			if err := f.snapshotRepo.LinkVersionToSnapshot(ctx, "_stage_", versionID); err != nil {
				return struct{}{}, fmt.Errorf("failed to link version %d to _stage_ snapshot: %w", versionID, err)
			}
		}

		executorCtx.SendRunning(fmt.Sprintf("Linking %d versions to _workdir_ snapshot...", len(workdirVersions)))
		for _, versionID := range workdirVersions {
			if err := f.snapshotRepo.LinkVersionToSnapshot(ctx, "_workdir_", versionID); err != nil {
				return struct{}{}, fmt.Errorf("failed to link version %d to _workdir_ snapshot: %w", versionID, err)
			}
		}

		executorCtx.SendCompleted("Snapshots finalized successfully")
		return struct{}{}, nil
	}
}
