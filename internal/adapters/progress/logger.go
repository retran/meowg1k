// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package progress provides progress logging for CLI operations.
package progress

import (
	"io"
	"os"
	"time"

	"github.com/charmbracelet/x/term"
)

// Logger provides progress and status logging to stderr.
type Logger interface {
	// Agent-style logging (for `do` command)
	Thought(message string)
	Action(tool string, args string)
	ActionResult(success bool, message string, duration time.Duration)

	// General status updates
	StartOperation(message string)
	CompleteOperation(message string, duration time.Duration)
	Info(message string)
	Success(message string)
	Warning(message string)
	Error(err error)

	// Progress bars (index, draft commands)
	StartProgress(label string, total int)
	UpdateProgress(current int, itemDetail string)
	FinishProgress(message string)

	// Spinners (quick operations with unknown duration)
	StartSpinner(message string)
	StopSpinner(success bool, finalMessage string)

	// Control
	Flush() error
	Close() error
}

// New creates a progress logger based on environment.
func New(silent bool, writer io.Writer) Logger {
	if silent {
		return &noopLogger{}
	}

	if writer == nil {
		writer = os.Stderr
	}

	if f, ok := writer.(*os.File); ok && term.IsTerminal(f.Fd()) {
		return NewTTYLogger(writer)
	}

	return NewSimpleLogger(writer)
}

// noopLogger is a no-op logger for silent mode.
type noopLogger struct{}

func (l *noopLogger) Thought(_ string)                               {}
func (l *noopLogger) Action(_ string, _ string)                      {}
func (l *noopLogger) ActionResult(_ bool, _ string, _ time.Duration) {}
func (l *noopLogger) StartOperation(_ string)                        {}
func (l *noopLogger) CompleteOperation(_ string, _ time.Duration)    {}
func (l *noopLogger) Info(_ string)                                  {}
func (l *noopLogger) Success(_ string)                               {}
func (l *noopLogger) Warning(_ string)                               {}
func (l *noopLogger) Error(_ error)                                  {}
func (l *noopLogger) StartProgress(_ string, _ int)                  {}
func (l *noopLogger) UpdateProgress(_ int, _ string)                 {}
func (l *noopLogger) FinishProgress(_ string)                        {}
func (l *noopLogger) StartSpinner(_ string)                          {}
func (l *noopLogger) StopSpinner(_ bool, _ string)                   {}
func (l *noopLogger) Flush() error                                   { return nil }
func (l *noopLogger) Close() error                                   { return nil }
