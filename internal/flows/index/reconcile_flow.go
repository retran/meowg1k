// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package index implements the workflow for indexing workspace files by computing embeddings and building vector indices.
package index

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/buildbatches"
	"github.com/retran/meowg1k/internal/activities/embedall"
	"github.com/retran/meowg1k/internal/activities/finalizeindex"
	"github.com/retran/meowg1k/internal/activities/prepareindex"
	"github.com/retran/meowg1k/internal/activities/savechunks"
	"github.com/retran/meowg1k/internal/activities/scanworktree"
	"github.com/retran/meowg1k/internal/activities/splitfiles"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/pkg/executor"
)

// Factory builds index reconciliation flows.
type Factory struct {
	cleanupFactory               executor.ActivityFactory[struct{}, struct{}]
	scanStateFactory             executor.ActivityFactory[struct{}, *scanworktree.Output]
	deduplicateAndPrepareFactory executor.ActivityFactory[*prepareindex.Input, *prepareindex.Output]
	chunkAllFilesFactory         executor.ActivityFactory[*splitfiles.Input, *splitfiles.Output]
	prepareBatchesFactory        executor.ActivityFactory[*buildbatches.Input, *buildbatches.Output]
	computeAllEmbeddingsFactory  executor.ActivityFactory[*embedall.Input, *embedall.Output]
	distributeAndSaveFactory     executor.ActivityFactory[*savechunks.Input, *savechunks.Output]
	finalizeSnapshotsFactory     executor.ActivityFactory[*finalizeindex.Input, struct{}]
	buildVectorIndicesFactory    executor.ActivityFactory[struct{}, struct{}]
	batchSize                    int
}

// NewFactory creates a reconciliation flow factory.
func NewFactory(
	cleanupFactory executor.ActivityFactory[struct{}, struct{}],
	scanStateFactory executor.ActivityFactory[struct{}, *scanworktree.Output],
	deduplicateAndPrepareFactory executor.ActivityFactory[*prepareindex.Input, *prepareindex.Output],
	chunkAllFilesFactory executor.ActivityFactory[*splitfiles.Input, *splitfiles.Output],
	prepareBatchesFactory executor.ActivityFactory[*buildbatches.Input, *buildbatches.Output],
	computeAllEmbeddingsFactory executor.ActivityFactory[*embedall.Input, *embedall.Output],
	distributeAndSaveFactory executor.ActivityFactory[*savechunks.Input, *savechunks.Output],
	finalizeSnapshotsFactory executor.ActivityFactory[*finalizeindex.Input, struct{}],
	buildVectorIndicesFactory executor.ActivityFactory[struct{}, struct{}],
	batchSize int,
) (*Factory, error) {
	if cleanupFactory == nil {
		return nil, fmt.Errorf("cleanupFactory is nil")
	}

	if scanStateFactory == nil {
		return nil, fmt.Errorf("scanStateFactory is nil")
	}

	if deduplicateAndPrepareFactory == nil {
		return nil, fmt.Errorf("deduplicateAndPrepareFactory is nil")
	}

	if chunkAllFilesFactory == nil {
		return nil, fmt.Errorf("chunkAllFilesFactory is nil")
	}

	if prepareBatchesFactory == nil {
		return nil, fmt.Errorf("prepareBatchesFactory is nil")
	}

	if computeAllEmbeddingsFactory == nil {
		return nil, fmt.Errorf("computeAllEmbeddingsFactory is nil")
	}

	if distributeAndSaveFactory == nil {
		return nil, fmt.Errorf("distributeAndSaveFactory is nil")
	}

	if finalizeSnapshotsFactory == nil {
		return nil, fmt.Errorf("finalizeSnapshotsFactory is nil")
	}

	if buildVectorIndicesFactory == nil {
		return nil, fmt.Errorf("buildVectorIndicesFactory is nil")
	}

	if batchSize <= 0 {
		return nil, fmt.Errorf("batchSize must be positive, got %d", batchSize)
	}

	return &Factory{
		cleanupFactory:               cleanupFactory,
		scanStateFactory:             scanStateFactory,
		deduplicateAndPrepareFactory: deduplicateAndPrepareFactory,
		chunkAllFilesFactory:         chunkAllFilesFactory,
		prepareBatchesFactory:        prepareBatchesFactory,
		computeAllEmbeddingsFactory:  computeAllEmbeddingsFactory,
		distributeAndSaveFactory:     distributeAndSaveFactory,
		finalizeSnapshotsFactory:     finalizeSnapshotsFactory,
		buildVectorIndicesFactory:    buildVectorIndicesFactory,
		batchSize:                    batchSize,
	}, nil
}

// NewFlow creates and returns the reconcile live context flow function.
// This flow orchestrates the complete reconciliation process for live contexts
// (_head_, _stage_, _workdir_). It follows a "Clean and Recreate" strategy to ensure
// 100% consistency by rebuilding all snapshots and vector indices from scratch.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		flowCtx.SendRunningWithDetails("I'm rebuilding the search index", "mode=full")

		exec, err := f.validateFlowContext(ctx, flowCtx)
		if err != nil {
			return err
		}

		if err := f.runCleanup(ctx, flowCtx, exec); err != nil {
			return err
		}

		scanResult, err := f.runScan(ctx, flowCtx, exec)
		if err != nil {
			return err
		}

		deduplicateResult, err := f.runDeduplicate(ctx, flowCtx, exec, scanResult)
		if err != nil {
			return err
		}

		newVersions, err := f.processNewFiles(ctx, flowCtx, exec, deduplicateResult.FilesToProcess)
		if err != nil {
			return err
		}

		if err := f.finalizeSnapshots(ctx, flowCtx, exec, scanResult, deduplicateResult.ExistingVersions, newVersions); err != nil {
			return err
		}

		if err := f.buildVectorIndices(ctx, flowCtx, exec); err != nil {
			return err
		}

		// Step 5.1.8: Flow completion
		flowCtx.SendCompletedWithDetails("I've rebuilt the search index", "mode=full")
		return nil
	}
}

