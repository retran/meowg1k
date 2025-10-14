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

package chunker

import (
	"path/filepath"
	"strings"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
)

// Service implements ports.ChunkerService.
type Service struct {
	strategies map[string]Strategy
}

// NewService creates a new chunker service with default strategies.
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

// Chunk splits content into semantic chunks based on file extension.
func (s *Service) Chunk(content []byte, filePath string) ([]domainindex.ChunkData, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	strategy, ok := s.strategies[ext]
	if !ok {
		strategy = s.strategies["default"]
	}

	return strategy.Chunk(content)
}
