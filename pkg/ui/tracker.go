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

// Package ui provides terminal-based user interface components
package ui

import (
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/retran/meowg1k/pkg/executor"
)

// ANSI color constants
const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGray   = "\033[90m"
)

// sanitizeDescription removes control characters and limits length for safe display
func sanitizeDescription(description string) string {
	if description == "" {
		return description
	}

	// Remove control characters and non-printable characters
	var cleaned strings.Builder

	cleaned.Grow(len(description))

	for _, r := range description {
		// Allow printable ASCII characters, spaces, and common unicode letters
		if unicode.IsPrint(r) && r != '\x1b' { // Exclude escape character
			cleaned.WriteRune(r)
		}
	}

	result := cleaned.String()

	// Limit length to prevent terminal overflow
	const maxLength = 100
	if len(result) > maxLength {
		result = result[:maxLength-3] + "..."
	}

	return result
}

// ExecutionTracker tracks and displays progress for executions
type ExecutionTracker struct {
	silent        bool
	mu            sync.RWMutex
	executions    map[string]*ExecutionProgress
	order         []string
	isRunning     bool
	maxExecutions int // Maximum number of executions to display
	minCompleted  int // Minimum number of completed executions to show

	// Display management
	ticker       *time.Ticker
	stopChan     chan struct{}
	spinnerChars []string
	spinnerIndex int64 // Use int64 for atomic operations
	lastLines    int
}

// ExecutionProgress represents the progress state of a single activity
type ExecutionProgress struct {
	Name       string
	Status     executor.Status
	Progress   float64
	Message    string
	StartTime  time.Time
	EndTime    *time.Time
	LastUpdate time.Time
	Error      error
	Metadata   map[string]any

	// Hierarchy support
	ParentName string   // Name of parent execution (empty for root execution)
	Children   []string // Names of child executions
	Level      int      // Nesting level (0 for root, 1 for first-level children, etc.)
}

// parseActivityHierarchy determines parent name and nesting level from activity name
func (t *ExecutionTracker) parseActivityHierarchy(activityName string) (parentName string, level int) {
	// Activity names are structured as "parent.child.grandchild"
	parts := strings.Split(activityName, ".")
	level = len(parts) - 1

	if level > 0 {
		// Parent is everything except the last part
		parentParts := parts[:len(parts)-1]
		parentName = strings.Join(parentParts, ".")
	}

	return parentName, level
}

// NewExecutionTracker creates a new activity progress tracker
func NewExecutionTracker(silent bool) *ExecutionTracker {
	tracker := &ExecutionTracker{
		silent:        silent,
		executions:    make(map[string]*ExecutionProgress),
		order:         make([]string, 0),
		maxExecutions: 6,
		minCompleted:  2,
		stopChan:      make(chan struct{}),
		spinnerChars:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}

	if !silent {
		tracker.ticker = time.NewTicker(80 * time.Millisecond)
		go tracker.displayLoop()
	}

	return tracker
}

// Start begins tracking activities
func (t *ExecutionTracker) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.isRunning = true
}

// Stop stops tracking and cleans up display
func (t *ExecutionTracker) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.isRunning = false

	if !t.silent && t.ticker != nil {
		t.ticker.Stop()
		close(t.stopChan)

		// Clear display
		if t.lastLines > 0 {
			for i := 0; i < t.lastLines; i++ {
				fmt.Fprintf(os.Stderr, "\033[1A\033[K")
			}
		}
	}
}

// FeedbackHandler returns a feedback handler compatible with executor.FeedbackHandler
func (t *ExecutionTracker) FeedbackHandler() executor.FeedbackHandler {
	return func(feedback *executor.Feedback) {
		t.UpdateActivity(feedback)
	}
}

