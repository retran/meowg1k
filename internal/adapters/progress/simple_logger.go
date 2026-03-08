// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const (
	symbolSuccess = "✓"
	symbolFailure = "✗"
)

// SimpleLogger provides plain text logging for non-TTY environments.
type SimpleLogger struct {
	err    error
	writer io.Writer
	mu     sync.Mutex
}

// NewSimpleLogger creates a new simple logger.
func NewSimpleLogger(writer io.Writer) *SimpleLogger {
	if writer == nil {
		writer = os.Stderr
	}
	return &SimpleLogger{
		writer: writer,
	}
}

func (l *SimpleLogger) write(format string, args ...interface{}) {
	if l.err != nil {
		return
	}
	_, l.err = fmt.Fprintf(l.writer, format, args...)
}

// Thought logs an agent thought message.
func (l *SimpleLogger) Thought(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write("Thought: %s\n", message)
}

// Action logs an agent tool invocation.
func (l *SimpleLogger) Action(tool string, args string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write("Action: %s(%s)\n", tool, args)
}

// ActionResult logs the result of an agent tool invocation.
func (l *SimpleLogger) ActionResult(success bool, message string, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	symbol := symbolSuccess
	if !success {
		symbol = symbolFailure
	}
	l.write("  %s %s (%.2fs)\n", symbol, message, duration.Seconds())
}

// StartOperation logs the beginning of a named operation.
func (l *SimpleLogger) StartOperation(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write("%s...\n", message)
}

// CompleteOperation logs the completion of a named operation with its duration.
func (l *SimpleLogger) CompleteOperation(message string, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write("  %s %s (%.2fs)\n", symbolSuccess, message, duration.Seconds())
}

// Info logs an informational message.
func (l *SimpleLogger) Info(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write("%s\n", message)
}

// Success logs a success message.
func (l *SimpleLogger) Success(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write("%s %s\n", symbolSuccess, message)
}

// Warning logs a warning message.
func (l *SimpleLogger) Warning(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write("Warning: %s\n", message)
}

// Error logs an error message.
func (l *SimpleLogger) Error(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write("%s Error: %s\n", symbolFailure, err.Error())
}

// StartProgress logs the start of a progress operation with a label and total count.
func (l *SimpleLogger) StartProgress(label string, total int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write("%s: 0/%d\n", label, total)
}

// UpdateProgress logs an item detail for the current progress step; the current count is unused in plain mode.
func (l *SimpleLogger) UpdateProgress(_ int, itemDetail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if itemDetail != "" {
		l.write("  • %s\n", itemDetail)
	}
}

// FinishProgress logs the completion of a progress operation.
func (l *SimpleLogger) FinishProgress(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write("  %s %s\n", symbolSuccess, message)
}

// StartSpinner logs the start of a spinner operation.
func (l *SimpleLogger) StartSpinner(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.write("%s...\n", message)
}

// StopSpinner logs the final result of a spinner operation.
func (l *SimpleLogger) StopSpinner(success bool, finalMessage string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	symbol := symbolSuccess
	if !success {
		symbol = symbolFailure
	}
	l.write("  %s %s\n", symbol, finalMessage)
}

// Flush returns any accumulated write error and resets the error state.
func (l *SimpleLogger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.err
	l.err = nil
	return err
}

// Close is a no-op for SimpleLogger as the underlying writer is managed externally.
func (l *SimpleLogger) Close() error {
	return nil
}
