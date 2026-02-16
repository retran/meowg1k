// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RenderWithPager displays content in a pager (like less) if it's too long.
// Falls back to direct output if pager is not available or content is short.
func RenderWithPager(content, title string, lineNumbers bool, opts RenderOptions) error {
	lines := strings.Split(content, "\n")
	
	// In plain mode or if content is short, just print it
	if opts.Plain || !opts.Terminal || len(lines) <= 30 {
		if title != "" {
			fmt.Fprintf(os.Stderr, "=== %s ===\n", title)
		}
		if lineNumbers {
			for i, line := range lines {
				fmt.Fprintf(os.Stderr, "%4d  %s\n", i+1, line)
			}
		} else {
			fmt.Fprintln(os.Stderr, content)
		}
		return nil
	}

	// Try to use less as pager
	lessPath, err := exec.LookPath("less")
	if err != nil {
		// Fallback to direct output if less not found
		if title != "" {
			fmt.Fprintf(os.Stderr, "=== %s ===\n", title)
		}
		fmt.Fprintln(os.Stderr, content)
		return nil
	}

	// Prepare content with line numbers if requested
	var displayContent string
	if lineNumbers {
		var numbered strings.Builder
		for i, line := range lines {
			numbered.WriteString(fmt.Sprintf("%4d  %s\n", i+1, line))
		}
		displayContent = numbered.String()
	} else {
		displayContent = content
	}

	// Add title if present
	if title != "" {
		displayContent = fmt.Sprintf("=== %s ===\n\n%s", title, displayContent)
	}

	// Setup less with appropriate flags:
	// -R: handle ANSI colors
	// -F: quit if content fits on one screen
	// -X: don't clear screen on exit
	// -S: chop long lines instead of wrapping
	cmd := exec.Command(lessPath, "-R", "-F", "-X", "-S")
	cmd.Stdin = strings.NewReader(displayContent)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// TruncateContent truncates content to maxLines with an indicator.
func TruncateContent(content string, maxLines int, opts RenderOptions) (string, bool) {
	if maxLines <= 0 {
		return content, false
	}

	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content, false
	}

	// Truncate and add indicator
	truncated := strings.Join(lines[:maxLines], "\n")
	
	var indicator string
	if opts.SupportsUnicode {
		indicator = fmt.Sprintf("\n⋮ [%d more lines]", len(lines)-maxLines)
	} else {
		indicator = fmt.Sprintf("\n... [%d more lines]", len(lines)-maxLines)
	}
	
	return truncated + indicator, true
}
