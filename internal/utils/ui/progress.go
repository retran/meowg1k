package ui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/retran/meowg1k/internal/flows"
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

// sanitizeTaskDescription removes control characters and limits length for safe display
func sanitizeTaskDescription(description string) string {
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

// FlowProgressTracker tracks and displays hierarchical progress with dynamic task status
type FlowProgressTracker struct {
	silent       bool
	mu           sync.RWMutex
	tasks        map[string]*TaskProgress
	taskOrder    []string
	flowName     string
	isRunning    bool
	maxTasks     int // Maximum number of tasks to display
	minCompleted int // Minimum number of completed tasks to show (reserved slots)

	// Display management
	ticker       *time.Ticker
	stopChan     chan struct{}
	spinnerChars []string
	spinnerIndex int64 // Use int64 for atomic operations
	lastLines    int
}

// TaskProgress represents the progress state of a single task with dynamic status
type TaskProgress struct {
	ID         string
	Name       string
	Status     string // "running", "completed", "failed", "retrying"
	StartTime  time.Time
	EndTime    *time.Time
	LastUpdate time.Time              // Time of last status change
	Metadata   map[string]interface{} // For dynamic status details
}

// NewFlowProgressTracker creates a new hierarchical progress tracker
func NewFlowProgressTracker(silent bool, flowName string) *FlowProgressTracker {
	// Sanitize flowName - if invalid, use a safe default
	if err := validateFlowName(flowName); err != nil {
		flowName = "Flow" // Safe default
	}

	tracker := &FlowProgressTracker{
		silent:       silent,
		tasks:        make(map[string]*TaskProgress),
		taskOrder:    make([]string, 0),
		flowName:     flowName,
		maxTasks:     6, // Show max 6 tasks total
		minCompleted: 2, // Always reserve 2 slots for recently completed tasks
		stopChan:     make(chan struct{}),
		spinnerChars: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}

	if !silent {
		tracker.ticker = time.NewTicker(80 * time.Millisecond)
		go tracker.displayLoop()
	}

	return tracker
}

// displayLoop handles the continuous update of the display
func (t *FlowProgressTracker) displayLoop() {
	for {
		select {
		case <-t.ticker.C:
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
func (t *FlowProgressTracker) updateDisplay() {
	// Clear previous output
	if t.lastLines > 0 {
		for i := 0; i < t.lastLines; i++ {
			if _, err := fmt.Fprintf(os.Stderr, "\033[1A\033[K"); err != nil {
				// Log error but continue, as this is display-related
				// In production, you might want to use a proper logger
				break
			}
		}
	}

	lines := []string{}

	// Check if we have any running tasks
	hasRunningTasks := false
	for _, task := range t.tasks {
		if task.Status == "running" || task.Status == "retrying" {
			hasRunningTasks = true
			break
		}
	}

	// Main flow line - only show if we have running tasks
	if hasRunningTasks {
		currentIndex := atomic.AddInt64(&t.spinnerIndex, 1) - 1
		currentSpinner := t.spinnerChars[int(currentIndex)%len(t.spinnerChars)]
		lines = append(lines, fmt.Sprintf(" %s%s%s %s", colorCyan, currentSpinner, colorReset, t.flowName))
	}

	// Task progress lines - show only recent tasks
	visibleTasks := t.getVisibleTasks()
	currentSpinnerIndex := int(atomic.LoadInt64(&t.spinnerIndex)) // Thread-safe read
	for _, taskID := range visibleTasks {
		task := t.tasks[taskID]
		if task == nil {
			continue
		}

		line := t.formatTaskLine(task, currentSpinnerIndex)
		if line != "" {
			lines = append(lines, line)
		}
	}

	// Print all lines
	for _, line := range lines {
		if _, err := fmt.Fprintf(os.Stderr, "%s\n", line); err != nil {
			// Log error but continue, as this is display-related
			// In production, you might want to use a proper logger
			break
		}
	}

	t.lastLines = len(lines)
}

// formatTaskLine formats a single task line with dynamic status
func (t *FlowProgressTracker) formatTaskLine(task *TaskProgress, spinnerIndex int) string {
	if task == nil {
		return ""
	}
	switch task.Status {
	case "running":
		currentSpinner := t.spinnerChars[spinnerIndex%len(t.spinnerChars)]
		details := t.getTaskDetails(task)
		if details != "" {
			return fmt.Sprintf("   %s%s%s %s %s", colorCyan, currentSpinner, colorReset, task.Name, details)
		}
		return fmt.Sprintf("   %s%s%s %s...", colorCyan, currentSpinner, colorReset, task.Name)

	case "completed":
		duration := t.getTaskDuration(task)
		if duration != "" {
			return fmt.Sprintf("   %s✓%s %s %s%s%s", colorGreen, colorReset, task.Name, colorGray, duration, colorReset)
		}
		return fmt.Sprintf("   %s✓%s %s", colorGreen, colorReset, task.Name)

	case "failed":
		errorMsg := t.getTaskError(task)
		if errorMsg != "" {
			return fmt.Sprintf("   %s✗%s %s %s(%s)%s", colorRed, colorReset, task.Name, colorRed, errorMsg, colorReset)
		}
		return fmt.Sprintf("   %s✗%s %s", colorRed, colorReset, task.Name)

	case "retrying":
		currentSpinner := t.spinnerChars[spinnerIndex%len(t.spinnerChars)]
		retryInfo := t.getRetryInfo(task)
		if retryInfo != "" {
			return fmt.Sprintf("   %s%s%s Retrying %s %s%s%s", colorYellow, currentSpinner, colorReset, task.Name, colorGray, retryInfo, colorReset)
		}
		return fmt.Sprintf("   %s%s%s Retrying %s...", colorYellow, currentSpinner, colorReset, task.Name)
	}

	return ""
}

// getVisibleTasks returns the tasks that should be visible using balanced display strategy
func (t *FlowProgressTracker) getVisibleTasks() []string {
	// Pre-allocate slices with reasonable capacity to avoid multiple allocations
	const initialCapacity = 10
	running := make([]string, 0, initialCapacity)
	completed := make([]string, 0, initialCapacity)
	failed := make([]string, 0, initialCapacity)

	// Single pass through tasks to categorize them
	for _, taskID := range t.taskOrder {
		task := t.tasks[taskID]
		if task == nil {
			continue
		}

		switch task.Status {
		case "running", "retrying":
			running = append(running, taskID)
		case "completed":
			completed = append(completed, taskID)
		case "failed":
			failed = append(failed, taskID)
		}
	}

	// Helper function to safely sort task slices by LastUpdate
	sortByLastUpdate := func(taskIDs []string) {
		if len(taskIDs) <= 1 {
			return // No need to sort
		}

		sort.Slice(taskIDs, func(i, j int) bool {
			taskI := t.tasks[taskIDs[i]]
			taskJ := t.tasks[taskIDs[j]]

			// Defensive programming: handle nil tasks
			if taskI == nil && taskJ == nil {
				return false
			}
			if taskI == nil {
				return false
			}
			if taskJ == nil {
				return true
			}

			return taskI.LastUpdate.After(taskJ.LastUpdate)
		})
	}

	// Sort each category efficiently - only if needed
	sortByLastUpdate(completed)
	sortByLastUpdate(failed)
	sortByLastUpdate(running)

	// Pre-allocate result slice with estimated capacity
	estimatedSize := len(failed) + min(len(completed), t.maxTasks) + min(len(running), t.maxTasks)
	visible := make([]string, 0, estimatedSize)

	// Strategy: Show all failed tasks first (don't consume slots), then completed tasks, then running tasks
	// 1. Always show ALL failed tasks first (most critical) - these don't count toward maxTasks
	// 2. Show completed tasks before running tasks for better user feedback
	// 3. Show running tasks up to threshold
	// 4. If running tasks > threshold, just show all running tasks

	const runningTaskThreshold = 6 // Threshold for when to limit display

	// Add ALL failed tasks first (these don't consume slots from maxTasks)
	visible = append(visible, failed...)

	// Determine how many running tasks to show
	if len(running) > runningTaskThreshold {
		// Too many running tasks - just show all running tasks
		visible = append(visible, running...)
	} else {
		// Show completed tasks first, then running tasks
		availableSlots := t.maxTasks - len(running)
		if availableSlots > 0 && len(completed) > 0 {
			// Fill available slots with most recent completed tasks
			completedToShow := min(len(completed), availableSlots)
			if completedToShow > 0 {
				visible = append(visible, completed[:completedToShow]...)
			}
		}

		// Add all running tasks after completed tasks
		visible = append(visible, running...)
	}

	// Final sort: failed first, then completed, then running/retrying
	// Use a more efficient sort that leverages the fact that each section is already sorted
	sort.Slice(visible, func(i, j int) bool {
		taskI := t.tasks[visible[i]]
		taskJ := t.tasks[visible[j]]

		// Defensive programming: handle nil tasks
		if taskI == nil && taskJ == nil {
			return false
		}
		if taskI == nil {
			return false
		}
		if taskJ == nil {
			return true
		}

		// Failed tasks always come first
		if taskI.Status == "failed" && taskJ.Status != "failed" {
			return true
		}
		if taskI.Status != "failed" && taskJ.Status == "failed" {
			return false
		}

		// Then completed tasks
		iCompleted := taskI.Status == "completed"
		jCompleted := taskJ.Status == "completed"
		if iCompleted && !jCompleted {
			return true
		}
		if !iCompleted && jCompleted {
			return false
		}

		// Then running/retrying tasks
		iRunning := taskI.Status == "running" || taskI.Status == "retrying"
		jRunning := taskJ.Status == "running" || taskJ.Status == "retrying"
		if iRunning && !jRunning {
			return true
		}
		if !iRunning && jRunning {
			return false
		}

		// Within same category, oldest first (most recent tasks lower in list)
		return taskI.LastUpdate.Before(taskJ.LastUpdate)
	})

	return visible
}

// validateFlowName validates the flow name for safety and display purposes
func validateFlowName(flowName string) error {
	// Check for nil or empty name
	if flowName == "" {
		return fmt.Errorf("flow name cannot be empty")
	}

	// Check length to prevent excessively long names
	if len(flowName) > 100 {
		return fmt.Errorf("flow name too long (max 100 characters), got %d", len(flowName))
	}

	// Check for dangerous characters that could be used for injection
	// Allow alphanumeric, spaces, hyphens, underscores, and basic punctuation
	for _, char := range flowName {
		if !isValidFlowNameChar(char) {
			return fmt.Errorf("flow name contains invalid character: %q", char)
		}
	}

	return nil
}

// isValidFlowNameChar checks if a character is safe for use in flow names
func isValidFlowNameChar(char rune) bool {
	// Allow letters, numbers, spaces, and safe punctuation
	return (char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') ||
		char == ' ' || char == '-' || char == '_' ||
		char == '.' || char == '(' || char == ')' ||
		char == '[' || char == ']' || char == ':' ||
		char == '/' || char == '\\'
}

// Helper functions for min/max since they're not built-in in older Go versions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// getTaskDetails extracts dynamic details from task metadata
func (t *FlowProgressTracker) getTaskDetails(task *TaskProgress) string {
	if task == nil || task.Metadata == nil {
		return ""
	}

	var details []string

	// Show progress percentage if available
	if progress := t.safeGetFloat64(task.Metadata, "progress"); progress > 0 {
		details = append(details, fmt.Sprintf("%.0f%%", progress*100))
	}

	// Show current step/operation
	if step := t.safeGetString(task.Metadata, "step"); step != "" {
		details = append(details, step)
	}

	// Show model/provider for AI tasks
	if model := t.safeGetString(task.Metadata, "model"); model != "" {
		details = append(details, fmt.Sprintf("(%s)", model))
	}

	if len(details) > 0 {
		return fmt.Sprintf("- %s", details[0])
	}

	return ""
}

// getTaskDuration returns formatted duration for completed tasks
func (t *FlowProgressTracker) getTaskDuration(task *TaskProgress) string {
	if task == nil || task.EndTime == nil {
		return ""
	}

	duration := task.EndTime.Sub(task.StartTime)
	if duration < time.Millisecond {
		return ""
	}

	if duration < time.Second {
		return fmt.Sprintf("(%dms)", duration.Milliseconds())
	}

	return fmt.Sprintf("(%.1fs)", duration.Seconds())
}

// getTaskError extracts error message from task metadata
func (t *FlowProgressTracker) getTaskError(task *TaskProgress) string {
	if task == nil || task.Metadata == nil {
		return ""
	}
	if errorMsg := t.safeGetString(task.Metadata, "error"); errorMsg != "" {
		// Truncate long error messages
		if len(errorMsg) > 50 {
			return errorMsg[:47] + "..."
		}
		return errorMsg
	}
	return ""
}

// getRetryInfo extracts retry information from task metadata
func (t *FlowProgressTracker) getRetryInfo(task *TaskProgress) string {
	if task == nil || task.Metadata == nil {
		return ""
	}

	var info []string

	if attempt := t.safeGetInt(task.Metadata, "attempt"); attempt > 0 {
		info = append(info, fmt.Sprintf("attempt %d", attempt))
	}

	if delay := t.safeGetDuration(task.Metadata, "delay"); delay > 0 {
		info = append(info, fmt.Sprintf("in %s", delay.Round(time.Second)))
	}

	if len(info) > 0 {
		return fmt.Sprintf("(%s)", info[0])
	}

	return ""
}

// FeedbackHandler returns a feedback handler for flows
func (t *FlowProgressTracker) FeedbackHandler() flows.FeedbackHandler {
	return func(feedback flows.Feedback) {
		t.mu.Lock()
		defer t.mu.Unlock()

		if t.silent {
			return
		}

		taskID := string(feedback.TaskID)
		description := feedback.Description
		if description == "" {
			description = taskID // Fallback to taskID if no description provided
		}

		// Extract metadata from feedback
		metadata := make(map[string]interface{})
		if feedback.Metrics != nil {
			// Copy all metrics as metadata
			for k, v := range feedback.Metrics {
				metadata[k] = v
			}
		}

		switch feedback.Status {
		case flows.WorkflowStarted:
			// Flow started

		case flows.TaskStarted:
			t.startTask(taskID, description, metadata)

		case flows.TaskCompleted:
			t.completeTask(taskID, description, metadata)

		case flows.TaskFailed:
			t.failTask(taskID, description, metadata)

		case flows.TaskRetrying:
			t.retryTask(taskID, description, metadata)

		case flows.WorkflowCompleted, flows.WorkflowFailed:
			t.finishFlow(feedback.Status == flows.WorkflowCompleted)
		}
	}
}

// startTask creates and starts a new task
func (t *FlowProgressTracker) startTask(taskID, description string, metadata map[string]interface{}) {
	now := time.Now()
	taskProgress := &TaskProgress{
		ID:         taskID,
		Name:       sanitizeTaskDescription(description),
		Status:     "running",
		StartTime:  now,
		LastUpdate: now,
		Metadata:   metadata,
	}

	t.tasks[taskID] = taskProgress

	// Add to order if not already there
	found := false
	for _, id := range t.taskOrder {
		if id == taskID {
			found = true
			break
		}
	}
	if !found {
		t.taskOrder = append(t.taskOrder, taskID)
	}
}

// completeTask marks a task as completed
func (t *FlowProgressTracker) completeTask(taskID, description string, metadata map[string]interface{}) {
	if task, exists := t.tasks[taskID]; exists {
		now := time.Now()
		task.Status = "completed"
		task.Name = sanitizeTaskDescription(description)
		task.EndTime = &now
		task.LastUpdate = now

		// Update metadata
		for k, v := range metadata {
			task.Metadata[k] = v
		}
	}
}

// failTask marks a task as failed
func (t *FlowProgressTracker) failTask(taskID, description string, metadata map[string]interface{}) {
	if task, exists := t.tasks[taskID]; exists {
		now := time.Now()
		task.Status = "failed"
		task.Name = sanitizeTaskDescription(description)
		task.EndTime = &now
		task.LastUpdate = now

		// Update metadata
		for k, v := range metadata {
			task.Metadata[k] = v
		}
	}
}

// retryTask shows retry status for a task
func (t *FlowProgressTracker) retryTask(taskID, description string, metadata map[string]interface{}) {
	if task, exists := t.tasks[taskID]; exists {
		task.Status = "retrying"
		task.Name = sanitizeTaskDescription(description)
		task.LastUpdate = time.Now()

		// Update metadata
		for k, v := range metadata {
			task.Metadata[k] = v
		}
	}
}

// finishFlow stops the flow and clears all progress display
func (t *FlowProgressTracker) finishFlow(success bool) {
	t.isRunning = false

	// Clear current display completely
	if t.lastLines > 0 {
		for i := 0; i < t.lastLines; i++ {
			if _, err := fmt.Fprintf(os.Stderr, "\033[1A\033[K"); err != nil {
				// Log error but continue, as this is display-related
				break
			}
		}
		t.lastLines = 0
	}

	// Don't show any final summary - just clear everything
	// The user wanted to remove all spinners and task statuses from stderr
}

// Start starts the progress display
func (t *FlowProgressTracker) Start() {
	if t.silent {
		return
	}

	t.mu.Lock()
	t.isRunning = true
	t.mu.Unlock()
}

// Stop stops the progress display and cleanup
func (t *FlowProgressTracker) Stop() {
	if t.silent {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.isRunning = false

	if t.ticker != nil {
		t.ticker.Stop()
	}

	close(t.stopChan)
}

// GetTasksSummary returns a summary of all tasks and their status
func (t *FlowProgressTracker) GetTasksSummary() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.tasks) == 0 {
		return ""
	}

	var completed, failed, running int
	for _, task := range t.tasks {
		switch task.Status {
		case "completed":
			completed++
		case "failed":
			failed++
		case "running", "retrying":
			running++
		}
	}

	if failed > 0 {
		return fmt.Sprintf("Tasks: %d completed, %d failed", completed, failed)
	}
	if running > 0 {
		return fmt.Sprintf("Tasks: %d completed, %d running", completed, running)
	}
	return fmt.Sprintf("Tasks: %d completed", completed)
}

// RunFlowWithProgress executes a flow with hierarchical progress tracking
func RunFlowWithProgress[T any](silent bool, flowName string, action func(*FlowProgressTracker) (T, error)) (T, error) {
	// Use background context for backward compatibility, but this is deprecated.
	// Consider using RunFlowWithProgressAndContext for better cancellation handling.
	return RunFlowWithProgressAndContext(context.Background(), silent, flowName, func(ctx context.Context, tracker *FlowProgressTracker) (T, error) {
		return action(tracker)
	})
}

// RunFlowWithProgressAndContext executes a flow with hierarchical progress tracking and explicit context handling
func RunFlowWithProgressAndContext[T any](ctx context.Context, silent bool, flowName string, action func(context.Context, *FlowProgressTracker) (T, error)) (T, error) {
	// Input validation
	if action == nil {
		var zero T
		return zero, fmt.Errorf("action function cannot be nil")
	}
	if ctx == nil {
		var zero T
		return zero, fmt.Errorf("context cannot be nil")
	}

	// Validate flowName to prevent injection attacks and ensure it's suitable for display
	if err := validateFlowName(flowName); err != nil {
		var zero T
		return zero, fmt.Errorf("invalid flow name: %w", err)
	}

	// Check if context is already cancelled before starting
	select {
	case <-ctx.Done():
		var zero T
		return zero, fmt.Errorf("operation cancelled before starting: %w", ctx.Err())
	default:
	}

	tracker := NewFlowProgressTracker(silent, flowName)

	if !silent {
		tracker.Start()
		defer tracker.Stop()
	}

	// Execute action with explicit context handling
	return action(ctx, tracker)
}

// Safe metadata getters to prevent panics and ensure type consistency

// safeGetString safely extracts a string value from metadata
func (t *FlowProgressTracker) safeGetString(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}

	value, exists := metadata[key]
	if !exists {
		return ""
	}

	if str, ok := value.(string); ok {
		return str
	}

	// Handle common type conversions
	if stringer, ok := value.(fmt.Stringer); ok {
		return stringer.String()
	}

	// As a last resort, use fmt.Sprintf for basic types
	switch v := value.(type) {
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%g", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		// Log unexpected type but don't panic
		return ""
	}
}

// safeGetFloat64 safely extracts a float64 value from metadata
func (t *FlowProgressTracker) safeGetFloat64(metadata map[string]interface{}, key string) float64 {
	if metadata == nil {
		return 0
	}

	value, exists := metadata[key]
	if !exists {
		return 0
	}

	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	default:
		return 0
	}
}

// safeGetInt safely extracts an int value from metadata
func (t *FlowProgressTracker) safeGetInt(metadata map[string]interface{}, key string) int {
	if metadata == nil {
		return 0
	}

	value, exists := metadata[key]
	if !exists {
		return 0
	}

	switch v := value.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		if v <= int64(^uint(0)>>1) { // Check if it fits in int
			return int(v)
		}
		return 0
	case uint:
		if v <= uint(^uint(0)>>1) { // Check if it fits in int
			return int(v)
		}
		return 0
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		if v <= uint64(^uint(0)>>1) { // Check if it fits in int
			return int(v)
		}
		return 0
	case float64:
		if v >= 0 && v <= float64(^uint(0)>>1) {
			return int(v)
		}
		return 0
	case float32:
		if v >= 0 && v <= float32(^uint(0)>>1) {
			return int(v)
		}
		return 0
	default:
		return 0
	}
}

// safeGetDuration safely extracts a time.Duration value from metadata
func (t *FlowProgressTracker) safeGetDuration(metadata map[string]interface{}, key string) time.Duration {
	if metadata == nil {
		return 0
	}

	value, exists := metadata[key]
	if !exists {
		return 0
	}

	if duration, ok := value.(time.Duration); ok {
		return duration
	}

	// Try to parse as int64 nanoseconds
	if ns := t.safeGetInt(metadata, key); ns > 0 {
		return time.Duration(ns)
	}

	// Try to parse string duration
	if str := t.safeGetString(metadata, key); str != "" {
		if duration, err := time.ParseDuration(str); err == nil {
			return duration
		}
	}

	return 0
}
