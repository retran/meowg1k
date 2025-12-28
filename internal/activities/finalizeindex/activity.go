// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package finalizeindex implements an activity that finalizes snapshot states in the database.
package finalizeindex

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/scanworktree"
	"github.com/retran/meowg1k/internal/core/index"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the payload for finalizing snapshots.
type Input struct {
	ScanResult       *scanworktree.Output
	ExistingVersions map[string]int64 // contentHash -> version_id (from deduplication)
	NewVersions      map[string]int64 // contentHash -> version_id (from pipeline)
}

// Factory builds finalizeindex activities.
type Factory struct {
	indexService *index.Service
}

var _ executor.ActivityFactory[*Input, struct{}] = (*Factory)(nil)

// NewFactory creates a finalizeindex activity factory.
func NewFactory(indexService *index.Service) (executor.ActivityFactory[*Input, struct{}], error) {
	if indexService == nil {
		return nil, fmt.Errorf("finalizeindex.NewFactory: indexService cannot be nil")
	}

	return &Factory{
		indexService: indexService,
	}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[*Input, struct{}] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (struct{}, error) {
		executorCtx.SendRunningWithDetails(
			"I'm finalizing index snapshots",
			fmt.Sprintf("existing=%d new=%d", len(input.ExistingVersions), len(input.NewVersions)),
		)

		serviceInput := &index.FinalizeInput{
			ScanResult:       input.ScanResult,
			ExistingVersions: input.ExistingVersions,
			NewVersions:      input.NewVersions,
		}

		if err := f.indexService.FinalizeLiveSnapshots(ctx, serviceInput); err != nil {
			return struct{}{}, fmt.Errorf("failed to finalize snapshots: %w", err)
		}

		executorCtx.SendCompletedWithDetails(
			"I've finalized index snapshots",
			fmt.Sprintf("existing=%d new=%d", len(input.ExistingVersions), len(input.NewVersions)),
		)
		return struct{}{}, nil
	}
}
