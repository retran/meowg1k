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

// Package index provides flows for document indexing operations.
package index

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/buildsqlsnapshots"
	"github.com/retran/meowg1k/internal/activities/chunkallfiles"
	"github.com/retran/meowg1k/internal/activities/computeallembeddings"
	"github.com/retran/meowg1k/internal/activities/distributeandsave"
	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
	"github.com/retran/meowg1k/pkg/executor"
)

// Factory creates instances of the reconcile live context flow with injected dependencies.
type Factory struct {
	cleanupFactory              executor.ActivityFactory[struct{}, struct{}]
	scanStateFactory            executor.ActivityFactory[struct{}, *scanworkspacestate.Output]
	chunkAllFilesFactory        executor.ActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]
	computeAllEmbeddingsFactory executor.ActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]
	distributeAndSaveFactory    executor.ActivityFactory[*distributeandsave.Input, *distributeandsave.Output]
	buildSqlSnapshotsFactory    executor.ActivityFactory[*buildsqlsnapshots.Input, struct{}]
	buildVectorIndicesFactory   executor.ActivityFactory[struct{}, struct{}]
}

// NewFactory creates a new reconcile live context flow factory with injected activity factories.
func NewFactory(
	cleanupFactory executor.ActivityFactory[struct{}, struct{}],
	scanStateFactory executor.ActivityFactory[struct{}, *scanworkspacestate.Output],
	chunkAllFilesFactory executor.ActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output],
	computeAllEmbeddingsFactory executor.ActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output],
	distributeAndSaveFactory executor.ActivityFactory[*distributeandsave.Input, *distributeandsave.Output],
	buildSqlSnapshotsFactory executor.ActivityFactory[*buildsqlsnapshots.Input, struct{}],
	buildVectorIndicesFactory executor.ActivityFactory[struct{}, struct{}],
) (*Factory, error) {
	if cleanupFactory == nil {
		return nil, fmt.Errorf("cleanupFactory is nil")
	}

	if scanStateFactory == nil {
		return nil, fmt.Errorf("scanStateFactory is nil")
	}

	if chunkAllFilesFactory == nil {
		return nil, fmt.Errorf("chunkAllFilesFactory is nil")
	}

	if computeAllEmbeddingsFactory == nil {
		return nil, fmt.Errorf("computeAllEmbeddingsFactory is nil")
	}

	if distributeAndSaveFactory == nil {
		return nil, fmt.Errorf("distributeAndSaveFactory is nil")
	}

	if buildSqlSnapshotsFactory == nil {
		return nil, fmt.Errorf("buildSqlSnapshotsFactory is nil")
	}

	if buildVectorIndicesFactory == nil {
		return nil, fmt.Errorf("buildVectorIndicesFactory is nil")
	}

	return &Factory{
		cleanupFactory:              cleanupFactory,
		scanStateFactory:            scanStateFactory,
		chunkAllFilesFactory:        chunkAllFilesFactory,
		computeAllEmbeddingsFactory: computeAllEmbeddingsFactory,
		distributeAndSaveFactory:    distributeAndSaveFactory,
		buildSqlSnapshotsFactory:    buildSqlSnapshotsFactory,
		buildVectorIndicesFactory:   buildVectorIndicesFactory,
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

		flowCtx.SendRunning("Starting index reconciliation...")

		// Get executor from flow context
		exec := flowCtx.GetExecutor()
		if exec == nil {
			return fmt.Errorf("executor not available in flow context")
		}

		// Step 5.1.1: Cleanup stale data
		// Remove all previous links and dumps for live snapshots to ensure clean state
		cleanupActivity := f.cleanupFactory.NewActivity()
		cleanupFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "Cleanup", cleanupActivity, struct{}{})
		if _, err := cleanupFuture.Get(ctx); err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}

		// Step 5.1.2: Scan workspace state
		// Collect file information from HEAD, staging area, and working directory in parallel
		scanActivity := f.scanStateFactory.NewActivity()
		scanFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "ScanState", scanActivity, struct{}{})
		scanResult, err := scanFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("workspace scan failed: %w", err)
		}

		// Step 5.1.3: Process HEAD state through pipeline: chunk → embed → save
		flowCtx.SendRunning("Processing HEAD state...")

		// Phase 1: Chunk all HEAD files
		chunkHeadActivity := f.chunkAllFilesFactory.NewActivity()
		chunkHeadInput := &chunkallfiles.Input{
			StateName: "HEAD",
			Files:     scanResult.HeadState,
		}
		chunkHeadFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "ChunkAll_HEAD", chunkHeadActivity, chunkHeadInput)
		headChunkResults, err := chunkHeadFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("HEAD chunking failed: %w", err)
		}

		// Phase 2: Compute embeddings for all HEAD chunks
		computeHeadActivity := f.computeAllEmbeddingsFactory.NewActivity()
		computeHeadInput := &computeallembeddings.Input{
			StateName:    "HEAD",
			ChunkResults: headChunkResults,
			BatchSize:    0, // Single batch for all chunks
		}
		computeHeadFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "ComputeEmbeddings_HEAD", computeHeadActivity, computeHeadInput)
		headEmbeddingResults, err := computeHeadFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("HEAD embedding computation failed: %w", err)
		}

		// Phase 3: Distribute embeddings and save HEAD documents
		saveHeadActivity := f.distributeAndSaveFactory.NewActivity()
		saveHeadInput := &distributeandsave.Input{
			StateName:        "HEAD",
			EmbeddingResults: headEmbeddingResults,
		}
		saveHeadFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "DistributeAndSave_HEAD", saveHeadActivity, saveHeadInput)
		headSaveResults, err := saveHeadFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("HEAD save failed: %w", err)
		}

		// Step 5.1.4: Process Stage state through pipeline: chunk → embed → save
		flowCtx.SendRunning("Processing Stage state...")

		// Phase 1: Chunk all Stage files
		chunkStageActivity := f.chunkAllFilesFactory.NewActivity()
		chunkStageInput := &chunkallfiles.Input{
			StateName: "Stage",
			Files:     scanResult.StageState,
		}
		chunkStageFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "ChunkAll_Stage", chunkStageActivity, chunkStageInput)
		stageChunkResults, err := chunkStageFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("stage chunking failed: %w", err)
		}

		// Phase 2: Compute embeddings for all Stage chunks
		computeStageActivity := f.computeAllEmbeddingsFactory.NewActivity()
		computeStageInput := &computeallembeddings.Input{
			StateName:    "Stage",
			ChunkResults: stageChunkResults,
			BatchSize:    0, // Single batch for all chunks
		}
		computeStageFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "ComputeEmbeddings_Stage", computeStageActivity, computeStageInput)
		stageEmbeddingResults, err := computeStageFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("stage embedding computation failed: %w", err)
		}

		// Phase 3: Distribute embeddings and save Stage documents
		saveStageActivity := f.distributeAndSaveFactory.NewActivity()
		saveStageInput := &distributeandsave.Input{
			StateName:        "Stage",
			EmbeddingResults: stageEmbeddingResults,
		}
		saveStageFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "DistributeAndSave_Stage", saveStageActivity, saveStageInput)
		stageSaveResults, err := saveStageFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("stage save failed: %w", err)
		}

		// Step 5.1.5: Process Workdir state through pipeline: chunk → embed → save
		flowCtx.SendRunning("Processing Workdir state...")

		// Phase 1: Chunk all Workdir files
		chunkWorkdirActivity := f.chunkAllFilesFactory.NewActivity()
		chunkWorkdirInput := &chunkallfiles.Input{
			StateName: "Workdir",
			Files:     scanResult.WorkdirState,
		}
		chunkWorkdirFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "ChunkAll_Workdir", chunkWorkdirActivity, chunkWorkdirInput)
		workdirChunkResults, err := chunkWorkdirFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("workdir chunking failed: %w", err)
		}

		// Phase 2: Compute embeddings for all Workdir chunks
		computeWorkdirActivity := f.computeAllEmbeddingsFactory.NewActivity()
		computeWorkdirInput := &computeallembeddings.Input{
			StateName:    "Workdir",
			ChunkResults: workdirChunkResults,
			BatchSize:    0, // Single batch for all chunks
		}
		computeWorkdirFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "ComputeEmbeddings_Workdir", computeWorkdirActivity, computeWorkdirInput)
		workdirEmbeddingResults, err := computeWorkdirFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("workdir embedding computation failed: %w", err)
		}

		// Phase 3: Distribute embeddings and save Workdir documents
		saveWorkdirActivity := f.distributeAndSaveFactory.NewActivity()
		saveWorkdirInput := &distributeandsave.Input{
			StateName:        "Workdir",
			EmbeddingResults: workdirEmbeddingResults,
		}
		saveWorkdirFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "DistributeAndSave_Workdir", saveWorkdirActivity, saveWorkdirInput)
		workdirSaveResults, err := saveWorkdirFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("workdir save failed: %w", err)
		}

		// Step 5.1.6: Build SQL snapshots
		// Create atomic snapshots for HEAD, staging, and working directory in the database
		buildSqlSnapshotsActivity := f.buildSqlSnapshotsFactory.NewActivity()
		sqlInput := &buildsqlsnapshots.Input{
			WorkspaceState: scanResult,
			Versions: &buildsqlsnapshots.VersionMaps{
				HeadVersions:    headSaveResults.VersionMap,
				StageVersions:   stageSaveResults.VersionMap,
				WorkdirVersions: workdirSaveResults.VersionMap,
			},
		}
		sqlSnapshotsFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "BuildSqlSnapshots", buildSqlSnapshotsActivity, sqlInput)
		if _, err := sqlSnapshotsFuture.Get(ctx); err != nil {
			return fmt.Errorf("SQL snapshots build failed: %w", err)
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
