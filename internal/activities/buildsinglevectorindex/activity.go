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

// Package buildsinglevectorindex provides an activity to build a single vector index.
package buildsinglevectorindex

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Factory creates instances of the BuildSingleVectorIndex activity with injected dependencies.
type Factory struct {
	vectorIndexSvc ports.VectorIndexService
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[string, struct{}] = (*Factory)(nil)

// NewFactory creates a new BuildSingleVectorIndex activity factory.
func NewFactory(vectorIndexSvc ports.VectorIndexService) (executor.ActivityFactory[string, struct{}], error) {
	if vectorIndexSvc == nil {
		return nil, fmt.Errorf("buildsinglevectorindex.NewFactory: vectorIndexSvc cannot be nil")
	}

	return &Factory{
		vectorIndexSvc: vectorIndexSvc,
	}, nil
}

// NewActivity creates and returns the BuildSingleVectorIndex activity function.
func (f *Factory) NewActivity() executor.Activity[string, struct{}] {
	return func(ctx context.Context, executorCtx *executor.Context, snapshotName string) (struct{}, error) {
		executorCtx.SendRunning(fmt.Sprintf("Building index for %s...", snapshotName))

		if err := f.vectorIndexSvc.BuildAndSave(snapshotName); err != nil {
			return struct{}{}, fmt.Errorf("failed to build vector index for %s: %w", snapshotName, err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("Index for %s is ready", snapshotName))
		return struct{}{}, nil
	}
}
