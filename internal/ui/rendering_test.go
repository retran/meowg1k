// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- RenderBanner ---

func TestRenderBanner_Plain(t *testing.T) {
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	result := RenderBanner("Hello", "subtext here", theme, opts)
	assert.Contains(t, result, "Hello")
	assert.Contains(t, result, "subtext here")
	assert.Contains(t, result, "===")
}

func TestRenderBanner_Plain_NoSubtext(t *testing.T) {
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	result := RenderBanner("Title", "", theme, opts)
	assert.Contains(t, result, "Title")
	assert.NotContains(t, result, "subtext")
}

func TestRenderBanner_Terminal_Unicode(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, SupportsUnicode: true}
	theme := DefaultThemeWithOptions(opts)
	result := RenderBanner("Title", "Sub", theme, opts)
	assert.Contains(t, result, "Title")
	assert.Contains(t, result, "═") // unicode border
}

func TestRenderBanner_Terminal_ASCII(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, SupportsUnicode: false}
	theme := DefaultThemeWithOptions(opts)
	result := RenderBanner("Title", "", theme, opts)
	assert.Contains(t, result, "Title")
	assert.Contains(t, result, "=") // ASCII border
}

// --- RenderLink ---

func TestRenderLink_Plain_SameTextURL(t *testing.T) {
	opts := RenderOptions{Plain: true}
	result := RenderLink("https://example.com", "https://example.com", opts)
	assert.Equal(t, "https://example.com", result)
}

func TestRenderLink_Plain_DifferentTextURL(t *testing.T) {
	opts := RenderOptions{Plain: true}
	result := RenderLink("Example", "https://example.com", opts)
	assert.Equal(t, "Example (https://example.com)", result)
}

func TestRenderLink_Terminal_NoOSC8(t *testing.T) {
	// In test environment, TERM_PROGRAM is likely not set to iTerm/WezTerm etc.
	opts := RenderOptions{Terminal: true, Plain: false}
	result := RenderLink("My Link", "https://example.com", opts)
	// Falls back to plain-style since no OSC8-supporting terminal is detected
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "My Link")
}

// --- LogThought / LogAction ---

func TestLogThought_Plain(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	LogThought("my thought", theme, opts, &buf)
	assert.Contains(t, buf.String(), "thinking:")
	assert.Contains(t, buf.String(), "my thought")
}

func TestLogThought_Terminal(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{Terminal: true, Plain: false}
	LogThought("deep thought", theme, opts, &buf)
	assert.NotEmpty(t, buf.String())
}

func TestLogThought_NilWriter(t *testing.T) {
	// Should not panic even with nil writer (falls back to stderr)
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	assert.NotPanics(t, func() {
		LogThought("test", theme, opts, nil)
	})
}

func TestLogAction_Plain(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	LogAction("search(query)", theme, opts, &buf)
	assert.Contains(t, buf.String(), "action:")
	assert.Contains(t, buf.String(), "search(query)")
}

func TestLogAction_Terminal(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{Terminal: true, Plain: false}
	LogAction("calc(1+1)", theme, opts, &buf)
	assert.NotEmpty(t, buf.String())
}

func TestLogAction_NilWriter(t *testing.T) {
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	assert.NotPanics(t, func() {
		LogAction("test", theme, opts, nil)
	})
}

// --- LogDivider ---

func TestLogDivider_PlainWriter(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	LogDivider("line", theme, opts, &buf)
	assert.NotEmpty(t, buf.String())
}

func TestLogDivider_NilWriter(t *testing.T) {
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	assert.NotPanics(t, func() {
		LogDivider("line", theme, opts, nil)
	})
}

// --- RenderTree ---

func TestRenderTree_EmptyTitle(t *testing.T) {
	opts := RenderOptions{Plain: true}
	theme := DefaultTheme()
	data := map[string]interface{}{"key": "value"}
	result := RenderTree(data, "", theme, opts)
	assert.Contains(t, result, "key")
	assert.Contains(t, result, "value")
}

