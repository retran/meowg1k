// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/mattn/go-runewidth"
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
func SupportsUnicode() bool {
	// Check common environment variables
	for _, env := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		val := os.Getenv(env)
		if val != "" {
			val = strings.ToUpper(val)
			// Check for UTF-8 encoding
			if strings.Contains(val, "UTF-8") || strings.Contains(val, "UTF8") {
				return true
			}
			// If explicitly set to C/POSIX, no Unicode support
			if val == "C" || val == "POSIX" {
				return false
			}
			// If explicitly set to something else without UTF, probably no Unicode
			if strings.Contains(val, "ASCII") || strings.Contains(val, "ANSI") {
				return false
			}
		}
	}
	
	// Check TERM variable for hints
	term := os.Getenv("TERM")
	if term != "" {
		term = strings.ToLower(term)
		// Modern terminal emulators typically support Unicode
		if strings.Contains(term, "xterm") || 
		   strings.Contains(term, "screen") ||
		   strings.Contains(term, "tmux") ||
		   strings.Contains(term, "rxvt") ||
		   strings.Contains(term, "alacritty") ||
		   strings.Contains(term, "kitty") ||
		   strings.Contains(term, "iterm") {
			return true
		}
	}
	
	// Default to true for modern systems
	// Most terminals support UTF-8 nowadays
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
	if runewidth.StringWidth(text) <= width {
		return text
	}
	ellipsis := "..."
	if width <= len(ellipsis) {
		return ellipsis[:width]
	}
	target := width - len(ellipsis)
	var b strings.Builder
	b.Grow(len(text))
	current := 0
	for _, r := range text {
		rw := runewidth.RuneWidth(r)
		if current+rw > target {
			break
		}
		b.WriteRune(r)
		current += rw
	}
	b.WriteString(ellipsis)
	return b.String()
}

// Clamp bounds an integer to the inclusive [min, max] range.
func Clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
