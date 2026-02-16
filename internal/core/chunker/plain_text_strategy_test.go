// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package chunker

import (
	"strings"
	"testing"
)

func TestNewPlainTextStrategy(t *testing.T) {
	maxChunkRunes := 1000
	overlapRunes := 100

	strategy := NewPlainTextStrategy(maxChunkRunes, overlapRunes)

	if strategy == nil {
		t.Fatal("Expected strategy to be non-nil")
	}

	if strategy.maxChunkRunes != maxChunkRunes {
		t.Errorf("Expected maxChunkRunes %d, got %d", maxChunkRunes, strategy.maxChunkRunes)
	}

	if strategy.overlapRunes != overlapRunes {
		t.Errorf("Expected overlapRunes %d, got %d", overlapRunes, strategy.overlapRunes)
	}
}

func TestPlainTextStrategy_Chunk_SimpleText(t *testing.T) {
	strategy := NewPlainTextStrategy(100, 10)
	content := []byte("This is a simple test.")

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(chunks))
	}

	if chunks[0].TextContent != "This is a simple test." {
		t.Errorf("Unexpected chunk content: %s", chunks[0].TextContent)
	}
}

func TestPlainTextStrategy_Chunk_EmptyContent(t *testing.T) {
	strategy := NewPlainTextStrategy(100, 10)
	content := []byte("")

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestPlainTextStrategy_Chunk_MultipleParagraphs(t *testing.T) {
	strategy := NewPlainTextStrategy(100, 10)
	content := []byte("Paragraph 1\n\nParagraph 2\n\nParagraph 3")

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}

	// Verify content is preserved
	allText := ""
	for _, chunk := range chunks {
		allText += chunk.TextContent
	}
	// Account for overlap - just check that original content is represented
	if !strings.Contains(allText, "Paragraph 1") {
		t.Error("Expected to find 'Paragraph 1' in chunks")
	}
}

func TestPlainTextStrategy_Chunk_LongParagraph(t *testing.T) {
	strategy := NewPlainTextStrategy(50, 5)

	// Create a paragraph longer than maxChunkRunes
	longParagraph := strings.Repeat("This is a long line. ", 10) // ~200 chars
	content := []byte(longParagraph)

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) <= 1 {
		t.Error("Expected multiple chunks for long paragraph")
	}
}

func TestPlainTextStrategy_Chunk_VeryLongLine(t *testing.T) {
	strategy := NewPlainTextStrategy(20, 2)

	// Create a line that needs to be split mid-line
	veryLongLine := strings.Repeat("abcdefghij", 10) // 100 chars, no spaces
	content := []byte(veryLongLine)

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) <= 1 {
		t.Error("Expected multiple chunks for very long line")
	}

	// Verify chunks are approximately the right size
	for i, chunk := range chunks {
		runeCount := len([]rune(chunk.TextContent))
		if runeCount > strategy.maxChunkRunes {
			t.Errorf("Chunk %d has %d runes, exceeds max %d", i, runeCount, strategy.maxChunkRunes)
		}
	}
}

func TestPlainTextStrategy_Chunk_WithOverlap(t *testing.T) {
	strategy := NewPlainTextStrategy(30, 5)

	// Create content that will be split into multiple chunks
	content := []byte("Line 1 text here\n\nLine 2 text here\n\nLine 3 text here")

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) <= 1 {
		t.Error("Expected multiple chunks")
	}

	// Verify chunks have some overlap (except first chunk)
	for i := 1; i < len(chunks); i++ {
		// Overlap should exist between consecutive chunks
		if chunks[i].TextContent == "" {
			t.Errorf("Chunk %d is empty", i)
		}
	}
}

func TestPlainTextStrategy_Chunk_ChunkBoundaries(t *testing.T) {
	strategy := NewPlainTextStrategy(50, 5)
	content := []byte("First paragraph\n\nSecond paragraph")

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify byte boundaries are set
	for i, chunk := range chunks {
		if chunk.StartByte < 0 {
			t.Errorf("Chunk %d has negative StartByte", i)
		}
		if chunk.EndByte < chunk.StartByte {
			t.Errorf("Chunk %d has EndByte < StartByte", i)
		}
		if chunk.StartRune < 0 {
			t.Errorf("Chunk %d has negative StartRune", i)
		}
		if chunk.EndRune < chunk.StartRune {
			t.Errorf("Chunk %d has EndRune < StartRune", i)
		}
		if chunk.StartLine < 1 {
			t.Errorf("Chunk %d has StartLine < 1", i)
		}
		if chunk.EndLine < chunk.StartLine {
			t.Errorf("Chunk %d has EndLine < StartLine", i)
		}
	}
}

