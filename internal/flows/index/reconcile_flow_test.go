// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package index

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/splitfiles"
	"github.com/retran/meowg1k/internal/activities/embedall"
	"github.com/retran/meowg1k/internal/activities/prepareindex"
	"github.com/retran/meowg1k/internal/activities/savechunks"
	"github.com/retran/meowg1k/internal/activities/finalizeindex"
	"github.com/retran/meowg1k/internal/activities/buildbatches"
	"github.com/retran/meowg1k/internal/activities/scanworktree"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/pkg/executor"
)

// Mock activity factory.
type mockActivityFactory[I, O any] struct {
	newActivityFunc func() executor.Activity[I, O]
}

func (m *mockActivityFactory[I, O]) NewActivity() executor.Activity[I, O] {
	if m.newActivityFunc != nil {
		return m.newActivityFunc()
	}
	return func(ctx context.Context, activityCtx *executor.Context, input I) (O, error) {
		var zero O
		return zero, nil
	}
}

func TestNewFactory(t *testing.T) {
	validCleanup := &mockActivityFactory[struct{}, struct{}]{}
	validScan := &mockActivityFactory[struct{}, *scanworktree.Output]{}
	validDeduplicate := &mockActivityFactory[*prepareindex.Input, *prepareindex.Output]{}
	validChunk := &mockActivityFactory[*splitfiles.Input, *splitfiles.Output]{}
	validPrepareBatches := &mockActivityFactory[*buildbatches.Input, *buildbatches.Output]{}
	validCompute := &mockActivityFactory[*embedall.Input, *embedall.Output]{}
	validDistribute := &mockActivityFactory[*savechunks.Input, *savechunks.Output]{}
	validFinalize := &mockActivityFactory[*finalizeindex.Input, struct{}]{}
	validBuildVector := &mockActivityFactory[struct{}, struct{}]{}

	tests := []struct {
		computeAllEmbeddingsFactory  executor.ActivityFactory[*embedall.Input, *embedall.Output]
		cleanupFactory               executor.ActivityFactory[struct{}, struct{}]
		scanStateFactory             executor.ActivityFactory[struct{}, *scanworktree.Output]
		deduplicateAndPrepareFactory executor.ActivityFactory[*prepareindex.Input, *prepareindex.Output]
		chunkAllFilesFactory         executor.ActivityFactory[*splitfiles.Input, *splitfiles.Output]
		prepareBatchesFactory        executor.ActivityFactory[*buildbatches.Input, *buildbatches.Output]
		distributeAndSaveFactory     executor.ActivityFactory[*savechunks.Input, *savechunks.Output]
		finalizeSnapshotsFactory     executor.ActivityFactory[*finalizeindex.Input, struct{}]
		buildVectorIndicesFactory    executor.ActivityFactory[struct{}, struct{}]
		name                         string
		expectedErrMsg               string
		batchSize                    int
		wantErr                      bool
	}{
		{
			name:                         "nil cleanupFactory",
			cleanupFactory:               nil,
			scanStateFactory:             validScan,
			deduplicateAndPrepareFactory: validDeduplicate,
			chunkAllFilesFactory:         validChunk,
			prepareBatchesFactory:        validPrepareBatches,
			computeAllEmbeddingsFactory:  validCompute,
			distributeAndSaveFactory:     validDistribute,
			finalizeSnapshotsFactory:     validFinalize,
			buildVectorIndicesFactory:    validBuildVector,
			batchSize:                    10,
			wantErr:                      true,
			expectedErrMsg:               "cleanupFactory is nil",
		},
		{
			name:                         "nil scanStateFactory",
			cleanupFactory:               validCleanup,
			scanStateFactory:             nil,
			deduplicateAndPrepareFactory: validDeduplicate,
			chunkAllFilesFactory:         validChunk,
			prepareBatchesFactory:        validPrepareBatches,
			computeAllEmbeddingsFactory:  validCompute,
			distributeAndSaveFactory:     validDistribute,
			finalizeSnapshotsFactory:     validFinalize,
			buildVectorIndicesFactory:    validBuildVector,
			batchSize:                    10,
			wantErr:                      true,
			expectedErrMsg:               "scanStateFactory is nil",
		},
		{
			name:                         "nil deduplicateAndPrepareFactory",
			cleanupFactory:               validCleanup,
			scanStateFactory:             validScan,
			deduplicateAndPrepareFactory: nil,
			chunkAllFilesFactory:         validChunk,
			prepareBatchesFactory:        validPrepareBatches,
			computeAllEmbeddingsFactory:  validCompute,
			distributeAndSaveFactory:     validDistribute,
			finalizeSnapshotsFactory:     validFinalize,
			buildVectorIndicesFactory:    validBuildVector,
			batchSize:                    10,
			wantErr:                      true,
			expectedErrMsg:               "deduplicateAndPrepareFactory is nil",
		},
		{
			name:                         "nil chunkAllFilesFactory",
			cleanupFactory:               validCleanup,
			scanStateFactory:             validScan,
			deduplicateAndPrepareFactory: validDeduplicate,
			chunkAllFilesFactory:         nil,
			prepareBatchesFactory:        validPrepareBatches,
			computeAllEmbeddingsFactory:  validCompute,
			distributeAndSaveFactory:     validDistribute,
			finalizeSnapshotsFactory:     validFinalize,
			buildVectorIndicesFactory:    validBuildVector,
			batchSize:                    10,
			wantErr:                      true,
			expectedErrMsg:               "chunkAllFilesFactory is nil",
		},
		{
			name:                         "nil prepareBatchesFactory",
			cleanupFactory:               validCleanup,
			scanStateFactory:             validScan,
			deduplicateAndPrepareFactory: validDeduplicate,
			chunkAllFilesFactory:         validChunk,
			prepareBatchesFactory:        nil,
			computeAllEmbeddingsFactory:  validCompute,
			distributeAndSaveFactory:     validDistribute,
			finalizeSnapshotsFactory:     validFinalize,
			buildVectorIndicesFactory:    validBuildVector,
			batchSize:                    10,
			wantErr:                      true,
			expectedErrMsg:               "prepareBatchesFactory is nil",
		},
		{
			name:                         "nil computeAllEmbeddingsFactory",
			cleanupFactory:               validCleanup,
			scanStateFactory:             validScan,
			deduplicateAndPrepareFactory: validDeduplicate,
			chunkAllFilesFactory:         validChunk,
			prepareBatchesFactory:        validPrepareBatches,
			computeAllEmbeddingsFactory:  nil,
			distributeAndSaveFactory:     validDistribute,
			finalizeSnapshotsFactory:     validFinalize,
			buildVectorIndicesFactory:    validBuildVector,
			batchSize:                    10,
			wantErr:                      true,
			expectedErrMsg:               "computeAllEmbeddingsFactory is nil",
		},
		{
			name:                         "nil distributeAndSaveFactory",
			cleanupFactory:               validCleanup,
			scanStateFactory:             validScan,
			deduplicateAndPrepareFactory: validDeduplicate,
			chunkAllFilesFactory:         validChunk,
			prepareBatchesFactory:        validPrepareBatches,
			computeAllEmbeddingsFactory:  validCompute,
			distributeAndSaveFactory:     nil,
			finalizeSnapshotsFactory:     validFinalize,
			buildVectorIndicesFactory:    validBuildVector,
			batchSize:                    10,
			wantErr:                      true,
			expectedErrMsg:               "distributeAndSaveFactory is nil",
		},
		{
			name:                         "nil finalizeSnapshotsFactory",
			cleanupFactory:               validCleanup,
			scanStateFactory:             validScan,
			deduplicateAndPrepareFactory: validDeduplicate,
			chunkAllFilesFactory:         validChunk,
			prepareBatchesFactory:        validPrepareBatches,
			computeAllEmbeddingsFactory:  validCompute,
			distributeAndSaveFactory:     validDistribute,
			finalizeSnapshotsFactory:     nil,
			buildVectorIndicesFactory:    validBuildVector,
			batchSize:                    10,
			wantErr:                      true,
			expectedErrMsg:               "finalizeSnapshotsFactory is nil",
		},
		{
			name:                         "nil buildVectorIndicesFactory",
			cleanupFactory:               validCleanup,
			scanStateFactory:             validScan,
			deduplicateAndPrepareFactory: validDeduplicate,
			chunkAllFilesFactory:         validChunk,
			prepareBatchesFactory:        validPrepareBatches,
			computeAllEmbeddingsFactory:  validCompute,
			distributeAndSaveFactory:     validDistribute,
			finalizeSnapshotsFactory:     validFinalize,
			buildVectorIndicesFactory:    nil,
			batchSize:                    10,
			wantErr:                      true,
			expectedErrMsg:               "buildVectorIndicesFactory is nil",
		},
		{
			name:                         "invalid batch size - zero",
			cleanupFactory:               validCleanup,
			scanStateFactory:             validScan,
			deduplicateAndPrepareFactory: validDeduplicate,
			chunkAllFilesFactory:         validChunk,
			prepareBatchesFactory:        validPrepareBatches,
			computeAllEmbeddingsFactory:  validCompute,
			distributeAndSaveFactory:     validDistribute,
			finalizeSnapshotsFactory:     validFinalize,
			buildVectorIndicesFactory:    validBuildVector,
			batchSize:                    0,
			wantErr:                      true,
			expectedErrMsg:               "batchSize must be positive",
		},
		{
			name:                         "invalid batch size - negative",
			cleanupFactory:               validCleanup,
			scanStateFactory:             validScan,
			deduplicateAndPrepareFactory: validDeduplicate,
			chunkAllFilesFactory:         validChunk,
			prepareBatchesFactory:        validPrepareBatches,
			computeAllEmbeddingsFactory:  validCompute,
			distributeAndSaveFactory:     validDistribute,
			finalizeSnapshotsFactory:     validFinalize,
			buildVectorIndicesFactory:    validBuildVector,
			batchSize:                    -1,
			wantErr:                      true,
			expectedErrMsg:               "batchSize must be positive",
		},
		{
			name:                         "successful factory creation",
			cleanupFactory:               validCleanup,
			scanStateFactory:             validScan,
			deduplicateAndPrepareFactory: validDeduplicate,
			chunkAllFilesFactory:         validChunk,
			prepareBatchesFactory:        validPrepareBatches,
			computeAllEmbeddingsFactory:  validCompute,
			distributeAndSaveFactory:     validDistribute,
			finalizeSnapshotsFactory:     validFinalize,
			buildVectorIndicesFactory:    validBuildVector,
			batchSize:                    10,
			wantErr:                      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, err := NewFactory(
				tt.cleanupFactory,
				tt.scanStateFactory,
				tt.deduplicateAndPrepareFactory,
				tt.chunkAllFilesFactory,
				tt.prepareBatchesFactory,
				tt.computeAllEmbeddingsFactory,
				tt.distributeAndSaveFactory,
				tt.finalizeSnapshotsFactory,
				tt.buildVectorIndicesFactory,
				tt.batchSize,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if factory != nil {
					t.Errorf("expected nil factory but got %v", factory)
				}
				if tt.expectedErrMsg != "" && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error message to contain %q, got %q", tt.expectedErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if factory == nil {
				t.Errorf("expected non-nil factory but got nil")
			}
		})
	}
}

