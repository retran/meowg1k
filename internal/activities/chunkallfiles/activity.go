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

// Package chunkallfiles provides an activity to chunk all files in parallel.
package chunkallfiles

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/chunkfile"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

type Input struct {
	StateName string
	Files     map[string]domainindex.FileState
}

type FileChunkResult struct {
	FilePath    string
	ContentHash string
	Content     []byte
	Chunks      []domainindex.ChunkData
}

type Output struct {
	StateName        string
	FileChunks       []FileChunkResult
	AllChunkTexts    []string
	ChunkToFileIndex []int // Maps chunk index to file index in FileChunks
}

type Factory struct {
	chunkFileFactory executor.ActivityFactory[*chunkfile.Input, *chunkfile.Output]
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory(
	chunkFileFactory executor.ActivityFactory[*chunkfile.Input, *chunkfile.Output],
) (executor.ActivityFactory[*Input, *Output], error) {
	if chunkFileFactory == nil {
		return nil, fmt.Errorf("chunkallfiles.NewFactory: chunkFileFactory cannot be nil")
	}

	return &Factory{
		chunkFileFactory: chunkFileFactory,
	}, nil
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning(fmt.Sprintf("Chunking %d files for %s...", len(input.Files), input.StateName))

		// Get executor from context
		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		// Launch chunking activities for all files in parallel
		chunkFutures := make(map[string]*future.Future[*chunkfile.Output])
		for filePath, fileState := range input.Files {
			activity := f.chunkFileFactory.NewActivity()
			fileInput := &chunkfile.Input{
				FilePath: filePath,
				Content:  fileState.Content,
			}
			fut := executor.ExecuteActivity(exec, ctx, executorCtx, fmt.Sprintf("Chunk_%s", filePath), activity, fileInput)
			chunkFutures[filePath] = fut
		}

		// Collect results
		var fileChunks []FileChunkResult
		var allChunkTexts []string
		var chunkToFileIndex []int

		for filePath, fut := range chunkFutures {
			result, err := fut.Get(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to chunk file %s: %w", filePath, err)
			}

			fileIndex := len(fileChunks)
			fileChunks = append(fileChunks, FileChunkResult{
				FilePath:    result.FilePath,
				ContentHash: result.ContentHash,
				Content:     result.Content,
				Chunks:      result.Chunks,
			})

			// Build mapping and collect texts
			for range result.Chunks {
				chunkToFileIndex = append(chunkToFileIndex, fileIndex)
			}
			for _, chunk := range result.Chunks {
				allChunkTexts = append(allChunkTexts, chunk.TextContent)
			}
		}

		executorCtx.SendCompleted(fmt.Sprintf("Chunked %d files into %d chunks for %s", len(fileChunks), len(allChunkTexts), input.StateName))
		return &Output{
			StateName:        input.StateName,
			FileChunks:       fileChunks,
			AllChunkTexts:    allChunkTexts,
			ChunkToFileIndex: chunkToFileIndex,
		}, nil
	}
}
