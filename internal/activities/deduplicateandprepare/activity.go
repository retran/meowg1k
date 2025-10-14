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

// Package deduplicateandprepare provides an activity to deduplicate files and prepare them for processing.
package deduplicateandprepare

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input contains the workspace state to process.
type Input struct {
	WorkspaceState *scanworkspacestate.Output
}

// Output contains deduplicated files split into existing and new files.
type Output struct {
	// ExistingVersions maps content hash to existing version IDs for files that are already indexed
	ExistingVersions map[string]int64

	// FilesToProcess contains files that need to be chunked, embedded, and saved
	// Maps a synthetic file path (first encountered) to file state (only unique files not in DB)
	FilesToProcess map[string]domainindex.FileState

	// ContentHashToVersionID maps content hash to version ID for all files (used in finalization)
	// Will be populated with both existing and new versions
	ContentHashMap map[string]string // filePath -> contentHash (for all files in all states)
}

// Factory creates instances of the DeduplicateAndPrepare activity with injected dependencies.
type Factory struct {
	indexRepo ports.IndexRepository
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new DeduplicateAndPrepare activity factory.
func NewFactory(indexRepo ports.IndexRepository) (executor.ActivityFactory[*Input, *Output], error) {
	if indexRepo == nil {
		return nil, fmt.Errorf("deduplicateandprepare.NewFactory: indexRepo cannot be nil")
	}

	return &Factory{
		indexRepo: indexRepo,
	}, nil
}

// NewActivity creates and returns the DeduplicateAndPrepare activity function.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning("Deduplicating files and preparing for processing...")

		// Step 1: Collect all unique content hashes from all three states (HEAD, Stage, Workdir)
		// Also build a map of all file paths to content hashes
		uniqueContentHashes := make(map[string]struct {
			fileState domainindex.FileState
			firstPath string
		}) // contentHash -> {FileState, firstPath}
		contentHashMap := make(map[string]string) // filePath -> contentHash

		// Collect from HEAD
		for filePath, fileState := range input.WorkspaceState.HeadState {
			contentHashMap[filePath] = fileState.ContentHash
			if _, exists := uniqueContentHashes[fileState.ContentHash]; !exists {
				uniqueContentHashes[fileState.ContentHash] = struct {
					fileState domainindex.FileState
					firstPath string
				}{fileState: fileState, firstPath: filePath}
			}
		}

		// Collect from Stage
		for filePath, fileState := range input.WorkspaceState.StageState {
			contentHashMap[filePath] = fileState.ContentHash
			if _, exists := uniqueContentHashes[fileState.ContentHash]; !exists {
				uniqueContentHashes[fileState.ContentHash] = struct {
					fileState domainindex.FileState
					firstPath string
				}{fileState: fileState, firstPath: filePath}
			}
		}

		// Collect from Workdir
		for filePath, fileState := range input.WorkspaceState.WorkdirState {
			contentHashMap[filePath] = fileState.ContentHash
			if _, exists := uniqueContentHashes[fileState.ContentHash]; !exists {
				uniqueContentHashes[fileState.ContentHash] = struct {
					fileState domainindex.FileState
					firstPath string
				}{fileState: fileState, firstPath: filePath}
			}
		}

		executorCtx.SendRunning(fmt.Sprintf("Found %d unique content hashes across all states", len(uniqueContentHashes)))

		// Step 2: Extract list of content hashes to check
		contentHashList := make([]string, 0, len(uniqueContentHashes))
		for contentHash := range uniqueContentHashes {
			contentHashList = append(contentHashList, contentHash)
		}

		// Step 3: Query database for existing versions by content hashes
		existingVersionsMap, err := f.indexRepo.FindVersionsByContentHashes(ctx, contentHashList)
		if err != nil {
			return nil, fmt.Errorf("failed to find existing versions: %w", err)
		}

		executorCtx.SendRunning(fmt.Sprintf("Found %d content hashes already indexed in database", len(existingVersionsMap)))

		// Step 4: Split files into existing and new based on content hash
		existingVersions := make(map[string]int64)               // contentHash -> version_id
		filesToProcess := make(map[string]domainindex.FileState) // synthetic filePath -> FileState

		for contentHash, data := range uniqueContentHashes {
			if existingVersion, exists := existingVersionsMap[contentHash]; exists {
				// Content hash already indexed - use existing version
				existingVersions[contentHash] = existingVersion.ID
			} else {
				// Content hash needs to be processed - add to processing queue
				// Use first encountered file path as key for processing
				filesToProcess[data.firstPath] = data.fileState
			}
		}

		executorCtx.SendCompleted(fmt.Sprintf("Prepared %d unique files for processing, %d content hashes already indexed",
			len(filesToProcess), len(existingVersions)))

		return &Output{
			ExistingVersions: existingVersions,
			FilesToProcess:   filesToProcess,
			ContentHashMap:   contentHashMap,
		}, nil
	}
}
