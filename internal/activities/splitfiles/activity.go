// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package splitfiles implements a parent activity that chunks multiple files sequentially.
package splitfiles

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/splitfile"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the request payload for chunking multiple files.
type Input struct {
	StateName string
	Files     []domainindex.FileToProcess
}

// FileChunkResult captures chunk data for a single file.
type FileChunkResult struct {
	FilePath    string
	ContentHash string
	Content     []byte
	Chunks      []domainindex.ChunkData
}

// Output contains the aggregated chunk results and index mappings.
type Output struct {
	StateName        string
	FileChunks       []FileChunkResult
	AllChunkTexts    []string
	ChunkToFileIndex []int // Maps chunk index to file index in FileChunks
}

// Factory builds splitfiles activities.
type Factory struct {
	chunkFileFactory executor.ActivityFactory[*splitfile.Input, *splitfile.Output]
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a splitfiles activity factory.
func NewFactory(
	chunkFileFactory executor.ActivityFactory[*splitfile.Input, *splitfile.Output],
) (executor.ActivityFactory[*Input, *Output], error) {
	if chunkFileFactory == nil {
		return nil, fmt.Errorf("splitfiles.NewFactory: chunkFileFactory cannot be nil")
	}

	return &Factory{
		chunkFileFactory: chunkFileFactory,
	}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunningWithDetails(
			"I'm splitting files into chunks",
			fmt.Sprintf("files=%d state=%s", len(input.Files), input.StateName),
		)

		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		var fileChunks []FileChunkResult
		var allChunkTexts []string
		var chunkToFileIndex []int

		for _, file := range input.Files {
			activity := f.chunkFileFactory.NewActivity()
			fileInput := &splitfile.Input{
				FilePath: file.FilePath,
				Content:  file.State.Content,
			}
			result, err := executor.ExecuteActivity(ctx, exec, executorCtx, fmt.Sprintf("Chunk_%s", file.FilePath), activity, fileInput)
			if err != nil {
				return nil, fmt.Errorf("failed to chunk file %s: %w", file.FilePath, err)
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

		executorCtx.SendCompletedWithDetails(
			"I've split the files into chunks",
			fmt.Sprintf("files=%d chunks=%d", len(fileChunks), len(allChunkTexts)),
		)
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
