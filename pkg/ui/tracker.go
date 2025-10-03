/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package ui provides terminal-based user interface components.
package ui

import (
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/retran/meowg1k/pkg/executor"
)

const (
	// ANSI colors
	colorReset = "\033[0m"
	colorCyan  = "\033[36m"
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
	colorGray  = "\033[90m"

	// Status icons
	iconRunning   = "→"
	iconCompleted = "✓"
	iconFailed    = "✗"
	iconPending   = "…"

	// Layout widths
	flowNameWidth = 42
	stepNameWidth = 40

	// Configuration
	feedbackChanSize = 128
	tickerInterval   = 100 * time.Millisecond
	maxMessageLength = 100
)

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ExecutionTracker tracks and displays the progress of executions in the terminal.
type ExecutionTracker struct {
	silent       bool
	wg           sync.WaitGroup
	mu           sync.RWMutex // Protects executions and order
	executions   map[string]*ExecutionProgress
	order        []string // Preserves insertion order for stable output
	feedbackChan chan *executor.Feedback

	spinnerIndex   int
	displayedLines int // Number of lines rendered in the last tick
}

// ExecutionProgress represents the progress state of a single execution.
type ExecutionProgress struct {
	Name       string
	Status     executor.Status
	Message    string
	Result     string
	StartTime  time.Time
	EndTime    *time.Time
	Error      error
	Metadata   map[string]any
	ParentName string
	Children   []string
	Level      int
}

// NewExecutionTracker creates a new progress tracker.
func NewExecutionTracker(silent bool) *ExecutionTracker {
	return &ExecutionTracker{
		silent:       silent,
		executions:   make(map[string]*ExecutionProgress),
		order:        make([]string, 0),
		feedbackChan: make(chan *executor.Feedback, feedbackChanSize),
	}
}

// Start launches the goroutine for processing events and rendering the UI.
func (t *ExecutionTracker) Start() {
	if t.silent {
		return
	}
	t.wg.Add(1)
	go t.run()
}

// Stop signals the tracker to stop and waits for it to finish.
func (t *ExecutionTracker) Stop() {
	if t.silent {
		return
	}
	close(t.feedbackChan)
	t.wg.Wait()
}

// FeedbackHandler returns a handler function to receive progress feedback.
func (t *ExecutionTracker) FeedbackHandler() executor.FeedbackHandler {
	return func(feedback *executor.Feedback) {
		t.feedbackChan <- feedback
	}
}

// run is the main event processing and rendering loop.
func (t *ExecutionTracker) run() {
	defer t.wg.Done()
	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	isDirty := true // A dirty flag schedules a full redraw.

	for {
		select {
		case feedback, ok := <-t.feedbackChan:
			if !ok {
				t.redraw()
				fmt.Fprint(os.Stderr, "\r\033[K") // Clear the spinner line
				return
			}
			t.updateExecution(feedback)
			isDirty = true

		case <-ticker.C:
			t.spinnerIndex++
			if isDirty {
				t.redraw()
				isDirty = false
			}
			t.drawSpinner()
		}
	}
}

// redraw clears the previous output and redraws the entire progress block.
func (t *ExecutionTracker) redraw() {
	if t.displayedLines > 0 {
		fmt.Fprintf(os.Stderr, "\033[%dA\r", t.displayedLines)
	}

	var output strings.Builder
	var newLinesCount int

	t.mu.RLock()
	for _, name := range t.order {
		exec, exists := t.executions[name]
		if !exists || exec.Level > 1 { // We only display level 0 and 1
			continue
		}

		style := lineStyle{
			nameWidth: flowNameWidth,
		}
		if exec.Level == 1 {
			style.indent = "  "
			style.nameWidth = stepNameWidth
		}

		output.WriteString(t.formatLine(exec, style))
		newLinesCount++
	}
	t.mu.RUnlock()

	output.WriteString("\033[J") // Clear screen from cursor down
	fmt.Fprint(os.Stderr, output.String())

	t.displayedLines = newLinesCount
}

// drawSpinner renders only the spinner line.
func (t *ExecutionTracker) drawSpinner() {
	spinner := spinnerChars[t.spinnerIndex%len(spinnerChars)]
	fmt.Fprintf(os.Stderr, "\r%s Working...\033[K", spinner)
}

// updateExecution updates the state of an execution based on feedback.
func (t *ExecutionTracker) updateExecution(feedback *executor.Feedback) {
	if feedback.ActivityName == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	exec, exists := t.executions[feedback.ActivityName]
	if !exists {
		parentName, level := parseActivityHierarchy(feedback.ActivityName)
		exec = &ExecutionProgress{
			Name:       feedback.ActivityName,
			StartTime:  feedback.Timestamp,
			Metadata:   make(map[string]any),
			ParentName: parentName,
			Children:   make([]string, 0),
			Level:      level,
		}
		t.executions[feedback.ActivityName] = exec
		t.order = append(t.order, feedback.ActivityName)

		if parent, ok := t.executions[parentName]; ok {
			parent.Children = append(parent.Children, feedback.ActivityName)
		}
	}

	exec.Status = feedback.Status
	exec.Error = feedback.Error
	if feedback.Metadata != nil {
		maps.Copy(exec.Metadata, feedback.Metadata)
	}

	switch feedback.Status {
	case executor.StatusRunning:
		exec.Message = sanitizeDescription(feedback.Message)
	case executor.StatusCompleted:
		exec.Result = sanitizeDescription(feedback.Message)
		fallthrough
	case executor.StatusFailed:
		endTime := feedback.Timestamp
		exec.EndTime = &endTime
	}
}

