// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package savechunks implements an activity that distributes embeddings to chunks and saves them to storage.
package savechunks

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/embedall"
	"github.com/retran/meowg1k/internal/activities/savefileversion"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the payload for distributing embeddings and saving documents.
type Input struct {
	EmbeddingResults *embedall.Output
	StateName        string
}

// Output contains the saved document version map for a snapshot.
type Output struct {
	VersionMap map[string]int64
	StateName  string
}

// Factory builds savechunks activities.
type Factory struct {
	saveDocumentVersionFactory executor.ActivityFactory[*savefileversion.Input, *savefileversion.Output]
	indexRepo                  ports.IndexRepository
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a savechunks activity factory.
func NewFactory(
	saveDocumentVersionFactory executor.ActivityFactory[*savefileversion.Input, *savefileversion.Output],
	indexRepo ports.IndexRepository,
) (executor.ActivityFactory[*Input, *Output], error) {
	if saveDocumentVersionFactory == nil {
		return nil, fmt.Errorf("savechunks.NewFactory: saveDocumentVersionFactory cannot be nil")
	}
	if indexRepo == nil {
		return nil, fmt.Errorf("savechunks.NewFactory: indexRepo cannot be nil")
	}

	return &Factory{
		saveDocumentVersionFactory: saveDocumentVersionFactory,
		indexRepo:                  indexRepo,
	}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		chunkResults := input.EmbeddingResults.PreparedBatches.ChunkResults
		executorCtx.SendRunningWithDetails(
			"I'm saving documents",
			fmt.Sprintf("count=%d state=%s", len(chunkResults.FileChunks), input.StateName),
		)

		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		fileEmbeddings := make([][]gateway.Embedding, len(chunkResults.FileChunks))
		for i := range fileEmbeddings {
			fileEmbeddings[i] = []gateway.Embedding{}
		}

		for chunkIdx, fileIdx := range chunkResults.ChunkToFileIndex {
			fileEmbeddings[fileIdx] = append(fileEmbeddings[fileIdx], input.EmbeddingResults.Embeddings[chunkIdx])
		}

		versionMap := make(map[string]int64)
		for fileIdx, fileResult := range chunkResults.FileChunks {
			fr := fileResult
			idx := fileIdx

			saveActivity := f.saveDocumentVersionFactory.NewActivity()
			saveInput := &savefileversion.Input{
				FilePath:    fr.FilePath,
				Content:     fr.Content,
				ContentHash: fr.ContentHash,
				Chunks:      fr.Chunks,
				Embeddings:  fileEmbeddings[idx],
			}
			result, err := executor.ExecuteActivity(ctx, exec, executorCtx, fmt.Sprintf("Save_%s", fr.FilePath), saveActivity, saveInput)
			if err != nil {
				return nil, fmt.Errorf("failed to save document with content hash %s: %w", fr.ContentHash, err)
			}
			versionMap[fr.ContentHash] = result.VersionID
		}

		if err := f.indexRepo.Checkpoint(ctx); err != nil {
			executorCtx.SendProgressWithDetails(
				"I'm continuing after a database checkpoint warning",
				fmt.Sprintf("warning=%v", err),
			)
		}

		executorCtx.SendCompletedWithDetails(
			fmt.Sprintf("I've saved %d document(s)", len(versionMap)),
			fmt.Sprintf("state=%s", input.StateName),
		)
		return &Output{
			StateName:  input.StateName,
			VersionMap: versionMap,
		}, nil
	}
}
