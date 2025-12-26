// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package cleanupstaledata implements an activity that removes outdated document versions and embeddings from storage.
package cleanupstaledata

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Factory builds cleanupstaledata activities.
type Factory struct {
	snapshotRepo ports.SnapshotRepository
	metaRepo     ports.MetaRepository
}

var _ executor.ActivityFactory[struct{}, struct{}] = (*Factory)(nil)

// NewFactory creates a cleanupstaledata activity factory.
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

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[struct{}, struct{}] {
	return func(ctx context.Context, executorCtx *executor.Context, _ struct{}) (struct{}, error) {
		executorCtx.SendRunning("Cleaning stale data")

		if err := f.snapshotRepo.ClearSnapshotLinks(ctx, "_head_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to clear _head_ snapshot links: %w", err)
		}

		if err := f.snapshotRepo.ClearSnapshotLinks(ctx, "_stage_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to clear _stage_ snapshot links: %w", err)
		}

		if err := f.snapshotRepo.ClearSnapshotLinks(ctx, "_workdir_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to clear _workdir_ snapshot links: %w", err)
		}

		if err := f.metaRepo.DeleteValue(ctx, "idx_dump_head"); err != nil {
			return struct{}{}, fmt.Errorf("failed to delete idx_dump_head: %w", err)
		}

		if err := f.metaRepo.DeleteValue(ctx, "idx_dump_stage"); err != nil {
			return struct{}{}, fmt.Errorf("failed to delete idx_dump_stage: %w", err)
		}

		if err := f.metaRepo.DeleteValue(ctx, "idx_dump_workdir"); err != nil {
			return struct{}{}, fmt.Errorf("failed to delete idx_dump_workdir: %w", err)
		}

		executorCtx.SendCompleted("Cleaned stale data")
		return struct{}{}, nil
	}
}
