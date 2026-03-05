// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package index provides core indexing operations for documents and snapshots.
package index

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
)

// Service coordinates index persistence and snapshot reconciliation.
type Service struct {
	indexRepo    ports.IndexRepository
	snapshotRepo ports.SnapshotRepository
}

// Ensure Service implements the IndexService interface.
var _ ports.IndexService = (*Service)(nil)

// NewService creates a new index service.
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

// PrepareOutput contains files selected for indexing and lookup maps.
type PrepareOutput struct {
	ExistingVersions map[string]int64
	ContentHashMap   map[string]string
	FilesToProcess   []domainindex.FileToProcess
}

// PrepareForProcessing normalizes workspace state for indexing.
func (s *Service) PrepareForProcessing(ctx context.Context, workspaceState interface{}) (interface{}, error) {
	wsState, ok := workspaceState.(*domainindex.WorkspaceState)
	if !ok {
		return nil, fmt.Errorf("invalid workspaceState type")
	}
	return s.prepareForProcessingImpl(ctx, wsState)
}

func (s *Service) prepareForProcessingImpl(
	ctx context.Context,
	workspaceState *domainindex.WorkspaceState,
) (*PrepareOutput, error) {
	if workspaceState == nil {
		return nil, fmt.Errorf("workspaceState cannot be nil")
	}

	uniqueContentHashes := make(map[string]contentHashEntry)
	encounterOrder := make([]string, 0)
	contentHashMap := make(map[string]string)

	s.collectContentHashes(workspaceState.HeadState, uniqueContentHashes, &encounterOrder, contentHashMap)
	s.collectContentHashes(workspaceState.StageState, uniqueContentHashes, &encounterOrder, contentHashMap)
	s.collectContentHashes(workspaceState.WorkdirState, uniqueContentHashes, &encounterOrder, contentHashMap)

	contentHashList := append([]string(nil), encounterOrder...)

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

	filesToProcess := make([]domainindex.FileToProcess, 0, len(uniqueContentHashes))
	for _, contentHash := range encounterOrder {
		entry := uniqueContentHashes[contentHash]
		if _, exists := existingVersions[contentHash]; !exists {
			filesToProcess = append(filesToProcess, domainindex.FileToProcess{
				FilePath: entry.firstPath,
				State:    entry.fileState,
			})
		}
	}

	return &PrepareOutput{
		ExistingVersions: existingVersions,
		FilesToProcess:   filesToProcess,
		ContentHashMap:   contentHashMap,
	}, nil
}

type contentHashEntry struct {
	firstPath string
	fileState domainindex.FileState
}

func (s *Service) collectContentHashes(
	state map[string]domainindex.FileState,
	uniqueContentHashes map[string]contentHashEntry,
	encounterOrder *[]string,
	contentHashMap map[string]string,
) {
	for filePath, fileState := range state {
		if isLikelyBinary(fileState.Content) {
			continue
		}
		contentHashMap[filePath] = fileState.ContentHash
		if _, exists := uniqueContentHashes[fileState.ContentHash]; exists {
			continue
		}

		uniqueContentHashes[fileState.ContentHash] = contentHashEntry{
			fileState: fileState,
			firstPath: filePath,
		}
		*encounterOrder = append(*encounterOrder, fileState.ContentHash)
	}
}

// SaveVersionInput defines the payload for saving a new document version.
type SaveVersionInput struct {
	FilePath    string
	Content     []byte
	ContentHash string
	Chunks      []domainindex.ChunkData
	Embeddings  []gateway.Embedding
}

// SaveVersionOutput reports the stored version ID.
type SaveVersionOutput struct {
	FilePath  string
	VersionID int64
}

func (s *Service) saveNewVersionImpl(
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

	versionID, err := s.indexRepo.AddDocumentVersionWithChunks(ctx, &docVersion, input.Content, chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to add document version with chunks: %w", err)
	}

	return &SaveVersionOutput{
		FilePath:  input.FilePath,
		VersionID: versionID,
	}, nil
}

// FinalizeInput defines the payload for snapshot finalization.
type FinalizeInput struct {
	ScanResult       *domainindex.WorkspaceState
	ExistingVersions map[string]int64
	NewVersions      map[string]int64
}

func (s *Service) finalizeLiveSnapshotsImpl(
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
			return fmt.Errorf(
				"inconsistency detected: no versionID found for content hash %s (file: %s)",
				fileState.ContentHash,
				filePath,
			)
		}

		if err := s.snapshotRepo.LinkVersionToSnapshot(ctx, snapshotName, versionID); err != nil {
			return fmt.Errorf("failed to link version %d to snapshot %s: %w", versionID, snapshotName, err)
		}
	}

	return nil
}

// isLikelyBinary checks if content appears to be binary (non-text) data.
func isLikelyBinary(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return true
		}
		if b < 0x09 || (b > 0x0D && b < 0x20) {
			return true
		}
	}
	return false
}

// Interface wrapper methods for ports.IndexService

// SaveNewVersion implements ports.IndexService.
func (s *Service) SaveNewVersion(ctx context.Context, input interface{}) (interface{}, error) {
	saveInput, ok := input.(*SaveVersionInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type")
	}
	return s.saveNewVersionImpl(ctx, saveInput)
}

// FinalizeLiveSnapshots implements ports.IndexService.
func (s *Service) FinalizeLiveSnapshots(ctx context.Context, input interface{}) error {
	finalizeInput, ok := input.(*FinalizeInput)
	if !ok {
		return fmt.Errorf("invalid input type")
	}
	return s.finalizeLiveSnapshotsImpl(ctx, finalizeInput)
}

// computeContentHash computes SHA-256 hash of content.
func computeContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
