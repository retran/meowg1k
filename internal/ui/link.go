// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"os"
)

// RenderLink creates a clickable hyperlink using OSC 8 escape sequences.
// Modern terminals (iTerm2, kitty, WezTerm, Windows Terminal) support this.
// Falls back to plain text if terminal doesn't support it.
func RenderLink(text, url string, opts RenderOptions) string {
	// In plain mode or non-terminal, just show URL
	if opts.Plain || !opts.Terminal {
		if text == url {
			return url
		}
		return fmt.Sprintf("%s (%s)", text, url)
	}

	// Check if terminal likely supports OSC 8
	// Most modern terminals do, but we can be conservative
	termEnv := os.Getenv("TERM_PROGRAM")
	supportsOSC8 := termEnv == "iTerm.app" || 
		termEnv == "WezTerm" || 
		termEnv == "vscode" ||
		os.Getenv("WT_SESSION") != "" // Windows Terminal

	if !supportsOSC8 {
		// Fallback: show text with URL
		if text == url {
			return url
		}
		return fmt.Sprintf("%s (%s)", text, url)
	}

	// OSC 8 format: \e]8;;URL\e\\TEXT\e]8;;\e\\
	// \e is ESC (0x1b), ]8;; starts hyperlink, \e\\ ends parameters
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}
