// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package output provides services for writing formatted output and progress feedback to the console.
package output

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/retran/meowg1k/internal/adapters/output/tui"
	outputdomain "github.com/retran/meowg1k/internal/domain/output"
	"github.com/retran/meowg1k/internal/ui"
)

// Service is the concrete implementation of ports.UIWriter.
//
// TTY mode — a Bubble Tea program (tui.Program) is started on the first call
// that needs live output.  TurnWriter methods are dispatched as messages to
// that program.  Print*/output calls buffer into outputBuf.
// Flush() stops the BubbleTea program, then copies outputBuf to destination.
//
// Non-TTY / plain mode — no BubbleTea program is started.  TurnWriter methods
// are no-ops.  Print* calls buffer into outputBuf.  Flush() copies outputBuf
// to destination.
//
// ctx.output is a pure byte buffer.  Streaming preview (LLM token deltas) is
// handled by TurnWriter.StreamToken (routes through the TUI StreamBlock).
type Service struct {
	destination io.Writer
	plainOutput bool
	noColor     bool
	isTerminal  bool
	cancel      func() // optional: called when user presses Ctrl+C in TUI

	// outputBuf accumulates all ctx.output.write* calls.
	// Flushed to destination when Flush() is called.
	outputBuf bytes.Buffer

	// prog is the BubbleTea program (TTY path only, lazily started).
	prog  *tui.Program
	theme ui.Theme
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

	opts := ui.NewRenderOptions()
	opts.NoColor = noColor
	opts.Plain = plainOutput
	theme := ui.DefaultThemeWithOptions(opts)

	return &Service{
		destination: destWriter,
		plainOutput: plainOutput,
		noColor:     noColor,
		isTerminal:  isTerminal,
		theme:       theme,
	}
}

// tuiActive returns true when we should route chrome through the BubbleTea program.
func (s *Service) tuiActive() bool {
	return s.isTerminal && !s.plainOutput
}

// ensureProgram lazily starts the BubbleTea program on the first TTY write.
func (s *Service) ensureProgram() {
	if s.prog == nil {
		s.prog = tui.New(s.theme, s.noColor, s.cancel)
	}
}

// ---------------------------------------------------------------------------
// OutputWriter implementation (buffered plain-text output)
// ---------------------------------------------------------------------------

// Print writes content without a trailing newline into the output buffer.
func (s *Service) Print(content string) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}
	_, err := fmt.Fprint(&s.outputBuf, content)
	return err
}

// PrintLine writes content followed by a newline into the output buffer.
func (s *Service) PrintLine(content string) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}
	_, err := fmt.Fprintln(&s.outputBuf, content)
	return err
}

// Printf writes formatted content into the output buffer.
func (s *Service) Printf(format string, args ...any) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}
	_, err := fmt.Fprintf(&s.outputBuf, format, args...)
	return err
}

// ---------------------------------------------------------------------------
// TurnWriter implementation
// ---------------------------------------------------------------------------

// SendHeader emits a one-line header above the conversation log.
func (s *Service) SendHeader(text string) {
	if s == nil || !s.tuiActive() {
		return
	}
	s.ensureProgram()
	s.prog.Send(tui.HeaderMsg{Text: text})
}

// BeginUserTurn opens a user-turn bubble with the given text.
func (s *Service) BeginUserTurn(text string) {
	if s == nil || !s.tuiActive() {
		return
	}
	s.ensureProgram()
	s.prog.Send(tui.BeginUserTurnMsg{Text: text})
}

// BeginAssistantTurn opens a new live assistant turn block.
func (s *Service) BeginAssistantTurn() {
	if s == nil || !s.tuiActive() {
		return
	}
	s.ensureProgram()
	s.prog.Send(tui.BeginAssistantTurnMsg{})
}

