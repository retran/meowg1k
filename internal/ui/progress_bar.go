// Copyright © 2025 The meowg1k Authors.
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
	theme     Theme
	startTime time.Time
	writer    io.Writer
	message   string
	total     int
	current   int
	mu        sync.Mutex
	opts      RenderOptions
	done      bool
}

// NewProgressBar creates a new progress bar.
func NewProgressBar(total int, message string, theme Theme, opts RenderOptions, writer io.Writer) *ProgressBar { //nolint:gocritic // hugeParam: Theme passed by value to avoid external mutation
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

	pb.render()

	return pb
}

func (pb *ProgressBar) render() { //nolint:gocognit // complexity inherent in adaptive terminal progress bar rendering
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.done {
		return
	}

	percentage := 0
	if pb.total > 0 {
		percentage = (pb.current * 100) / pb.total
	}

	if pb.opts.Plain || !pb.opts.Terminal {
		_, _ = fmt.Fprintf(pb.writer, "> %s (%d/%d)\n", pb.message, pb.current, pb.total) //nolint:errcheck // write errors to stderr are intentionally ignored
		return
	}

	// Fixed 20-column width (never exceed 30, scale down for small terminals)
	width := 20
	if fd := os.Stderr.Fd(); term.IsTerminal(fd) { //nolint:nestif // nested terminal size detection with fallback width
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

	prefixStyle := lipgloss.NewStyle().Foreground(pb.theme.Action)
	filledStyle := lipgloss.NewStyle().Foreground(pb.theme.Spinner)
	emptyStyle := lipgloss.NewStyle().Foreground(pb.theme.Surface1)
	percentStyle := lipgloss.NewStyle().Foreground(pb.theme.Thought)

	// Format: › Message [████░░░░░░░░░░░░░░░░] 30%
	_, _ = fmt.Fprintf(pb.writer, "\r%s %s [%s%s] %s", //nolint:errcheck // write errors to stderr are intentionally ignored
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

	if pb.opts.Plain || !pb.opts.Terminal {
		_, _ = fmt.Fprintf(pb.writer, "+ %s · %d/%d · %s\n", message, pb.total, pb.total, duration.Round(time.Millisecond)) //nolint:errcheck // write errors to stderr are intentionally ignored
	} else {
		style := pb.theme.StatusSuccess
		_, _ = fmt.Fprintf(pb.writer, "\r\033[K%s\n", //nolint:errcheck // write errors to stderr are intentionally ignored
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

	if pb.opts.Plain || !pb.opts.Terminal {
		_, _ = fmt.Fprintf(pb.writer, "- %s at %d/%d · %s\n", message, pb.current, pb.total, duration.Round(time.Millisecond)) //nolint:errcheck // write errors to stderr are intentionally ignored
	} else {
		style := pb.theme.StatusError
		_, _ = fmt.Fprintf(pb.writer, "\r\033[K%s\n", //nolint:errcheck // write errors to stderr are intentionally ignored
			style.Render(fmt.Sprintf("✗ %s at %d/%d", message, pb.current, pb.total)))
	}
}
