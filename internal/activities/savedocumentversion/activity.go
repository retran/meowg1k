// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package savedocumentversion implements an activity that saves a document version with its chunks to storage.
package savedocumentversion

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/index"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the payload for saving a document version.
type Input struct {
	FilePath    string
	Content     []byte
	ContentHash string
	Chunks      []domainindex.ChunkData
	Embeddings  []gateway.Embedding
}

// Output contains the saved version metadata.
type Output struct {
	FilePath  string
	VersionID int64
}

// Factory builds savedocumentversion activities.
type Factory struct {
	indexService ports.IndexService
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a savedocumentversion activity factory.
func NewFactory(indexService ports.IndexService) (executor.ActivityFactory[*Input, *Output], error) {
	if indexService == nil {
		return nil, fmt.Errorf("savedocumentversion.NewFactory: indexService cannot be nil")
	}

	return &Factory{
		indexService: indexService,
	}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning(fmt.Sprintf("I'm saving %s", input.FilePath))

		serviceInput := &index.SaveVersionInput{
			FilePath:    input.FilePath,
			Content:     input.Content,
			ContentHash: input.ContentHash,
			Chunks:      input.Chunks,
			Embeddings:  input.Embeddings,
		}

		result, err := f.indexService.SaveNewVersion(ctx, serviceInput)
		if err != nil {
			return nil, fmt.Errorf("failed to save document version: %w", err)
		}

		// Type assert the result to the expected type
		saveResult, ok := result.(*index.SaveVersionOutput)
		if !ok {
			return nil, fmt.Errorf("unexpected result type from SaveNewVersion")
		}

		executorCtx.SendCompletedWithDetails(
			fmt.Sprintf("I saved %s", input.FilePath),
			fmt.Sprintf("version %d\nchunks %d", saveResult.VersionID, len(input.Chunks)),
		)
		return &Output{
			FilePath:  saveResult.FilePath,
			VersionID: saveResult.VersionID,
		}, nil
	}
}
