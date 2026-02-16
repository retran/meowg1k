// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Activity represents a long-running operation with a simple spinner.
type Activity struct {
	message   string
	done      bool
	paused    bool
	mu        sync.Mutex
	theme     Theme
	opts      RenderOptions
	writer    io.Writer
	startTime time.Time
	stopChan  chan struct{}
	pauseChan chan bool
}

// Braille spinner frames (smooth, minimal width, widely supported)
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ASCII fallback spinner frames
var spinnerFramesASCII = []string{"-", "\\", "|", "/"}

// NewActivity creates a new activity indicator.
func NewActivity(message string, theme Theme, opts RenderOptions, writer io.Writer) *Activity {
	if writer == nil {
		writer = os.Stderr
	}

	a := &Activity{
		message:   message,
		done:      false,
		paused:    false,
		theme:     theme,
		opts:      opts,
		writer:    writer,
		startTime: time.Now(),
		stopChan:  make(chan struct{}),
		pauseChan: make(chan bool),
	}

	// Plain mode: static ellipsis (CI/log friendly)
	if opts.Plain || !opts.Terminal {
		fmt.Fprintf(writer, "> %s…\n", message)
		return a
	}

	// Terminal mode: start spinner in goroutine
	go a.spin()

	return a
}

func (a *Activity) spin() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	frame := 0
	frames := spinnerFrames
	if !a.opts.SupportsUnicode {
		frames = spinnerFramesASCII
	}

	spinnerStyle := lipgloss.NewStyle().Foreground(a.theme.Spinner)
	isPaused := false

	for {
		select {
		case <-a.stopChan:
			// Clear the line
			fmt.Fprintf(a.writer, "\r\033[K")
			return
		case pause := <-a.pauseChan:
			isPaused = pause
			if pause {
				// Clear the spinner line when pausing
				fmt.Fprintf(a.writer, "\r\033[K")
			}
		case <-ticker.C:
			if !isPaused {
				a.mu.Lock()
				msg := a.message
				a.mu.Unlock()

				// Format: ⠋ message (spinner Teal, text normal)
				fmt.Fprintf(a.writer, "\r%s %s",
					spinnerStyle.Render(frames[frame]),
					msg)

				frame = (frame + 1) % len(frames)
			}
		}
	}
}

// Pause temporarily hides the spinner to allow other output.
func (a *Activity) Pause() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.done || a.paused || a.opts.Plain || !a.opts.Terminal {
		return
	}

	a.paused = true
	a.pauseChan <- true
	time.Sleep(20 * time.Millisecond) // Give spinner time to clear
}

// Resume restarts the spinner after pausing.
func (a *Activity) Resume() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.done || !a.paused || a.opts.Plain || !a.opts.Terminal {
		return
	}

	a.paused = false
	a.pauseChan <- false
}

// Update changes the activity message.
func (a *Activity) Update(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.done {
		return
	}

	a.message = message

	if a.opts.Plain || !a.opts.Terminal {
		fmt.Fprintf(a.writer, "> %s…\n", message)
	}
}

// Success completes the activity successfully.
func (a *Activity) Success(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.done {
		return
	}

	a.done = true
	duration := time.Since(a.startTime)

	// Stop spinner
	if !(a.opts.Plain || !a.opts.Terminal) {
		close(a.stopChan)
		time.Sleep(100 * time.Millisecond) // Wait for spinner to clear
	}

	// Print success message
	if a.opts.Plain || !a.opts.Terminal {
		fmt.Fprintf(a.writer, "+ %s · %s\n", message, duration.Round(time.Millisecond))
	} else {
		style := a.theme.StatusSuccess
		fmt.Fprintf(a.writer, "%s\n",
			style.Render(fmt.Sprintf("✓ %s · %s", message, duration.Round(time.Millisecond))))
	}
}

// Fail completes the activity with an error.
func (a *Activity) Fail(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.done {
		return
	}

	a.done = true
	duration := time.Since(a.startTime)

	// Stop spinner
	if !(a.opts.Plain || !a.opts.Terminal) {
		close(a.stopChan)
		time.Sleep(100 * time.Millisecond)
	}

	// Print error message
	if a.opts.Plain || !a.opts.Terminal {
		fmt.Fprintf(a.writer, "- %s · %s\n", message, duration.Round(time.Millisecond))
	} else {
		style := a.theme.StatusError
		fmt.Fprintf(a.writer, "%s\n",
			style.Render(fmt.Sprintf("✗ %s · %s", message, duration.Round(time.Millisecond))))
	}
}

// Done silently completes the activity without printing a message.
func (a *Activity) Done() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.done {
		return
	}

	a.done = true

	// Stop spinner and clear the line
	if !(a.opts.Plain || !a.opts.Terminal) {
		close(a.stopChan)
		time.Sleep(100 * time.Millisecond)
		// Clear the spinner line
		fmt.Fprint(a.writer, "\r\033[K")
	}
}
