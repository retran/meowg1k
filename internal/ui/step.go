// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Step represents a hierarchical visual group for related operations.
// It renders with borders and vertical lines to show context nesting.
type Step struct {
	title   string
	icon    string
	indent  int
	theme   Theme
	opts    RenderOptions
	started time.Time
	writer  io.Writer
	active  bool
}

// NewStep creates and starts a new visual step context.
// The step will render a top border immediately.
func NewStep(title, icon string, indent int, theme Theme, opts RenderOptions, writer io.Writer) *Step {
	if writer == nil {
		writer = os.Stderr
	}

	step := &Step{
		title:   title,
		icon:    icon,
		indent:  indent,
		theme:   theme,
		opts:    opts,
		started: time.Now(),
		writer:  writer,
		active:  true,
	}

	step.renderTop()
	return step
}

// renderTop renders the top border of the step.
func (s *Step) renderTop() {
	if s.opts.Plain || !s.opts.Terminal {
		// Plain mode: ASCII header
		fmt.Fprintf(s.writer, "-- %s\n", s.title)
		return
	}

	// Build indentation prefix
	indentPrefix := strings.Repeat("│  ", s.indent)

	// Choose border characters
	var topLeft string
	if s.opts.SupportsUnicode {
		topLeft = "╭─"
	} else {
		topLeft = "--"
	}

	// Icon and title (icon is optional, prefer text-only for professionalism)
	iconPart := ""
	if s.icon != "" {
		iconPart = s.icon + " "
	}

	titlePart := s.theme.StepTitle.Render(iconPart + s.title)

	// Build the top line
	borderStyle := s.theme.StepBorder

	// Minimal style: No right tail
	topLine := fmt.Sprintf("%s%s %s",
		indentPrefix,
		borderStyle.Render(topLeft),
		titlePart,
	)

	fmt.Fprintln(s.writer, topLine)
}

// Write outputs content within the step context (with vertical line prefix).
func (s *Step) Write(text string) {
	if !s.active {
		return
	}

	if s.opts.Plain || !s.opts.Terminal {
		// Plain mode: just output text
		fmt.Fprintln(s.writer, text)
		return
	}

	// Build indentation with vertical lines
	indentPrefix := strings.Repeat("│  ", s.indent)

	var vert string
	if s.opts.SupportsUnicode {
		vert = "│"
	} else {
		vert = "|"
	}

	borderStyle := s.theme.StepBorder
	prefix := indentPrefix + borderStyle.Render(vert) + "  "

	// Handle multi-line text
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Fprintf(s.writer, "%s%s\n", prefix, line)
		}
	}
}

// Done completes the step successfully and renders the bottom border.
func (s *Step) Done(message string) {
	if !s.active {
		return
	}

	s.active = false
	duration := time.Since(s.started)

	if s.opts.Plain || !s.opts.Terminal {
		// Plain mode: simple completion message
		if message != "" {
			fmt.Fprintf(s.writer, "+ %s\n", message)
		}
		return
	}

	s.renderBottom(true, message, duration)
}

// Fail completes the step with an error and renders the bottom border.
func (s *Step) Fail(message string) {
	if !s.active {
		return
	}

	s.active = false
	duration := time.Since(s.started)

	if s.opts.Plain || !s.opts.Terminal {
		// Plain mode: simple error message
		if message != "" {
			fmt.Fprintf(s.writer, "- %s\n", message)
		}
		return
	}

	s.renderBottom(false, message, duration)
}

// renderBottom renders the bottom border with optional message.
func (s *Step) renderBottom(success bool, message string, duration time.Duration) {
	// Build indentation prefix
	indentPrefix := strings.Repeat("│  ", s.indent)

	var bottomLeft string
	if s.opts.SupportsUnicode {
		bottomLeft = "╰"
	} else {
		bottomLeft = "+"
	}

	// Choose style and icon
	var statusIcon string
	if success {
		statusIcon = s.theme.StepSuccess.Render("✓")
	} else {
		statusIcon = s.theme.StepError.Render("✗")
	}

	// Format message with duration
	finalMessage := ""
	if message != "" {
		if duration > 0 {
			finalMessage = fmt.Sprintf("%s · %.2fs", message, duration.Seconds())
		} else {
			finalMessage = message
		}
		// Apply color to status icon and message
		finalMessage = statusIcon + " " + finalMessage
	}

	borderStyle := s.theme.StepBorder

	// Connector for bottom line
	connector := "─"
	if !s.opts.SupportsUnicode {
		connector = "-"
	}

	// Render bottom line
	var bottomLine string

	if finalMessage != "" {
		// With message: ╰─ Message
		bottomLine = fmt.Sprintf("%s%s %s",
			indentPrefix,
			borderStyle.Render(bottomLeft+connector),
			finalMessage,
		)
	} else {
		// Just close: ╰─
		bottomLine = fmt.Sprintf("%s%s",
			indentPrefix,
			borderStyle.Render(bottomLeft+connector),
		)
	}

	fmt.Fprintln(s.writer, bottomLine)
	fmt.Fprintln(s.writer) // Empty line after section
}

// Close implements io.Closer for use with defer.
func (s *Step) Close() error {
	if s.active {
		s.Done("")
	}
	return nil
}

// Indent returns the current indentation level.
func (s *Step) Indent() int {
	return s.indent
}
