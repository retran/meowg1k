// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

// TerminalWidth returns the current terminal width or a fallback value.
func TerminalWidth(fallback int) int {
	width, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || width <= 0 {
		return fallback
	}
	return width
}

// IsTerminal checks if the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	return term.IsTerminal(fd)
}

// SupportsUnicode checks if the terminal supports Unicode characters.
// Checks LANG, LC_ALL, LC_CTYPE environment variables for UTF-8 encoding.
func SupportsUnicode() bool { //nolint:gocognit,gocyclo // complexity inherent in checking multiple locale environment variables
	for _, env := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		val := os.Getenv(env)
		if val != "" {
			val = strings.ToUpper(val)
			if strings.Contains(val, "UTF-8") || strings.Contains(val, "UTF8") {
				return true
			}
			if val == "C" || val == "POSIX" {
				return false
			}
			if strings.Contains(val, "ASCII") || strings.Contains(val, "ANSI") {
				return false
			}
		}
	}

	// Check TERM variable for hints
	termEnv := os.Getenv("TERM")
	if termEnv != "" {
		termEnv = strings.ToLower(termEnv)
		if strings.Contains(termEnv, "xterm") ||
			strings.Contains(termEnv, "screen") ||
			strings.Contains(termEnv, "tmux") ||
			strings.Contains(termEnv, "rxvt") ||
			strings.Contains(termEnv, "alacritty") ||
			strings.Contains(termEnv, "kitty") ||
			strings.Contains(termEnv, "iterm") {
			return true
		}
	}

	// Default to true: most modern terminals support UTF-8.
	return true
}

// IndentLines prefixes each line with the given indent string.
func IndentLines(text, indent string) string {
	if indent == "" || text == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

// VisibleWidth returns the display width of a string, ignoring ANSI sequences.
func VisibleWidth(text string) int {
	return lipgloss.Width(text)
}

// PadRight pads a string with spaces to a target visible width.
func PadRight(text string, width int) string {
	if width <= 0 {
		return ""
	}
	pad := width - VisibleWidth(text)
	if pad <= 0 {
		return text
	}
	return text + strings.Repeat(" ", pad)
}

// TruncatePlain truncates a plain string to a visible width and appends "...".
func TruncatePlain(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(text) <= width {
		return text
	}
	ellipsis := "..."
	if width <= len(ellipsis) {
		return ellipsis[:width]
	}
	return ansi.Truncate(text, width-len(ellipsis), "") + ellipsis
}

// Clamp bounds an integer to the inclusive [lo, hi] range.
func Clamp(value, lo, hi int) int {
	if value < lo {
		return lo
	}
	if value > hi {
		return hi
	}
	return value
}