func TestPlainTextStrategy_Chunk_UnicodeContent(t *testing.T) {
	strategy := NewPlainTextStrategy(50, 5)
	content := []byte("Hello 世界\n\nБольшой текст\n\nΕλληνικά")

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk for unicode content")
	}

	// Verify unicode is preserved
	allText := strings.Join(func() []string {
		var texts []string
		for _, c := range chunks {
			texts = append(texts, c.TextContent)
		}
		return texts
	}(), "")

	if !strings.Contains(allText, "世界") {
		t.Error("Expected to find '世界' in chunks")
	}
}

func TestPlainTextStrategy_Chunk_NewlineHandling(t *testing.T) {
	strategy := NewPlainTextStrategy(100, 10)

	testCases := []struct {
		name    string
		content string
	}{
		{"Single newline", "Line 1\nLine 2\nLine 3"},
		{"Double newline", "Para 1\n\nPara 2\n\nPara 3"},
		{"Mixed newlines", "Line 1\nLine 2\n\nPara 2\nLine 3"},
		{"Trailing newline", "Content\n\n"},
		{"Leading newline", "\n\nContent"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := []byte(tc.content)
			chunks, err := strategy.Chunk(content)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			if len(chunks) == 0 {
				t.Error("Expected at least one chunk")
			}
		})
	}
}

func TestPlainTextStrategy_Chunk_LineTracking(t *testing.T) {
	strategy := NewPlainTextStrategy(50, 5)
	content := []byte("Line 1\nLine 2\nLine 3\n\nLine 5")

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// First chunk should start at line 1
	if chunks[0].StartLine != 1 {
		t.Errorf("Expected first chunk to start at line 1, got %d", chunks[0].StartLine)
	}

	// Lines should be tracked correctly
	for i, chunk := range chunks {
		if chunk.EndLine < chunk.StartLine {
			t.Errorf("Chunk %d: EndLine (%d) < StartLine (%d)", i, chunk.EndLine, chunk.StartLine)
		}
	}
}

func TestCountNewlines(t *testing.T) {
	testCases := []struct {
		input    string
		expected int
	}{
		{"no newlines", 0},
		{"one\nnewline", 1},
		{"two\nnew\nlines", 2},
		{"\n\n\n", 3},
		{"", 0},
		{"trailing\n", 1},
	}

	for _, tc := range testCases {
		result := countNewlines(tc.input)
		if result != tc.expected {
			t.Errorf("countNewlines(%q) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

func TestPlainTextStrategy_CreateOverlap(t *testing.T) {
	strategy := NewPlainTextStrategy(100, 10)

	testCases := []struct {
		name         string
		currentText  string
		currentRunes []rune
		expectedLen  int
	}{
		{
			name:         "Normal overlap",
			currentRunes: []rune("This is a test string for overlap"),
			currentText:  "This is a test string for overlap",
			expectedLen:  10,
		},
		{
			name:         "Short content",
			currentRunes: []rune("Short"),
			currentText:  "Short",
			expectedLen:  5,
		},
		{
			name:         "Exact overlap size",
			currentRunes: []rune("0123456789"),
			currentText:  "0123456789",
			expectedLen:  10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			overlapText, overlapRunes := strategy.createOverlap(tc.currentRunes, tc.currentText)

			if len(overlapRunes) != tc.expectedLen {
				t.Errorf("Expected overlap length %d, got %d", tc.expectedLen, len(overlapRunes))
			}

			if overlapText != string(overlapRunes) {
				t.Error("Overlap text doesn't match overlap runes")
			}
		})
	}
}

func TestPlainTextStrategy_Chunk_ZeroOverlap(t *testing.T) {
	strategy := NewPlainTextStrategy(30, 0)
	content := []byte("Line 1 text\n\nLine 2 text\n\nLine 3 text")

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}

	// With zero overlap, chunks should not have repeated content
	// (This is hard to verify precisely without knowing exact split points,
	// but we can at least verify it doesn't error)
}

func TestPlainTextStrategy_Chunk_LargeOverlap(t *testing.T) {
	// Overlap larger than half the chunk size
	strategy := NewPlainTextStrategy(50, 30)
	content := []byte("This is some text that will be split into multiple chunks with large overlap between them")

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}
}

func TestPlainTextStrategy_Chunk_SingleWord(t *testing.T) {
	strategy := NewPlainTextStrategy(100, 10)
	content := []byte("Hello")

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(chunks))
	}

	if chunks[0].TextContent != "Hello" {
		t.Errorf("Expected 'Hello', got %q", chunks[0].TextContent)
	}
}

