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
	// Status icons.
	iconRunning   = ""
	iconCompleted = ""
	iconFailed    = "FAILED"
	iconPending   = "..."

	// Configuration.
	feedbackChanSize = 128
	tickerInterval   = 100 * time.Millisecond
	maxMessageLength = 100
)

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Tracker tracks and displays the progress of executions in the terminal.
type Tracker struct {
	executions          map[string]*Execution
	feedbackChan        chan *Feedback
	batchProgress       map[string]*batchProgressTracker // Track progress for batch operations
	order               []string
	wg                  sync.WaitGroup
	mu                  sync.RWMutex
	spinnerIndex        int
	currentRunningIndex int
	silent              bool
	spinnerVisible      bool
}

// batchProgressTracker tracks progress for batch operations like "Fetching 36 files".
type batchProgressTracker struct {
	activity  string // e.g., "Fetching staged diffs", "Summarizing files"
	total     int
	completed int
}

// NewTracker creates a new progress tracker.
func NewTracker(silent bool) *Tracker {
	return &Tracker{
		silent:              silent,
		executions:          make(map[string]*Execution),
		order:               make([]string, 0),
		feedbackChan:        make(chan *Feedback, feedbackChanSize),
		currentRunningIndex: -1,
		batchProgress:       make(map[string]*batchProgressTracker),
	}
}

// Start launches background processing for events and rendering the UI.
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

	for {
		select {
		case feedback, ok := <-t.feedbackChan:
			if !ok {
				t.clearSpinnerLine()
				return
			}
			exec, shouldLog := t.updateExecution(feedback)
			if shouldLog && exec != nil {
				t.clearSpinnerLine()
				fmt.Fprintln(os.Stderr, t.formatLogLine(exec))
			}

		case <-ticker.C:
			t.spinnerIndex++
			t.drawSpinner()
		}
	}
}

// drawSpinner renders only the spinner line.
func (t *Tracker) drawSpinner() {
	if t == nil {
		return
	}

	t.mu.RLock()
	runningIndex := t.currentRunningIndex
	var runningName string
	if runningIndex >= 0 && runningIndex < len(t.order) {
		runningName = t.order[runningIndex]
	}
	t.mu.RUnlock()

	if runningName == "" {
		t.clearSpinnerLine()
		return
	}

	exec := t.GetExecution(runningName)
	if exec == nil {
		t.clearSpinnerLine()
		return
	}

	displayName := buildDisplayName(exec)

	// Add batch progress if available
	if progress, exists := t.batchProgress[exec.Name]; exists {
		percentage := 0
		if progress.total > 0 {
			percentage = (progress.completed * 100) / progress.total
		}
		displayName = fmt.Sprintf("%s [%d/%d] %d%%", displayName, progress.completed, progress.total, percentage)
	}

	spinner := spinnerChars[t.spinnerIndex%len(spinnerChars)]
	fmt.Fprintf(os.Stderr, "\r%s %s\033[K", spinner, displayName)
	t.spinnerVisible = true
}

