// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"database/sql"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/retran/meowg1k/internal/core/vector"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
)

// IndexServices holds references to index-related services.
type IndexServices struct {
	IndexRepo          ports.IndexRepository
	SnapshotRepo       ports.SnapshotRepository
	VectorIndexService ports.VectorIndexService
	SearchService      vector.Searcher
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
		"search":             starlark.NewBuiltin("search", r.indexSearch),

		// Constants - Chunking strategies
		"STRATEGY_FIXED":    starlark.String("fixed"),
		"STRATEGY_SEMANTIC": starlark.String("semantic"),
		"STRATEGY_AST":      starlark.String("ast"),
	})
}

// indexFindVersions implements index.find_versions().
// Returns a dict mapping content hashes to version IDs (or None if not found).
func (r *Runtime) indexFindVersions(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var contentHashes *starlark.List

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"content_hashes", &contentHashes,
	); err != nil {
		return nil, fmt.Errorf("index.find_versions: %w", err)
	}

	if r.indexServices == nil || r.indexServices.IndexRepo == nil {
		return nil, fmt.Errorf("index services not configured")
	}

	hashes := []string{}
	for i := 0; i < contentHashes.Len(); i++ {
		if str, ok := contentHashes.Index(i).(starlark.String); ok {
			hashes = append(hashes, string(str))
		}
	}

	ctx := context.Background()

	versions, err := r.indexServices.IndexRepo.FindVersionsByContentHashes(ctx, hashes)
	if err != nil {
		return nil, fmt.Errorf("failed to find versions: %w", err)
	}

	result := starlark.NewDict(len(hashes))
	for _, hash := range hashes {
		version, exists := versions[hash]
		if exists && version != nil {
			result.SetKey(starlark.String(hash), starlark.MakeInt64(version.ID)) //nolint:errcheck // starlark dict operations with known-compatible types
		} else {
			result.SetKey(starlark.String(hash), starlark.None) //nolint:errcheck // starlark dict operations with known-compatible types
		}
	}

	return result, nil
}

// indexSaveVersion implements index.save_version().
// Saves a document version with chunks and embeddings, returns version_id.
func (r *Runtime) indexSaveVersion(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) { //nolint:gocognit // complexity inherent in multi-step document indexing
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
		return nil, fmt.Errorf("index.save_version: %w", err)
	}

	if r.indexServices == nil || r.indexServices.IndexRepo == nil {
		return nil, fmt.Errorf("index services not configured")
	}

	if chunkslist.Len() != embeddingslist.Len() {
		return nil, fmt.Errorf("chunks and embeddings must have same length, got %d and %d", chunkslist.Len(), embeddingslist.Len())
	}

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

	docVersion := domainindex.DocumentVersion{
		FilePath:               path,
		ContentHash:            contentHash,
		GitCommitHashFirstSeen: sql.NullString{Valid: false},
	}

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

	versionID, err := r.indexServices.IndexRepo.AddDocumentVersionWithChunks(ctx, &docVersion, []byte(content), dbChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	return starlark.MakeInt64(versionID), nil
}

// indexLinkSnapshot implements index.link_snapshot().
// Links document versions to a snapshot.
func (r *Runtime) indexLinkSnapshot(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var snapshot string
	var versionIDs *starlark.List

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"snapshot", &snapshot,
		"version_ids", &versionIDs,
	); err != nil {
		return nil, fmt.Errorf("index.link_snapshot: %w", err)
	}

	if r.indexServices == nil || r.indexServices.SnapshotRepo == nil {
		return nil, fmt.Errorf("index services not configured")
	}

	ctx := context.Background()

	for i := 0; i < versionIDs.Len(); i++ {
		versionID, ok := versionIDs.Index(i).(starlark.Int)
		if !ok {
			return nil, fmt.Errorf("version_id %d is not an int", i)
		}

		id, ok := versionID.Int64()
		if !ok {
			return nil, fmt.Errorf("version_id %d too large", i)
		}

		snapshotName := mapSnapshotName(snapshot)

		if err := r.indexServices.SnapshotRepo.LinkVersionToSnapshot(ctx, snapshotName, id); err != nil {
			return nil, fmt.Errorf("failed to link version %d to snapshot %s: %w", id, snapshot, err)
		}
	}

	return starlark.None, nil
}

