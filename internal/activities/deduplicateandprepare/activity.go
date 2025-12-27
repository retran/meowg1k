// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

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

// Input defines the payload for deduplicating workspace files.
type Input struct {
	WorkspaceState *scanworkspacestate.Output
}

// Output contains the deduplicated file metadata and mappings.
type Output struct {
	ExistingVersions map[string]int64
	ContentHashMap   map[string]string
	FilesToProcess   []domainindex.FileToProcess
}

// Factory builds deduplicateandprepare activities.
type Factory struct {
	indexService ports.IndexService
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a deduplicateandprepare activity factory.
func NewFactory(indexService ports.IndexService) (executor.ActivityFactory[*Input, *Output], error) {
	if indexService == nil {
		return nil, fmt.Errorf("deduplicateandprepare.NewFactory: indexService cannot be nil")
	}

	return &Factory{
		indexService: indexService,
	}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning("I'm checking what needs to be indexed")

		result, err := f.indexService.PrepareForProcessing(ctx, input.WorkspaceState)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare for processing: %w", err)
		}

		// Type assert the result to the expected type
		prepareResult, ok := result.(*index.PrepareOutput)
		if !ok {
			return nil, fmt.Errorf("unexpected result type from PrepareForProcessing")
		}

		executorCtx.SendCompleted(fmt.Sprintf("I found %d file(s) to index (%d already cached)",
			len(prepareResult.FilesToProcess), len(prepareResult.ExistingVersions)))

		return &Output{
			ExistingVersions: prepareResult.ExistingVersions,
			FilesToProcess:   prepareResult.FilesToProcess,
			ContentHashMap:   prepareResult.ContentHashMap,
		}, nil
	}
}
