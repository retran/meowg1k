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

// Package buildvectorindices implements a parent activity that builds vector indices for multiple snapshots in parallel.
package buildvectorindices

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/buildsinglevectorindex"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Factory struct {
	vectorIndexSvc ports.VectorIndexService
}

var _ executor.ActivityFactory[struct{}, struct{}] = (*Factory)(nil)

func NewFactory(vectorIndexSvc ports.VectorIndexService) (executor.ActivityFactory[struct{}, struct{}], error) {
	if vectorIndexSvc == nil {
		return nil, fmt.Errorf("buildvectorindices.NewFactory: vectorIndexSvc cannot be nil")
	}

	return &Factory{
		vectorIndexSvc: vectorIndexSvc,
	}, nil
}

func (f *Factory) NewActivity() executor.Activity[struct{}, struct{}] {
	return func(ctx context.Context, executorCtx *executor.Context, _ struct{}) (struct{}, error) {
		executorCtx.SendRunning("Building indices")

		exec := executorCtx.GetExecutor()
		if exec == nil {
			return struct{}{}, fmt.Errorf("executor not available in activity context")
		}

		childFactory, err := buildsinglevectorindex.NewFactory(f.vectorIndexSvc)
		if err != nil {
			return struct{}{}, fmt.Errorf("failed to create child factory: %w", err)
		}

		headFuture := executor.ExecuteActivity(exec, ctx, executorCtx, "build-vector-index-head", childFactory.NewActivity(), "_head_")
		stageFuture := executor.ExecuteActivity(exec, ctx, executorCtx, "build-vector-index-stage", childFactory.NewActivity(), "_stage_")
		workdirFuture := executor.ExecuteActivity(exec, ctx, executorCtx, "build-vector-index-workdir", childFactory.NewActivity(), "_workdir_")

		if _, err := headFuture.Get(ctx); err != nil {
			return struct{}{}, fmt.Errorf("failed to build _head_ index: %w", err)
		}

		if _, err := stageFuture.Get(ctx); err != nil {
			return struct{}{}, fmt.Errorf("failed to build _stage_ index: %w", err)
		}

		if _, err := workdirFuture.Get(ctx); err != nil {
			return struct{}{}, fmt.Errorf("failed to build _workdir_ index: %w", err)
		}

		executorCtx.SendCompleted("Indices ready")
		return struct{}{}, nil
	}
}
