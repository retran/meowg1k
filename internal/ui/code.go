// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// RenderCode renders a code block with syntax highlighting.
// If maxLines > 0, content will be truncated to that many lines.
func RenderCode(content, language, title string, theme Theme, opts RenderOptions) string {
	return RenderCodeWithMaxLines(content, language, title, 0, theme, opts)
}

// RenderCodeWithMaxLines renders a code block with optional truncation.
func RenderCodeWithMaxLines(content, language, title string, maxLines int, theme Theme, opts RenderOptions) string {
	// Truncate if requested
	var wasTruncated bool
	if maxLines > 0 {
		content, wasTruncated = TruncateContent(content, maxLines, opts)
	}

	var highlighted string

	if opts.Plain || !opts.Terminal {
		// Plain mode: just add code fence
		truncateNote := ""
		if wasTruncated {
			truncateNote = " (truncated)"
		}
		if title != "" {
			return fmt.Sprintf("--- %s%s ---\n%s\n---", title, truncateNote, content)
		}
		return fmt.Sprintf("```\n%s\n```", content)
	}

	// Try to highlight with chroma
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		highlighted = content
	} else {
		buf := new(bytes.Buffer)
		err = formatter.Format(buf, style, iterator)
		if err != nil {
			highlighted = content
		} else {
			highlighted = buf.String()
		}
	}

	// Remove trailing newline if present
	highlighted = strings.TrimSuffix(highlighted, "\n")

	// Create border
	lines := strings.Split(highlighted, "\n")
	maxWidth := 0
	for _, line := range lines {
		// Count visible characters (strip ANSI codes for width calculation)
		visibleLen := lipgloss.Width(line)
		if visibleLen > maxWidth {
			maxWidth = visibleLen
		}
	}

	// Ensure minimum width
	if maxWidth < 40 {
		maxWidth = 40
	}

	var borderChar, topLeft, topRight, bottomLeft, bottomRight string
	if opts.SupportsUnicode {
		borderChar = "─"
		topLeft = "╭"
		topRight = "╮"
		bottomLeft = "╰"
		bottomRight = "╯"
	} else {
		borderChar = "-"
		topLeft = "+"
		topRight = "+"
		bottomLeft = "+"
		bottomRight = "+"
	}

	// Build output
	var result strings.Builder

	// Top border
	if title != "" {
		titleStr := fmt.Sprintf(" %s ", title)
		// Recalculate maxWidth if title is longer than content
		if len(titleStr) > maxWidth {
			maxWidth = len(titleStr) // No extra padding needed here, title str has spaces
		}

		// The math here is tricky. MaxWidth is the INNER width of content.
		// We pad content with +1 space on left and +1 space on right.
		// So total inner width is maxWidth + 2.

		totalInnerWidth := maxWidth + 2

		leftLen := (totalInnerWidth - len(titleStr)) / 2
		rightLen := totalInnerWidth - len(titleStr) - leftLen

		if leftLen < 0 {
			leftLen = 0
		}
		if rightLen < 0 {
			rightLen = 0
		}

		result.WriteString(theme.SystemStyle.Render(fmt.Sprintf("%s%s%s%s%s",
			topLeft,
			strings.Repeat(borderChar, leftLen),
			titleStr,
			strings.Repeat(borderChar, rightLen),
			topRight,
		)) + "\n")
	} else {
		result.WriteString(theme.SystemStyle.Render(fmt.Sprintf("%s%s%s",
			topLeft,
			strings.Repeat(borderChar, maxWidth+2),
			topRight,
		)) + "\n")
	}

	// Content lines - vertical borders need to align with maxWidth
	for _, line := range lines {
		// Calculate visible length
		visibleLen := lipgloss.Width(line)
		padding := maxWidth - visibleLen
		if padding < 0 {
			padding = 0
		}

		result.WriteString(theme.SystemStyle.Render("│ ") + line + strings.Repeat(" ", padding) + theme.SystemStyle.Render(" │") + "\n")
	}

	// Bottom border
	result.WriteString(theme.SystemStyle.Render(fmt.Sprintf("%s%s%s",
		bottomLeft,
		strings.Repeat(borderChar, maxWidth+2), // +2 for the left and right padding spaces in content
		bottomRight,
	)))

	return result.String()
}
