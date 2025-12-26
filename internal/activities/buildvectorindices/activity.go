// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package buildvectorindices implements a parent activity that builds vector indices for multiple snapshots sequentially.
package buildvectorindices

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/buildsinglevectorindex"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Factory builds buildvectorindices activities.
type Factory struct {
	vectorIndexSvc ports.VectorIndexService
}

var _ executor.ActivityFactory[struct{}, struct{}] = (*Factory)(nil)

// NewFactory creates a buildvectorindices activity factory.
func NewFactory(vectorIndexSvc ports.VectorIndexService) (executor.ActivityFactory[struct{}, struct{}], error) {
	if vectorIndexSvc == nil {
		return nil, fmt.Errorf("buildvectorindices.NewFactory: vectorIndexSvc cannot be nil")
	}

	return &Factory{
		vectorIndexSvc: vectorIndexSvc,
	}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[struct{}, struct{}] {
	return func(ctx context.Context, executorCtx *executor.Context, _ struct{}) (struct{}, error) {
		executorCtx.SendRunning("Building vector indices")

		exec := executorCtx.GetExecutor()
		if exec == nil {
			return struct{}{}, fmt.Errorf("executor not available in activity context")
		}

		childFactory, err := buildsinglevectorindex.NewFactory(f.vectorIndexSvc)
		if err != nil {
			return struct{}{}, fmt.Errorf("failed to create child factory: %w", err)
		}

		if _, err := executor.ExecuteActivity(ctx, exec, executorCtx, "build-vector-index-head", childFactory.NewActivity(), "_head_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to build _head_ index: %w", err)
		}

		if _, err := executor.ExecuteActivity(ctx, exec, executorCtx, "build-vector-index-stage", childFactory.NewActivity(), "_stage_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to build _stage_ index: %w", err)
		}

		if _, err := executor.ExecuteActivity(ctx, exec, executorCtx, "build-vector-index-workdir", childFactory.NewActivity(), "_workdir_"); err != nil {
			return struct{}{}, fmt.Errorf("failed to build _workdir_ index: %w", err)
		}

		executorCtx.SendCompleted("Vector indices ready")
		return struct{}{}, nil
	}
}
