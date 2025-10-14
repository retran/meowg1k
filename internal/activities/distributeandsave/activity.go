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
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// Input contains embeddings to distribute and save.
type Input struct {
	StateName        string
	EmbeddingResults *computeallembeddings.Output
}

// Output contains the version map.
type Output struct {
	StateName  string
	VersionMap map[string]int64
}

// Factory creates instances of the DistributeAndSave activity with injected dependencies.
type Factory struct {
	saveDocumentVersionFactory executor.ActivityFactory[*savedocumentversion.Input, *savedocumentversion.Output]
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new DistributeAndSave activity factory.
func NewFactory(
	saveDocumentVersionFactory executor.ActivityFactory[*savedocumentversion.Input, *savedocumentversion.Output],
) (executor.ActivityFactory[*Input, *Output], error) {
	if saveDocumentVersionFactory == nil {
		return nil, fmt.Errorf("distributeandsave.NewFactory: saveDocumentVersionFactory cannot be nil")
	}

	return &Factory{
		saveDocumentVersionFactory: saveDocumentVersionFactory,
	}, nil
}

// NewActivity creates and returns the DistributeAndSave activity function.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning(fmt.Sprintf("Distributing embeddings and saving %d documents (%s)...", len(input.EmbeddingResults.ChunkResults.FileChunks), input.StateName))

		// Get executor from context
		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		// Distribute embeddings back to files
		fileEmbeddings := make([][]gateway.Embedding, len(input.EmbeddingResults.ChunkResults.FileChunks))
		for i := range fileEmbeddings {
			fileEmbeddings[i] = []gateway.Embedding{}
		}

		for chunkIdx, fileIdx := range input.EmbeddingResults.ChunkResults.ChunkToFileIndex {
			fileEmbeddings[fileIdx] = append(fileEmbeddings[fileIdx], input.EmbeddingResults.Embeddings[chunkIdx])
		}

		// Save all documents in parallel
		saveFutures := make(map[string]*future.Future[*savedocumentversion.Output])
		for fileIdx, fileResult := range input.EmbeddingResults.ChunkResults.FileChunks {
			saveActivity := f.saveDocumentVersionFactory.NewActivity()
			saveInput := &savedocumentversion.Input{
				FilePath:    fileResult.FilePath,
				Content:     fileResult.Content,
				ContentHash: fileResult.ContentHash,
				Chunks:      fileResult.Chunks,
				Embeddings:  fileEmbeddings[fileIdx],
			}
			fut := executor.ExecuteActivity(exec, ctx, executorCtx, fmt.Sprintf("Save_%s", fileResult.FilePath), saveActivity, saveInput)
			saveFutures[fileResult.FilePath] = fut
		}

		// Collect version map
		versionMap := make(map[string]int64)
		for filePath, fut := range saveFutures {
			result, err := fut.Get(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to save document %s: %w", filePath, err)
			}
			versionMap[filePath] = result.VersionID
		}

		executorCtx.SendCompleted(fmt.Sprintf("Saved %d documents for %s", len(versionMap), input.StateName))
		return &Output{
			StateName:  input.StateName,
			VersionMap: versionMap,
		}, nil
	}
}
