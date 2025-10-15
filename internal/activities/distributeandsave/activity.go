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

// Package distributeandsave provides an activity to distribute embeddings and save documents.
package distributeandsave

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/computeallembeddings"
	"github.com/retran/meowg1k/internal/activities/savedocumentversion"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	StateName        string
	EmbeddingResults *computeallembeddings.Output
}

type Output struct {
	StateName  string
	VersionMap map[string]int64 // contentHash -> version_id
}

type Factory struct {
	saveDocumentVersionFactory executor.ActivityFactory[*savedocumentversion.Input, *savedocumentversion.Output]
	indexRepo                  ports.IndexRepository
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory(
	saveDocumentVersionFactory executor.ActivityFactory[*savedocumentversion.Input, *savedocumentversion.Output],
	indexRepo ports.IndexRepository,
) (executor.ActivityFactory[*Input, *Output], error) {
	if saveDocumentVersionFactory == nil {
		return nil, fmt.Errorf("distributeandsave.NewFactory: saveDocumentVersionFactory cannot be nil")
	}
	if indexRepo == nil {
		return nil, fmt.Errorf("distributeandsave.NewFactory: indexRepo cannot be nil")
	}

	return &Factory{
		saveDocumentVersionFactory: saveDocumentVersionFactory,
		indexRepo:                  indexRepo,
	}, nil
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		chunkResults := input.EmbeddingResults.PreparedBatches.ChunkResults
		executorCtx.SendRunning(fmt.Sprintf("Distributing embeddings and saving %d documents (%s)...", len(chunkResults.FileChunks), input.StateName))

		// Get executor from context
		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		// Distribute embeddings back to files
		fileEmbeddings := make([][]gateway.Embedding, len(chunkResults.FileChunks))
		for i := range fileEmbeddings {
			fileEmbeddings[i] = []gateway.Embedding{}
		}

		for chunkIdx, fileIdx := range chunkResults.ChunkToFileIndex {
			fileEmbeddings[fileIdx] = append(fileEmbeddings[fileIdx], input.EmbeddingResults.Embeddings[chunkIdx])
		}

		// Save all documents sequentially to avoid race conditions
		versionMap := make(map[string]int64)
		for fileIdx, fileResult := range chunkResults.FileChunks {
			saveActivity := f.saveDocumentVersionFactory.NewActivity()
			saveInput := &savedocumentversion.Input{
				FilePath:    fileResult.FilePath,
				Content:     fileResult.Content,
				ContentHash: fileResult.ContentHash,
				Chunks:      fileResult.Chunks,
				Embeddings:  fileEmbeddings[fileIdx],
			}
			fut := executor.ExecuteActivity(exec, ctx, executorCtx, fmt.Sprintf("Save_%s", fileResult.FilePath), saveActivity, saveInput)
			result, err := fut.Get(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to save document %s: %w", fileResult.FilePath, err)
			}
			versionMap[fileResult.ContentHash] = result.VersionID
		}

		// Perform WAL checkpoint to ensure all writes are visible to readers
		// This helps avoid race conditions where finalization might not see recently saved versions
		if err := f.indexRepo.Checkpoint(ctx); err != nil {
			// Log but don't fail - checkpoint failure is not critical
			executorCtx.SendRunning(fmt.Sprintf("Warning: WAL checkpoint failed: %v", err))
		}

		executorCtx.SendCompleted(fmt.Sprintf("Saved %d documents for %s", len(versionMap), input.StateName))
		return &Output{
			StateName:  input.StateName,
			VersionMap: versionMap,
		}, nil
	}
}
