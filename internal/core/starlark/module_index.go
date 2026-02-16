// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
)

// IndexServices holds references to index-related services.
type IndexServices struct {
	IndexRepo          ports.IndexRepository
	SnapshotRepo       ports.SnapshotRepository
	VectorIndexService ports.VectorIndexService
}

// SetIndexServices configures the index module with required services.
func (r *Runtime) SetIndexServices(services *IndexServices) {
	r.indexServices = services
}

// createIndexModule creates the index built-in module.
func (r *Runtime) createIndexModule() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		// Functions
		"find_versions":      starlark.NewBuiltin("find_versions", r.indexFindVersions),
		"save_version":       starlark.NewBuiltin("save_version", r.indexSaveVersion),
		"link_snapshot":      starlark.NewBuiltin("link_snapshot", r.indexLinkSnapshot),
		"clear_snapshot":     starlark.NewBuiltin("clear_snapshot", r.indexClearSnapshot),
		"build_vector_index": starlark.NewBuiltin("build_vector_index", r.indexBuildVectorIndex),

		// Constants - Chunking strategies
		"STRATEGY_FIXED":    starlark.String("fixed"),
		"STRATEGY_SEMANTIC": starlark.String("semantic"),
		"STRATEGY_AST":      starlark.String("ast"),
	})
}

// indexFindVersions implements index.find_versions().
// Returns a dict mapping content hashes to version IDs (or None if not found).
func (r *Runtime) indexFindVersions(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var contentHashes *starlark.List

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"content_hashes", &contentHashes,
	); err != nil {
		return nil, err
	}

	if r.indexServices == nil || r.indexServices.IndexRepo == nil {
		return nil, fmt.Errorf("index services not configured")
	}

	// Convert Starlark list to Go slice
	hashes := []string{}
	for i := 0; i < contentHashes.Len(); i++ {
		if str, ok := contentHashes.Index(i).(starlark.String); ok {
			hashes = append(hashes, string(str))
		}
	}

	ctx := context.Background()

	// Find versions
	versions, err := r.indexServices.IndexRepo.FindVersionsByContentHashes(ctx, hashes)
	if err != nil {
		return nil, fmt.Errorf("failed to find versions: %w", err)
	}

	// Build result dict
	result := starlark.NewDict(len(hashes))
	for _, hash := range hashes {
		version, exists := versions[hash]
		if exists && version != nil {
			result.SetKey(starlark.String(hash), starlark.MakeInt64(version.ID))
		} else {
			result.SetKey(starlark.String(hash), starlark.None)
		}
	}

	return result, nil
}