// UpdateActivity updates the progress of an activity
func (t *ExecutionTracker) UpdateActivity(feedback *executor.Feedback) {
	if t.silent {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	activityName := feedback.ActivityName
	if activityName == "" {
		return
	}

	// Get or create activity progress
	activityProgress, exists := t.executions[activityName]
	if !exists {
		// Determine parent and level from activity name
		parentName, level := t.parseActivityHierarchy(activityName)

		activityProgress = &ExecutionProgress{
			Name:       sanitizeDescription(activityName),
			StartTime:  feedback.Timestamp,
			Metadata:   make(map[string]any),
			ParentName: parentName,
			Children:   make([]string, 0),
			Level:      level,
		}
		t.executions[activityName] = activityProgress

		// Add to parent's children list if this is a sub-activity
		if parentName != "" {
			if parentActivity, exists := t.executions[parentName]; exists {
				parentActivity.Children = append(parentActivity.Children, activityName)
			}
		}

		t.order = append(t.order, activityName)
	}

	// Update activity state
	activityProgress.Status = feedback.Status
	activityProgress.Progress = feedback.Progress
	activityProgress.Message = sanitizeDescription(feedback.Message)
	activityProgress.LastUpdate = feedback.Timestamp
	activityProgress.Error = feedback.Error

	// Update metadata if provided
	if feedback.Metadata != nil {
		maps.Copy(activityProgress.Metadata, feedback.Metadata)
	}

	// Set end time for completed/failed activities
	if feedback.Status == executor.StatusCompleted || feedback.Status == executor.StatusFailed {
		endTime := feedback.Timestamp
		activityProgress.EndTime = &endTime
	}
}

// displayLoop handles the continuous update of the display
func (t *ExecutionTracker) displayLoop() {
	for {
		select {
		case <-t.ticker.C:
			atomic.AddInt64(&t.spinnerIndex, 1)
			t.mu.RLock()
			if t.isRunning {
				t.updateDisplay()
			}
			t.mu.RUnlock()
		case <-t.stopChan:
			return
		}
	}
}

// updateDisplay redraws the entire progress display
func (t *ExecutionTracker) updateDisplay() {
	// Clear previous output
	if t.lastLines > 0 {
		for i := 0; i < t.lastLines; i++ {
			fmt.Fprintf(os.Stderr, "\033[1A\033[K")
		}
	}

	lines := []string{}

	// Activity progress lines - show only recent activities
	visibleActivities := t.getVisibleActivities()
	currentSpinnerIndex := int(atomic.LoadInt64(&t.spinnerIndex))

	for _, activityName := range visibleActivities {
		activity := t.executions[activityName]
		if activity == nil {
			continue
		}

		line := t.formatActivityLine(activity, currentSpinnerIndex)
		if line != "" {
			lines = append(lines, line)
		}
	}

	// Print all lines
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%s\n", line)
	}

	t.lastLines = len(lines)
}

// formatActivityLine formats a single activity line with proper indentation
func (t *ExecutionTracker) formatActivityLine(activityItem *ExecutionProgress, spinnerIndex int) string {
	if activityItem == nil {
		return ""
	}

	// Create indentation based on activity level
	indent := strings.Repeat("  ", activityItem.Level) // +1 for base indentation
	prefix := indent

	message := activityItem.Message
	if message == "" {
		message = "..."
	}

	duration := t.getActivityDuration(activityItem)

	switch activityItem.Status {
	case executor.StatusPending:
		return fmt.Sprintf("%s%s⏸%s %s(%s)%s %s", prefix, colorYellow, colorReset, colorGray, duration, colorReset, message)

	case executor.StatusStarted, executor.StatusRunning:
		currentSpinner := t.spinnerChars[spinnerIndex%len(t.spinnerChars)]

		// Determine the display based on metadata
		var statusIcon, statusColor string
		if retryAttempt, ok := activityItem.Metadata["retry_attempt"].(int); ok {
			// This is a retry
			statusIcon = currentSpinner
			statusColor = colorYellow
			message = fmt.Sprintf("retry %d - %s", retryAttempt, message)
		} else {
			// This is normal running
			statusIcon = currentSpinner
			statusColor = colorCyan
		}

		progressStr := ""
		if activityItem.Progress > 0 {
			progressStr = fmt.Sprintf(" (%.0f%%)", activityItem.Progress*100)
		}

		return fmt.Sprintf("%s%s%s%s %s(%s)%s %s%s",
			prefix, statusColor, statusIcon, colorReset, colorGray, duration, colorReset, progressStr, message)

	case executor.StatusCompleted:
		return fmt.Sprintf("%s%s✓%s %s(%s)%s %s", prefix, colorGreen, colorReset, colorGray, duration, colorReset, message)

	case executor.StatusFailed:
		errorMsg := ""
		if activityItem.Error != nil {
			errorMsg = fmt.Sprintf(" - %s", activityItem.Error.Error())
			if len(errorMsg) > 50 {
				errorMsg = errorMsg[:47] + "..."
			}
		}
		return fmt.Sprintf("%s%s✗%s %s(%s)%s %s%s",
			prefix, colorRed, colorReset, colorGray, duration, colorReset, message, errorMsg)

	default:
		return ""
	}
}

