// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package index

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/chunkallfiles"
	"github.com/retran/meowg1k/internal/activities/computeallembeddings"
	"github.com/retran/meowg1k/internal/activities/deduplicateandprepare"
	"github.com/retran/meowg1k/internal/activities/distributeandsave"
	"github.com/retran/meowg1k/internal/activities/finalizesnapshots"
	"github.com/retran/meowg1k/internal/activities/preparebatches"
	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
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
	validScan := &mockActivityFactory[struct{}, *scanworkspacestate.Output]{}
	validDeduplicate := &mockActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]{}
	validChunk := &mockActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]{}
	validPrepareBatches := &mockActivityFactory[*preparebatches.Input, *preparebatches.Output]{}
	validCompute := &mockActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]{}
	validDistribute := &mockActivityFactory[*distributeandsave.Input, *distributeandsave.Output]{}
	validFinalize := &mockActivityFactory[*finalizesnapshots.Input, struct{}]{}
	validBuildVector := &mockActivityFactory[struct{}, struct{}]{}

	tests := []struct {
		computeAllEmbeddingsFactory  executor.ActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]
		cleanupFactory               executor.ActivityFactory[struct{}, struct{}]
		scanStateFactory             executor.ActivityFactory[struct{}, *scanworkspacestate.Output]
		deduplicateAndPrepareFactory executor.ActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]
		chunkAllFilesFactory         executor.ActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]
		prepareBatchesFactory        executor.ActivityFactory[*preparebatches.Input, *preparebatches.Output]
		distributeAndSaveFactory     executor.ActivityFactory[*distributeandsave.Input, *distributeandsave.Output]
		finalizeSnapshotsFactory     executor.ActivityFactory[*finalizesnapshots.Input, struct{}]
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
					&mockActivityFactory[struct{}, *scanworkspacestate.Output]{},
					&mockActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]{},
					&mockActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]{},
					&mockActivityFactory[*preparebatches.Input, *preparebatches.Output]{},
					&mockActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]{},
					&mockActivityFactory[*distributeandsave.Input, *distributeandsave.Output]{},
					&mockActivityFactory[*finalizesnapshots.Input, struct{}]{},
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
					&mockActivityFactory[struct{}, *scanworkspacestate.Output]{},
					&mockActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]{},
					&mockActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]{},
					&mockActivityFactory[*preparebatches.Input, *preparebatches.Output]{},
					&mockActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]{},
					&mockActivityFactory[*distributeandsave.Input, *distributeandsave.Output]{},
					&mockActivityFactory[*finalizesnapshots.Input, struct{}]{},
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
					&mockActivityFactory[struct{}, *scanworkspacestate.Output]{},
					&mockActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]{},
					&mockActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]{},
					&mockActivityFactory[*preparebatches.Input, *preparebatches.Output]{},
					&mockActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]{},
					&mockActivityFactory[*distributeandsave.Input, *distributeandsave.Output]{},
					&mockActivityFactory[*finalizesnapshots.Input, struct{}]{},
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
					&mockActivityFactory[struct{}, *scanworkspacestate.Output]{},
					&mockActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]{},
					&mockActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]{},
					&mockActivityFactory[*preparebatches.Input, *preparebatches.Output]{},
					&mockActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]{},
					&mockActivityFactory[*distributeandsave.Input, *distributeandsave.Output]{},
					&mockActivityFactory[*finalizesnapshots.Input, struct{}]{},
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
				mockScan := &mockActivityFactory[struct{}, *scanworkspacestate.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworkspacestate.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworkspacestate.Output, error) {
							return nil, errors.New("scan error")
						}
					},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					mockScan,
					&mockActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]{},
					&mockActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]{},
					&mockActivityFactory[*preparebatches.Input, *preparebatches.Output]{},
					&mockActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]{},
					&mockActivityFactory[*distributeandsave.Input, *distributeandsave.Output]{},
					&mockActivityFactory[*finalizesnapshots.Input, struct{}]{},
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
				mockScan := &mockActivityFactory[struct{}, *scanworkspacestate.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworkspacestate.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworkspacestate.Output, error) {
							return &scanworkspacestate.Output{}, nil
						}
					},
				}
				mockDeduplicate := &mockActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]{
					newActivityFunc: func() executor.Activity[*deduplicateandprepare.Input, *deduplicateandprepare.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *deduplicateandprepare.Input) (*deduplicateandprepare.Output, error) {
							return nil, errors.New("deduplicate error")
						}
					},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					mockScan,
					mockDeduplicate,
					&mockActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]{},
					&mockActivityFactory[*preparebatches.Input, *preparebatches.Output]{},
					&mockActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]{},
					&mockActivityFactory[*distributeandsave.Input, *distributeandsave.Output]{},
					&mockActivityFactory[*finalizesnapshots.Input, struct{}]{},
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
				mockScan := &mockActivityFactory[struct{}, *scanworkspacestate.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworkspacestate.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworkspacestate.Output, error) {
							return &scanworkspacestate.Output{}, nil
						}
					},
				}
				mockDeduplicate := &mockActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]{
					newActivityFunc: func() executor.Activity[*deduplicateandprepare.Input, *deduplicateandprepare.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *deduplicateandprepare.Input) (*deduplicateandprepare.Output, error) {
							return &deduplicateandprepare.Output{
								ExistingVersions: make(map[string]int64),
								FilesToProcess:   []domainindex.FileToProcess{},
							}, nil
						}
					},
				}
				mockFinalize := &mockActivityFactory[*finalizesnapshots.Input, struct{}]{
					newActivityFunc: func() executor.Activity[*finalizesnapshots.Input, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input *finalizesnapshots.Input) (struct{}, error) {
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
					&mockActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]{},
					&mockActivityFactory[*preparebatches.Input, *preparebatches.Output]{},
					&mockActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]{},
					&mockActivityFactory[*distributeandsave.Input, *distributeandsave.Output]{},
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
				mockScan := &mockActivityFactory[struct{}, *scanworkspacestate.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworkspacestate.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworkspacestate.Output, error) {
							return &scanworkspacestate.Output{}, nil
						}
					},
				}
				mockDeduplicate := &mockActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]{
					newActivityFunc: func() executor.Activity[*deduplicateandprepare.Input, *deduplicateandprepare.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *deduplicateandprepare.Input) (*deduplicateandprepare.Output, error) {
							files := []domainindex.FileToProcess{{
								FilePath: "test.go",
								State: domainindex.FileState{
									ContentHash: "hash1",
									Content:     []byte("content"),
								},
							}}
							return &deduplicateandprepare.Output{
								ExistingVersions: make(map[string]int64),
								FilesToProcess:   files,
							}, nil
						}
					},
				}
				mockChunk := &mockActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]{
					newActivityFunc: func() executor.Activity[*chunkallfiles.Input, *chunkallfiles.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *chunkallfiles.Input) (*chunkallfiles.Output, error) {
							return &chunkallfiles.Output{}, nil
						}
					},
				}
				mockPrepareBatches := &mockActivityFactory[*preparebatches.Input, *preparebatches.Output]{
					newActivityFunc: func() executor.Activity[*preparebatches.Input, *preparebatches.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *preparebatches.Input) (*preparebatches.Output, error) {
							return &preparebatches.Output{}, nil
						}
					},
				}
				mockCompute := &mockActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]{
					newActivityFunc: func() executor.Activity[*computeallembeddings.Input, *computeallembeddings.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *computeallembeddings.Input) (*computeallembeddings.Output, error) {
							return &computeallembeddings.Output{}, nil
						}
					},
				}
				mockDistribute := &mockActivityFactory[*distributeandsave.Input, *distributeandsave.Output]{
					newActivityFunc: func() executor.Activity[*distributeandsave.Input, *distributeandsave.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *distributeandsave.Input) (*distributeandsave.Output, error) {
							return &distributeandsave.Output{
								VersionMap: map[string]int64{"hash1": 1},
							}, nil
						}
					},
				}
				mockFinalize := &mockActivityFactory[*finalizesnapshots.Input, struct{}]{
					newActivityFunc: func() executor.Activity[*finalizesnapshots.Input, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input *finalizesnapshots.Input) (struct{}, error) {
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
				mockScan := &mockActivityFactory[struct{}, *scanworkspacestate.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworkspacestate.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworkspacestate.Output, error) {
							return &scanworkspacestate.Output{}, nil
						}
					},
				}
				mockDeduplicate := &mockActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]{
					newActivityFunc: func() executor.Activity[*deduplicateandprepare.Input, *deduplicateandprepare.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *deduplicateandprepare.Input) (*deduplicateandprepare.Output, error) {
							return &deduplicateandprepare.Output{
								ExistingVersions: make(map[string]int64),
								FilesToProcess:   []domainindex.FileToProcess{},
							}, nil
						}
					},
				}
				mockFinalize := &mockActivityFactory[*finalizesnapshots.Input, struct{}]{
					newActivityFunc: func() executor.Activity[*finalizesnapshots.Input, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input *finalizesnapshots.Input) (struct{}, error) {
							return struct{}{}, errors.New("finalize error")
						}
					},
				}
				factory, _ := NewFactory(
					&mockActivityFactory[struct{}, struct{}]{},
					mockScan,
					mockDeduplicate,
					&mockActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]{},
					&mockActivityFactory[*preparebatches.Input, *preparebatches.Output]{},
					&mockActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]{},
					&mockActivityFactory[*distributeandsave.Input, *distributeandsave.Output]{},
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
				mockScan := &mockActivityFactory[struct{}, *scanworkspacestate.Output]{
					newActivityFunc: func() executor.Activity[struct{}, *scanworkspacestate.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input struct{}) (*scanworkspacestate.Output, error) {
							return &scanworkspacestate.Output{}, nil
						}
					},
				}
				mockDeduplicate := &mockActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]{
					newActivityFunc: func() executor.Activity[*deduplicateandprepare.Input, *deduplicateandprepare.Output] {
						return func(ctx context.Context, activityCtx *executor.Context, input *deduplicateandprepare.Input) (*deduplicateandprepare.Output, error) {
							return &deduplicateandprepare.Output{
								ExistingVersions: make(map[string]int64),
								FilesToProcess:   []domainindex.FileToProcess{},
							}, nil
						}
					},
				}
				mockFinalize := &mockActivityFactory[*finalizesnapshots.Input, struct{}]{
					newActivityFunc: func() executor.Activity[*finalizesnapshots.Input, struct{}] {
						return func(ctx context.Context, activityCtx *executor.Context, input *finalizesnapshots.Input) (struct{}, error) {
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
					&mockActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]{},
					&mockActivityFactory[*preparebatches.Input, *preparebatches.Output]{},
					&mockActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]{},
					&mockActivityFactory[*distributeandsave.Input, *distributeandsave.Output]{},
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
