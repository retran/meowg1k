// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gosuri/uilive"
)

// Spinner frames for animation - modern Unicode spinner.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// TTYLogger provides rich progress logging with spinners and progress bars.
type TTYLogger struct {
	writer   *uilive.Writer
	spinner  *spinnerState
	progress *progressState
	err      error
	buffer   strings.Builder
	mu       sync.Mutex
}

type spinnerState struct {
	ticker  *time.Ticker
	done    chan bool
	message string
	frame   int
	active  bool
}

type progressState struct {
	ticker  *time.Ticker
	done    chan bool
	label   string
	detail  string
	total   int
	current int
	frame   int
	active  bool
}

// NewTTYLogger creates a new TTY logger with uilive.
func NewTTYLogger(writer io.Writer) *TTYLogger {
	uiWriter := uilive.New()
	uiWriter.Out = writer
	uiWriter.Start()
	return &TTYLogger{
		writer: uiWriter,
	}
}

// Thought logs an agent thought message.
func (l *TTYLogger) Thought(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLineUnsafe(fmt.Sprintf("Thought: %s", message))
}

// Action logs an agent tool invocation and starts a spinner.
func (l *TTYLogger) Action(tool string, args string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLineUnsafe(fmt.Sprintf("Action: %s(%s)", tool, args))
	l.startSpinnerUnsafe("  ...")
}

// ActionResult logs the result of an agent tool invocation.
func (l *TTYLogger) ActionResult(success bool, message string, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	symbol := symbolSuccess
	if !success {
		symbol = symbolFailure
	}
	l.writeLineUnsafe(fmt.Sprintf("  %s %s (%.2fs)", symbol, message, duration.Seconds()))
}

// StartOperation logs the beginning of a named operation and starts a spinner.
func (l *TTYLogger) StartOperation(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLineUnsafe(message)
	l.startSpinnerUnsafe("  ...")
}

// CompleteOperation logs the completion of a named operation with its duration.
func (l *TTYLogger) CompleteOperation(message string, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLineUnsafe(fmt.Sprintf("  %s %s (%.2fs)", symbolSuccess, message, duration.Seconds()))
}

// Info logs an informational message.
func (l *TTYLogger) Info(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLineUnsafe(message)
}

// Success logs a success message.
func (l *TTYLogger) Success(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLineUnsafe(fmt.Sprintf("%s %s", symbolSuccess, message))
}

// Warning logs a warning message.
func (l *TTYLogger) Warning(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLineUnsafe(fmt.Sprintf("Warning: %s", message))
}

// Error logs an error message.
func (l *TTYLogger) Error(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLineUnsafe(fmt.Sprintf("%s Error: %s", symbolFailure, err.Error()))
}

// StartProgress starts an animated progress bar with a label and total count.
func (l *TTYLogger) StartProgress(label string, total int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.stopProgressUnsafe()
	l.writeLineUnsafe(label)
	l.progress = &progressState{
		active:  true,
		label:   label,
		total:   total,
		current: 0,
		frame:   0,
		ticker:  time.NewTicker(80 * time.Millisecond),
		done:    make(chan bool),
	}
	l.updateProgressDisplay()

	go func() {
		for {
			select {
			case <-l.progress.done:
				return
			case <-l.progress.ticker.C:
				l.mu.Lock()
				if l.progress != nil && l.progress.active {
					l.progress.frame = (l.progress.frame + 1) % len(spinnerFrames)
					l.updateProgressDisplay()
				}
				l.mu.Unlock()
			}
		}
	}()
}

// UpdateProgress updates the current progress count and item detail.
func (l *TTYLogger) UpdateProgress(current int, itemDetail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.progress == nil || !l.progress.active {
		return
	}
	l.progress.current = current
	l.progress.detail = itemDetail
	l.updateProgressDisplay()
}

// FinishProgress stops the progress bar and logs a completion message.
func (l *TTYLogger) FinishProgress(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopProgressUnsafe()
	if err := l.flushUnsafe(); err != nil {
		l.err = err
		return
	}
	l.writeLineUnsafe(fmt.Sprintf("  %s %s", symbolSuccess, message))
}

// StartSpinner starts an animated spinner with a message.
func (l *TTYLogger) StartSpinner(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLineUnsafe(message)
	l.startSpinnerUnsafe("  ...")
}