func TestRenderTree_WithTitle_Plain(t *testing.T) {
	opts := RenderOptions{Plain: true}
	theme := DefaultTheme()
	data := map[string]interface{}{"name": "test"}
	result := RenderTree(data, "My Tree", theme, opts)
	assert.Contains(t, result, "My Tree")
	assert.Contains(t, result, "name")
}

func TestRenderTree_Nested_Unicode(t *testing.T) {
	opts := RenderOptions{Plain: false, Terminal: true, SupportsUnicode: true}
	theme := DefaultThemeWithOptions(opts)
	data := map[string]interface{}{
		"parent": map[string]interface{}{
			"child": "leaf",
		},
	}
	result := RenderTree(data, "", theme, opts)
	assert.Contains(t, result, "parent")
	assert.Contains(t, result, "child")
	assert.Contains(t, result, "leaf")
}

func TestRenderTree_Nested_ASCII(t *testing.T) {
	opts := RenderOptions{Plain: false, Terminal: true, SupportsUnicode: false}
	theme := DefaultThemeWithOptions(opts)
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "c",
		},
	}
	result := RenderTree(data, "", theme, opts)
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "b")
}

func TestRenderTree_List(t *testing.T) {
	opts := RenderOptions{Plain: true}
	theme := DefaultTheme()
	data := []interface{}{"one", "two", "three"}
	result := RenderTree(data, "", theme, opts)
	assert.Contains(t, result, "one")
	assert.Contains(t, result, "two")
	assert.Contains(t, result, "three")
}

func TestRenderTree_ListOfMaps(t *testing.T) {
	opts := RenderOptions{Plain: true, SupportsUnicode: false}
	theme := DefaultTheme()
	data := []interface{}{
		map[string]interface{}{"key": "val"},
	}
	result := RenderTree(data, "", theme, opts)
	assert.Contains(t, result, "key")
	assert.Contains(t, result, "val")
}

func TestRenderTree_Scalar(t *testing.T) {
	opts := RenderOptions{Plain: true}
	theme := DefaultTheme()
	result := RenderTree("just a string", "", theme, opts)
	assert.Contains(t, result, "just a string")
}

func TestRenderTree_WithTitle_Terminal(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, SupportsUnicode: true}
	theme := DefaultThemeWithOptions(opts)
	data := map[string]interface{}{"k": "v"}
	result := RenderTree(data, "Root", theme, opts)
	assert.Contains(t, result, "Root")
}

// --- RenderMarkdown ---

func TestRenderMarkdown_Basic(t *testing.T) {
	result, err := RenderMarkdown("# Hello\n\nWorld", 80, false)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestRenderMarkdown_NoColor(t *testing.T) {
	result, err := RenderMarkdown("**bold** text", 80, true)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestRenderMarkdown_ZeroWidth(t *testing.T) {
	// Width <= 0 falls back to 80
	result, err := RenderMarkdown("hello", 0, false)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestRenderMarkdown_ExceedMaxWidth(t *testing.T) {
	// Width > 120 is clamped to 120
	result, err := RenderMarkdown("hello world", 200, false)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestRenderMarkdown_Cached(t *testing.T) {
	// Calling twice with same params should use cache (no panic, same result)
	r1, err1 := RenderMarkdown("test content", 80, false)
	r2, err2 := RenderMarkdown("test content", 80, false)
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, r1, r2)
}

// --- RenderCode ---

func TestRenderCode_Plain(t *testing.T) {
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	result := RenderCode("fmt.Println(\"hello\")", "go", "", theme, opts)
	assert.Contains(t, result, "fmt.Println")
	assert.Contains(t, result, "```")
}

func TestRenderCode_Plain_WithTitle(t *testing.T) {
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	result := RenderCode("x := 1", "go", "example.go", theme, opts)
	assert.Contains(t, result, "example.go")
	assert.Contains(t, result, "x := 1")
}

func TestRenderCode_Terminal_NoTitle(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, SupportsUnicode: true}
	theme := DefaultThemeWithOptions(opts)
	result := RenderCode("x = 1", "python", "", theme, opts)
	assert.NotEmpty(t, result)
}

func TestRenderCode_Terminal_WithTitle_Unicode(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, SupportsUnicode: true}
	theme := DefaultThemeWithOptions(opts)
	result := RenderCode("x = 1", "python", "script.py", theme, opts)
	assert.NotEmpty(t, result)
}

func TestRenderCode_Terminal_WithTitle_ASCII(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, SupportsUnicode: false}
	theme := DefaultThemeWithOptions(opts)
	result := RenderCode("x = 1", "python", "script.py", theme, opts)
	assert.NotEmpty(t, result)
}