// indexSaveVersion implements index.save_version().
// Saves a document version with chunks and embeddings, returns version_id.
func (r *Runtime) indexSaveVersion(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path, content, contentHash string
	var chunkslist *starlark.List
	var embeddingslist *starlark.List

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"path", &path,
		"content", &content,
		"content_hash", &contentHash,
		"chunks", &chunkslist,
		"embeddings", &embeddingslist,
	); err != nil {
		return nil, err
	}

	if r.indexServices == nil || r.indexServices.IndexRepo == nil {
		return nil, fmt.Errorf("index services not configured")
	}

	if chunkslist.Len() != embeddingslist.Len() {
		return nil, fmt.Errorf("chunks and embeddings must have same length, got %d and %d", chunkslist.Len(), embeddingslist.Len())
	}

	// Convert Starlark chunks to Go ChunkData
	chunks := make([]domainindex.ChunkData, chunkslist.Len())
	for i := 0; i < chunkslist.Len(); i++ {
		chunkDict, ok := chunkslist.Index(i).(*starlark.Dict)
		if !ok {
			return nil, fmt.Errorf("chunk %d is not a dict", i)
		}

		chunk, err := convertStarlarkChunkToChunkData(chunkDict)
		if err != nil {
			return nil, fmt.Errorf("failed to convert chunk %d: %w", i, err)
		}
		chunks[i] = chunk
	}

	// Convert Starlark embeddings to Go Embeddings
	embeddings := make([]gateway.Embedding, embeddingslist.Len())
	for i := 0; i < embeddingslist.Len(); i++ {
		embList, ok := embeddingslist.Index(i).(*starlark.List)
		if !ok {
			return nil, fmt.Errorf("embedding %d is not a list", i)
		}

		emb, err := convertStarlarkListToEmbedding(embList)
		if err != nil {
			return nil, fmt.Errorf("failed to convert embedding %d: %w", i, err)
		}
		embeddings[i] = emb
	}

	ctx := context.Background()

	// Create document version
	docVersion := domainindex.DocumentVersion{
		FilePath:               path,
		ContentHash:            contentHash,
		GitCommitHashFirstSeen: sql.NullString{Valid: false},
	}

	// Prepare chunks with embeddings
	dbChunks := make([]domainindex.Chunk, len(chunks))
	for i, chunkData := range chunks {
		dbChunks[i] = domainindex.Chunk{
			ChunkType:   "plain_text",
			StartLine:   chunkData.StartLine,
			EndLine:     chunkData.EndLine,
			StartByte:   chunkData.StartByte,
			EndByte:     chunkData.EndByte,
			StartRune:   chunkData.StartRune,
			EndRune:     chunkData.EndRune,
			TextContent: chunkData.TextContent,
			Embedding:   embeddings[i],
		}
	}

	// Save to database
	versionID, err := r.indexServices.IndexRepo.AddDocumentVersionWithChunks(ctx, &docVersion, []byte(content), dbChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	return starlark.MakeInt64(versionID), nil
}

// indexLinkSnapshot implements index.link_snapshot().
// Links document versions to a snapshot.
func (r *Runtime) indexLinkSnapshot(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var snapshot string
	var versionIDs *starlark.List

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"snapshot", &snapshot,
		"version_ids", &versionIDs,
	); err != nil {
		return nil, err
	}

	if r.indexServices == nil || r.indexServices.SnapshotRepo == nil {
		return nil, fmt.Errorf("index services not configured")
	}

	ctx := context.Background()

	// Link each version to snapshot
	for i := 0; i < versionIDs.Len(); i++ {
		versionID, ok := versionIDs.Index(i).(starlark.Int)
		if !ok {
			return nil, fmt.Errorf("version_id %d is not an int", i)
		}

		id, ok := versionID.Int64()
		if !ok {
			return nil, fmt.Errorf("version_id %d too large", i)
		}

		// Use internal snapshot name mapping
		snapshotName := mapSnapshotName(snapshot)

		if err := r.indexServices.SnapshotRepo.LinkVersionToSnapshot(ctx, snapshotName, id); err != nil {
			return nil, fmt.Errorf("failed to link version %d to snapshot %s: %w", id, snapshot, err)
		}
	}

	return starlark.None, nil
}

// indexClearSnapshot implements index.clear_snapshot().
// Clears all links for a snapshot.
func (r *Runtime) indexClearSnapshot(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var snapshot string

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"snapshot", &snapshot,
	); err != nil {
		return nil, err
	}

	if r.indexServices == nil || r.indexServices.SnapshotRepo == nil {
		return nil, fmt.Errorf("index services not configured")
	}

	ctx := context.Background()

	// Use internal snapshot name mapping
	snapshotName := mapSnapshotName(snapshot)

	if err := r.indexServices.SnapshotRepo.ClearSnapshotLinks(ctx, snapshotName); err != nil {
		return nil, fmt.Errorf("failed to clear snapshot %s: %w", snapshot, err)
	}

	return starlark.None, nil
}

