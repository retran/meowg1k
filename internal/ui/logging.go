// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"os"
)

// LogThought outputs an italic "thinking" message (for LLM-generated reasoning).
func LogThought(message string, theme Theme, opts RenderOptions, writer io.Writer) { //nolint:gocritic // hugeParam: Theme passed by value to avoid external mutation
	if writer == nil {
		writer = os.Stderr
	}

	if opts.Plain || !opts.Terminal {
		_, _ = fmt.Fprintf(writer, "thinking: %s\n", message) //nolint:errcheck // write errors to stderr are intentionally ignored
		return
	}

	styled := theme.ThoughtStyle.Render("… " + message)
	_, _ = fmt.Fprintln(writer, styled) //nolint:errcheck // write errors to stderr are intentionally ignored
}

// LogAction outputs a mauve "action" message (for tool/API calls).
func LogAction(message string, theme Theme, opts RenderOptions, writer io.Writer) { //nolint:gocritic // hugeParam: Theme passed by value to avoid external mutation
	if writer == nil {
		writer = os.Stderr
	}

	if opts.Plain || !opts.Terminal {
		_, _ = fmt.Fprintf(writer, "action: %s\n", message) //nolint:errcheck // write errors to stderr are intentionally ignored
		return
	}

	styled := theme.ActionStyle.Render("› " + message)
	_, _ = fmt.Fprintln(writer, styled) //nolint:errcheck // write errors to stderr are intentionally ignored
}
