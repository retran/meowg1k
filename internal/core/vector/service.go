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

package vector

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"github.com/coder/hnsw"

	"github.com/retran/meowg1k/internal/ports"
)

// IndexDump contains the serialized HNSW graph and mapping data.
type IndexDump struct {
	HNSWData    []byte
	IDToChunkID map[uint32]int64
	ChunkIDToID map[int64]uint32
}

// Service implements VectorIndexService.
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

// BuildAndSave builds a vector index for the given snapshot and saves it.
func (s *Service) BuildAndSave(snapshotName string) error {
	ctx := context.Background()

	// Step 1: Get all version IDs for the snapshot
	versionIDs, err := s.snapshotRepo.GetVersionIDsForSnapshot(ctx, snapshotName)
	if err != nil {
		return fmt.Errorf("failed to get version IDs for snapshot %s: %w", snapshotName, err)
	}

	if len(versionIDs) == 0 {
		return fmt.Errorf("no versions found for snapshot %s", snapshotName)
	}

	// Step 2: Get all chunks for these versions
	var allChunks []struct {
		chunkID   int64
		embedding []float32
	}

	for _, versionID := range versionIDs {
		chunks, err := s.indexRepo.GetChunksByVersionID(ctx, versionID)
		if err != nil {
			return fmt.Errorf("failed to get chunks for version %d: %w", versionID, err)
		}

		for _, chunk := range chunks {
			// Convert gateway.Embedding ([]float64) to []float32 for HNSW
			embedding := make([]float32, len(chunk.Embedding))
			for i, val := range chunk.Embedding {
				embedding[i] = float32(val)
			}

			allChunks = append(allChunks, struct {
				chunkID   int64
				embedding []float32
			}{
				chunkID:   chunk.ID,
				embedding: embedding,
			})
		}
	}

	if len(allChunks) == 0 {
		return fmt.Errorf("no chunks found for snapshot %s", snapshotName)
	}

	// Step 3: Build HNSW index
	// Create HNSW graph
	hnswIndex := hnsw.NewGraph[int64]()

	// Create mapping tables
	idToChunkID := make(map[uint32]int64, len(allChunks))
	chunkIDToID := make(map[int64]uint32, len(allChunks))

	// Add all embeddings to the index
	for i, chunk := range allChunks {
		hnswID := uint32(i)

		// Add to HNSW index using the chunk ID as the key
		node := hnsw.MakeNode(chunk.chunkID, chunk.embedding)
		hnswIndex.Add(node)

		// Add to mapping tables
		idToChunkID[hnswID] = chunk.chunkID
		chunkIDToID[chunk.chunkID] = hnswID
	}

	// Step 4: Serialize the index
	var hnswBuffer bytes.Buffer
	encoder := gob.NewEncoder(&hnswBuffer)
	if err := encoder.Encode(hnswIndex); err != nil {
		return fmt.Errorf("failed to serialize HNSW index: %w", err)
	}

	// Step 5: Create dump structure
	dump := IndexDump{
		HNSWData:    hnswBuffer.Bytes(),
		IDToChunkID: idToChunkID,
		ChunkIDToID: chunkIDToID,
	}

	// Step 6: Serialize dump using gob
	var dumpBuffer bytes.Buffer
	dumpEncoder := gob.NewEncoder(&dumpBuffer)
	if err := dumpEncoder.Encode(dump); err != nil {
		return fmt.Errorf("failed to encode index dump: %w", err)
	}

	// Step 7: Save to meta repository
	key := fmt.Sprintf("idx_dump_%s", snapshotName)
	if err := s.metaRepo.SetValue(ctx, key, dumpBuffer.Bytes()); err != nil {
		return fmt.Errorf("failed to save index dump: %w", err)
	}

	return nil
}
