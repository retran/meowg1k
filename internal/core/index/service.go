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

package index

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
)

type Service struct {
	indexRepo    ports.IndexRepository
	snapshotRepo ports.SnapshotRepository
}

func NewService(
	indexRepo ports.IndexRepository,
	snapshotRepo ports.SnapshotRepository,
) (*Service, error) {
	if indexRepo == nil {
		return nil, fmt.Errorf("index.NewService: indexRepo cannot be nil")
	}
	if snapshotRepo == nil {
		return nil, fmt.Errorf("index.NewService: snapshotRepo cannot be nil")
	}

	return &Service{
		indexRepo:    indexRepo,
		snapshotRepo: snapshotRepo,
	}, nil
}

type PrepareOutput struct {
	ExistingVersions map[string]int64
	FilesToProcess   map[string]domainindex.FileState
	ContentHashMap   map[string]string
}

func (s *Service) PrepareForProcessing(
	ctx context.Context,
	workspaceState *scanworkspacestate.Output,
) (*PrepareOutput, error) {
	if workspaceState == nil {
		return nil, fmt.Errorf("workspaceState cannot be nil")
	}

	uniqueContentHashes := make(map[string]struct {
		fileState domainindex.FileState
		firstPath string
	})
	contentHashMap := make(map[string]string)

	for filePath, fileState := range workspaceState.HeadState {
		// Skip binary files
		if isLikelyBinary(fileState.Content) {
			continue
		}
		contentHashMap[filePath] = fileState.ContentHash
		if _, exists := uniqueContentHashes[fileState.ContentHash]; !exists {
			uniqueContentHashes[fileState.ContentHash] = struct {
				fileState domainindex.FileState
				firstPath string
			}{fileState: fileState, firstPath: filePath}
		}
	}

	for filePath, fileState := range workspaceState.StageState {
		// Skip binary files
		if isLikelyBinary(fileState.Content) {
			continue
		}
		contentHashMap[filePath] = fileState.ContentHash
		if _, exists := uniqueContentHashes[fileState.ContentHash]; !exists {
			uniqueContentHashes[fileState.ContentHash] = struct {
				fileState domainindex.FileState
				firstPath string
			}{fileState: fileState, firstPath: filePath}
		}
	}

	for filePath, fileState := range workspaceState.WorkdirState {
		// Skip binary files
		if isLikelyBinary(fileState.Content) {
			continue
		}
		contentHashMap[filePath] = fileState.ContentHash
		if _, exists := uniqueContentHashes[fileState.ContentHash]; !exists {
			uniqueContentHashes[fileState.ContentHash] = struct {
				fileState domainindex.FileState
				firstPath string
			}{fileState: fileState, firstPath: filePath}
		}
	}

	contentHashList := make([]string, 0, len(uniqueContentHashes))
	for contentHash := range uniqueContentHashes {
		contentHashList = append(contentHashList, contentHash)
	}

	existingVersionsMap, err := s.indexRepo.FindVersionsByContentHashes(ctx, contentHashList)
	if err != nil {
		return nil, fmt.Errorf("failed to find existing versions: %w", err)
	}

	existingVersions := make(map[string]int64)
	for contentHash, version := range existingVersionsMap {
		if version != nil {
			existingVersions[contentHash] = version.ID
		}
	}

	filesToProcess := make(map[string]domainindex.FileState)
	for contentHash, entry := range uniqueContentHashes {
		if _, exists := existingVersions[contentHash]; !exists {
			filesToProcess[entry.firstPath] = entry.fileState
		}
	}

	return &PrepareOutput{
		ExistingVersions: existingVersions,
		FilesToProcess:   filesToProcess,
		ContentHashMap:   contentHashMap,
	}, nil
}

type SaveVersionInput struct {
	FilePath    string
	Content     []byte
	ContentHash string
	Chunks      []domainindex.ChunkData
	Embeddings  []gateway.Embedding
}

type SaveVersionOutput struct {
	FilePath  string
	VersionID int64
}

