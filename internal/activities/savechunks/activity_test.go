// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package savechunks

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/activities/buildbatches"
	"github.com/retran/meowg1k/internal/activities/embedall"
	"github.com/retran/meowg1k/internal/activities/savefileversion"
	"github.com/retran/meowg1k/internal/activities/splitfiles"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type fakeSaveFactory struct {
	calls []*savefileversion.Input
}

func (f *fakeSaveFactory) NewActivity() executor.Activity[*savefileversion.Input, *savefileversion.Output] {
	return func(ctx context.Context, execCtx *executor.Context, input *savefileversion.Input) (*savefileversion.Output, error) {
		_ = ctx
		_ = execCtx
		f.calls = append(f.calls, input)
		id := int64(len(f.calls))
		return &savefileversion.Output{FilePath: input.FilePath, VersionID: id}, nil
	}
}

type stubIndexRepo struct {
	ports.IndexRepository
	checkpointErr error
	checkpoints   int
}

func (s *stubIndexRepo) Checkpoint(ctx context.Context) error {
	_ = ctx
	s.checkpoints++
	return s.checkpointErr
}

func TestNewFactoryErrors(t *testing.T) {
	_, err := NewFactory(nil, &stubIndexRepo{})
	assert.Error(t, err)
	_, err = NewFactory(&fakeSaveFactory{}, nil)
	assert.Error(t, err)
}

func TestSaveChunksActivity_Success(t *testing.T) {
	saveFactory := &fakeSaveFactory{}
	indexRepo := &stubIndexRepo{}

	factory, err := NewFactory(saveFactory, indexRepo)
	require.NoError(t, err)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	chunkResults := &splitfiles.Output{
		FileChunks: []splitfiles.FileChunkResult{{
			FilePath:    "a.txt",
			ContentHash: "hash-a",
			Content:     []byte("alpha"),
			Chunks: []domainindex.ChunkData{
				{TextContent: "alpha", StartLine: 1, EndLine: 1},
				{TextContent: "beta", StartLine: 2, EndLine: 2},
			},
		}},
		ChunkToFileIndex: []int{0, 0},
	}
	prepared := &buildbatches.Output{
		StateName:    "workspace",
		ChunkResults: chunkResults,
	}
	embeddingOutput := &embedall.Output{
		StateName:       "workspace",
		PreparedBatches: prepared,
		Embeddings: []gateway.Embedding{
			{1.0},
			{2.0},
		},
	}

	output, err := activity(context.Background(), flowCtx, &Input{
		EmbeddingResults: embeddingOutput,
		StateName:        "workspace",
	})
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "workspace", output.StateName)
	assert.Equal(t, int64(1), output.VersionMap["hash-a"])

	require.Len(t, saveFactory.calls, 1)
	assert.Equal(t, "a.txt", saveFactory.calls[0].FilePath)
	assert.Len(t, saveFactory.calls[0].Embeddings, 2)
	assert.Equal(t, 1, indexRepo.checkpoints)
}

func TestSaveChunksActivity_CheckpointWarning(t *testing.T) {
	saveFactory := &fakeSaveFactory{}
	indexRepo := &stubIndexRepo{checkpointErr: errors.New("checkpoint warning")}

	factory, err := NewFactory(saveFactory, indexRepo)
	require.NoError(t, err)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	chunkResults := &splitfiles.Output{
		FileChunks: []splitfiles.FileChunkResult{{
			FilePath:    "a.txt",
			ContentHash: "hash-a",
			Content:     []byte("alpha"),
			Chunks:      []domainindex.ChunkData{{TextContent: "alpha", StartLine: 1, EndLine: 1}},
		}},
		ChunkToFileIndex: []int{0},
	}
	embeddingOutput := &embedall.Output{
		StateName: "workspace",
		PreparedBatches: &buildbatches.Output{
			StateName:    "workspace",
			ChunkResults: chunkResults,
		},
		Embeddings: []gateway.Embedding{{1.0}},
	}

	output, err := activity(context.Background(), flowCtx, &Input{
		EmbeddingResults: embeddingOutput,
		StateName:        "workspace",
	})
	require.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, 1, indexRepo.checkpoints)
}

func TestSaveChunksActivity_MissingExecutor(t *testing.T) {
	saveFactory := &fakeSaveFactory{}
	indexRepo := &stubIndexRepo{}

	factory, err := NewFactory(saveFactory, indexRepo)
	require.NoError(t, err)
	activity := factory.NewActivity()

	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, nil)

	_, err = activity(context.Background(), flowCtx, &Input{
		EmbeddingResults: &embedall.Output{
			PreparedBatches: &buildbatches.Output{
				ChunkResults: &splitfiles.Output{},
			},
		},
		StateName: "workspace",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "executor not available in context")
}