// lineStyle defines the formatting for a single line.
type lineStyle struct {
	indent    string
	nameWidth int
}

// formatLine formats a single execution's status into a string.
func (t *ExecutionTracker) formatLine(exec *ExecutionProgress, style lineStyle) string {
	var icon, color string
	switch exec.Status {
	case executor.StatusRunning:
		icon, color = iconRunning, colorCyan
	case executor.StatusCompleted:
		icon, color = iconCompleted, colorGreen
	case executor.StatusFailed:
		icon, color = iconFailed, colorRed
	default: // executor.StatusPending
		icon, color = iconPending, colorGray
	}

	displayName := exec.Message
	if displayName == "" {
		parts := strings.Split(exec.Name, "::")
		displayName = convertCamelToReadable(parts[len(parts)-1])
	}
	truncatedName := truncateString(displayName, style.nameWidth)
	paddedName := fmt.Sprintf("%-*s", style.nameWidth, truncatedName)

	duration := getDurationString(exec)
	progressInfo := getChildProgressInfo(t, exec)

	var errorMsg string
	if exec.Status == executor.StatusFailed && exec.Error != nil {
		errorMsg = fmt.Sprintf(" (%s)", exec.Error.Error())
	}

	return fmt.Sprintf("%s%s%s%s %s %6s  %s%s\033[K\n",
		style.indent, color, icon, colorReset,
		paddedName, duration, progressInfo, errorMsg)
}

func sanitizeDescription(description string) string {
	if description == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(description))
	for _, r := range description {
		if unicode.IsPrint(r) && r != '\x1b' {
			b.WriteRune(r)
		}
	}
	return truncateString(b.String(), maxMessageLength)
}

func truncateString(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func convertCamelToReadable(s string) string {
	if s == "" {
		return ""
	}
	var result strings.Builder
	result.Grow(len(s) + 5)
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune(' ')
		}
		if i == 0 {
			result.WriteRune(unicode.ToUpper(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func parseActivityHierarchy(activityName string) (parentName string, level int) {
	parts := strings.Split(activityName, "::")
	level = len(parts) - 1
	if level > 0 {
		parentName = strings.Join(parts[:level], "::")
	}
	return parentName, level
}

func getChildProgressInfo(t *ExecutionTracker, exec *ExecutionProgress) string {
	total := len(exec.Children)
	if total == 0 {
		if exec.Status == executor.StatusCompleted {
			return exec.Result
		}
		return ""
	}

	var completed, failed, running int
	for _, childName := range exec.Children {
		if child, exists := t.executions[childName]; exists {
			switch child.Status {
			case executor.StatusCompleted:
				completed++
			case executor.StatusFailed:
				failed++
			case executor.StatusRunning:
				running++
			}
		}
	}

	done := completed + failed
	if done == total {
		if failed > 0 {
			return fmt.Sprintf("[%d/%d, %d failed]", completed, total, failed)
		}
		return fmt.Sprintf("[%d/%d]", completed, total)
	}

	var parts []string
	if running > 0 {
		parts = append(parts, fmt.Sprintf("%d running", running))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}

	if len(parts) > 0 {
		return fmt.Sprintf("[%d/%d, %s]", done, total, strings.Join(parts, ", "))
	}
	return fmt.Sprintf("[%d/%d]", done, total)
}

func getDurationString(exec *ExecutionProgress) string {
	if exec.EndTime == nil {
		return ""
	}
	duration := exec.EndTime.Sub(exec.StartTime)
	switch {
	case duration < time.Second:
		return fmt.Sprintf("%dms", duration.Milliseconds())
	case duration < time.Minute:
		return fmt.Sprintf("%.1fs", duration.Seconds())
	default:
		return fmt.Sprintf("%.1fm", duration.Minutes())
	}
}

// GetExecution returns a copy of the execution progress for the given name.
// Returns nil if the execution doesn't exist.
// This method is thread-safe and primarily intended for testing.
func (t *ExecutionTracker) GetExecution(name string) *ExecutionProgress {
	t.mu.RLock()
	defer t.mu.RUnlock()

	exec, exists := t.executions[name]
	if !exists {
		return nil
	}

	// Return a shallow copy to avoid race conditions
	copyExec := *exec
	return &copyExec
}

// GetExecutionCount returns the number of tracked executions.
// This method is thread-safe and primarily intended for testing.
func (t *ExecutionTracker) GetExecutionCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.executions)
}
