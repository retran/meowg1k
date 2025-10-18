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
	"github.com/retran/meowg1k/pkg/executor"
)

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
		if f == nil {
			return fmt.Errorf("factory is nil")
		}

		if ctx == nil {
			return fmt.Errorf("context is nil")
		}

		if flowCtx == nil {
			return fmt.Errorf("flow context is nil")
		}

		flowCtx.SendRunning("Starting index reconciliation")

		exec := flowCtx.GetExecutor()
		if exec == nil {
			return fmt.Errorf("executor not available in flow context")
		}

		cleanupActivity := f.cleanupFactory.NewActivity()
		cleanupFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "Cleanup", cleanupActivity, struct{}{})
		if _, err := cleanupFuture.Get(ctx); err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}

		scanActivity := f.scanStateFactory.NewActivity()
		scanFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "ScanState", scanActivity, struct{}{})
		scanResult, err := scanFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("workspace scan failed: %w", err)
		}

		flowCtx.SendRunning("Deduplicating files")
		deduplicateActivity := f.deduplicateAndPrepareFactory.NewActivity()
		deduplicateInput := &deduplicateandprepare.Input{
			WorkspaceState: scanResult,
		}
		deduplicateFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "DeduplicateAndPrepare", deduplicateActivity, deduplicateInput)
		deduplicateResult, err := deduplicateFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("deduplication failed: %w", err)
		}

		var newVersions map[string]int64
		if len(deduplicateResult.FilesToProcess) > 0 {
			flowCtx.SendRunning(fmt.Sprintf("Processing %d unique files", len(deduplicateResult.FilesToProcess)))

			// Phase 1: Chunk all unique files
			chunkActivity := f.chunkAllFilesFactory.NewActivity()
			chunkInput := &chunkallfiles.Input{
				StateName: "Deduplicated",
				Files:     deduplicateResult.FilesToProcess,
			}
			chunkFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "ChunkAll_Unique", chunkActivity, chunkInput)
			chunkResults, err := chunkFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("chunking failed: %w", err)
			}

			// Phase 2a: Prepare batches
			prepareBatchesActivity := f.prepareBatchesFactory.NewActivity()
			prepareBatchesInput := &preparebatches.Input{
				StateName:    "Deduplicated",
				ChunkResults: chunkResults,
				BatchSize:    f.batchSize,
			}
			prepareBatchesFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "PrepareBatches_Unique", prepareBatchesActivity, prepareBatchesInput)
			preparedBatches, err := prepareBatchesFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("batch preparation failed: %w", err)
			}

			// Phase 2b: Compute embeddings for all batches (rate limiter controls concurrency)
			computeActivity := f.computeAllEmbeddingsFactory.NewActivity()
			computeInput := &computeallembeddings.Input{
				StateName:       "Deduplicated",
				PreparedBatches: preparedBatches,
			}
			computeFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "ComputeEmbeddings_Unique", computeActivity, computeInput)
			embeddingResults, err := computeFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("embedding computation failed: %w", err)
			}

			// Phase 3: Distribute embeddings and save unique documents
			saveActivity := f.distributeAndSaveFactory.NewActivity()
			saveInput := &distributeandsave.Input{
				StateName:        "Deduplicated",
				EmbeddingResults: embeddingResults,
			}
			saveFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "DistributeAndSave_Unique", saveActivity, saveInput)
			saveResults, err := saveFuture.Get(ctx)
			if err != nil {
				return fmt.Errorf("save failed: %w", err)
			}

			newVersions = saveResults.VersionMap
		} else {
			// No new files to process
			newVersions = make(map[string]int64)
			flowCtx.SendRunning("No new files to process - all files already indexed")
		}

		// Step 5.1.5: Finalize snapshots
		// Combine existing versions with newly created versions and build snapshots
		flowCtx.SendRunning("Finalizing snapshots...")

		finalizeSnapshotsActivity := f.finalizeSnapshotsFactory.NewActivity()
		finalizeInput := &finalizesnapshots.Input{
			ScanResult:       scanResult,
			ExistingVersions: deduplicateResult.ExistingVersions,
			NewVersions:      newVersions,
		}
		finalizeSnapshotsFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "FinalizeSnapshots", finalizeSnapshotsActivity, finalizeInput)
		if _, err := finalizeSnapshotsFuture.Get(ctx); err != nil {
			return fmt.Errorf("snapshots finalization failed: %w", err)
		}

		// Step 5.1.7: Build vector indices
		// Build HNSW indices for all three snapshots in parallel
		buildVectorIndicesActivity := f.buildVectorIndicesFactory.NewActivity()
		vectorIndicesFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "BuildVectorIndices", buildVectorIndicesActivity, struct{}{})
		if _, err := vectorIndicesFuture.Get(ctx); err != nil {
			return fmt.Errorf("vector indices build failed: %w", err)
		}

		// Step 5.1.8: Flow completion
		flowCtx.SendCompleted("Index reconciliation complete")
		return nil
	}
}
