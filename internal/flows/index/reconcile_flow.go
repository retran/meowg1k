// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package index implements the workflow for indexing workspace files by computing embeddings and building vector indices.
package index

import (
	"context"
	"fmt"

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

// Factory builds index reconciliation flows.
type Factory struct {
	cleanupFactory               executor.ActivityFactory[struct{}, struct{}]
	scanStateFactory             executor.ActivityFactory[struct{}, *scanworkspacestate.Output]
	deduplicateAndPrepareFactory executor.ActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]
	chunkAllFilesFactory         executor.ActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]
	prepareBatchesFactory        executor.ActivityFactory[*preparebatches.Input, *preparebatches.Output]
	computeAllEmbeddingsFactory  executor.ActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]
	distributeAndSaveFactory     executor.ActivityFactory[*distributeandsave.Input, *distributeandsave.Output]
	finalizeSnapshotsFactory     executor.ActivityFactory[*finalizesnapshots.Input, struct{}]
	buildVectorIndicesFactory    executor.ActivityFactory[struct{}, struct{}]
	batchSize                    int
}

// NewFactory creates a reconciliation flow factory.
func NewFactory(
	cleanupFactory executor.ActivityFactory[struct{}, struct{}],
	scanStateFactory executor.ActivityFactory[struct{}, *scanworkspacestate.Output],
	deduplicateAndPrepareFactory executor.ActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output],
	chunkAllFilesFactory executor.ActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output],
	prepareBatchesFactory executor.ActivityFactory[*preparebatches.Input, *preparebatches.Output],
	computeAllEmbeddingsFactory executor.ActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output],
	distributeAndSaveFactory executor.ActivityFactory[*distributeandsave.Input, *distributeandsave.Output],
	finalizeSnapshotsFactory executor.ActivityFactory[*finalizesnapshots.Input, struct{}],
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
		flowCtx.SendRunning("Starting index reconciliation")

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
		flowCtx.SendCompleted("Index reconciliation complete")
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
	cleanupFuture := executor.ExecuteActivity(ctx, exec, flowCtx, "Cleanup", cleanupActivity, struct{}{})
	if _, err := cleanupFuture.Get(ctx); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}
	return nil
}

func (f *Factory) runScan(ctx context.Context, flowCtx *executor.Context, exec executor.Executor) (*scanworkspacestate.Output, error) {
	scanActivity := f.scanStateFactory.NewActivity()
	scanFuture := executor.ExecuteActivity(ctx, exec, flowCtx, "ScanState", scanActivity, struct{}{})
	scanResult, err := scanFuture.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("workspace scan failed: %w", err)
	}
	return scanResult, nil
}

func (f *Factory) runDeduplicate(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	scanResult *scanworkspacestate.Output,
) (*deduplicateandprepare.Output, error) {
	flowCtx.SendRunning("Deduplicating files")
	deduplicateActivity := f.deduplicateAndPrepareFactory.NewActivity()
	deduplicateInput := &deduplicateandprepare.Input{
		WorkspaceState: scanResult,
	}
	deduplicateFuture := executor.ExecuteActivity(ctx, exec, flowCtx, "DeduplicateAndPrepare", deduplicateActivity, deduplicateInput)
	deduplicateResult, err := deduplicateFuture.Get(ctx)
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
		flowCtx.SendRunning("No new files to process - all files already indexed")
		return make(map[string]int64), nil
	}

	flowCtx.SendRunning(fmt.Sprintf("Processing %d unique files", len(filesToProcess)))

	chunkActivity := f.chunkAllFilesFactory.NewActivity()
	chunkInput := &chunkallfiles.Input{
		StateName: "Deduplicated",
		Files:     filesToProcess,
	}
	chunkFuture := executor.ExecuteActivity(ctx, exec, flowCtx, "ChunkAll_Unique", chunkActivity, chunkInput)
	chunkResults, err := chunkFuture.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("chunking failed: %w", err)
	}

	prepareBatchesActivity := f.prepareBatchesFactory.NewActivity()
	prepareBatchesInput := &preparebatches.Input{
		StateName:    "Deduplicated",
		ChunkResults: chunkResults,
		BatchSize:    f.batchSize,
	}
	prepareBatchesFuture := executor.ExecuteActivity(ctx, exec, flowCtx, "PrepareBatches_Unique", prepareBatchesActivity, prepareBatchesInput)
	preparedBatches, err := prepareBatchesFuture.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("batch preparation failed: %w", err)
	}

	computeActivity := f.computeAllEmbeddingsFactory.NewActivity()
	computeInput := &computeallembeddings.Input{
		StateName:       "Deduplicated",
		PreparedBatches: preparedBatches,
	}
	computeFuture := executor.ExecuteActivity(ctx, exec, flowCtx, "ComputeEmbeddings_Unique", computeActivity, computeInput)
	embeddingResults, err := computeFuture.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("embedding computation failed: %w", err)
	}

	saveActivity := f.distributeAndSaveFactory.NewActivity()
	saveInput := &distributeandsave.Input{
		StateName:        "Deduplicated",
		EmbeddingResults: embeddingResults,
	}
	saveFuture := executor.ExecuteActivity(ctx, exec, flowCtx, "DistributeAndSave_Unique", saveActivity, saveInput)
	saveResults, err := saveFuture.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("save failed: %w", err)
	}

	return saveResults.VersionMap, nil
}

func (f *Factory) finalizeSnapshots(
	ctx context.Context,
	flowCtx *executor.Context,
	exec executor.Executor,
	scanResult *scanworkspacestate.Output,
	existingVersions map[string]int64,
	newVersions map[string]int64,
) error {
	flowCtx.SendRunning("Finalizing snapshots...")

	finalizeSnapshotsActivity := f.finalizeSnapshotsFactory.NewActivity()
	finalizeInput := &finalizesnapshots.Input{
		ScanResult:       scanResult,
		ExistingVersions: existingVersions,
		NewVersions:      newVersions,
	}
	finalizeSnapshotsFuture := executor.ExecuteActivity(ctx, exec, flowCtx, "FinalizeSnapshots", finalizeSnapshotsActivity, finalizeInput)
	if _, err := finalizeSnapshotsFuture.Get(ctx); err != nil {
		return fmt.Errorf("snapshots finalization failed: %w", err)
	}
	return nil
}

func (f *Factory) buildVectorIndices(ctx context.Context, flowCtx *executor.Context, exec executor.Executor) error {
	buildVectorIndicesActivity := f.buildVectorIndicesFactory.NewActivity()
	vectorIndicesFuture := executor.ExecuteActivity(ctx, exec, flowCtx, "BuildVectorIndices", buildVectorIndicesActivity, struct{}{})
	if _, err := vectorIndicesFuture.Get(ctx); err != nil {
		return fmt.Errorf("vector indices build failed: %w", err)
	}
	return nil
}
