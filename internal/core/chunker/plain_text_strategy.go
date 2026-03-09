// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

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

type chunkState struct {
	builder        strings.Builder
	runes          []rune
	currentByte    int
	currentRune    int
	currentLine    int
	chunkStartByte int
	chunkStartRune int
	chunkStartLine int
}

func newChunkState() *chunkState {
	return &chunkState{
		currentLine:    1,
		chunkStartLine: 1,
	}
}

func (state *chunkState) appendText(text string) {
	if text == "" {
		return
	}
	runes := []rune(text)
	state.appendRunes(text, runes)
}

func (state *chunkState) appendRunes(text string, runes []rune) {
	if text == "" {
		return
	}
	state.builder.WriteString(text)
	state.runes = append(state.runes, runes...)
	state.currentByte += len(text)
	state.currentRune += len(runes)
	state.currentLine += countNewlines(text)
}

func (state *chunkState) resetEmpty() {
	state.builder.Reset()
	state.runes = nil
	state.chunkStartByte = state.currentByte
	state.chunkStartRune = state.currentRune
	state.chunkStartLine = state.currentLine
}

func (state *chunkState) resetWithOverlap(overlapText string, overlapRunes []rune) {
	state.builder.Reset()
	state.builder.WriteString(overlapText)
	state.runes = overlapRunes
	state.chunkStartByte = state.currentByte - len(overlapText)
	state.chunkStartRune = state.currentRune - len(overlapRunes)
	state.chunkStartLine = state.currentLine - countNewlines(overlapText)
}

// Chunk implements the chunking algorithm for plain text.
func (s *PlainTextStrategy) Chunk(content []byte) ([]domainindex.ChunkData, error) {
	text := string(content)
	paragraphs := strings.Split(text, "\n\n")
	state := newChunkState()
	var chunks []domainindex.ChunkData

	for _, paragraph := range paragraphs {
		paragraphRunes := []rune(paragraph)

		// If paragraph itself is too large, split it by lines
		if len(paragraphRunes) > s.maxChunkRunes {
			s.processOversizeParagraph(paragraph, state, &chunks)
			continue
		}

		s.processParagraph(paragraph, paragraphRunes, state, &chunks)
	}

	if state.builder.Len() > 0 {
		s.appendChunk(state, &chunks)
	}

	return chunks, nil
}

func (s *PlainTextStrategy) processOversizeParagraph(paragraph string, state *chunkState, chunks *[]domainindex.ChunkData) {
	lines := strings.Split(paragraph, "\n")
	for _, line := range lines {
		lineRunes := []rune(line)
		if len(lineRunes) > s.maxChunkRunes {
			s.processOversizeLine(lineRunes, state, chunks)
			continue
		}
		s.processLine(line, lineRunes, state, chunks)
	}
	state.currentLine += countNewlines(paragraph)
}

func (s *PlainTextStrategy) processOversizeLine(lineRunes []rune, state *chunkState, chunks *[]domainindex.ChunkData) {
	lineRuneCount := len(lineRunes)
	step := s.maxChunkRunes - s.overlapRunes
	for i := 0; i < lineRuneCount; i += step {
		end := i + s.maxChunkRunes
		if end > lineRuneCount {
			end = lineRuneCount
		}

		linePartRunes := lineRunes[i:end]
		linePartText := string(linePartRunes)

		if state.builder.Len() > 0 {
			s.flushAndReset(state, chunks)
		}

		state.appendRunes(linePartText, linePartRunes)
	}
}

func (s *PlainTextStrategy) processLine(line string, lineRunes []rune, state *chunkState, chunks *[]domainindex.ChunkData) {
	extraRunes := len(lineRunes)
	if state.builder.Len() > 0 {
		extraRunes++
	}

	if len(state.runes)+extraRunes > s.maxChunkRunes && state.builder.Len() > 0 {
		s.flushWithOverlap(state, chunks)
	}

	if state.builder.Len() > 0 {
		state.appendText("\n")
	}

	state.appendRunes(line, lineRunes)
}

func (s *PlainTextStrategy) processParagraph(
	paragraph string,
	paragraphRunes []rune,
	state *chunkState,
	chunks *[]domainindex.ChunkData,
) {
	if len(state.runes)+len(paragraphRunes) > s.maxChunkRunes && state.builder.Len() > 0 {
		s.flushWithOverlap(state, chunks)
	}

	if state.builder.Len() > 0 {
		state.appendText("\n\n")
	}

	state.appendRunes(paragraph, paragraphRunes)
}

func (s *PlainTextStrategy) appendChunk(state *chunkState, chunks *[]domainindex.ChunkData) {
	chunk := s.finalizeChunk(
		state.builder.String(),
		state.runes,
		state.chunkStartByte,
		state.chunkStartRune,
		state.chunkStartLine,
		state.currentByte,
		state.currentRune,
		state.currentLine,
	)
	*chunks = append(*chunks, chunk)
}

func (s *PlainTextStrategy) flushWithOverlap(state *chunkState, chunks *[]domainindex.ChunkData) {
	s.appendChunk(state, chunks)
	overlapText, overlapRunes := s.createOverlap(state.runes, state.builder.String())
	state.resetWithOverlap(overlapText, overlapRunes)
}

func (s *PlainTextStrategy) flushAndReset(state *chunkState, chunks *[]domainindex.ChunkData) {
	s.appendChunk(state, chunks)
	state.resetEmpty()
}

// finalizeChunk creates a ChunkData from the current chunk state.
func (s *PlainTextStrategy) finalizeChunk(
	text string,
	_ []rune,
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
func (s *PlainTextStrategy) createOverlap(currentRunes []rune, currentText string) (overlapText string, overlapRunes []rune) {
	if len(currentRunes) <= s.overlapRunes {
		return currentText, currentRunes
	}

	overlapStartIndex := len(currentRunes) - s.overlapRunes
	overlapRunes = currentRunes[overlapStartIndex:]
	overlapText = string(overlapRunes)

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