// indexBuildVectorIndex implements index.build_vector_index().
// Builds HNSW vector index for a snapshot.
func (r *Runtime) indexBuildVectorIndex(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var snapshot string

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"snapshot", &snapshot,
	); err != nil {
		return nil, err
	}

	if r.indexServices == nil || r.indexServices.VectorIndexService == nil {
		return nil, fmt.Errorf("index services not configured")
	}

	// Use internal snapshot name mapping
	snapshotName := mapSnapshotName(snapshot)

	if err := r.indexServices.VectorIndexService.BuildAndSave(snapshotName); err != nil {
		return nil, fmt.Errorf("failed to build vector index for %s: %w", snapshot, err)
	}

	return starlark.None, nil
}

// Helper functions

// mapSnapshotName maps user-facing snapshot names to internal names.
func mapSnapshotName(snapshot string) string {
	switch snapshot {
	case "HEAD", "head":
		return "_head_"
	case "stage", "staged":
		return "_stage_"
	case "workdir", "working":
		return "_workdir_"
	default:
		return snapshot
	}
}

// convertStarlarkChunkToChunkData converts a Starlark dict to ChunkData.
func convertStarlarkChunkToChunkData(chunkDict *starlark.Dict) (domainindex.ChunkData, error) {
	var chunk domainindex.ChunkData

	// Get text
	textVal, found, err := chunkDict.Get(starlark.String("text"))
	if err != nil || !found {
		return chunk, fmt.Errorf("chunk missing 'text' field")
	}
	textStr, ok := textVal.(starlark.String)
	if !ok {
		return chunk, fmt.Errorf("chunk 'text' must be string")
	}
	chunk.TextContent = string(textStr)

	// Get optional line numbers
	if val, found, _ := chunkDict.Get(starlark.String("start_line")); found {
		if intVal, ok := val.(starlark.Int); ok {
			if i, ok := intVal.Int64(); ok {
				chunk.StartLine = int(i)
			}
		}
	}

	if val, found, _ := chunkDict.Get(starlark.String("end_line")); found {
		if intVal, ok := val.(starlark.Int); ok {
			if i, ok := intVal.Int64(); ok {
				chunk.EndLine = int(i)
			}
		}
	}

	// Get optional byte positions
	if val, found, _ := chunkDict.Get(starlark.String("start_byte")); found {
		if intVal, ok := val.(starlark.Int); ok {
			if i, ok := intVal.Int64(); ok {
				chunk.StartByte = int(i)
			}
		}
	}

	if val, found, _ := chunkDict.Get(starlark.String("end_byte")); found {
		if intVal, ok := val.(starlark.Int); ok {
			if i, ok := intVal.Int64(); ok {
				chunk.EndByte = int(i)
			}
		}
	}

	// Get optional rune positions
	if val, found, _ := chunkDict.Get(starlark.String("start_rune")); found {
		if intVal, ok := val.(starlark.Int); ok {
			if i, ok := intVal.Int64(); ok {
				chunk.StartRune = int(i)
			}
		}
	}

	if val, found, _ := chunkDict.Get(starlark.String("end_rune")); found {
		if intVal, ok := val.(starlark.Int); ok {
			if i, ok := intVal.Int64(); ok {
				chunk.EndRune = int(i)
			}
		}
	}

	return chunk, nil
}

// convertStarlarkListToEmbedding converts a Starlark list of floats to an Embedding.
func convertStarlarkListToEmbedding(embList *starlark.List) (gateway.Embedding, error) {
	embedding := make(gateway.Embedding, embList.Len())

	for i := 0; i < embList.Len(); i++ {
		val := embList.Index(i)

		var floatVal float64
		switch v := val.(type) {
		case starlark.Float:
			floatVal = float64(v)
		case starlark.Int:
			if i64, ok := v.Int64(); ok {
				floatVal = float64(i64)
			} else {
				return nil, fmt.Errorf("embedding value %d too large", i)
			}
		default:
			return nil, fmt.Errorf("embedding value %d must be float or int, got %T", i, val)
		}

		embedding[i] = floatVal
	}

	return embedding, nil
}

// computeContentHashFromString computes SHA256 hash of string content.
func computeContentHashFromString(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
