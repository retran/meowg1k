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

// Package distributeandsave implements an activity that distributes embeddings to chunks and saves them to storage.
package distributeandsave

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/computeallembeddings"
	"github.com/retran/meowg1k/internal/activities/savedocumentversion"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
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
		executorCtx.SendRunning(fmt.Sprintf("Saving %d documents (%s)", len(chunkResults.FileChunks), input.StateName))

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

		futures := make(map[string]*future.Future[*savedocumentversion.Output])
		for fileIdx, fileResult := range chunkResults.FileChunks {
			fr := fileResult
			idx := fileIdx

			saveActivity := f.saveDocumentVersionFactory.NewActivity()
			saveInput := &savedocumentversion.Input{
				FilePath:    fr.FilePath,
				Content:     fr.Content,
				ContentHash: fr.ContentHash,
				Chunks:      fr.Chunks,
				Embeddings:  fileEmbeddings[idx],
			}
			fut := executor.ExecuteActivity(exec, ctx, executorCtx, fmt.Sprintf("Save_%s", fr.FilePath), saveActivity, saveInput)
			futures[fr.ContentHash] = fut
		}

		versionMap := make(map[string]int64)
		for contentHash, fut := range futures {
			result, err := fut.Get(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to save document with content hash %s: %w", contentHash, err)
			}
			versionMap[contentHash] = result.VersionID
		}

		if err := f.indexRepo.Checkpoint(ctx); err != nil {
			executorCtx.SendRunning(fmt.Sprintf("Warning: WAL checkpoint failed: %v", err))
		}

		executorCtx.SendCompleted(fmt.Sprintf("Saved %d documents (%s)", len(versionMap), input.StateName))
		return &Output{
			StateName:  input.StateName,
			VersionMap: versionMap,
		}, nil
	}
}
