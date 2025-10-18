// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package chunker

import domainindex "github.com/retran/meowg1k/internal/domain/index"

// Strategy defines the interface for different chunking algorithms.
type Strategy interface {
	Chunk(content []byte) ([]domainindex.ChunkData, error)
}
