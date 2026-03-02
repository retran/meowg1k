// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package output provides services for writing formatted output and progress feedback to the console.
package output

import (
	"fmt"
	"io"
	"os"

	"github.com/retran/meowg1k/internal/adapters/output/tui"
	outputdomain "github.com/retran/meowg1k/internal/domain/output"
	"github.com/retran/meowg1k/internal/ui"
)

// Service is the concrete implementation of the Writer interface.
//
// TTY mode  — a Bubble Tea program (tui.Program) is started on the first call
// that needs live output; all subsequent writes are dispatched as messages to
// that program.  Print* calls send LogLineMsg so they appear in the block log.
// Flush() stops the BubbleTea program, then writes the accumulated output
// buffer to stdout.
//
// Non-TTY / plain mode — no BubbleTea program is started.  Print* calls write
// directly to the destination.  Flush() writes the output buffer to stdout.
//
// ctx.output is a pure byte buffer.  Streaming preview (LLM token deltas) is
// handled separately by the ui module (ui.stream), not by this service.
type Service struct {
	destination io.Writer
	plainOutput bool
	noColor     bool
	isTerminal  bool

	// prog is the BubbleTea program (TTY path only, lazily started).
	prog *tui.Program
}

// NewService creates a new instance of the output service.
func NewService(destination outputdomain.Destination) *Service {
	return NewServiceWithOptions(destination, false, false)
}

// NewServiceWithOptions creates a new instance with formatting options.
func NewServiceWithOptions(destination outputdomain.Destination, plainOutput, noColor bool) *Service {
	var destWriter io.Writer
	var isTerminal bool

	switch destination {
	case outputdomain.Stdout:
		destWriter = os.Stdout
		if f, ok := destWriter.(*os.File); ok {
			isTerminal = ui.IsTerminal(f.Fd())
		}
	case outputdomain.Stderr:
		destWriter = os.Stderr
		if f, ok := destWriter.(*os.File); ok {
			isTerminal = ui.IsTerminal(f.Fd())
		}
	case outputdomain.Discard:
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

// tuiActive returns true when we should route output through the BubbleTea program.
func (s *Service) tuiActive() bool {
	return s.isTerminal && !s.plainOutput
}

// ensureProgram lazily starts the BubbleTea program on the first TTY write.
func (s *Service) ensureProgram() {
	if s.prog == nil {
		s.prog = tui.NewInline(s.noColor)
	}
}

// Print writes content without a trailing newline.
func (s *Service) Print(content string) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}
	if !s.tuiActive() {
		_, err := fmt.Fprint(s.destination, content)
		return err
	}
	s.ensureProgram()
	s.prog.Send(tui.LogLineMsg{Line: content})
	return nil
}

// PrintLine writes content followed by a newline.
func (s *Service) PrintLine(content string) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}
	if !s.tuiActive() {
		_, err := fmt.Fprintln(s.destination, content)
		return err
	}
	s.ensureProgram()
	s.prog.Send(tui.LogLineMsg{Line: content})
	return nil
}

// Printf writes formatted content.
func (s *Service) Printf(format string, args ...any) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}
	text := fmt.Sprintf(format, args...)
	if !s.tuiActive() {
		_, err := fmt.Fprint(s.destination, text)
		return err
	}
	s.ensureProgram()
	s.prog.Send(tui.LogLineMsg{Line: text})
	return nil
}

// LogWriter returns an io.Writer that appends static lines to the TUI block
// log on TTY, or writes directly to the destination on non-TTY.
// Use this to route ui.step / ui.success / ui.divider output through the
// same program that owns the streaming area, preventing interleaved writes.
func (s *Service) LogWriter() io.Writer {
	if s == nil || !s.tuiActive() {
		if s == nil {
			return io.Discard
		}
		return s.destination
	}
	return &tuiLogWriter{svc: s}
}

// tuiLogWriter is an io.Writer that forwards each write as a LogLineMsg.
type tuiLogWriter struct{ svc *Service }

func (w *tuiLogWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.svc.ensureProgram()
	w.svc.prog.Send(tui.LogLineMsg{Line: string(p)})
	return len(p), nil
}

// StreamToken sends a streaming token delta to the TUI on TTY, no-op on non-TTY.
// delta is the raw text chunk; done=true seals the current StreamBlock.
func (s *Service) StreamToken(delta string, done bool) {
	if s == nil || !s.tuiActive() {
		return
	}
	s.ensureProgram()
	if done {
		if delta != "" {
			s.prog.Send(tui.TokenDeltaMsg{Text: delta})
		}
		s.prog.Send(tui.StreamDoneMsg{})
	} else if delta != "" {
		s.prog.Send(tui.TokenDeltaMsg{Text: delta})
	}
}

// IsTTY returns true if the output destination is a terminal.
func (s *Service) IsTTY() bool {
	if s == nil {
		return false
	}
	return s.isTerminal
}

// Flush stops the BubbleTea program (if running) and waits for it to exit,
// restoring the terminal.  On non-TTY it is a no-op.
func (s *Service) Flush() error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}
	if s.prog != nil {
		s.prog.Stop()
		s.prog = nil
	}
	return nil
}
