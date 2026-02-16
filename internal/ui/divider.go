// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	
	"golang.org/x/term"
)

// RenderDivider creates a horizontal divider line.
func RenderDivider(style string, theme Theme, opts RenderOptions) string {
	if opts.Plain || !opts.Terminal {
		return "---"
	}
	
	// Get terminal width
	width := 80
	if fd := int(os.Stderr.Fd()); term.IsTerminal(fd) {
		if w, _, err := term.GetSize(fd); err == nil && w > 0 {
			width = w
		}
	}
	
	var char string
	switch style {
	case "line":
		if opts.SupportsUnicode {
			char = "─"
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
		char = "─"
	}
	
	line := strings.Repeat(char, width)
	return theme.SystemStyle.Render(line)
}

// LogDivider outputs a divider to the given writer.
func LogDivider(style string, theme Theme, opts RenderOptions, writer io.Writer) {
	if writer == nil {
		writer = os.Stderr
	}
	
	divider := RenderDivider(style, theme, opts)
	if divider != "" {
		fmt.Fprintln(writer, divider)
	}
}
