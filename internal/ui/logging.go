// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"os"
)

// LogThought outputs an italic "thinking" message (for LLM-generated reasoning).
func LogThought(message string, theme Theme, opts RenderOptions, writer io.Writer) {
	if writer == nil {
		writer = os.Stderr
	}

	if opts.Plain || !opts.Terminal {
		// Plain mode: prefix with "thinking:"
		fmt.Fprintf(writer, "thinking: %s\n", message)
		return
	}

	// Styled output - Subtext color, quiet and skippable
	styled := theme.ThoughtStyle.Render("… " + message)
	fmt.Fprintln(writer, styled)
}

// LogAction outputs a mauve "action" message (for tool/API calls).
func LogAction(message string, theme Theme, opts RenderOptions, writer io.Writer) {
	if writer == nil {
		writer = os.Stderr
	}

	if opts.Plain || !opts.Terminal {
		// Plain mode: prefix with "action:"
		fmt.Fprintf(writer, "action: %s\n", message)
		return
	}

	// Styled output
	styled := theme.ActionStyle.Render("› " + message)
	fmt.Fprintln(writer, styled)
}
