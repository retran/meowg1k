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

// Package deduplicateandprepare implements an activity that deduplicates files and prepares them for processing.
package deduplicateandprepare

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
	"github.com/retran/meowg1k/internal/core/index"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	WorkspaceState *scanworkspacestate.Output
}

type Output struct {
	// ExistingVersions maps content hash to existing version IDs for files that are already indexed
	ExistingVersions map[string]int64

	// FilesToProcess contains files that need to be chunked, embedded, and saved
	// Maps a synthetic file path (first encountered) to file state (only unique files not in DB)
	FilesToProcess map[string]domainindex.FileState

	// ContentHashToVersionID maps content hash to version ID for all files (used in finalization)
	// Will be populated with both existing and new versions
	ContentHashMap map[string]string // filePath -> contentHash (for all files in all states)
}

type Factory struct {
	indexService ports.IndexService
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory(indexService ports.IndexService) (executor.ActivityFactory[*Input, *Output], error) {
	if indexService == nil {
		return nil, fmt.Errorf("deduplicateandprepare.NewFactory: indexService cannot be nil")
	}

	return &Factory{
		indexService: indexService,
	}, nil
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning("Deduplicating files")

		result, err := f.indexService.PrepareForProcessing(ctx, input.WorkspaceState)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare for processing: %w", err)
		}

		// Type assert the result to the expected type
		prepareResult, ok := result.(*index.PrepareOutput)
		if !ok {
			return nil, fmt.Errorf("unexpected result type from PrepareForProcessing")
		}

		executorCtx.SendCompleted(fmt.Sprintf("Prepared %d files (%d cached)",
			len(prepareResult.FilesToProcess), len(prepareResult.ExistingVersions)))

		return &Output{
			ExistingVersions: prepareResult.ExistingVersions,
			FilesToProcess:   prepareResult.FilesToProcess,
			ContentHashMap:   prepareResult.ContentHashMap,
		}, nil
	}
}
