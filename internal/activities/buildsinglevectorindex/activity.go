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

// Package buildsinglevectorindex implements an activity that builds and saves a vector index for a single snapshot.
package buildsinglevectorindex

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Factory struct {
	vectorIndexSvc ports.VectorIndexService
}

var _ executor.ActivityFactory[string, struct{}] = (*Factory)(nil)

func NewFactory(vectorIndexSvc ports.VectorIndexService) (executor.ActivityFactory[string, struct{}], error) {
	if vectorIndexSvc == nil {
		return nil, fmt.Errorf("buildsinglevectorindex.NewFactory: vectorIndexSvc cannot be nil")
	}

	return &Factory{
		vectorIndexSvc: vectorIndexSvc,
	}, nil
}

func (f *Factory) NewActivity() executor.Activity[string, struct{}] {
	return func(ctx context.Context, executorCtx *executor.Context, snapshotName string) (struct{}, error) {
		executorCtx.SendRunning(fmt.Sprintf("Building index: %s", snapshotName))

		if err := f.vectorIndexSvc.BuildAndSave(snapshotName); err != nil {
			return struct{}{}, fmt.Errorf("failed to build vector index for %s: %w", snapshotName, err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("Built index: %s", snapshotName))
		return struct{}{}, nil
	}
}