// StopSpinner stops the spinner and logs the final result.
func (l *TTYLogger) StopSpinner(success bool, finalMessage string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	symbol := symbolSuccess
	if !success {
		symbol = symbolFailure
	}
	l.writeLineUnsafe(fmt.Sprintf("  %s %s", symbol, finalMessage))
}

// Flush flushes any buffered output to the terminal and returns any accumulated error.
func (l *TTYLogger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		err := l.err
		l.err = nil
		return err
	}
	return l.flushUnsafe()
}

// Close stops all active spinners and progress bars, flushes output, and stops the writer.
func (l *TTYLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.stopProgressUnsafe()
	if l.err != nil {
		err := l.err
		l.err = nil
		return err
	}
	if err := l.flushUnsafe(); err != nil {
		return err
	}
	l.writer.Stop()
	return nil
}

// Internal methods (must be called with lock held).

func (l *TTYLogger) writeLineUnsafe(message string) {
	if l.err != nil {
		return
	}
	// strings.Builder.Write never returns an error, so Fprintln result is safe to ignore
	l.buffer.WriteString(message)
	l.buffer.WriteByte('\n')
	if err := l.flushUnsafe(); err != nil {
		l.err = err
	}
}

func (l *TTYLogger) flushUnsafe() error {
	if l.buffer.Len() > 0 {
		if _, err := l.writer.Write([]byte(l.buffer.String())); err != nil {
			return fmt.Errorf("failed to write: %w", err)
		}
		l.buffer.Reset()
	}
	if err := l.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush: %w", err)
	}
	return nil
}

func (l *TTYLogger) tickSpinner() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.spinner == nil || !l.spinner.active {
		return
	}
	l.spinner.frame = (l.spinner.frame + 1) % len(spinnerFrames)
	frame := spinnerFrames[l.spinner.frame]
	if _, err := fmt.Fprintf(l.writer, "%s %s\n", frame, l.spinner.message); err != nil {
		l.err = fmt.Errorf("failed to write spinner: %w", err)
	} else if err := l.writer.Flush(); err != nil {
		l.err = fmt.Errorf("failed to flush: %w", err)
	}
}

func (l *TTYLogger) startSpinnerUnsafe(message string) {
	if l.spinner != nil && l.spinner.active {
		return
	}

	l.spinner = &spinnerState{
		active:  true,
		message: message,
		frame:   0,
		ticker:  time.NewTicker(80 * time.Millisecond),
		done:    make(chan bool),
	}

	go func() {
		for {
			select {
			case <-l.spinner.done:
				return
			case <-l.spinner.ticker.C:
				l.tickSpinner()
			}
		}
	}()
}

func (l *TTYLogger) stopSpinnerUnsafe() {
	if l.spinner == nil || !l.spinner.active {
		return
	}

	l.spinner.active = false
	l.spinner.ticker.Stop()
	close(l.spinner.done)
	l.spinner = nil

	// Flush to clear the spinner
	if err := l.writer.Flush(); err != nil {
		l.err = err
	}
}

func (l *TTYLogger) stopProgressUnsafe() {
	if l.progress == nil || !l.progress.active {
		return
	}

	l.progress.active = false
	l.progress.ticker.Stop()
	close(l.progress.done)
	l.progress = nil

	// Flush to clear the progress bar
	if err := l.writer.Flush(); err != nil {
		l.err = err
	}
}

func (l *TTYLogger) updateProgressDisplay() {
	if l.progress == nil || !l.progress.active {
		return
	}

	percent := 0
	if l.progress.total > 0 {
		percent = (l.progress.current * 100) / l.progress.total
	}

	barWidth := 50
	filled := (percent * barWidth) / 100
	bar := strings.Repeat("█", filled) + strings.Repeat(" ", barWidth-filled)

	display := fmt.Sprintf("[%s] %d/%d (%d%%)",
		bar, l.progress.current, l.progress.total, percent)

	if l.progress.detail != "" {
		// Put spinner on the detail line
		frame := spinnerFrames[l.progress.frame]
		display += fmt.Sprintf("\n  %s %s", frame, l.progress.detail)
	}

	if _, err := fmt.Fprintf(l.writer, "%s\n", display); err != nil {
		l.err = fmt.Errorf("failed to write progress: %w", err)
		return
	}
	if err := l.writer.Flush(); err != nil {
		l.err = fmt.Errorf("failed to flush: %w", err)
	}
}