func TestFactory_NewFlow(t *testing.T) {
	tests := []struct {
		setupFactory   func() *Factory
		setupContext   func() (context.Context, *executor.Context)
		name           string
		expectedErrMsg string
		wantErr        bool
	}{
		{
			name: "nil factory",
			setupFactory: func() *Factory {
				return nil
			},
			setupContext: func() (context.Context, *executor.Context) {
				return context.Background(), executor.NewContext("test", nil, nil)
			},
			wantErr:        true,
			expectedErrMsg: "factory is nil",
		},
		{
			name: "nil context",
			setupFactory: func() *Factory {
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					&mockActivityFactory[struct{}, *scanworktree.Output]{},
					&mockActivityFactory[*prepareindex.Input, *prepareindex.Output]{},
					&mockActivityFactory[*splitfiles.Input, *splitfiles.Output]{},
					&mockActivityFactory[*buildbatches.Input, *buildbatches.Output]{},
					&mockActivityFactory[*embedall.Input, *embedall.Output]{},
					&mockActivityFactory[*savechunks.Input, *savechunks.Output]{},
					&mockActivityFactory[*finalizeindex.Input, struct{}]{},
					&mockActivityFactory[struct{}, struct{}]{},
					10,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				return nil, executor.NewContext("test", nil, nil)
			},
			wantErr:        true,
			expectedErrMsg: "context is nil",
		},
		{
			name: "nil flow context",
			setupFactory: func() *Factory {
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					&mockActivityFactory[struct{}, *scanworktree.Output]{},
					&mockActivityFactory[*prepareindex.Input, *prepareindex.Output]{},
					&mockActivityFactory[*splitfiles.Input, *splitfiles.Output]{},
					&mockActivityFactory[*buildbatches.Input, *buildbatches.Output]{},
					&mockActivityFactory[*embedall.Input, *embedall.Output]{},
					&mockActivityFactory[*savechunks.Input, *savechunks.Output]{},
					&mockActivityFactory[*finalizeindex.Input, struct{}]{},
					&mockActivityFactory[struct{}, struct{}]{},
					10,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				return context.Background(), nil
			},
			wantErr:        true,
			expectedErrMsg: "flow context is nil",
		},
		{
			name: "executor not available",
			setupFactory: func() *Factory {
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					&mockActivityFactory[struct{}, *scanworktree.Output]{},
					&mockActivityFactory[*prepareindex.Input, *prepareindex.Output]{},
					&mockActivityFactory[*splitfiles.Input, *splitfiles.Output]{},
					&mockActivityFactory[*buildbatches.Input, *buildbatches.Output]{},
					&mockActivityFactory[*embedall.Input, *embedall.Output]{},
					&mockActivityFactory[*savechunks.Input, *savechunks.Output]{},
					&mockActivityFactory[*finalizeindex.Input, struct{}]{},
					&mockActivityFactory[struct{}, struct{}]{},
					10,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				flowCtx := executor.NewContext("test", nil, nil)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "executor not available in flow context",
		},
		{
			name: "cleanup activity error",
			setupFactory: func() *Factory {
				mockCleanup := &mockActivityFactory[struct{}, struct{}]{
					newActivityFunc: func() executor.Activity[struct{}, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (struct{}, error) {
							return struct{}{}, errors.New("cleanup error")
						}
					},
				}
				factory, _ := NewFactory(
					mockCleanup,
					&mockActivityFactory[struct{}, *scanworktree.Output]{},
					&mockActivityFactory[*prepareindex.Input, *prepareindex.Output]{},
					&mockActivityFactory[*splitfiles.Input, *splitfiles.Output]{},
					&mockActivityFactory[*buildbatches.Input, *buildbatches.Output]{},
					&mockActivityFactory[*embedall.Input, *embedall.Output]{},
					&mockActivityFactory[*savechunks.Input, *savechunks.Output]{},
					&mockActivityFactory[*finalizeindex.Input, struct{}]{},
					&mockActivityFactory[struct{}, struct{}]{},
					10,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				exec := executor.NewExecutor(0)
				flowCtx := executor.NewContext("test", nil, exec)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "cleanup failed",
		},
		{
			name: "scan activity error",
			setupFactory: func() *Factory {
				mockScan := &mockActivityFactory[struct{}, *scanworktree.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworktree.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworktree.Output, error) {
							return nil, errors.New("scan error")
						}
					},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					mockScan,
					&mockActivityFactory[*prepareindex.Input, *prepareindex.Output]{},
					&mockActivityFactory[*splitfiles.Input, *splitfiles.Output]{},
					&mockActivityFactory[*buildbatches.Input, *buildbatches.Output]{},
					&mockActivityFactory[*embedall.Input, *embedall.Output]{},
					&mockActivityFactory[*savechunks.Input, *savechunks.Output]{},
					&mockActivityFactory[*finalizeindex.Input, struct{}]{},
					&mockActivityFactory[struct{}, struct{}]{},
					10,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				exec := executor.NewExecutor(0)
				flowCtx := executor.NewContext("test", nil, exec)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "workspace scan failed",
		},
		{
			name: "deduplicate activity error",
			setupFactory: func() *Factory {
				mockScan := &mockActivityFactory[struct{}, *scanworktree.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworktree.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworktree.Output, error) {
							return &scanworktree.Output{}, nil
						}
					},
				}
				mockDeduplicate := &mockActivityFactory[*prepareindex.Input, *prepareindex.Output]{
					newActivityFunc: func() executor.Activity[*prepareindex.Input, *prepareindex.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *prepareindex.Input) (*prepareindex.Output, error) {
							return nil, errors.New("deduplicate error")
						}
					},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					mockScan,
					mockDeduplicate,
					&mockActivityFactory[*splitfiles.Input, *splitfiles.Output]{},
					&mockActivityFactory[*buildbatches.Input, *buildbatches.Output]{},
					&mockActivityFactory[*embedall.Input, *embedall.Output]{},
					&mockActivityFactory[*savechunks.Input, *savechunks.Output]{},
					&mockActivityFactory[*finalizeindex.Input, struct{}]{},
					&mockActivityFactory[struct{}, struct{}]{},
					10,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				exec := executor.NewExecutor(0)
				flowCtx := executor.NewContext("test", nil, exec)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "deduplication failed",
		},
		{
			name: "successful flow - no new files to process",
			setupFactory: func() *Factory {
				mockScan := &mockActivityFactory[struct{}, *scanworktree.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworktree.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworktree.Output, error) {
							return &scanworktree.Output{}, nil
						}
					},
				}
				mockDeduplicate := &mockActivityFactory[*prepareindex.Input, *prepareindex.Output]{
					newActivityFunc: func() executor.Activity[*prepareindex.Input, *prepareindex.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *prepareindex.Input) (*prepareindex.Output, error) {
							return &prepareindex.Output{
								ExistingVersions: make(map[string]int64),
								FilesToProcess:   []domainindex.FileToProcess{},
							}, nil
						}
					},
				}
				mockFinalize := &mockActivityFactory[*finalizeindex.Input, struct{}]{
					newActivityFunc: func() executor.Activity[*finalizeindex.Input, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input *finalizeindex.Input) (struct{}, error) {
							return struct{}{}, nil
						}
					},
				}
				mockBuildVector := &mockActivityFactory[struct{}, struct{}]{
					newActivityFunc: func() executor.Activity[struct{}, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (struct{}, error) {
							return struct{}{}, nil
						}
					},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					mockScan,
					mockDeduplicate,
					&mockActivityFactory[*splitfiles.Input, *splitfiles.Output]{},
					&mockActivityFactory[*buildbatches.Input, *buildbatches.Output]{},
					&mockActivityFactory[*embedall.Input, *embedall.Output]{},
					&mockActivityFactory[*savechunks.Input, *savechunks.Output]{},
					mockFinalize,
					mockBuildVector,
					10,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				exec := executor.NewExecutor(0)
				flowCtx := executor.NewContext("test", nil, exec)
				return ctx, flowCtx
			},
			wantErr: false,
		},
		{
			name: "successful flow - with files to process",
			setupFactory: func() *Factory {
				mockScan := &mockActivityFactory[struct{}, *scanworktree.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworktree.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworktree.Output, error) {
							return &scanworktree.Output{}, nil
						}
					},
				}
				mockDeduplicate := &mockActivityFactory[*prepareindex.Input, *prepareindex.Output]{
					newActivityFunc: func() executor.Activity[*prepareindex.Input, *prepareindex.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *prepareindex.Input) (*prepareindex.Output, error) {
							files := []domainindex.FileToProcess{{
								FilePath: "test.go",
								State: domainindex.FileState{
									ContentHash: "hash1",
									Content:     []byte("content"),
								},
							}}
							return &prepareindex.Output{
								ExistingVersions: make(map[string]int64),
								FilesToProcess:   files,
							}, nil
						}
					},
				}
				mockChunk := &mockActivityFactory[*splitfiles.Input, *splitfiles.Output]{
					newActivityFunc: func() executor.Activity[*splitfiles.Input, *splitfiles.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *splitfiles.Input) (*splitfiles.Output, error) {
							return &splitfiles.Output{}, nil
						}
					},
				}
				mockPrepareBatches := &mockActivityFactory[*buildbatches.Input, *buildbatches.Output]{
					newActivityFunc: func() executor.Activity[*buildbatches.Input, *buildbatches.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *buildbatches.Input) (*buildbatches.Output, error) {
							return &buildbatches.Output{}, nil
						}
					},
				}
				mockCompute := &mockActivityFactory[*embedall.Input, *embedall.Output]{
					newActivityFunc: func() executor.Activity[*embedall.Input, *embedall.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *embedall.Input) (*embedall.Output, error) {
							return &embedall.Output{}, nil
						}
					},
				}
				mockDistribute := &mockActivityFactory[*savechunks.Input, *savechunks.Output]{
					newActivityFunc: func() executor.Activity[*savechunks.Input, *savechunks.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *savechunks.Input) (*savechunks.Output, error) {
							return &savechunks.Output{
								VersionMap: map[string]int64{"hash1": 1},
							}, nil
						}
					},
				}
				mockFinalize := &mockActivityFactory[*finalizeindex.Input, struct{}]{
					newActivityFunc: func() executor.Activity[*finalizeindex.Input, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input *finalizeindex.Input) (struct{}, error) {
							return struct{}{}, nil
						}
					},
				}
				mockBuildVector := &mockActivityFactory[struct{}, struct{}]{
					newActivityFunc: func() executor.Activity[struct{}, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (struct{}, error) {
							return struct{}{}, nil
						}
					},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					mockScan,
					mockDeduplicate,
					mockChunk,
					mockPrepareBatches,
					mockCompute,
					mockDistribute,
					mockFinalize,
					mockBuildVector,
					10,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				exec := executor.NewExecutor(0)
				flowCtx := executor.NewContext("test", nil, exec)
				return ctx, flowCtx
			},
			wantErr: false,
		},
		{
			name: "finalize snapshots error",
			setupFactory: func() *Factory {
				mockScan := &mockActivityFactory[struct{}, *scanworktree.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworktree.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworktree.Output, error) {
							return &scanworktree.Output{}, nil
						}
					},
				}
				mockDeduplicate := &mockActivityFactory[*prepareindex.Input, *prepareindex.Output]{
					newActivityFunc: func() executor.Activity[*prepareindex.Input, *prepareindex.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *prepareindex.Input) (*prepareindex.Output, error) {
							return &prepareindex.Output{
								ExistingVersions: make(map[string]int64),
								FilesToProcess:   []domainindex.FileToProcess{},
							}, nil
						}
					},
				}
				mockFinalize := &mockActivityFactory[*finalizeindex.Input, struct{}]{
					newActivityFunc: func() executor.Activity[*finalizeindex.Input, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input *finalizeindex.Input) (struct{}, error) {
							return struct{}{}, errors.New("finalize error")
						}
					},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					mockScan,
					mockDeduplicate,
					&mockActivityFactory[*splitfiles.Input, *splitfiles.Output]{},
					&mockActivityFactory[*buildbatches.Input, *buildbatches.Output]{},
					&mockActivityFactory[*embedall.Input, *embedall.Output]{},
					&mockActivityFactory[*savechunks.Input, *savechunks.Output]{},
					mockFinalize,
					&mockActivityFactory[struct{}, struct{}]{},
					10,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				exec := executor.NewExecutor(0)
				flowCtx := executor.NewContext("test", nil, exec)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "snapshots finalization failed",
		},
		{
			name: "build vector indices error",
			setupFactory: func() *Factory {
				mockScan := &mockActivityFactory[struct{}, *scanworktree.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworktree.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworktree.Output, error) {
							return &scanworktree.Output{}, nil
						}
					},
				}
				mockDeduplicate := &mockActivityFactory[*prepareindex.Input, *prepareindex.Output]{
					newActivityFunc: func() executor.Activity[*prepareindex.Input, *prepareindex.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *prepareindex.Input) (*prepareindex.Output, error) {
							return &prepareindex.Output{
								ExistingVersions: make(map[string]int64),
								FilesToProcess:   []domainindex.FileToProcess{},
							}, nil
						}
					},
				}
				mockFinalize := &mockActivityFactory[*finalizeindex.Input, struct{}]{
					newActivityFunc: func() executor.Activity[*finalizeindex.Input, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input *finalizeindex.Input) (struct{}, error) {
							return struct{}{}, nil
						}
					},
				}
				mockBuildVector := &mockActivityFactory[struct{}, struct{}]{
					newActivityFunc: func() executor.Activity[struct{}, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (struct{}, error) {
							return struct{}{}, errors.New("vector build error")
						}
					},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					mockScan,
					mockDeduplicate,
					&mockActivityFactory[*splitfiles.Input, *splitfiles.Output]{},
					&mockActivityFactory[*buildbatches.Input, *buildbatches.Output]{},
					&mockActivityFactory[*embedall.Input, *embedall.Output]{},
					&mockActivityFactory[*savechunks.Input, *savechunks.Output]{},
					mockFinalize,
					mockBuildVector,
					10,
				)
				return factory
			},
			setupContext: func() (context.Context, *executor.Context) {
				ctx := context.Background()
				exec := executor.NewExecutor(0)
				flowCtx := executor.NewContext("test", nil, exec)
				return ctx, flowCtx
			},
			wantErr:        true,
			expectedErrMsg: "vector indices build failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := tt.setupFactory()
			ctx, flowCtx := tt.setupContext()

			flow := factory.NewFlow()
			err := flow(ctx, flowCtx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				}
				if tt.expectedErrMsg != "" && err != nil && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error message to contain %q, got %q", tt.expectedErrMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}
