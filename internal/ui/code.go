// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderCode renders a code block with syntax highlighting.
// If maxLines > 0, content will be truncated to that many lines.
func RenderCode(content, language, title string, theme Theme, opts RenderOptions) string { //nolint:gocritic // hugeParam: Theme passed by value for immutability
	return RenderCodeWithMaxLines(content, language, title, 0, theme, opts)
}

// RenderCodeWithMaxLines renders a code block with optional truncation.
// In terminal mode the code is rendered by wrapping it in a fenced markdown
// block and passing it through glamour (which uses chroma internally).
// In plain/CI mode a simple text fence is returned instead.
func RenderCodeWithMaxLines(content, language, title string, maxLines int, theme Theme, opts RenderOptions) string { //nolint:gocritic,gocognit,gocyclo,funlen // hugeParam: Theme passed by value for immutability; complexity inherent in multi-mode rendering
	var wasTruncated bool
	if maxLines > 0 {
		content, wasTruncated = TruncateContent(content, maxLines, opts)
	}

	if opts.Plain || !opts.Terminal {
		truncateNote := ""
		if wasTruncated {
			truncateNote = " (truncated)"
		}
		if title != "" {
			return fmt.Sprintf("--- %s%s ---\n%s\n---", title, truncateNote, content)
		}
		return fmt.Sprintf("```\n%s\n```", content)
	}

	// Render via glamour using a fenced markdown block so that glamour's
	// built-in chroma integration handles syntax highlighting.
	lang := language
	if lang == "" || lang == "text" {
		lang = ""
	}

	fence := "```" + lang + "\n" + content + "\n```"
	highlighted, err := RenderMarkdown(fence, TerminalWidth(120), false)
	if err != nil {
		highlighted = content
	}
	// RenderMarkdown returns a trailing newline; strip it for consistent rendering.
	highlighted = strings.TrimRight(highlighted, "\n")

	if title == "" && !wasTruncated {
		return highlighted
	}

	// Wrap with a titled border when a title is present.
	lines := strings.Split(highlighted, "\n")
	maxWidth := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > maxWidth {
			maxWidth = w
		}
	}
	if maxWidth < 40 {
		maxWidth = 40
	}

	var borderChar, topLeft, topRight, bottomLeft, bottomRight string
	if opts.SupportsUnicode {
		borderChar = dividerCharLine
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

	var result strings.Builder

	if title != "" { //nolint:nestif // nested layout calculation for title bar with border drawing
		titleStr := fmt.Sprintf(" %s ", title)
		totalInnerWidth := maxWidth + 2
		if len(titleStr) > totalInnerWidth {
			totalInnerWidth = len(titleStr)
		}
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

	for _, line := range lines {
		padding := maxWidth - lipgloss.Width(line)
		if padding < 0 {
			padding = 0
		}
		result.WriteString(theme.SystemStyle.Render("│ ") + line + strings.Repeat(" ", padding) + theme.SystemStyle.Render(" │") + "\n")
	}

	result.WriteString(theme.SystemStyle.Render(fmt.Sprintf("%s%s%s",
		bottomLeft,
		strings.Repeat(borderChar, maxWidth+2),
		bottomRight,
	)))

	return result.String()
}
