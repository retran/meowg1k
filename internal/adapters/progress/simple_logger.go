// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// SimpleLogger provides plain text logging for non-TTY environments.
type SimpleLogger struct {
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

func (l *SimpleLogger) Thought(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "Thought: %s\n", message)
}

func (l *SimpleLogger) Action(tool string, args string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "Action: %s(%s)\n", tool, args)
}

func (l *SimpleLogger) ActionResult(success bool, message string, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	symbol := "✓"
	if !success {
		symbol = "✗"
	}
	fmt.Fprintf(l.writer, "  %s %s (%.2fs)\n", symbol, message, duration.Seconds())
}

func (l *SimpleLogger) StartOperation(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "%s...\n", message)
}

func (l *SimpleLogger) CompleteOperation(message string, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "  ✓ %s (%.2fs)\n", message, duration.Seconds())
}

func (l *SimpleLogger) Info(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "%s\n", message)
}

func (l *SimpleLogger) Success(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "✓ %s\n", message)
}

func (l *SimpleLogger) Warning(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "Warning: %s\n", message)
}

func (l *SimpleLogger) Error(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "✗ Error: %s\n", err.Error())
}

func (l *SimpleLogger) StartProgress(label string, total int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "%s: 0/%d\n", label, total)
}

func (l *SimpleLogger) UpdateProgress(current int, itemDetail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if itemDetail != "" {
		fmt.Fprintf(l.writer, "  • %s\n", itemDetail)
	}
}

func (l *SimpleLogger) FinishProgress(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "  ✓ %s\n", message)
}

func (l *SimpleLogger) StartSpinner(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "%s...\n", message)
}

func (l *SimpleLogger) StopSpinner(success bool, finalMessage string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	symbol := "✓"
	if !success {
		symbol = "✗"
	}
	fmt.Fprintf(l.writer, "  %s %s\n", symbol, finalMessage)
}

func (l *SimpleLogger) Flush() error {
	return nil
}

func (l *SimpleLogger) Close() error {
	return nil
}
