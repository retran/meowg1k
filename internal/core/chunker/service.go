// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package chunker provides services for splitting file content into chunks for embedding.
package chunker

import (
	"fmt"
	"path/filepath"
	"strings"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
)

// Service selects chunking strategies based on file extensions.
type Service struct {
	strategies map[string]Strategy
}

// NewService creates a chunker service with default strategies.
func NewService(maxChunkRunes, overlapRunes int) *Service {
	plainTextStrategy := NewPlainTextStrategy(maxChunkRunes, overlapRunes)

	return &Service{
		strategies: map[string]Strategy{
			"default": plainTextStrategy,
			".txt":    plainTextStrategy,
			".md":     plainTextStrategy,
		},
	}
}

// Chunk splits file content into chunks using the matching strategy.
func (s *Service) Chunk(content []byte, filePath string) ([]domainindex.ChunkData, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	strategy, ok := s.strategies[ext]
	if !ok {
		strategy = s.strategies["default"]
	}

	chunks, err := strategy.Chunk(content)
	if err != nil {
		return nil, fmt.Errorf("failed to chunk content: %w", err)
	}
	return chunks, nil
}