func TestRenderCodeWithMaxLines_Truncation(t *testing.T) {
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	// Create content with 10 lines
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "line content"
	}
	content := strings.Join(lines, "\n")
	result := RenderCodeWithMaxLines(content, "", "", 3, theme, opts)
	assert.Contains(t, result, "more lines")
}

// --- RenderDiff / RenderDiffEnhanced / RenderDiffLegacy ---

func TestRenderDiff_Terminal(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false}
	theme := DefaultThemeWithOptions(opts)
	diff := "+added line\n-removed line\n context"
	result := RenderDiff(diff, theme, opts)
	assert.NotEmpty(t, result)
}

func TestRenderDiff_NoColor(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, NoColor: true}
	theme := DefaultThemeWithOptions(opts)
	diff := "+added\n-removed"
	result := RenderDiff(diff, theme, opts)
	// NoColor mode returns as-is
	assert.Equal(t, diff, result)
}

func TestRenderDiff_Terminal_AllPrefixes(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false}
	theme := DefaultThemeWithOptions(opts)
	diff := "diff --git a/f b/f\nindex 1234..abcd 100644\n--- a/f\n+++ b/f\n@@ -1,1 +1,1 @@\n-old\n+new\n unchanged"
	result := RenderDiff(diff, theme, opts)
	assert.NotEmpty(t, result)
}

func TestRenderDiffEnhanced_Plain_NoTitle(t *testing.T) {
	opts := RenderOptions{Plain: true}
	theme := DefaultTheme()
	result := RenderDiffEnhanced("+added\n-removed", "", theme, opts)
	assert.Contains(t, result, "added")
	assert.Contains(t, result, "removed")
}

func TestRenderDiffEnhanced_Plain_WithTitle(t *testing.T) {
	opts := RenderOptions{Plain: true}
	theme := DefaultTheme()
	result := RenderDiffEnhanced("+added", "my.go", theme, opts)
	assert.Contains(t, result, "my.go")
	assert.Contains(t, result, "added")
}

func TestRenderDiffEnhanced_Terminal_Unicode(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, SupportsUnicode: true}
	theme := DefaultThemeWithOptions(opts)
	result := RenderDiffEnhanced("+added\n-removed\n context", "", theme, opts)
	assert.NotEmpty(t, result)
}

func TestRenderDiffEnhanced_Terminal_ASCII(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, SupportsUnicode: false}
	theme := DefaultThemeWithOptions(opts)
	result := RenderDiffEnhanced("+added\n-removed", "", theme, opts)
	assert.NotEmpty(t, result)
}

func TestRenderDiffEnhanced_Terminal_WithTitle(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, SupportsUnicode: true}
	theme := DefaultThemeWithOptions(opts)
	result := RenderDiffEnhanced("+added", "changes.go", theme, opts)
	assert.NotEmpty(t, result)
}

func TestRenderDiffEnhancedWithMaxLines_Truncation_Plain(t *testing.T) {
	opts := RenderOptions{Plain: true}
	theme := DefaultTheme()
	content := strings.Repeat("+line\n", 20)
	result := RenderDiffEnhancedWithMaxLines(content, "", 3, theme, opts)
	assert.Contains(t, result, "more lines")
}

func TestRenderDiffEnhancedWithMaxLines_Truncation_WithTitle(t *testing.T) {
	opts := RenderOptions{Plain: true}
	theme := DefaultTheme()
	content := strings.Repeat("+line\n", 20)
	result := RenderDiffEnhancedWithMaxLines(content, "diff.patch", 3, theme, opts)
	assert.Contains(t, result, "diff.patch")
	assert.Contains(t, result, "truncated")
}