// updateExecution updates the state of an execution based on feedback.
func (t *Tracker) updateExecution(feedback *Feedback) (*Execution, bool) {
	if t == nil || feedback == nil || feedback.ActivityName == "" {
		return nil, false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	exec, execIndex := t.ensureExecution(feedback)
	exec.Status = feedback.Status
	exec.Error = feedback.Error
	if feedback.Metadata != nil {
		maps.Copy(exec.Metadata, feedback.Metadata)
	}

	shouldLog := t.applyFeedbackStatus(exec, feedback, execIndex)

	return execCopy(exec), shouldLog
}

func (t *Tracker) ensureExecution(feedback *Feedback) (exec *Execution, execIndex int) {
	exec, exists := t.executions[feedback.ActivityName]
	if exists {
		return exec, -1
	}

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
	execIndex = len(t.order)
	t.order = append(t.order, feedback.ActivityName)

	if parent, ok := t.executions[parentName]; ok && parent != nil {
		parent.Children = append(parent.Children, feedback.ActivityName)
	}

	return exec, execIndex
}

func (t *Tracker) applyFeedbackStatus(exec *Execution, feedback *Feedback, execIndex int) bool {
	switch feedback.Status {
	case StatusPending:
		exec.Message = sanitizeDescription(feedback.Message)
		return false
	case StatusRunning:
		exec.Message = sanitizeDescription(feedback.Message)
		t.setCurrentRunning(execIndex, feedback.ActivityName)
		// Check if this is a batch operation and init progress tracking
		t.initBatchProgress(exec)
		return false
	case StatusCompleted:
		exec.Result = sanitizeDescription(feedback.Message)
		t.finishExecution(exec, feedback)
		return true
	case StatusFailed:
		t.finishExecution(exec, feedback)
		return true
	default:
		return false
	}
}

func (t *Tracker) finishExecution(exec *Execution, feedback *Feedback) {
	exec.Message = sanitizeDescription(feedback.Message)
	endTime := feedback.Timestamp
	exec.EndTime = &endTime
	t.clearCurrentRunningIfMatch(feedback.ActivityName)
}

func (t *Tracker) setCurrentRunning(execIndex int, name string) {
	if execIndex == -1 {
		execIndex = findExecutionIndex(t.order, name)
	}
	if execIndex >= 0 {
		t.currentRunningIndex = execIndex
	}
}

func (t *Tracker) clearCurrentRunningIfMatch(name string) {
	if t.currentRunningIndex >= 0 && t.currentRunningIndex < len(t.order) && t.order[t.currentRunningIndex] == name {
		t.currentRunningIndex = -1
	}
}

func findExecutionIndex(order []string, name string) int {
	for i, entry := range order {
		if entry == name {
			return i
		}
	}
	return -1
}

func (t *Tracker) formatLogLine(exec *Execution) string {
	if t == nil || exec == nil {
		return ""
	}

	// Check if this is a child of a batch operation - if so, hide it
	if t.isChildOfBatchOperation(exec) {
		// Update batch progress instead
		t.updateBatchProgress(exec)
		return ""
	}

	icon := getIcon(exec.Status)

	displayName := buildDisplayName(exec)
	indent := strings.Repeat("  ", exec.Level)

	// Add batch progress if this is a batch operation
	if progress, exists := t.batchProgress[exec.Name]; exists && exec.Status == StatusRunning {
		displayName = fmt.Sprintf("%s [%d/%d]", displayName, progress.completed, progress.total)
	}

	duration := exec.getDurationString()
	if duration != "" {
		duration = fmt.Sprintf(" (%s)", duration)
	}

	var errorMsg string
	if exec.Status == StatusFailed && exec.Error != nil {
		errorMsg = fmt.Sprintf(" — %s", sanitizeDescription(exec.Error.Error()))
	}

	prefix := indent
	if icon != "" {
		prefix = fmt.Sprintf("%s%s ", indent, icon)
	}

	return fmt.Sprintf("%s%s%s%s", prefix, displayName, duration, errorMsg)
}

// isChildOfBatchOperation checks if an execution is a child of a batch operation.
func (t *Tracker) isChildOfBatchOperation(exec *Execution) bool {
	if exec == nil || exec.ParentName == "" {
		return false
	}

	// Check if parent has batch progress tracking
	_, exists := t.batchProgress[exec.ParentName]
	return exists
}

// updateBatchProgress updates the progress counter for batch operations.
func (t *Tracker) updateBatchProgress(exec *Execution) {
	if exec == nil || exec.ParentName == "" || exec.Status != StatusCompleted {
		return
	}

	if progress, exists := t.batchProgress[exec.ParentName]; exists {
		progress.completed++
	}
}

// initBatchProgress initializes batch progress tracking based on the activity message.
func (t *Tracker) initBatchProgress(exec *Execution) {
	if exec == nil || exec.Message == "" {
		return
	}

	// Look for patterns like "Fetching staged diffs for 36 files" or "Summarizing changes in 36 files"
	msg := exec.Message
	var total int
	var activity string

	// Pattern 1: "Fetching ... for N files"
	if n, err := fmt.Sscanf(msg, "Fetching staged diffs for %d files", &total); err == nil && n == 1 {
		activity = "Fetching staged diffs"
	} else if n, err := fmt.Sscanf(msg, "Summarizing changes in %d files", &total); err == nil && n == 1 {
		activity = "Summarizing files"
	} else if n, err := fmt.Sscanf(msg, "Fetching branch diffs for %d files", &total); err == nil && n == 1 {
		activity = "Fetching branch diffs"
	}

	if total > 0 && activity != "" {
		t.batchProgress[exec.Name] = &batchProgressTracker{
			total:     total,
			completed: 0,
			activity:  activity,
		}
	}
}

func (t *Tracker) clearSpinnerLine() {
	if t == nil || !t.spinnerVisible {
		return
	}
	fmt.Fprint(os.Stderr, "\r\033[K")
	t.spinnerVisible = false
}

func execCopy(exec *Execution) *Execution {
	if exec == nil {
		return nil
	}
	copyExec := *exec
	return &copyExec
}

func buildDisplayName(exec *Execution) string {
	if exec == nil {
		return ""
	}

	if exec.Result != "" {
		return truncateString(exec.Result, maxMessageLength)
	}
	if exec.Message != "" {
		return truncateString(exec.Message, maxMessageLength)
	}

	parts := strings.Split(exec.Name, "::")
	if len(parts) == 0 {
		return ""
	}
	return convertCamelToReadable(parts[len(parts)-1])
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

func getIcon(status Status) string {
	switch status {
	case StatusPending:
		return iconPending
	case StatusRunning:
		return iconRunning
	case StatusCompleted:
		return iconCompleted
	case StatusFailed:
		return iconFailed
	}
	return iconRunning
}
