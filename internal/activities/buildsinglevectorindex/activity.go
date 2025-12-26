// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package buildsinglevectorindex implements an activity that builds and saves a vector index for a single snapshot.
package buildsinglevectorindex

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Factory builds activities for vector index creation.
type Factory struct {
	vectorIndexSvc ports.VectorIndexService
}

var _ executor.ActivityFactory[string, struct{}] = (*Factory)(nil)

// NewFactory creates a buildsinglevectorindex activity factory.
func NewFactory(vectorIndexSvc ports.VectorIndexService) (executor.ActivityFactory[string, struct{}], error) {
	if vectorIndexSvc == nil {
		return nil, fmt.Errorf("buildsinglevectorindex.NewFactory: vectorIndexSvc cannot be nil")
	}

	return &Factory{
		vectorIndexSvc: vectorIndexSvc,
	}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[string, struct{}] {
	return func(_ context.Context, executorCtx *executor.Context, snapshotName string) (struct{}, error) {
		executorCtx.SendRunning(fmt.Sprintf("Building index for %s", snapshotName))

		if err := f.vectorIndexSvc.BuildAndSave(snapshotName); err != nil {
			return struct{}{}, fmt.Errorf("failed to build vector index for %s: %w", snapshotName, err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("Built index for %s", snapshotName))
		return struct{}{}, nil
	}
}
