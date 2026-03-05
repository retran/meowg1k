// Copyright © 2025 The meowg1k Authors
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
	mu       sync.Mutex
	spinner  *spinnerState
	progress *progressState
	buffer   strings.Builder
}

type spinnerState struct {
	active  bool
	message string
	frame   int
	ticker  *time.Ticker
	done    chan bool
}

type progressState struct {
	active  bool
	label   string
	total   int
	current int
	detail  string
	frame   int
	ticker  *time.Ticker
	done    chan bool
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

func (l *TTYLogger) Thought(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLine(fmt.Sprintf("Thought: %s", message))
}

func (l *TTYLogger) Action(tool string, args string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLine(fmt.Sprintf("Action: %s(%s)", tool, args))
	l.startSpinnerUnsafe("  ...")
}

func (l *TTYLogger) ActionResult(success bool, message string, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	symbol := "✓"
	if !success {
		symbol = "✗"
	}
	l.writeLine(fmt.Sprintf("  %s %s (%.2fs)", symbol, message, duration.Seconds()))
}

func (l *TTYLogger) StartOperation(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLine(message)
	l.startSpinnerUnsafe("  ...")
}

func (l *TTYLogger) CompleteOperation(message string, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLine(fmt.Sprintf("  ✓ %s (%.2fs)", message, duration.Seconds()))
}

func (l *TTYLogger) Info(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLine(message)
}

func (l *TTYLogger) Success(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLine(fmt.Sprintf("✓ %s", message))
}

func (l *TTYLogger) Warning(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLine(fmt.Sprintf("Warning: %s", message))
}

func (l *TTYLogger) Error(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLine(fmt.Sprintf("✗ Error: %s", err.Error()))
}

func (l *TTYLogger) StartProgress(label string, total int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.stopProgressUnsafe()
	l.writeLine(label)
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

func (l *TTYLogger) FinishProgress(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopProgressUnsafe()
	l.flushUnsafe()
	l.writeLine(fmt.Sprintf("  ✓ %s", message))
}

func (l *TTYLogger) StartSpinner(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.writeLine(message)
	l.startSpinnerUnsafe("  ...")
}

func (l *TTYLogger) StopSpinner(success bool, finalMessage string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	symbol := "✓"
	if !success {
		symbol = "✗"
	}
	l.writeLine(fmt.Sprintf("  %s %s", symbol, finalMessage))
}

func (l *TTYLogger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.flushUnsafe()
}

func (l *TTYLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerUnsafe()
	l.stopProgressUnsafe()
	if err := l.flushUnsafe(); err != nil {
		return err
	}
	l.writer.Stop()
	return nil
}

// Internal methods (must be called with lock held)

func (l *TTYLogger) writeLine(message string) {
	fmt.Fprintln(&l.buffer, message)
	l.flushUnsafe()
}

func (l *TTYLogger) flushUnsafe() error {
	if l.buffer.Len() > 0 {
		if _, err := l.writer.Write([]byte(l.buffer.String())); err != nil {
			return err
		}
		l.buffer.Reset()
	}
	return l.writer.Flush()
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
				l.mu.Lock()
				if l.spinner != nil && l.spinner.active {
					l.spinner.frame = (l.spinner.frame + 1) % len(spinnerFrames)
					frame := spinnerFrames[l.spinner.frame]
					fmt.Fprintf(l.writer, "%s %s\n", frame, l.spinner.message)
					l.writer.Flush()
				}
				l.mu.Unlock()
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
	l.writer.Flush()
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
	l.writer.Flush()
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

	fmt.Fprintf(l.writer, "%s\n", display)
	l.writer.Flush()
}
