// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderDiff applies styling to a unified diff string.
// In plain mode, returns the diff without styling.
func RenderDiff(diff string, theme Theme, opts RenderOptions) string { //nolint:gocritic // hugeParam: Theme passed by value for immutability
	if opts.Plain || !opts.Terminal || opts.NoColor {
		return diff
	}

	var b strings.Builder
	lines := strings.Split(diff, "\n")
	for i, line := range lines {
		var style lipgloss.Style
		switch {
		case strings.HasPrefix(line, "diff --git"):
			style = lipgloss.NewStyle().Foreground(theme.DiffHeader).Bold(true)
		case strings.HasPrefix(line, "index "):
			style = lipgloss.NewStyle().Foreground(theme.DiffHeader)
		case strings.HasPrefix(line, "--- "):
			style = lipgloss.NewStyle().Foreground(theme.DiffHeader)
		case strings.HasPrefix(line, "+++ "):
			style = lipgloss.NewStyle().Foreground(theme.DiffHeader)
		case strings.HasPrefix(line, "@@"):
			style = lipgloss.NewStyle().Foreground(theme.DiffHunk)
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			style = lipgloss.NewStyle().Foreground(theme.DiffAdd)
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			style = lipgloss.NewStyle().Foreground(theme.DiffDel)
		default:
			style = lipgloss.NewStyle().Foreground(theme.Text)
		}
		b.WriteString(style.Render(line))
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// RenderDiffEnhanced renders a git diff with colored additions/deletions and a border.
func RenderDiffEnhanced(content, title string, theme Theme, opts RenderOptions) string { //nolint:gocritic // hugeParam: Theme passed by value for immutability
	return RenderDiffEnhancedWithMaxLines(content, title, 0, theme, opts)
}

// RenderDiffEnhancedWithMaxLines renders a git diff with optional truncation.
func RenderDiffEnhancedWithMaxLines(content, title string, maxLines int, theme Theme, opts RenderOptions) string { //nolint:gocritic,gocognit,gocyclo,funlen // hugeParam: Theme passed by value for immutability; complexity inherent in diff rendering with multiple modes
	// Truncate if requested
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
		return fmt.Sprintf("--- diff ---\n%s\n---", content)
	}

	lines := strings.Split(content, "\n")

	addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	metaStyle := lipgloss.NewStyle().Faint(true)

	var coloredLines []string
	for _, line := range lines {
		if line == "" {
			coloredLines = append(coloredLines, "")
			continue
		}

		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			coloredLines = append(coloredLines, addedStyle.Render(line))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			coloredLines = append(coloredLines, removedStyle.Render(line))
		case strings.HasPrefix(line, "@@") || strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			coloredLines = append(coloredLines, metaStyle.Render(line))
		default:
			coloredLines = append(coloredLines, line)
		}
	}

	maxWidth := 0
	for _, line := range coloredLines {
		visibleLen := lipgloss.Width(line)
		if visibleLen > maxWidth {
			maxWidth = visibleLen
		}
	}

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

	var result strings.Builder

	if title != "" {
		titleStr := fmt.Sprintf(" %s ", title)
		borderLen := maxWidth - len(titleStr)
		if borderLen < 0 {
			borderLen = 0
		}
		result.WriteString(theme.SystemStyle.Render(fmt.Sprintf("%s%s%s%s\n",
			topLeft,
			strings.Repeat(borderChar, borderLen/2),
			titleStr,
			strings.Repeat(borderChar, borderLen-borderLen/2)+topRight,
		)))
	} else {
		result.WriteString(theme.SystemStyle.Render(fmt.Sprintf("%s%s%s\n",
			topLeft,
			strings.Repeat(borderChar, maxWidth),
			topRight,
		)))
	}

	for _, line := range coloredLines {
		result.WriteString(theme.SystemStyle.Render("│") + " " + line + "\n")
	}

	result.WriteString(theme.SystemStyle.Render(fmt.Sprintf("%s%s%s",
		bottomLeft,
		strings.Repeat(borderChar, maxWidth),
		bottomRight,
	)))

	return result.String()
}

// RenderDiffLegacy maintains backward compatibility.
func RenderDiffLegacy(diff string, theme Theme) string { //nolint:gocritic // hugeParam: Theme passed by value for immutability
	return RenderDiff(diff, theme, NewRenderOptions())
}