func TestRenderDiffLegacy(t *testing.T) {
	theme := DefaultTheme()
	// Just check it doesn't panic
	result := RenderDiffLegacy("+added\n-removed", theme)
	assert.NotEmpty(t, result)
}

// --- RenderPanelLegacy ---

func TestRenderPanelLegacy(t *testing.T) {
	theme := DefaultTheme()
	result := RenderPanelLegacy("content", "title", "info", theme)
	assert.NotEmpty(t, result)
}

// --- ProgressBar ---

func TestNewProgressBar_Plain(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	pb := NewProgressBar(10, "loading", theme, opts, &buf)
	require.NotNil(t, pb)
	assert.Contains(t, buf.String(), "loading")
}

func TestNewProgressBar_NilWriter(t *testing.T) {
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	// Should not panic (falls back to stderr)
	assert.NotPanics(t, func() {
		pb := NewProgressBar(5, "test", theme, opts, nil)
		require.NotNil(t, pb)
	})
}

func TestProgressBar_Inc(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	pb := NewProgressBar(10, "loading", theme, opts, &buf)
	pb.Inc(3)
	pb.Inc(3)
	// Should not panic, current should not exceed total
	pb.Inc(100)
}

func TestProgressBar_Set(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	pb := NewProgressBar(10, "loading", theme, opts, &buf)
	pb.Set(5)
	pb.Set(100) // Should clamp to total
}

func TestProgressBar_Done(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	pb := NewProgressBar(10, "loading", theme, opts, &buf)
	pb.Done("complete")
	out := buf.String()
	assert.Contains(t, out, "complete")
	// Calling Done again should be a no-op
	pb.Done("again")
}

func TestProgressBar_Fail(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{Plain: true}
	pb := NewProgressBar(10, "loading", theme, opts, &buf)
	pb.Inc(3)
	pb.Fail("error occurred")
	out := buf.String()
	assert.Contains(t, out, "error occurred")
	// Calling Fail again should be a no-op
	pb.Fail("again")
}

// --- TruncateContent ---

func TestTruncateContent_NoTruncation(t *testing.T) {
	opts := RenderOptions{Plain: true}
	content := "line1\nline2\nline3"
	result, wasTruncated := TruncateContent(content, 5, opts)
	assert.Equal(t, content, result)
	assert.False(t, wasTruncated)
}

func TestTruncateContent_ZeroMaxLines(t *testing.T) {
	opts := RenderOptions{Plain: true}
	content := "line1\nline2"
	result, wasTruncated := TruncateContent(content, 0, opts)
	assert.Equal(t, content, result)
	assert.False(t, wasTruncated)
}

func TestTruncateContent_Unicode(t *testing.T) {
	opts := RenderOptions{SupportsUnicode: true}
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	content := strings.Join(lines, "\n")
	result, wasTruncated := TruncateContent(content, 3, opts)
	assert.True(t, wasTruncated)
	assert.Contains(t, result, "⋮")
}

func TestTruncateContent_ASCII(t *testing.T) {
	opts := RenderOptions{SupportsUnicode: false}
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	content := strings.Join(lines, "\n")
	result, wasTruncated := TruncateContent(content, 3, opts)
	assert.True(t, wasTruncated)
	assert.Contains(t, result, "...")
}

// --- HighlightSubstring ---

func TestHighlightSubstring_Found(t *testing.T) {
	result := HighlightSubstring("hello world", "world", lipgloss.NewStyle().Bold(true))
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "hello")
}

func TestHighlightSubstring_NotFound(t *testing.T) {
	result := HighlightSubstring("hello world", "xyz", lipgloss.NewStyle().Bold(true))
	assert.Equal(t, "hello world", result)
}

func TestHighlightSubstring_Plain(t *testing.T) {
	// HighlightSubstring always applies the style; plain-text callers should pass an empty style
	result := HighlightSubstring("hello world", "world", lipgloss.NewStyle())
	assert.Contains(t, result, "hello")
	assert.Contains(t, result, "world")
}

func TestHighlightSubstring_EmptyQuery(t *testing.T) {
	result := HighlightSubstring("hello world", "", lipgloss.NewStyle().Bold(true))
	assert.Equal(t, "hello world", result)
}