func (s *Service) SaveNewVersion(
	ctx context.Context,
	input *SaveVersionInput,
) (*SaveVersionOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}

	if len(input.Chunks) != len(input.Embeddings) {
		return nil, fmt.Errorf("chunk count (%d) does not match embedding count (%d)", len(input.Chunks), len(input.Embeddings))
	}

	docVersion := domainindex.DocumentVersion{
		FilePath:               input.FilePath,
		GitCommitHashFirstSeen: sql.NullString{Valid: false},
		ContentHash:            input.ContentHash,
	}

	// Prepare chunks if any
	var chunks []domainindex.Chunk
	if len(input.Chunks) > 0 {
		chunks = make([]domainindex.Chunk, len(input.Chunks))
		for i, chunkData := range input.Chunks {
			chunks[i] = domainindex.Chunk{
				// DocumentVersionID will be set by the repository
				ChunkType:   "plain_text",
				StartLine:   chunkData.StartLine,
				EndLine:     chunkData.EndLine,
				StartByte:   chunkData.StartByte,
				EndByte:     chunkData.EndByte,
				StartRune:   chunkData.StartRune,
				EndRune:     chunkData.EndRune,
				TextContent: chunkData.TextContent,
				Embedding:   input.Embeddings[i],
			}
		}
	}

	// Save document version and chunks in a single transaction
	versionID, err := s.indexRepo.AddDocumentVersionWithChunks(ctx, docVersion, input.Content, chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to add document version with chunks: %w", err)
	}

	return &SaveVersionOutput{
		FilePath:  input.FilePath,
		VersionID: versionID,
	}, nil
}

type FinalizeInput struct {
	ScanResult       *scanworkspacestate.Output
	ExistingVersions map[string]int64
	NewVersions      map[string]int64
}

func (s *Service) FinalizeLiveSnapshots(
	ctx context.Context,
	input *FinalizeInput,
) error {
	if input == nil {
		return fmt.Errorf("input cannot be nil")
	}
	if input.ScanResult == nil {
		return fmt.Errorf("scanResult cannot be nil")
	}

	allVersions := make(map[string]int64)
	for contentHash, versionID := range input.ExistingVersions {
		allVersions[contentHash] = versionID
	}
	for contentHash, versionID := range input.NewVersions {
		allVersions[contentHash] = versionID
	}

	if err := s.finalizeSnapshot(ctx, "_head_", input.ScanResult.HeadState, allVersions); err != nil {
		return fmt.Errorf("failed to finalize _head_ snapshot: %w", err)
	}

	if err := s.finalizeSnapshot(ctx, "_stage_", input.ScanResult.StageState, allVersions); err != nil {
		return fmt.Errorf("failed to finalize _stage_ snapshot: %w", err)
	}

	if err := s.finalizeSnapshot(ctx, "_workdir_", input.ScanResult.WorkdirState, allVersions); err != nil {
		return fmt.Errorf("failed to finalize _workdir_ snapshot: %w", err)
	}

	return nil
}

func (s *Service) finalizeSnapshot(
	ctx context.Context,
	snapshotName string,
	fileStates map[string]domainindex.FileState,
	versionMap map[string]int64,
) error {
	if err := s.snapshotRepo.ClearSnapshotLinks(ctx, snapshotName); err != nil {
		return fmt.Errorf("failed to clear snapshot links for %s: %w", snapshotName, err)
	}

	for filePath, fileState := range fileStates {
		// Skip binary files - they are not indexed
		if isLikelyBinary(fileState.Content) {
			continue
		}

		versionID, exists := versionMap[fileState.ContentHash]
		if !exists {
			// Version not in our cache - query from database
			// This can happen if the file was saved in a previous run or by another process
			version, err := s.indexRepo.FindVersionByContentHash(ctx, filePath, fileState.ContentHash)
			if err != nil {
				return fmt.Errorf("failed to find version for content hash %s (file: %s): %w", fileState.ContentHash, filePath, err)
			}
			if version == nil {
				return fmt.Errorf("no version found for content hash %s (file: %s) in %s", fileState.ContentHash, filePath, snapshotName)
			}
			versionID = version.ID
			// Cache it for future iterations
			versionMap[fileState.ContentHash] = versionID
		}

		if err := s.snapshotRepo.LinkVersionToSnapshot(ctx, snapshotName, versionID); err != nil {
			return fmt.Errorf("failed to link version %d to snapshot %s: %w", versionID, snapshotName, err)
		}
	}

	return nil
}

// isLikelyBinary checks if content appears to be binary (non-text) data.
// It uses a simple heuristic: if the first 512 bytes (or less) contain
// a null byte or have too many non-printable characters, it's likely binary.
func isLikelyBinary(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	// Check up to first 512 bytes
	checkLen := len(content)
	if checkLen > 512 {
		checkLen = 512
	}

	sample := content[:checkLen]

	// If contains null byte, it's binary
	if bytes.IndexByte(sample, 0) != -1 {
		return true
	}

	// Count non-printable characters
	nonPrintable := 0
	for _, b := range sample {
		// Allow common text characters: printable ASCII, tabs, newlines, carriage returns
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			nonPrintable++
		} else if b == 127 || b > 127 {
			// DEL character or non-ASCII (could be UTF-8, but be conservative)
			nonPrintable++
		}
	}

	// If more than 30% non-printable, consider it binary
	threshold := checkLen * 30 / 100
	return nonPrintable > threshold
}
