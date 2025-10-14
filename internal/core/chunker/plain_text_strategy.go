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
	"strings"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
)

// PlainTextStrategy implements Strategy for plain text files.
type PlainTextStrategy struct {
	maxChunkRunes int
	overlapRunes  int
}

// NewPlainTextStrategy creates a new plain text chunking strategy.
func NewPlainTextStrategy(maxChunkRunes, overlapRunes int) *PlainTextStrategy {
	return &PlainTextStrategy{
		maxChunkRunes: maxChunkRunes,
		overlapRunes:  overlapRunes,
	}
}

// Chunk implements the chunking algorithm for plain text.
func (s *PlainTextStrategy) Chunk(content []byte) ([]domainindex.ChunkData, error) {
	text := string(content)

	var chunks []domainindex.ChunkData

	// Split text into paragraphs
	paragraphs := strings.Split(text, "\n\n")

	var currentChunkBuilder strings.Builder
	var currentChunkRunes []rune

	currentByte := 0
	currentRune := 0
	currentLine := 1

	chunkStartByte := 0
	chunkStartRune := 0
	chunkStartLine := 1

	for _, paragraph := range paragraphs {
		paragraphRunes := []rune(paragraph)
		paragraphRuneCount := len(paragraphRunes)

		// Check if adding this paragraph would exceed max chunk size
		if len(currentChunkRunes)+paragraphRuneCount > s.maxChunkRunes && currentChunkBuilder.Len() > 0 {
			// Finalize current chunk
			chunk := s.finalizeChunk(
				currentChunkBuilder.String(),
				currentChunkRunes,
				chunkStartByte,
				chunkStartRune,
				chunkStartLine,
				currentByte,
				currentRune,
				currentLine,
			)
			chunks = append(chunks, chunk)

			// Create new chunk with overlap
			overlapText, overlapRunes := s.createOverlap(currentChunkRunes, currentChunkBuilder.String())
			currentChunkBuilder.Reset()
			currentChunkBuilder.WriteString(overlapText)
			currentChunkRunes = overlapRunes

			// Update start positions for new chunk
			chunkStartByte = currentByte - len([]byte(overlapText))
			chunkStartRune = currentRune - len(overlapRunes)
			chunkStartLine = currentLine - countNewlines(overlapText)
		}

		// Add paragraph to current chunk
		if currentChunkBuilder.Len() > 0 {
			currentChunkBuilder.WriteString("\n\n")
			currentChunkRunes = append(currentChunkRunes, []rune("\n\n")...)
			currentByte += 2
			currentRune += 2
			currentLine += 2
		}

		currentChunkBuilder.WriteString(paragraph)
		currentChunkRunes = append(currentChunkRunes, paragraphRunes...)

		// Update positions
		currentByte += len([]byte(paragraph))
		currentRune += paragraphRuneCount
		currentLine += countNewlines(paragraph)
	}

	// Finalize last chunk if there's content
	if currentChunkBuilder.Len() > 0 {
		chunk := s.finalizeChunk(
			currentChunkBuilder.String(),
			currentChunkRunes,
			chunkStartByte,
			chunkStartRune,
			chunkStartLine,
			currentByte,
			currentRune,
			currentLine,
		)
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// finalizeChunk creates a ChunkData from the current chunk state.
func (s *PlainTextStrategy) finalizeChunk(
	text string,
	runes []rune,
	startByte, startRune, startLine int,
	endByte, endRune, endLine int,
) domainindex.ChunkData {
	return domainindex.ChunkData{
		TextContent: text,
		StartByte:   startByte,
		EndByte:     endByte,
		StartRune:   startRune,
		EndRune:     endRune,
		StartLine:   startLine,
		EndLine:     endLine,
	}
}

// createOverlap extracts the last overlapRunes from the current chunk.
func (s *PlainTextStrategy) createOverlap(currentRunes []rune, currentText string) (string, []rune) {
	if len(currentRunes) <= s.overlapRunes {
		return currentText, currentRunes
	}

	overlapStartIndex := len(currentRunes) - s.overlapRunes
	overlapRunes := currentRunes[overlapStartIndex:]
	overlapText := string(overlapRunes)

	return overlapText, overlapRunes
}

// countNewlines counts the number of newline characters in a string.
func countNewlines(s string) int {
	count := 0
	for _, r := range s {
		if r == '\n' {
			count++
		}
	}
	return count
}