// OpenStep creates a new live step inside the current assistant turn.
func (s *Service) OpenStep(text string) string {
	if s == nil || !s.tuiActive() {
		return ""
	}
	s.ensureProgram()
	id := fmt.Sprintf("step-%d", nextStepID())
	s.prog.Send(tui.OpenStepMsg{ID: id, Text: text})
	return id
}

// UpdateStep changes the label of an open step.
func (s *Service) UpdateStep(id, text string) {
	if s == nil || !s.tuiActive() || id == "" {
		return
	}
	s.ensureProgram()
	s.prog.Send(tui.UpdateStepMsg{ID: id, Text: text})
}

// AddStepInfo appends an info line to an open step.
func (s *Service) AddStepInfo(id, text string) {
	if s == nil || !s.tuiActive() || id == "" {
		return
	}
	s.ensureProgram()
	s.prog.Send(tui.AddStepInfoMsg{ID: id, Text: text})
}

// CloseStep marks a step done (ok=true) or failed (ok=false).
func (s *Service) CloseStep(id string, ok bool, summary string) {
	if s == nil || !s.tuiActive() || id == "" {
		return
	}
	s.ensureProgram()
	s.prog.Send(tui.CloseStepMsg{ID: id, OK: ok, Summary: summary})
}

// StreamToken delivers one LLM token delta to the TUI on TTY, no-op on non-TTY.
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

// BeginSubTurn opens a nested subturn with the given label inside the current
// assistant turn. Subsequent step and stream calls route into this subturn.
func (s *Service) BeginSubTurn(label string) {
	if s == nil || !s.tuiActive() {
		return
	}
	s.ensureProgram()
	s.prog.Send(tui.BeginSubTurnMsg{Label: label})
}

// EndSubTurn closes the active subturn. Subsequent calls route back to the
// parent turn.
func (s *Service) EndSubTurn() {
	if s == nil || !s.tuiActive() {
		return
	}
	s.ensureProgram()
	s.prog.Send(tui.EndSubTurnMsg{})
}

// EndTurn closes the current assistant turn with an optional summary line.
func (s *Service) EndTurn(summary string) {
	if s == nil || !s.tuiActive() {
		return
	}
	s.ensureProgram()
	s.prog.Send(tui.EndTurnMsg{Summary: summary})
}

// SetStatus updates the single-line pinned status bar at the bottom.
func (s *Service) SetStatus(text string) {
	if s == nil || !s.tuiActive() {
		return
	}
	s.ensureProgram()
	s.prog.Send(tui.SetStatusMsg{Text: text})
}

// ---------------------------------------------------------------------------
// UIWriter extras
// ---------------------------------------------------------------------------

// SetCancel registers a cancel func that is invoked when the user presses
// Ctrl+C or q in the TUI. Must be called before the BubbleTea program starts
// (i.e. before the first TurnWriter call).
func (s *Service) SetCancel(cancel func()) {
	if s == nil {
		return
	}
	s.cancel = cancel
}

// IsTTY returns true if the output destination is a terminal.
func (s *Service) IsTTY() bool {
	if s == nil {
		return false
	}
	return s.isTerminal
}

// LogWriter returns an io.Writer that appends static lines to the TUI block
// log on TTY, or writes directly to the destination on non-TTY.
// Use this to route ui.step / ui.success / ui.divider output through the
// same program that owns the streaming area, preventing interleaved writes.
func (s *Service) LogWriter() io.Writer {
	if s == nil {
		return io.Discard
	}
	if !s.tuiActive() {
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

// Flush stops the BubbleTea program (if running), waits for it to exit, then
// copies the output buffer to the destination.
func (s *Service) Flush() error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}
	if s.prog != nil {
		s.prog.Stop()
		s.prog = nil
	}
	if s.outputBuf.Len() > 0 {
		_, err := io.Copy(s.destination, &s.outputBuf)
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// Step ID counter
// ---------------------------------------------------------------------------

var stepIDCounter int64

func nextStepID() int64 {
	stepIDCounter++
	return stepIDCounter
}
