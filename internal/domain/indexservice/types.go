// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package indexservice defines types used by the indexing service API.
package indexservice

import (
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
)

// PrepareForProcessingOutput represents the output of the PrepareForProcessing operation.
type PrepareForProcessingOutput struct {
	ExistingVersions map[string]int64
	ContentHashMap   map[string]string
	FilesToProcess   []domainindex.FileToProcess
}

// SaveVersionInput represents the input for saving a new version.
type SaveVersionInput struct {
	FilePath    string
	Content     []byte
	ContentHash string
	Chunks      []domainindex.ChunkData
	Embeddings  []gateway.Embedding
}

// SaveVersionOutput represents the output of saving a new version.
type SaveVersionOutput struct {
	FilePath  string
	VersionID int64
}

// FinalizeInput represents the input for finalizing live snapshots.
type FinalizeInput struct {
	ScanResult       *domainindex.WorkspaceState
	ExistingVersions map[string]int64
	NewVersions      map[string]int64
}
