// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
)

const dividerCharLine = "─"

// RenderDivider creates a horizontal divider line.
func RenderDivider(style string, theme Theme, opts RenderOptions) string { //nolint:gocritic,gocognit // hugeParam: Theme passed by value to avoid external mutation; complexity inherent in style selection
	if opts.Plain || !opts.Terminal {
		return "---"
	}

	// Get terminal width
	width := 80
	if fd := os.Stderr.Fd(); term.IsTerminal(fd) {
		if w, _, err := term.GetSize(fd); err == nil && w > 0 {
			width = w
		}
	}

	var char string
	switch style {
	case "line":
		if opts.SupportsUnicode {
			char = dividerCharLine
		} else {
			char = "-"
		}
	case "thick":
		if opts.SupportsUnicode {
			char = "━"
		} else {
			char = "="
		}
	case "dotted":
		if opts.SupportsUnicode {
			char = "·"
		} else {
			char = "."
		}
	case "empty":
		return ""
	default:
		char = dividerCharLine
	}

	line := strings.Repeat(char, width)
	return theme.SystemStyle.Render(line)
}

// LogDivider outputs a divider to the given writer.
func LogDivider(style string, theme Theme, opts RenderOptions, writer io.Writer) { //nolint:gocritic // hugeParam: Theme passed by value to avoid external mutation
	if writer == nil {
		writer = os.Stderr
	}

	divider := RenderDivider(style, theme, opts)
	if divider != "" {
		_, _ = fmt.Fprintln(writer, divider) //nolint:errcheck // write errors to stderr are intentionally ignored
	}
}
