// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package output provides services for writing formatted output and progress feedback to the console.
package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gosuri/uilive"
	"github.com/retran/meowg1k/internal/domain/output"
	"github.com/retran/meowg1k/internal/ui"
)

// Service is the concrete implementation of the Writer interface.
type Service struct {
	destination io.Writer
	buffer      strings.Builder
	liveWriter  *uilive.Writer
	liveBuffer  strings.Builder
	liveActive  bool
	plainOutput bool
	noColor     bool
	isTerminal  bool
}

// NewService creates a new instance of the buffered output service.
func NewService(destination output.Destination) *Service {
	return NewServiceWithOptions(destination, false, false)
}

// NewServiceWithOptions creates a new instance with formatting options.
func NewServiceWithOptions(destination output.Destination, plainOutput, noColor bool) *Service {
	var destWriter io.Writer
	var isTerminal bool

	switch destination {
	case output.Stdout:
		destWriter = os.Stdout
		if f, ok := destWriter.(*os.File); ok {
			isTerminal = ui.IsTerminal(f.Fd())
		}
	case output.Stderr:
		destWriter = os.Stderr
		if f, ok := destWriter.(*os.File); ok {
			isTerminal = ui.IsTerminal(f.Fd())
		}
	case output.Discard:
		destWriter = io.Discard
	default:
		destWriter = io.Discard
	}

	return &Service{
		destination: destWriter,
		plainOutput: plainOutput,
		noColor:     noColor,
		isTerminal:  isTerminal,
	}
}

// Print adds content to the buffer.
func (s *Service) Print(content string) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}

	s.buffer.WriteString(content)
	return nil
}

// PrintLine adds content with a newline to the buffer.
func (s *Service) PrintLine(content string) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}

	s.buffer.WriteString(content)
	s.buffer.WriteString("\n")
	return nil
}

// Printf adds formatted content to the buffer.
func (s *Service) Printf(format string, args ...any) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}

	_, err := fmt.Fprintf(&s.buffer, format, args...)
	if err != nil {
		return fmt.Errorf("failed to write to buffer: %w", err)
	}

	return nil
}

// PrintMarkdown renders Markdown to terminal output and buffers it.
func (s *Service) PrintMarkdown(content string) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}

	// Skip formatting if plain output is requested or output is not to a terminal
	if s.plainOutput || !s.isTerminal {
		s.buffer.WriteString(content)
		return nil
	}

	rendered, err := ui.RenderMarkdown(content, ui.TerminalWidth(120), s.noColor)
	if err != nil {
		rendered = content
	}
	s.buffer.WriteString(rendered)
	return nil
}

// StreamMarkdown renders Markdown incrementally and updates the terminal live area.
func (s *Service) StreamMarkdown(content string, done bool) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}

	// For plain output or non-terminal, just buffer and print when done
	if s.plainOutput || !s.isTerminal {
		s.liveBuffer.WriteString(content)
		if done {
			s.buffer.WriteString(s.liveBuffer.String())
			s.liveBuffer.Reset()
		}
		return nil
	}

	if !s.liveActive {
		s.liveWriter = uilive.New()
		s.liveWriter.Out = s.destination
		s.liveWriter.Start()
		s.liveActive = true
	}

	s.liveBuffer.WriteString(content)
	rendered, err := ui.RenderMarkdown(s.liveBuffer.String(), ui.TerminalWidth(120), s.noColor)
	if err != nil {
		rendered = s.liveBuffer.String()
	}
	_, writeErr := fmt.Fprint(s.liveWriter, rendered)
	if writeErr != nil {
		return fmt.Errorf("failed to stream markdown: %w", writeErr)
	}

	if done {
		s.liveWriter.Stop()
		s.liveActive = false
		s.liveBuffer.Reset()
		_, _ = fmt.Fprint(s.destination, "\n")
	}

	return nil
}

// IsTTY returns true if the output destination is a terminal.
func (s *Service) IsTTY() bool {
	if s == nil {
		return false
	}
	return s.isTerminal
}

// Flush writes all accumulated content from the buffer to the destination
// in a single write operation and then clears the buffer.
func (s *Service) Flush() error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}

	if s.liveActive {
		s.liveWriter.Stop()
		s.liveActive = false
		s.liveBuffer.Reset()
	}

	if s.buffer.Len() == 0 {
		return nil
	}

	content := s.buffer.String()
	s.buffer.Reset()

	_, err := fmt.Fprint(s.destination, content)
	if err != nil {
		return fmt.Errorf("failed to write to destination: %w", err)
	}

	return nil
}