func (f *Factory) validateFlowContext(ctx context.Context, flowCtx *executor.Context) (executor.Executor, error) {
	if f == nil {
		return nil, fmt.Errorf("factory is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if flowCtx == nil {
		return nil, fmt.Errorf("flow context is nil")
	}
	exec := flowCtx.GetExecutor()
	if exec == nil {
		return nil, fmt.Errorf("executor not available in flow context")
	}
	return exec, nil
}

func (f *Factory) runCleanup(ctx context.Context, flowCtx *executor.Context, exec executor.Executor) error {
	cleanupActivity := f.cleanupFactory.NewActivity()
	if _, err := executor.ExecuteActivity(ctx, exec, flowCtx, "Cleanup", cleanupActivity, struct{}{}); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}
	return nil
}

func (f *Factory) runScan(ctx context.Context, flowCtx *executor.Context, exec executor.Executor) (*scanworktree.Output, error) {
	scanActivity := f.scanStateFactory.NewActivity()
	scanResult, err := executor.ExecuteActivity(ctx, exec, flowCtx, "ScanState", scanActivity, struct{}{})
	if err != nil {
		return nil, fmt.Errorf("workspace scan failed: %w", err)
	}
	return scanResult, nil
}

func (f *Factory) runDeduplicate(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	scanResult *scanworktree.Output,
) (*prepareindex.Output, error) {
	deduplicateActivity := f.deduplicateAndPrepareFactory.NewActivity()
	deduplicateInput := &prepareindex.Input{
		WorkspaceState: scanResult,
	}
	deduplicateResult, err := executor.ExecuteActivity(ctx, exec, flowCtx, "DeduplicateAndPrepare", deduplicateActivity, deduplicateInput)
	if err != nil {
		return nil, fmt.Errorf("deduplication failed: %w", err)
	}
	return deduplicateResult, nil
}

func (f *Factory) processNewFiles(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	filesToProcess []domainindex.FileToProcess,
) (map[string]int64, error) {
	if len(filesToProcess) == 0 {
		flowCtx.SendCompletedWithDetails("I've already indexed everything", "files=0")
		return make(map[string]int64), nil
	}

	flowCtx.SendRunningWithDetails("I'm indexing files", fmt.Sprintf("files=%d", len(filesToProcess)))

	chunkActivity := f.chunkAllFilesFactory.NewActivity()
	chunkInput := &splitfiles.Input{
		StateName: "Deduplicated",
		Files:     filesToProcess,
	}
	chunkResults, err := executor.ExecuteActivity(ctx, exec, flowCtx, "ChunkAll_Unique", chunkActivity, chunkInput)
	if err != nil {
		return nil, fmt.Errorf("chunking failed: %w", err)
	}

	prepareBatchesActivity := f.prepareBatchesFactory.NewActivity()
	prepareBatchesInput := &buildbatches.Input{
		StateName:    "Deduplicated",
		ChunkResults: chunkResults,
		BatchSize:    f.batchSize,
	}
	preparedBatches, err := executor.ExecuteActivity(ctx, exec, flowCtx, "PrepareBatches_Unique", prepareBatchesActivity, prepareBatchesInput)
	if err != nil {
		return nil, fmt.Errorf("batch preparation failed: %w", err)
	}

	computeActivity := f.computeAllEmbeddingsFactory.NewActivity()
	computeInput := &embedall.Input{
		StateName:       "Deduplicated",
		PreparedBatches: preparedBatches,
	}
	embeddingResults, err := executor.ExecuteActivity(ctx, exec, flowCtx, "ComputeEmbeddings_Unique", computeActivity, computeInput)
	if err != nil {
		return nil, fmt.Errorf("embedding computation failed: %w", err)
	}

	saveActivity := f.distributeAndSaveFactory.NewActivity()
	saveInput := &savechunks.Input{
		StateName:        "Deduplicated",
		EmbeddingResults: embeddingResults,
	}
	saveResults, err := executor.ExecuteActivity(ctx, exec, flowCtx, "DistributeAndSave_Unique", saveActivity, saveInput)
	if err != nil {
		return nil, fmt.Errorf("save failed: %w", err)
	}

	flowCtx.SendCompletedWithDetails("I've indexed the files", fmt.Sprintf("count=%d", len(saveResults.VersionMap)))
	return saveResults.VersionMap, nil
}

func (f *Factory) finalizeSnapshots(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	scanResult *scanworktree.Output,
	existingVersions map[string]int64,
	newVersions map[string]int64,
) error {
	finalizeSnapshotsActivity := f.finalizeSnapshotsFactory.NewActivity()
	finalizeInput := &finalizeindex.Input{
		ScanResult:       scanResult,
		ExistingVersions: existingVersions,
		NewVersions:      newVersions,
	}
	if _, err := executor.ExecuteActivity(ctx, exec, flowCtx, "FinalizeSnapshots", finalizeSnapshotsActivity, finalizeInput); err != nil {
		return fmt.Errorf("snapshots finalization failed: %w", err)
	}
	return nil
}

func (f *Factory) buildVectorIndices(ctx context.Context, flowCtx *executor.Context, exec executor.Executor) error {
	buildVectorIndicesActivity := f.buildVectorIndicesFactory.NewActivity()
	if _, err := executor.ExecuteActivity(ctx, exec, flowCtx, "BuildVectorIndices", buildVectorIndicesActivity, struct{}{}); err != nil {
		return fmt.Errorf("vector indices build failed: %w", err)
	}
	return nil
}
