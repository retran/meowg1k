// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

// ProgressBar represents a deterministic progress indicator.
type ProgressBar struct {
	total     int
	current   int
	message   string
	done      bool
	mu        sync.Mutex
	theme     Theme
	opts      RenderOptions
	writer    io.Writer
	startTime time.Time
}

// NewProgressBar creates a new progress bar.
func NewProgressBar(total int, message string, theme Theme, opts RenderOptions, writer io.Writer) *ProgressBar {
	if writer == nil {
		writer = os.Stderr
	}

	pb := &ProgressBar{
		total:     total,
		current:   0,
		message:   message,
		done:      false,
		theme:     theme,
		opts:      opts,
		writer:    writer,
		startTime: time.Now(),
	}

	// Initial render
	pb.render()

	return pb
}

func (pb *ProgressBar) render() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	percentage := 0
	if pb.total > 0 {
		percentage = (pb.current * 100) / pb.total
	}

	// Plain/CI mode: milestone output (no bar)
	if pb.opts.Plain || !pb.opts.Terminal {
		fmt.Fprintf(pb.writer, "> %s (%d/%d)\n", pb.message, pb.current, pb.total)
		return
	}

	// Terminal mode: visual progress bar
	// Fixed 20-column width (never exceed 30, scale down for small terminals)
	width := 20
	if fd := os.Stderr.Fd(); term.IsTerminal(fd) {
		if w, _, err := term.GetSize(fd); err == nil {
			if w < 60 {
				width = 15 // Smaller for narrow terminals
			} else if w > 100 {
				width = 25 // Slightly larger for wide terminals
			}
		}
	}
	if width > 30 {
		width = 30
	}

	filled := 0
	if pb.total > 0 {
		filled = (pb.current * width) / pb.total
	}

	// Build bar with blocks
	filledChar := "█"
	emptyChar := "░"

	if !pb.opts.SupportsUnicode {
		filledChar = "#"
		emptyChar = "-"
	}

	var filledPart, emptyPart strings.Builder
	for i := 0; i < filled; i++ {
		filledPart.WriteString(filledChar)
	}
	for i := filled; i < width; i++ {
		emptyPart.WriteString(emptyChar)
	}

	// Style the parts
	prefixStyle := lipgloss.NewStyle().Foreground(pb.theme.Action)
	filledStyle := lipgloss.NewStyle().Foreground(pb.theme.Spinner) // Teal
	emptyStyle := lipgloss.NewStyle().Foreground(pb.theme.Surface1)
	percentStyle := lipgloss.NewStyle().Foreground(pb.theme.Thought) // Subtext0

	// Format: › Message [████░░░░░░░░░░░░░░░░] 30%
	fmt.Fprintf(pb.writer, "\r%s %s [%s%s] %s",
		prefixStyle.Render("›"),
		pb.message,
		filledStyle.Render(filledPart.String()),
		emptyStyle.Render(emptyPart.String()),
		percentStyle.Render(fmt.Sprintf("%3d%%", percentage)))
}

// Inc increments the progress by the given amount.
func (pb *ProgressBar) Inc(amount int) {
	pb.mu.Lock()
	pb.current += amount
	if pb.current > pb.total {
		pb.current = pb.total
	}
	pb.mu.Unlock()

	pb.render()
}

// Set sets the progress to a specific value.
func (pb *ProgressBar) Set(value int) {
	pb.mu.Lock()
	pb.current = value
	if pb.current > pb.total {
		pb.current = pb.total
	}
	pb.mu.Unlock()

	pb.render()
}

// Done completes the progress bar.
func (pb *ProgressBar) Done(message string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	pb.done = true
	pb.current = pb.total
	duration := time.Since(pb.startTime)

	// Final render - collapse bar into status line
	if pb.opts.Plain || !pb.opts.Terminal {
		fmt.Fprintf(pb.writer, "+ %s · %d/%d · %s\n", message, pb.total, pb.total, duration.Round(time.Millisecond))
	} else {
		style := pb.theme.StatusSuccess
		fmt.Fprintf(pb.writer, "\r\033[K%s\n",
			style.Render(fmt.Sprintf("✓ %s · %d/%d · %s", message, pb.total, pb.total, duration.Round(time.Millisecond))))
	}
}

// Fail completes the progress bar with an error.
func (pb *ProgressBar) Fail(message string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	pb.done = true
	duration := time.Since(pb.startTime)

	// Final render - collapse bar into error status
	if pb.opts.Plain || !pb.opts.Terminal {
		fmt.Fprintf(pb.writer, "- %s at %d/%d · %s\n", message, pb.current, pb.total, duration.Round(time.Millisecond))
	} else {
		style := pb.theme.StatusError
		fmt.Fprintf(pb.writer, "\r\033[K%s\n",
			style.Render(fmt.Sprintf("✗ %s at %d/%d", message, pb.current, pb.total)))
	}
}