// getVisibleActivities returns activities that should be visible in the display
// Takes into account hierarchy - shows parents if children are running
func (t *ExecutionTracker) getVisibleActivities() []string {
	// Create hierarchical ordering
	hierarchicalOrder := t.createHierarchicalOrder()

	if len(hierarchicalOrder) <= t.maxExecutions {
		return hierarchicalOrder
	}

	// Get running activities and their ancestors
	runningWithAncestors, completed := t.categorizeActivities(hierarchicalOrder)

	// Add running activities and their ancestors
	visible := t.addRunningActivities(hierarchicalOrder, runningWithAncestors)

	// Add recent completed activities if space available
	visible = t.addRecentCompletedActivities(visible, completed)

	return visible
}

// categorizeActivities separates activities into running/ancestors and completed categories.
func (t *ExecutionTracker) categorizeActivities(
	hierarchicalOrder []string,
) (runningWithAncestors map[string]bool, completed []string) {
	runningWithAncestors = make(map[string]bool)
	completed = []string{}

	for _, name := range hierarchicalOrder {
		activityItem := t.executions[name]
		if activityItem == nil {
			continue
		}

		switch activityItem.Status {
		case executor.StatusStarted, executor.StatusRunning:
			// Mark this activity and all its ancestors as needing to be shown
			t.markActivityAndAncestors(name, runningWithAncestors)
		case executor.StatusCompleted, executor.StatusFailed:
			completed = append(completed, name)
		}
	}

	return runningWithAncestors, completed
}

// addRunningActivities adds running activities and their ancestors in hierarchical order.
func (t *ExecutionTracker) addRunningActivities(
	hierarchicalOrder []string, runningWithAncestors map[string]bool,
) []string {
	visible := []string{}

	for _, name := range hierarchicalOrder {
		if runningWithAncestors[name] {
			visible = append(visible, name)
		}
	}

	return visible
}

// addRecentCompletedActivities adds recent completed activities if space is available.
func (t *ExecutionTracker) addRecentCompletedActivities(visible, completed []string) []string {
	recentCompleted := t.minCompleted
	availableSlots := t.maxExecutions - len(visible)

	if availableSlots > 0 && recentCompleted > availableSlots {
		recentCompleted = availableSlots
	}

	if len(completed) > 0 && recentCompleted > 0 {
		// Filter completed to show only root activities
		rootCompleted := t.filterRootCompleted(completed)

		start := max(len(rootCompleted)-recentCompleted, 0)
		if start < len(rootCompleted) {
			visible = append(visible, rootCompleted[start:]...)
		}
	}

	return visible
}

// filterRootCompleted filters completed activities to show only root level activities.
func (t *ExecutionTracker) filterRootCompleted(completed []string) []string {
	rootCompleted := []string{}

	for _, name := range completed {
		if activityItem := t.executions[name]; activityItem != nil && activityItem.Level == 0 {
			rootCompleted = append(rootCompleted, name)
		}
	}

	return rootCompleted
}

// createHierarchicalOrder creates a hierarchically ordered list of activities
func (t *ExecutionTracker) createHierarchicalOrder() []string {
	result := []string{}
	processed := make(map[string]bool)

	// Add activities in hierarchical order
	for _, name := range t.order {
		if !processed[name] {
			t.addActivityHierarchically(name, &result, processed)
		}
	}

	return result
}

// addActivityHierarchically adds an activity and its children in hierarchical order
func (t *ExecutionTracker) addActivityHierarchically(name string, result *[]string, processed map[string]bool) {
	if processed[name] {
		return
	}

	activity := t.executions[name]
	if activity == nil {
		return
	}

	processed[name] = true

	*result = append(*result, name)

	// Add children in order they were created
	for _, childName := range activity.Children {
		if !processed[childName] {
			t.addActivityHierarchically(childName, result, processed)
		}
	}
}

// markActivityAndAncestors marks an activity and all its ancestors for display
func (t *ExecutionTracker) markActivityAndAncestors(name string, marked map[string]bool) {
	activityItem := t.executions[name]
	if activityItem == nil {
		return
	}

	marked[name] = true

	// Mark parent and its ancestors
	if activityItem.ParentName != "" {
		t.markActivityAndAncestors(activityItem.ParentName, marked)
	}
}

// getActivityDuration returns a human-readable duration string
func (t *ExecutionTracker) getActivityDuration(activity *ExecutionProgress) string {
	if activity == nil {
		return "0s"
	}

	endTime := time.Now()
	if activity.EndTime != nil {
		endTime = *activity.EndTime
	}

	duration := endTime.Sub(activity.StartTime)

	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	} else if duration < time.Minute {
		return fmt.Sprintf("%.1fs", duration.Seconds())
	} else {
		return fmt.Sprintf("%.1fm", duration.Minutes())
	}
}
