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

// Package finalizesnapshots implements an activity that finalizes snapshot states in the database.
package finalizesnapshots

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
	"github.com/retran/meowg1k/internal/core/index"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	ScanResult       *scanworkspacestate.Output
	ExistingVersions map[string]int64 // contentHash -> version_id (from deduplication)
	NewVersions      map[string]int64 // contentHash -> version_id (from pipeline)
}

type Factory struct {
	indexService *index.Service
}

var _ executor.ActivityFactory[*Input, struct{}] = (*Factory)(nil)

func NewFactory(indexService *index.Service) (executor.ActivityFactory[*Input, struct{}], error) {
	if indexService == nil {
		return nil, fmt.Errorf("finalizesnapshots.NewFactory: indexService cannot be nil")
	}

	return &Factory{
		indexService: indexService,
	}, nil
}

func (f *Factory) NewActivity() executor.Activity[*Input, struct{}] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (struct{}, error) {
		executorCtx.SendRunning("Finalizing snapshots")

		serviceInput := &index.FinalizeInput{
			ScanResult:       input.ScanResult,
			ExistingVersions: input.ExistingVersions,
			NewVersions:      input.NewVersions,
		}

		if err := f.indexService.FinalizeLiveSnapshots(ctx, serviceInput); err != nil {
			return struct{}{}, fmt.Errorf("failed to finalize snapshots: %w", err)
		}

		executorCtx.SendCompleted("Finalized snapshots")
		return struct{}{}, nil
	}
}
