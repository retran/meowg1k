// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package vector

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"github.com/coder/hnsw"

	"github.com/retran/meowg1k/internal/ports"
)

// IndexDump stores serialized HNSW data with ID mappings.
type IndexDump struct {
	IDToChunkID map[uint32]int64
	ChunkIDToID map[int64]uint32
	HNSWData    []byte
}

// Service builds and stores vector indices for snapshots.
type Service struct {
	indexRepo    ports.IndexRepository
	snapshotRepo ports.SnapshotRepository
	metaRepo     ports.MetaRepository
}

// NewService creates a new vector index service.
func NewService(
	indexRepo ports.IndexRepository,
	snapshotRepo ports.SnapshotRepository,
	metaRepo ports.MetaRepository,
) *Service {
	return &Service{
		indexRepo:    indexRepo,
		snapshotRepo: snapshotRepo,
		metaRepo:     metaRepo,
	}
}

// BuildAndSave builds the vector index for a snapshot and stores it in meta storage.
func (s *Service) BuildAndSave(snapshotName string) error {
	ctx := context.Background()

	versionIDs, err := s.snapshotRepo.GetVersionIDsForSnapshot(ctx, snapshotName)
	if err != nil {
		return fmt.Errorf("failed to get version IDs for snapshot %s: %w", snapshotName, err)
	}

	if len(versionIDs) == 0 {
		return s.clearIndexDump(ctx, snapshotName)
	}

	allChunks, err := s.collectChunks(ctx, versionIDs)
	if err != nil {
		return err
	}

	if len(allChunks) == 0 {
		return fmt.Errorf("no chunks found for snapshot %s", snapshotName)
	}

	dump, err := buildIndexDump(allChunks)
	if err != nil {
		return err
	}

	serialized, err := serializeIndexDump(dump)
	if err != nil {
		return err
	}

	return s.saveIndexDump(ctx, snapshotName, serialized)
}

type indexedChunk struct {
	embedding []float32
	chunkID   int64
}

func (s *Service) collectChunks(ctx context.Context, versionIDs []int64) ([]indexedChunk, error) {
	var allChunks []indexedChunk

	for _, versionID := range versionIDs {
		chunks, err := s.indexRepo.GetChunksByVersionID(ctx, versionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get chunks for version %d: %w", versionID, err)
		}

		for _, chunk := range chunks {
			embedding := make([]float32, len(chunk.Embedding))
			for i, val := range chunk.Embedding {
				embedding[i] = float32(val)
			}

			allChunks = append(allChunks, indexedChunk{
				chunkID:   chunk.ID,
				embedding: embedding,
			})
		}
	}

	return allChunks, nil
}

func buildIndexDump(chunks []indexedChunk) (IndexDump, error) {
	hnswIndex := hnsw.NewGraph[int64]()

	idToChunkID := make(map[uint32]int64, len(chunks))
	chunkIDToID := make(map[int64]uint32, len(chunks))

	for i, chunk := range chunks {
		if i > int(^uint32(0)) {
			return IndexDump{}, fmt.Errorf("too many chunks (%d) to fit in uint32 index", len(chunks))
		}
		// #nosec G115 -- overflow is checked above
		hnswID := uint32(i)

		node := hnsw.MakeNode(chunk.chunkID, chunk.embedding)
		hnswIndex.Add(node)

		idToChunkID[hnswID] = chunk.chunkID
		chunkIDToID[chunk.chunkID] = hnswID
	}

	var hnswBuffer bytes.Buffer
	if err := hnswIndex.Export(&hnswBuffer); err != nil {
		return IndexDump{}, fmt.Errorf("failed to export HNSW index: %w", err)
	}

	return IndexDump{
		HNSWData:    hnswBuffer.Bytes(),
		IDToChunkID: idToChunkID,
		ChunkIDToID: chunkIDToID,
	}, nil
}

func serializeIndexDump(dump IndexDump) ([]byte, error) {
	var dumpBuffer bytes.Buffer
	dumpEncoder := gob.NewEncoder(&dumpBuffer)
	if err := dumpEncoder.Encode(dump); err != nil {
		return nil, fmt.Errorf("failed to encode index dump: %w", err)
	}
	return dumpBuffer.Bytes(), nil
}

func (s *Service) saveIndexDump(ctx context.Context, snapshotName string, data []byte) error {
	key := fmt.Sprintf("idx_dump_%s", snapshotName)
	if err := s.metaRepo.SetValue(ctx, key, data); err != nil {
		return fmt.Errorf("failed to save index dump: %w", err)
	}
	return nil
}

func (s *Service) clearIndexDump(ctx context.Context, snapshotName string) error {
	key := fmt.Sprintf("idx_dump_%s", snapshotName)
	_ = s.metaRepo.DeleteValue(ctx, key) //nolint:errcheck // Ignore error if key doesn't exist
	return nil
}
