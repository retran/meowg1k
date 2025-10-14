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

// Package cleanupstaledata provides an activity to clean up stale snapshot data.
package cleanupstaledata

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Factory struct {
	snapshotRepo ports.SnapshotRepository
	metaRepo     ports.MetaRepository
}

var _ executor.ActivityFactory[struct{}, struct{}] = (*Factory)(nil)

func NewFactory(snapshotRepo ports.SnapshotRepository, metaRepo ports.MetaRepository) (executor.ActivityFactory[struct{}, struct{}], error) {
	if snapshotRepo == nil {
		return nil, fmt.Errorf("cleanupstaledata.NewFactory: snapshotRepo cannot be nil")
	}
	if metaRepo == nil {
		return nil, fmt.Errorf("cleanupstaledata.NewFactory: metaRepo cannot be nil")
	}

	return &Factory{
		snapshotRepo: snapshotRepo,
		metaRepo:     metaRepo,
	}, nil
}

func (f *Factory) NewActivity() executor.Activity[struct{}, struct{}] {
	return func(ctx context.Context, executorCtx *executor.Context, _ struct{}) (struct{}, error) {
		executorCtx.SendRunning("Cleaning up stale data...")

		// Clear snapshot links for all live snapshots
		if err := f.snapshotRepo.ClearSnapshotLinks(ctx, "_head_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to clear _head_ snapshot links: %w", err)
		}

		if err := f.snapshotRepo.ClearSnapshotLinks(ctx, "_stage_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to clear _stage_ snapshot links: %w", err)
		}

		if err := f.snapshotRepo.ClearSnapshotLinks(ctx, "_workdir_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to clear _workdir_ snapshot links: %w", err)
		}

		// Delete vector index dumps for all live snapshots
		if err := f.metaRepo.DeleteValue(ctx, "idx_dump_head"); err != nil {
			return struct{}{}, fmt.Errorf("failed to delete idx_dump_head: %w", err)
		}

		if err := f.metaRepo.DeleteValue(ctx, "idx_dump_stage"); err != nil {
			return struct{}{}, fmt.Errorf("failed to delete idx_dump_stage: %w", err)
		}

		if err := f.metaRepo.DeleteValue(ctx, "idx_dump_workdir"); err != nil {
			return struct{}{}, fmt.Errorf("failed to delete idx_dump_workdir: %w", err)
		}

		executorCtx.SendCompleted("Cleanup complete")
		return struct{}{}, nil
	}
}