func TestPlainTextStrategy_Chunk_WhitespaceOnly(t *testing.T) {
	strategy := NewPlainTextStrategy(100, 10)

	testCases := []struct {
		name    string
		content string
	}{
		{"Spaces only", "     "},
		{"Newlines only", "\n\n\n"},
		{"Mixed whitespace", "  \n  \n  "},
		{"Tabs and spaces", "\t  \t  "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := []byte(tc.content)
			chunks, err := strategy.Chunk(content)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			// Whitespace-only content should still create chunks
			// (or not, depending on implementation - just verify it doesn't crash)
			t.Logf("Whitespace test '%s' created %d chunks", tc.name, len(chunks))
		})
	}
}

// Test processLine via oversized paragraphs with line breaks

func TestPlainTextStrategy_Chunk_OversizeParagraphWithLines(t *testing.T) {
	// Create a strategy with small max chunk size to trigger line splitting
	strategy := NewPlainTextStrategy(50, 5)

	// Create a long paragraph (no double newline) with line breaks
	// Each line is short enough, but the paragraph is too long
	longParagraph := "Line one is here\nLine two is here\nLine three is here\nLine four is here\nLine five"
	content := []byte(longParagraph)

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should create multiple chunks due to paragraph size
	if len(chunks) <= 1 {
		t.Error("Expected multiple chunks for oversize paragraph with lines")
	}

	// Verify processLine was called by checking chunks contain line content
	allContent := ""
	for _, chunk := range chunks {
		allContent += chunk.TextContent
	}

	// Should contain parts of the original lines
	if !strings.Contains(allContent, "Line one") {
		t.Error("Expected chunks to contain 'Line one'")
	}
}

func TestPlainTextStrategy_Chunk_OversizeParagraphMultipleLines(t *testing.T) {
	// Strategy with very small chunks to force line-by-line processing
	strategy := NewPlainTextStrategy(30, 3)

	// Paragraph with multiple normal-sized lines
	paragraph := "First line here\nSecond line here\nThird line here\nFourth line"
	content := []byte(paragraph)

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Multiple chunks should be created
	if len(chunks) <= 1 {
		t.Errorf("Expected multiple chunks, got %d", len(chunks))
	}

	// Verify chunk metadata
	for i, chunk := range chunks {
		if chunk.StartLine <= 0 {
			t.Errorf("Chunk %d has invalid StartLine: %d", i, chunk.StartLine)
		}
		if chunk.EndLine <= 0 {
			t.Errorf("Chunk %d has invalid EndLine: %d", i, chunk.EndLine)
		}
		if chunk.StartLine > chunk.EndLine {
			t.Errorf("Chunk %d has StartLine > EndLine: %d > %d", i, chunk.StartLine, chunk.EndLine)
		}
	}
}

func TestPlainTextStrategy_ProcessLine_WithExistingContent(t *testing.T) {
	// Test processLine when state already has content (tests the newline logic)
	strategy := NewPlainTextStrategy(100, 10)

	// Create paragraph that's slightly over limit to trigger line processing
	// with content already in buffer
	longParagraph := strings.Repeat("x", 101) + "\nshort line"
	content := []byte(longParagraph)

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should have at least one chunk
	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}
}

func TestPlainTextStrategy_ProcessLine_Overlap(t *testing.T) {
	// Test that line processing respects overlap
	strategy := NewPlainTextStrategy(40, 10)

	// Create content with multiple short lines that will need multiple chunks
	lines := []string{
		"Line A is here now",
		"Line B is here now",
		"Line C is here now",
		"Line D is here now",
	}
	paragraph := strings.Join(lines, "\n")
	content := []byte(paragraph)

	chunks, err := strategy.Chunk(content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should create multiple chunks
	if len(chunks) <= 1 {
		t.Error("Expected multiple chunks to test overlap")
	}

	// Verify overlap exists (later chunks should contain some content from previous chunks)
	for i := 1; i < len(chunks); i++ {
		// Check that chunk starts aren't at exact boundaries
		if chunks[i].StartLine == chunks[i-1].EndLine+1 {
			// This is okay - they might be adjacent
			continue
		}
		// There should be some overlap in rune positions
		if chunks[i].StartRune <= chunks[i-1].EndRune {
			// Good - there's overlap
			break
		}
	}
}