// indexClearSnapshot implements index.clear_snapshot().
// Clears all links for a snapshot.
func (r *Runtime) indexClearSnapshot(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var snapshot string

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"snapshot", &snapshot,
	); err != nil {
		return nil, fmt.Errorf("index.clear_snapshot: %w", err)
	}

	if r.indexServices == nil || r.indexServices.SnapshotRepo == nil {
		return nil, fmt.Errorf("index services not configured")
	}

	ctx := context.Background()

	snapshotName := mapSnapshotName(snapshot)

	if err := r.indexServices.SnapshotRepo.ClearSnapshotLinks(ctx, snapshotName); err != nil {
		return nil, fmt.Errorf("failed to clear snapshot %s: %w", snapshot, err)
	}

	return starlark.None, nil
}

// indexBuildVectorIndex implements index.build_vector_index().
// Builds HNSW vector index for a snapshot.
func (r *Runtime) indexBuildVectorIndex(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var snapshot string

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"snapshot", &snapshot,
	); err != nil {
		return nil, fmt.Errorf("index.build_vector_index: %w", err)
	}

	if r.indexServices == nil || r.indexServices.VectorIndexService == nil {
		return nil, fmt.Errorf("index services not configured")
	}

	snapshotName := mapSnapshotName(snapshot)

	if err := r.indexServices.VectorIndexService.BuildAndSave(snapshotName); err != nil {
		return nil, fmt.Errorf("failed to build vector index for %s: %w", snapshot, err)
	}

	return starlark.None, nil
}

// indexSearch implements index.search().
// Performs vector similarity search across the given snapshots.
//
//	search(embedding, snapshots, top_k, min_score) → list of structs
//	  embedding: list of floats (pre-computed query embedding from ctx.llm.embed)
//	  snapshots: list of snapshot name strings
//	  top_k:     int, maximum results per snapshot
//	  min_score: float, minimum cosine similarity threshold (0.0–1.0)
//
// Returns a list of structs with fields: file_path, start_line, end_line, score, content.
func (r *Runtime) indexSearch(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) { //nolint:gocognit,gocyclo,funlen // complexity inherent in semantic search with multiple filter options
	var embeddingList *starlark.List
	var snapshotsList *starlark.List
	var topK int
	var minScore starlark.Float

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"embedding", &embeddingList,
		"snapshots", &snapshotsList,
		"top_k", &topK,
		"min_score", &minScore,
	); err != nil {
		return nil, fmt.Errorf("index.search: %w", err)
	}

	if r.indexServices == nil || r.indexServices.SearchService == nil {
		return nil, fmt.Errorf("index search service not configured")
	}
	if r.indexServices.IndexRepo == nil {
		return nil, fmt.Errorf("index repository not configured")
	}

	// Convert Starlark embedding list to gateway.Embedding
	queryEmbedding, err := convertStarlarkListToEmbedding(embeddingList)
	if err != nil {
		return nil, fmt.Errorf("failed to convert embedding: %w", err)
	}

	// Collect snapshot name strings
	snapshots := make([]string, 0, snapshotsList.Len())
	for i := 0; i < snapshotsList.Len(); i++ {
		s, ok := snapshotsList.Index(i).(starlark.String)
		if !ok {
			return nil, fmt.Errorf("snapshots[%d] is not a string", i)
		}
		snapshots = append(snapshots, string(s))
	}

	ctx := context.Background()
	threshold := float32(minScore)

	// Search each snapshot, deduplicate by chunk ID keeping best score
	type bestResult struct {
		snapshotName string
		score        float32
	}
	best := make(map[int64]bestResult)

	for _, snap := range snapshots {
		internalName := mapSnapshotName(snap)
		results, err := r.indexServices.SearchService.Search(ctx, internalName, queryEmbedding, topK)
		if err != nil {
			// Skip snapshots with no index (not built yet) rather than failing
			continue
		}
		for _, qr := range results {
			if qr.Score < threshold {
				continue
			}
			if prev, exists := best[qr.ChunkID]; !exists || qr.Score > prev.score {
				best[qr.ChunkID] = bestResult{score: qr.Score, snapshotName: qr.SnapshotName}
			}
		}
	}

	if len(best) == 0 {
		return starlark.NewList(nil), nil
	}

	// Collect chunk IDs
	chunkIDs := make([]int64, 0, len(best))
	for id := range best {
		chunkIDs = append(chunkIDs, id)
	}

	// Batch-fetch chunks
	chunks, err := r.indexServices.IndexRepo.GetChunksByIDs(ctx, chunkIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chunks: %w", err)
	}

	// Collect version IDs for path resolution
	versionIDSet := make(map[int64]struct{})
	for _, c := range chunks {
		versionIDSet[c.DocumentVersionID] = struct{}{}
	}
	versionIDs := make([]int64, 0, len(versionIDSet))
	for id := range versionIDSet {
		versionIDs = append(versionIDs, id)
	}

	versions, err := r.indexServices.IndexRepo.GetVersionsByIDs(ctx, versionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch document versions: %w", err)
	}

	versionByID := make(map[int64]domainindex.DocumentVersion, len(versions))
	for _, v := range versions {
		versionByID[v.ID] = v
	}

	// Build result list, sorted by score descending
	type searchResult struct {
		filePath  string
		content   string
		startLine int
		endLine   int
		score     float32
	}
	resultList := make([]searchResult, 0, len(chunks))

	for _, c := range chunks {
		b, ok := best[c.ID]
		if !ok {
			continue
		}
		v, ok := versionByID[c.DocumentVersionID]
		if !ok {
			continue
		}
		resultList = append(resultList, searchResult{
			filePath:  v.FilePath,
			startLine: c.StartLine,
			endLine:   c.EndLine,
			score:     b.score,
			content:   c.TextContent,
		})
	}

	// Sort by score descending
	for i := 0; i < len(resultList); i++ {
		for j := i + 1; j < len(resultList); j++ {
			if resultList[j].score > resultList[i].score {
				resultList[i], resultList[j] = resultList[j], resultList[i]
			}
		}
	}

	// Trim to top_k
	if len(resultList) > topK {
		resultList = resultList[:topK]
	}

	// Build Starlark list of structs
	out := make([]starlark.Value, len(resultList))
	for i, r := range resultList {
		members := starlark.StringDict{
			"file_path":  starlark.String(r.filePath),
			"start_line": starlark.MakeInt(r.startLine),
			"end_line":   starlark.MakeInt(r.endLine),
			"score":      starlark.Float(r.score),
			"content":    starlark.String(r.content),
		}
		out[i] = starlarkstruct.FromStringDict(starlarkstruct.Default, members)
	}

	return starlark.NewList(out), nil
}

