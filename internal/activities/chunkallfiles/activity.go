// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package chunkallfiles implements a parent activity that chunks multiple files in parallel.
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
	Files     []domainindex.FileToProcess
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
		executorCtx.SendRunning(fmt.Sprintf("Chunking %d files (%s)", len(input.Files), input.StateName))

		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		type chunkFuture struct {
			file   domainindex.FileToProcess
			future *future.Future[*chunkfile.Output]
		}
		chunkFutures := make([]chunkFuture, 0, len(input.Files))

		for _, file := range input.Files {
			activity := f.chunkFileFactory.NewActivity()
			fileInput := &chunkfile.Input{
				FilePath: file.FilePath,
				Content:  file.State.Content,
			}
			fut := executor.ExecuteActivity(exec, ctx, executorCtx, fmt.Sprintf("Chunk_%s", file.FilePath), activity, fileInput)
			chunkFutures = append(chunkFutures, chunkFuture{
				file:   file,
				future: fut,
			})
		}

		var fileChunks []FileChunkResult
		var allChunkTexts []string
		var chunkToFileIndex []int

		for _, entry := range chunkFutures {
			result, err := entry.future.Get(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to chunk file %s: %w", entry.file.FilePath, err)
			}

			fileIndex := len(fileChunks)
			fileChunks = append(fileChunks, FileChunkResult{
				FilePath:    result.FilePath,
				ContentHash: result.ContentHash,
				Content:     result.Content,
				Chunks:      result.Chunks,
			})

			for range result.Chunks {
				chunkToFileIndex = append(chunkToFileIndex, fileIndex)
			}
			for _, chunk := range result.Chunks {
				chunkText := formatChunkWithMetadata(chunk, result.FilePath, input.StateName)
				allChunkTexts = append(allChunkTexts, chunkText)
			}
		}

		executorCtx.SendCompleted(fmt.Sprintf("Chunked %d files → %d chunks (%s)", len(fileChunks), len(allChunkTexts), input.StateName))
		return &Output{
			StateName:        input.StateName,
			FileChunks:       fileChunks,
			AllChunkTexts:    allChunkTexts,
			ChunkToFileIndex: chunkToFileIndex,
		}, nil
	}
}

func formatChunkWithMetadata(chunk domainindex.ChunkData, filePath, stateName string) string {
	var sourceDesc string
	switch stateName {
	case "head":
		sourceDesc = "committed (HEAD)"
	case "staging":
		sourceDesc = "staged for commit"
	case "workspace":
		sourceDesc = "modified in workspace"
	default:
		sourceDesc = stateName
	}

	return fmt.Sprintf("[file: %s, lines: %d-%d, source: %s]\n%s",
		filePath,
		chunk.StartLine,
		chunk.EndLine,
		sourceDesc,
		chunk.TextContent,
	)
}
