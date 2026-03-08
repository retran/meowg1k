// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TableOptions configures table rendering.
type TableOptions struct {
	Theme    Theme
	Title    string
	Query    string
	MaxWidth int
	Opts     RenderOptions
}

// RenderTable renders a simple ASCII table for rows and columns.
// In plain mode, renders as tab-separated values without borders.
func RenderTable(rows []map[string]string, columns []string, opts TableOptions) string { //nolint:gocritic // hugeParam: TableOptions passed by value for immutability
	if len(columns) == 0 {
		return ""
	}
	theme := opts.Theme
	if theme.Text == "" {
		theme = DefaultTheme()
	}

	if opts.Opts.Plain || !opts.Opts.Terminal || opts.Opts.NoBorders {
		return renderPlainTable(rows, columns, opts)
	}

	return renderBorderedTable(rows, columns, opts, theme)
}

// renderPlainTable renders a table without borders (TSV-style).
func renderPlainTable(rows []map[string]string, columns []string, opts TableOptions) string { //nolint:gocritic // hugeParam: TableOptions passed by value for immutability
	var b strings.Builder

	if opts.Title != "" {
		b.WriteString(opts.Title)
		b.WriteString("\n\n")
	}

	b.WriteString(strings.Join(columns, "\t"))
	b.WriteString("\n")

	for _, row := range rows {
		values := make([]string, len(columns))
		for i, col := range columns {
			values[i] = row[col]
		}
		b.WriteString(strings.Join(values, "\t"))
		b.WriteString("\n")
	}

	return b.String()
}

// renderBorderedTable renders a table with borders (Unicode or ASCII based on terminal support).
func renderBorderedTable(rows []map[string]string, columns []string, opts TableOptions, theme Theme) string { //nolint:gocritic // hugeParam: TableOptions and Theme passed by value for immutability
	maxWidth := opts.MaxWidth
	if maxWidth <= 0 {
		maxWidth = TerminalWidth(120)
	}

	colWidths := measureColumns(rows, columns)
	shrinkColumns(colWidths, maxWidth, len(columns))

	// Use Unicode box-drawing characters when supported; otherwise fall back to ASCII.
	var corner, horiz, vert string
	if opts.Opts.SupportsUnicode {
		corner = "┼"
		horiz = "─"
		vert = "│"
	} else {
		corner = "+"
		horiz = "-"
		vert = "|"
	}

	var b strings.Builder
	if opts.Title != "" {
		titleStyle := theme.PanelTitleStyle
		if opts.Opts.NoColor {
			titleStyle = lipgloss.NewStyle()
		}
		b.WriteString(titleStyle.Render(opts.Title))
		b.WriteString("\n")
	}

	borderLine := renderBorder(colWidths, corner, horiz)
	borderStyle := theme.TableBorder
	if opts.Opts.NoColor {
		borderStyle = lipgloss.NewStyle()
	}

	b.WriteString(borderStyle.Render(borderLine))
	b.WriteString("\n")

	headerStyle := theme.TableHeader
	if opts.Opts.NoColor {
		headerStyle = lipgloss.NewStyle()
	}
	headerLine := renderRow(columns, colWidths, headerStyle, opts.Query, theme.SelectMatch, opts.Opts.NoColor, vert)
	b.WriteString(headerLine)
	b.WriteString("\n")
	b.WriteString(borderStyle.Render(borderLine))
	b.WriteString("\n")

	for i, row := range rows {
		values := make([]string, len(columns))
		for c, col := range columns {
			values[c] = row[col]
		}
		style := theme.TableRow
		if i%2 == 1 {
			style = theme.TableAltRow
		}
		if opts.Opts.NoColor {
			style = lipgloss.NewStyle()
		}
		line := renderRow(values, colWidths, style, opts.Query, theme.SelectMatch, opts.Opts.NoColor, vert)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString(borderStyle.Render(borderLine))
	return b.String()
}

func renderBorder(widths []int, corner, horiz string) string {
	var b strings.Builder
	b.WriteString(corner)
	for i, w := range widths {
		b.WriteString(strings.Repeat(horiz, w+2))
		if i < len(widths)-1 {
			b.WriteString(corner)
		}
	}
	b.WriteString(corner)
	return b.String()
}

func renderRow(values []string, widths []int, style lipgloss.Style, query string, matchStyle lipgloss.Style, noColor bool, vert string) string { //nolint:gocritic // hugeParam: lipgloss.Style passed by value as required by API
	var b strings.Builder
	b.WriteString(vert)
	for i, val := range values {
		content := val
		if width := widths[i]; width > 0 {
			content = TruncatePlain(content, width)
		}
		if query != "" && !noColor {
			content = HighlightSubstring(content, query, matchStyle)
		}
		content = PadRight(content, widths[i])
		cell := " " + content + " "
		b.WriteString(style.Render(cell))
		b.WriteString(vert)
	}
	return b.String()
}

func measureColumns(rows []map[string]string, columns []string) []int {
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = VisibleWidth(col)
	}
	for _, row := range rows {
		for i, col := range columns {
			value := row[col]
			if w := VisibleWidth(value); w > widths[i] {
				widths[i] = w
			}
		}
	}
	for i, w := range widths {
		if w == 0 {
			widths[i] = 1
		}
	}
	return widths
}

func shrinkColumns(widths []int, maxWidth int, columns int) {
	if maxWidth <= 0 || columns == 0 {
		return
	}
	total := tableWidth(widths)
	if total <= maxWidth {
		return
	}

	minWidth := 6
	overflow := total - maxWidth
	for overflow > 0 {
		shrunk := false
		for i := len(widths) - 1; i >= 0 && overflow > 0; i-- {
			if widths[i] > minWidth {
				widths[i]--
				overflow--
				shrunk = true
			}
		}
		if !shrunk {
			break
		}
	}
}

func tableWidth(widths []int) int {
	total := 1
	for _, w := range widths {
		total += w + 3
	}
	return total
}

// HighlightSubstring highlights the first case-insensitive match of query in text.
func HighlightSubstring(text, query string, style lipgloss.Style) string { //nolint:gocritic // hugeParam: lipgloss.Style passed by value as required by API
	if query == "" {
		return text
	}
	lower := strings.ToLower(text)
	q := strings.ToLower(query)
	idx := strings.Index(lower, q)
	if idx == -1 {
		return text
	}
	head := text[:idx]
	match := text[idx : idx+len(query)]
	tail := text[idx+len(query):]
	return head + style.Render(match) + tail
}