// Helper functions.

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
func convertStarlarkChunkToChunkData(chunkDict *starlark.Dict) (domainindex.ChunkData, error) { //nolint:gocognit,gocyclo // complexity inherent in validating and mapping multiple optional chunk fields
	var chunk domainindex.ChunkData

	textVal, found, err := chunkDict.Get(starlark.String("text"))
	if err != nil || !found {
		return chunk, fmt.Errorf("chunk missing 'text' field")
	}
	textStr, ok := textVal.(starlark.String)
	if !ok {
		return chunk, fmt.Errorf("chunk 'text' must be string")
	}
	chunk.TextContent = string(textStr)

	if val, found, _ := chunkDict.Get(starlark.String("start_line")); found { //nolint:errcheck // starlark dict lookup; error only on unhashable key
		if intVal, ok := val.(starlark.Int); ok {
			if i, ok := intVal.Int64(); ok {
				chunk.StartLine = int(i)
			}
		}
	}

	if val, found, _ := chunkDict.Get(starlark.String("end_line")); found { //nolint:errcheck // starlark dict lookup; error only on unhashable key
		if intVal, ok := val.(starlark.Int); ok {
			if i, ok := intVal.Int64(); ok {
				chunk.EndLine = int(i)
			}
		}
	}

	if val, found, _ := chunkDict.Get(starlark.String("start_byte")); found { //nolint:errcheck // starlark dict lookup; error only on unhashable key
		if intVal, ok := val.(starlark.Int); ok {
			if i, ok := intVal.Int64(); ok {
				chunk.StartByte = int(i)
			}
		}
	}

	if val, found, _ := chunkDict.Get(starlark.String("end_byte")); found { //nolint:errcheck // starlark dict lookup; error only on unhashable key
		if intVal, ok := val.(starlark.Int); ok {
			if i, ok := intVal.Int64(); ok {
				chunk.EndByte = int(i)
			}
		}
	}

	if val, found, _ := chunkDict.Get(starlark.String("start_rune")); found { //nolint:errcheck // starlark dict lookup; error only on unhashable key
		if intVal, ok := val.(starlark.Int); ok {
			if i, ok := intVal.Int64(); ok {
				chunk.StartRune = int(i)
			}
		}
	}

	if val, found, _ := chunkDict.Get(starlark.String("end_rune")); found { //nolint:errcheck // starlark dict lookup; error only on unhashable key
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
