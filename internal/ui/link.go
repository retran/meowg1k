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
	if opts.Plain || !opts.Terminal {
		if text == url {
			return url
		}
		return fmt.Sprintf("%s (%s)", text, url)
	}

	// OSC 8 hyperlinks are supported by iTerm2, WezTerm, vscode, and Windows
	// Terminal (WT_SESSION). Fall back to plain text for other terminals.
	termEnv := os.Getenv("TERM_PROGRAM")
	supportsOSC8 := termEnv == "iTerm.app" ||
		termEnv == "WezTerm" ||
		termEnv == "vscode" ||
		os.Getenv("WT_SESSION") != ""

	if !supportsOSC8 {
		if text == url {
			return url
		}
		return fmt.Sprintf("%s (%s)", text, url)
	}

	// OSC 8 format: \e]8;;URL\e\\TEXT\e]8;;\e\\
	// \e is ESC (0x1b), ]8;; starts the hyperlink, \e\\ ends parameters.
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}
