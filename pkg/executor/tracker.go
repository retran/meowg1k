// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	// ANSI colors.
	colorReset = "\033[0m"
	colorCyan  = "\033[36m"
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
	colorGray  = "\033[90m"

	// Status icons.
	iconRunning   = "→"
	iconCompleted = "✓"
	iconFailed    = "✗"
	iconPending   = "…"

	// Layout widths.
	flowNameWidth = 42
	stepNameWidth = 40

	// Configuration.
	feedbackChanSize = 128
	tickerInterval   = 100 * time.Millisecond
	maxMessageLength = 100
)

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Tracker tracks and displays the progress of executions in the terminal.
type Tracker struct {
	executions     map[string]*Execution
	feedbackChan   chan *Feedback
	order          []string
	wg             sync.WaitGroup
	spinnerIndex   int
	displayedLines int
	mu             sync.RWMutex
	silent         bool
}

// NewTracker creates a new progress tracker.
func NewTracker(silent bool) *Tracker {
	return &Tracker{
		silent:       silent,
		executions:   make(map[string]*Execution),
		order:        make([]string, 0),
		feedbackChan: make(chan *Feedback, feedbackChanSize),
	}
}

// Start launches the goroutine for processing events and rendering the UI.
func (t *Tracker) Start() {
	if t == nil || t.silent {
		return
	}

	t.wg.Add(1)

	go t.run()
}

// Stop signals the tracker to stop and waits for it to finish.
func (t *Tracker) Stop() {
	if t == nil || t.silent {
		return
	}

	if t.feedbackChan != nil {
		close(t.feedbackChan)
	}

	t.wg.Wait()
}

// FeedbackHandler returns a handler function to receive progress feedback.
func (t *Tracker) FeedbackHandler() FeedbackHandler {
	return func(feedback *Feedback) {
		if t == nil || t.feedbackChan == nil || feedback == nil {
			return
		}
		t.feedbackChan <- feedback
	}
}

// GetExecution returns a copy of the execution progress for the given name.
// Returns nil if the execution doesn't exist.
// This method is thread-safe and primarily intended for testing.
func (t *Tracker) GetExecution(name string) *Execution {
	if t == nil {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	exec, exists := t.executions[name]
	if !exists || exec == nil {
		return nil
	}

	// Return a shallow copy to avoid race conditions
	copyExec := *exec
	return &copyExec
}

// GetExecutionCount returns the number of tracked executions.
// This method is thread-safe and primarily intended for testing.
func (t *Tracker) GetExecutionCount() int {
	if t == nil {
		return 0
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.executions)
}

// run is the main event processing and rendering loop.
func (t *Tracker) run() {
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
func (t *Tracker) redraw() {
	if t == nil {
		return
	}

	if t.displayedLines > 0 {
		fmt.Fprintf(os.Stderr, "\033[%dA\r", t.displayedLines)
	}

	var output strings.Builder
	var newLinesCount int

	t.mu.RLock()
	for _, name := range t.order {
		exec, exists := t.executions[name]
		if !exists || exec == nil || exec.Level > 1 { // We only display level 0 and 1
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
func (t *Tracker) drawSpinner() {
	spinner := spinnerChars[t.spinnerIndex%len(spinnerChars)]
	fmt.Fprintf(os.Stderr, "\r%s Working...\033[K", spinner)
}

// updateExecution updates the state of an execution based on feedback.
func (t *Tracker) updateExecution(feedback *Feedback) {
	if t == nil || feedback == nil || feedback.ActivityName == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	exec, exists := t.executions[feedback.ActivityName]
	if !exists {
		parentName, level := parseActivityHierarchy(feedback.ActivityName)
		exec = &Execution{
			Name:       feedback.ActivityName,
			StartTime:  feedback.Timestamp,
			Metadata:   make(map[string]any),
			ParentName: parentName,
			Children:   make([]string, 0),
			Level:      level,
		}
		t.executions[feedback.ActivityName] = exec
		t.order = append(t.order, feedback.ActivityName)

		if parent, ok := t.executions[parentName]; ok && parent != nil {
			parent.Children = append(parent.Children, feedback.ActivityName)
		}
	}

	exec.Status = feedback.Status
	exec.Error = feedback.Error
	if feedback.Metadata != nil {
		maps.Copy(exec.Metadata, feedback.Metadata)
	}

	switch feedback.Status {
	case StatusPending:
		exec.Message = sanitizeDescription(feedback.Message)
	case StatusRunning:
		exec.Message = sanitizeDescription(feedback.Message)
	case StatusCompleted:
		exec.Result = sanitizeDescription(feedback.Message)
		fallthrough
	case StatusFailed:
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
func (t *Tracker) formatLine(exec *Execution, style lineStyle) string {
	if t == nil || exec == nil {
		return ""
	}

	var icon, color string
	switch exec.Status {
	case StatusPending:
		icon, color = iconPending, colorGray
	case StatusRunning:
		icon, color = iconRunning, colorCyan
	case StatusCompleted:
		icon, color = iconCompleted, colorGreen
	case StatusFailed:
		icon, color = iconFailed, colorRed
	}

	displayName := exec.Message
	if displayName == "" {
		parts := strings.Split(exec.Name, "::")
		displayName = convertCamelToReadable(parts[len(parts)-1])
	}
	truncatedName := truncateString(displayName, style.nameWidth)
	paddedName := fmt.Sprintf("%-*s", style.nameWidth, truncatedName)

	duration := exec.getDurationString()
	progressInfo := t.getChildProgressInfo(exec)

	var errorMsg string
	if exec.Status == StatusFailed && exec.Error != nil {
		errorMsg = fmt.Sprintf(" (%s)", exec.Error.Error())
	}

	return fmt.Sprintf("%s%s%s%s %s %6s  %s%s\033[K\n",
		style.indent, color, icon, colorReset,
		paddedName, duration, progressInfo, errorMsg)
}

func (t *Tracker) getChildProgressInfo(exec *Execution) string {
	total := len(exec.Children)
	if total == 0 {
		if exec.Status == StatusCompleted {
			return exec.Result
		}
		return ""
	}

	var completed, failed, running int
	for _, childName := range exec.Children {
		if child, exists := t.executions[childName]; exists {
			switch child.Status {
			case StatusPending:
				// Pending children are not included in progress counts.
			case StatusCompleted:
				completed++
			case StatusFailed:
				failed++
			case StatusRunning:
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

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
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
