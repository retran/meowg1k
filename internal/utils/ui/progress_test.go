package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/flows"
)

func TestFlowProgressTracker_TaskVisibility(t *testing.T) {
	tests := []struct {
		name     string
		tasks    map[string]*TaskProgress
		expected int
		scenario string
	}{
		{
			name:     "empty tracker",
			tasks:    map[string]*TaskProgress{},
			expected: 0,
			scenario: "should show no tasks when empty",
		},
		{
			name: "only running tasks",
			tasks: map[string]*TaskProgress{
				"task1": {ID: "task1", Name: "Task 1", Status: "running", LastUpdate: time.Now()},
				"task2": {ID: "task2", Name: "Task 2", Status: "running", LastUpdate: time.Now().Add(-1 * time.Minute)},
				"task3": {ID: "task3", Name: "Task 3", Status: "retrying", LastUpdate: time.Now().Add(-2 * time.Minute)},
			},
			expected: 3,
			scenario: "should show all running tasks when under limit",
		},
		{
			name: "only completed tasks",
			tasks: map[string]*TaskProgress{
				"task1": {ID: "task1", Name: "Task 1", Status: "completed", LastUpdate: time.Now()},
				"task2": {ID: "task2", Name: "Task 2", Status: "completed", LastUpdate: time.Now().Add(-1 * time.Minute)},
				"task3": {ID: "task3", Name: "Task 3", Status: "completed", LastUpdate: time.Now().Add(-2 * time.Minute)},
			},
			expected: 3,
			scenario: "should show recent completed tasks when no running tasks",
		},
		{
			name: "failed tasks priority",
			tasks: map[string]*TaskProgress{
				"task1": {ID: "task1", Name: "Task 1", Status: "failed", LastUpdate: time.Now()},
				"task2": {ID: "task2", Name: "Task 2", Status: "running", LastUpdate: time.Now().Add(-1 * time.Minute)},
				"task3": {ID: "task3", Name: "Task 3", Status: "completed", LastUpdate: time.Now().Add(-2 * time.Minute)},
			},
			expected: 3,
			scenario: "should prioritize failed tasks first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewFlowProgressTracker(true, "test-flow") // silent mode

			// Add tasks to tracker
			for id, task := range tt.tasks {
				tracker.tasks[id] = task
				tracker.taskOrder = append(tracker.taskOrder, id)
			}

			visible := tracker.getVisibleTasks()

			if len(visible) != tt.expected {
				t.Errorf("Expected %d visible tasks, got %d. Scenario: %s", tt.expected, len(visible), tt.scenario)
			}
		})
	}
}

func TestFlowProgressTracker_BalancedDisplay(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")
	now := time.Now()

	// Create scenario: many running tasks + recent completions
	tasks := map[string]*TaskProgress{
		"running1":   {ID: "running1", Name: "Running 1", Status: "running", LastUpdate: now},
		"running2":   {ID: "running2", Name: "Running 2", Status: "running", LastUpdate: now.Add(-1 * time.Minute)},
		"running3":   {ID: "running3", Name: "Running 3", Status: "running", LastUpdate: now.Add(-2 * time.Minute)},
		"running4":   {ID: "running4", Name: "Running 4", Status: "running", LastUpdate: now.Add(-3 * time.Minute)},
		"running5":   {ID: "running5", Name: "Running 5", Status: "running", LastUpdate: now.Add(-4 * time.Minute)},
		"completed1": {ID: "completed1", Name: "Completed 1", Status: "completed", LastUpdate: now.Add(-30 * time.Second)},
		"completed2": {ID: "completed2", Name: "Completed 2", Status: "completed", LastUpdate: now.Add(-90 * time.Second)},
		"completed3": {ID: "completed3", Name: "Completed 3", Status: "completed", LastUpdate: now.Add(-150 * time.Second)},
		"failed1":    {ID: "failed1", Name: "Failed 1", Status: "failed", LastUpdate: now.Add(-10 * time.Second)},
	}

	// Add tasks to tracker
	for id, task := range tasks {
		tracker.tasks[id] = task
		tracker.taskOrder = append(tracker.taskOrder, id)
	}

	visible := tracker.getVisibleTasks()

	// Count tasks by type to verify display logic
	var failedCount, completedCount, runningCount int
	for _, taskID := range visible {
		task := tracker.tasks[taskID]
		switch task.Status {
		case "failed":
			failedCount++
		case "completed":
			completedCount++
		case "running", "retrying":
			runningCount++
		}
	}

	// Failed tasks don't count toward maxTasks limit, so we verify:
	// non-failed tasks should respect maxTasks limit
	nonFailedTasks := completedCount + runningCount
	if nonFailedTasks > tracker.maxTasks {
		t.Errorf("Non-failed tasks (%d) exceed maxTasks limit (%d)", nonFailedTasks, tracker.maxTasks)
	}

	// Should include at least one failed task (highest priority)
	if failedCount == 0 {
		t.Error("Expected at least one failed task to be visible")
	}

	if completedCount == 0 {
		t.Error("Expected at least one completed task to be visible for user feedback")
	}

	if runningCount == 0 {
		t.Error("Expected at least one running task to be visible")
	}
}

func TestFlowProgressTracker_TaskSorting(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")
	now := time.Now()

	// Create tasks with specific order expectations
	tasks := map[string]*TaskProgress{
		"failed":    {ID: "failed", Name: "Failed Task", Status: "failed", LastUpdate: now.Add(-1 * time.Hour)},
		"running":   {ID: "running", Name: "Running Task", Status: "running", LastUpdate: now.Add(-30 * time.Minute)},
		"completed": {ID: "completed", Name: "Completed Task", Status: "completed", LastUpdate: now.Add(-5 * time.Minute)},
	}

	// Add tasks to tracker
	for id, task := range tasks {
		tracker.tasks[id] = task
		tracker.taskOrder = append(tracker.taskOrder, id)
	}

	visible := tracker.getVisibleTasks()

	if len(visible) != 3 {
		t.Fatalf("Expected 3 visible tasks, got %d", len(visible))
	}

	// Verify sorting: failed first, then completed, then running
	if tracker.tasks[visible[0]].Status != "failed" {
		t.Error("Failed task should be first in display order")
	}

	if tracker.tasks[visible[1]].Status != "completed" {
		t.Error("Completed task should be second in display order")
	}

	if tracker.tasks[visible[2]].Status != "running" {
		t.Error("Running task should be third in display order")
	}
}

func TestFlowProgressTracker_FeedbackHandler(t *testing.T) {
	tracker := NewFlowProgressTracker(false, "test-flow") // NOT silent mode
	handler := tracker.FeedbackHandler()

	// Test task operations through feedback handler
	taskID := "test-task"
	taskName := "Test Task"

	// Start task
	handler(flows.Feedback{
		TaskID:      flows.TaskID(taskID),
		Status:      flows.TaskStarted,
		Description: taskName,
		Timestamp:   time.Now(),
	})

	if _, exists := tracker.tasks[taskID]; !exists {
		t.Error("Task should exist after task_started feedback")
	}

	task := tracker.tasks[taskID]
	if task.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", task.Status)
	}

	if task.Name != taskName {
		t.Errorf("Expected name '%s', got '%s'", taskName, task.Name)
	}

	// Complete task
	handler(flows.Feedback{
		TaskID:      flows.TaskID(taskID),
		Status:      flows.TaskCompleted,
		Description: taskName,
		Timestamp:   time.Now(),
	})

	if task.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", task.Status)
	}

	if task.EndTime == nil {
		t.Error("EndTime should be set after task_completed feedback")
	}
}

func TestFlowProgressTracker_RetryOperations(t *testing.T) {
	tracker := NewFlowProgressTracker(false, "test-flow") // NOT silent mode
	handler := tracker.FeedbackHandler()

	taskID := "retry-task"

	// Start task
	handler(flows.Feedback{
		TaskID:      flows.TaskID(taskID),
		Status:      flows.TaskStarted,
		Description: "Retry Task",
		Timestamp:   time.Now(),
	})

	// Mark as retrying with metadata
	retryMetadata := map[string]interface{}{
		"attempt":    1,
		"maxRetries": 3,
		"error":      "network error",
	}
	handler(flows.Feedback{
		TaskID:      flows.TaskID(taskID),
		Status:      flows.TaskRetrying,
		Description: "Retry Task",
		Metrics:     retryMetadata,
		Timestamp:   time.Now(),
	})

	task := tracker.tasks[taskID]
	if task.Status != "retrying" {
		t.Errorf("Expected status 'retrying', got '%s'", task.Status)
	}

	// Check metadata
	if task.Metadata == nil {
		t.Fatal("Metadata should be set for retrying task")
	}

	if attempt, ok := task.Metadata["attempt"].(int); !ok || attempt != 1 {
		t.Error("Retry attempt should be stored in metadata")
	}

	if maxRetries, ok := task.Metadata["maxRetries"].(int); !ok || maxRetries != 3 {
		t.Error("Max retries should be stored in metadata")
	}
}

func TestFlowProgressTracker_FailureOperations(t *testing.T) {
	tracker := NewFlowProgressTracker(false, "test-flow") // NOT silent mode
	handler := tracker.FeedbackHandler()

	taskID := "fail-task"
	errorMsg := "test error"

	// Start and fail task
	handler(flows.Feedback{
		TaskID:      flows.TaskID(taskID),
		Status:      flows.TaskStarted,
		Description: "Fail Task",
		Timestamp:   time.Now(),
	})

	failMetadata := map[string]interface{}{
		"error": errorMsg,
	}
	handler(flows.Feedback{
		TaskID:      flows.TaskID(taskID),
		Status:      flows.TaskFailed,
		Description: "Fail Task",
		Metrics:     failMetadata,
		Timestamp:   time.Now(),
	})

	task := tracker.tasks[taskID]
	if task.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", task.Status)
	}

	if task.EndTime == nil {
		t.Error("EndTime should be set after task_failed feedback")
	}

	// Check error metadata
	if task.Metadata == nil {
		t.Fatal("Metadata should be set for failed task")
	}

	if storedError, ok := task.Metadata["error"].(string); !ok || storedError != errorMsg {
		t.Error("Error message should be stored in metadata")
	}
}

func TestFlowProgressTracker_ProgressMetadata(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")

	taskID := "progress-task"
	taskName := "Progress Task"

	// Start task and simulate progress metadata
	tracker.startTask(taskID, taskName, nil)

	// Create a task with progress metadata
	progressMetadata := map[string]interface{}{
		"progress": 50.0,
		"details":  "Processing files",
	}

	// Update task metadata directly for testing
	task := tracker.tasks[taskID]
	task.Metadata = progressMetadata

	if progress, ok := task.Metadata["progress"].(float64); !ok || progress != 50.0 {
		t.Error("Progress should be stored in metadata")
	}

	if details, ok := task.Metadata["details"].(string); !ok || details != "Processing files" {
		t.Error("Progress details should be stored in metadata")
	}
}

func TestFlowProgressTracker_MinMaxHelpers(t *testing.T) {
	// Test min function
	if min(5, 3) != 3 {
		t.Error("min(5, 3) should return 3")
	}
	if min(1, 10) != 1 {
		t.Error("min(1, 10) should return 1")
	}
	if min(7, 7) != 7 {
		t.Error("min(7, 7) should return 7")
	}

	// Test max function
	if max(5, 3) != 5 {
		t.Error("max(5, 3) should return 5")
	}
	if max(1, 10) != 10 {
		t.Error("max(1, 10) should return 10")
	}
	if max(7, 7) != 7 {
		t.Error("max(7, 7) should return 7")
	}
}

func TestFlowProgressTracker_EdgeCases(t *testing.T) {
	tracker := NewFlowProgressTracker(false, "test-flow") // NOT silent mode
	handler := tracker.FeedbackHandler()

	// Test operations on non-existent task through feedback handler
	handler(flows.Feedback{
		TaskID:      flows.TaskID("non-existent"),
		Status:      flows.TaskCompleted,
		Description: "task",
		Timestamp:   time.Now(),
	})
	handler(flows.Feedback{
		TaskID:      flows.TaskID("non-existent"),
		Status:      flows.TaskFailed,
		Description: "task",
		Metrics:     map[string]interface{}{"error": "error"},
		Timestamp:   time.Now(),
	})
	handler(flows.Feedback{
		TaskID:      flows.TaskID("non-existent"),
		Status:      flows.TaskRetrying,
		Description: "task",
		Metrics:     map[string]interface{}{"attempt": 1},
		Timestamp:   time.Now(),
	})

	// Should not crash and should have no visible tasks
	visible := tracker.getVisibleTasks()
	if len(visible) != 0 {
		t.Error("Should have no visible tasks when operating on non-existent tasks")
	}

	// Test with nil tasks in map (shouldn't happen but defensive)
	tracker.tasks["nil-task"] = nil
	tracker.taskOrder = append(tracker.taskOrder, "nil-task")

	visible = tracker.getVisibleTasks()
	// Should handle nil tasks gracefully and not include them
	for _, taskID := range visible {
		if tracker.tasks[taskID] == nil {
			t.Error("Visible tasks should not include nil tasks")
		}
	}
}

func TestFlowProgressTracker_FlowOperations(t *testing.T) {
	tracker := NewFlowProgressTracker(false, "test-flow") // NOT silent mode

	// Test flow lifecycle
	tracker.Start()
	if !tracker.isRunning {
		t.Error("Tracker should be running after Start()")
	}

	tracker.Stop()
	if tracker.isRunning {
		t.Error("Tracker should not be running after Stop()")
	}
}

func TestFlowProgressTracker_GetTasksSummary(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")

	// Add some test tasks
	now := time.Now()
	tracker.tasks["task1"] = &TaskProgress{
		ID:         "task1",
		Name:       "Task 1",
		Status:     "completed",
		StartTime:  now.Add(-5 * time.Minute),
		EndTime:    &now,
		LastUpdate: now,
	}
	tracker.tasks["task2"] = &TaskProgress{
		ID:         "task2",
		Name:       "Task 2",
		Status:     "failed",
		StartTime:  now.Add(-3 * time.Minute),
		EndTime:    &now,
		LastUpdate: now,
		Metadata:   map[string]interface{}{"error": "test error"},
	}
	tracker.taskOrder = []string{"task1", "task2"}

	summary := tracker.GetTasksSummary()
	if summary == "" {
		t.Error("Summary should not be empty when tasks exist")
	}

	// Summary should contain task information
	if len(summary) < 10 { // Basic sanity check
		t.Error("Summary seems too short to contain meaningful information")
	}
}

func TestFlowProgressTracker_TimeBasedSorting(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")
	now := time.Now()

	// Create tasks with different timestamps within the same status
	tasks := map[string]*TaskProgress{
		"failed_old":    {ID: "failed_old", Name: "Failed Old", Status: "failed", LastUpdate: now.Add(-2 * time.Hour)},
		"failed_new":    {ID: "failed_new", Name: "Failed New", Status: "failed", LastUpdate: now.Add(-1 * time.Hour)},
		"completed_old": {ID: "completed_old", Name: "Completed Old", Status: "completed", LastUpdate: now.Add(-30 * time.Minute)},
		"completed_new": {ID: "completed_new", Name: "Completed New", Status: "completed", LastUpdate: now.Add(-10 * time.Minute)},
		"running_old":   {ID: "running_old", Name: "Running Old", Status: "running", LastUpdate: now.Add(-5 * time.Minute)},
		"running_new":   {ID: "running_new", Name: "Running New", Status: "running", LastUpdate: now.Add(-1 * time.Minute)},
	}

	// Add tasks to tracker
	for id, task := range tasks {
		tracker.tasks[id] = task
		tracker.taskOrder = append(tracker.taskOrder, id)
	}

	visible := tracker.getVisibleTasks()

	// Find positions of tasks in the visible list
	positions := make(map[string]int)
	for i, taskID := range visible {
		positions[taskID] = i
	}

	// Within failed tasks: older should come before newer (oldest first, most recent lower)
	if pos_old, ok := positions["failed_old"]; ok {
		if pos_new, ok := positions["failed_new"]; ok {
			if pos_old > pos_new {
				t.Error("Within failed tasks, older task should appear before newer task")
			}
		}
	}

	// Within completed tasks: older should come before newer
	if pos_old, ok := positions["completed_old"]; ok {
		if pos_new, ok := positions["completed_new"]; ok {
			if pos_old > pos_new {
				t.Error("Within completed tasks, older task should appear before newer task")
			}
		}
	}

	// Within running tasks: older should come before newer
	if pos_old, ok := positions["running_old"]; ok {
		if pos_new, ok := positions["running_new"]; ok {
			if pos_old > pos_new {
				t.Error("Within running tasks, older task should appear before newer task")
			}
		}
	}

	// Verify overall category ordering is maintained
	var firstFailed, firstCompleted, firstRunning int = -1, -1, -1
	for i, taskID := range visible {
		task := tracker.tasks[taskID]
		switch task.Status {
		case "failed":
			if firstFailed == -1 {
				firstFailed = i
			}
		case "completed":
			if firstCompleted == -1 {
				firstCompleted = i
			}
		case "running":
			if firstRunning == -1 {
				firstRunning = i
			}
		}
	}

	// Failed tasks should come before completed tasks
	if firstFailed != -1 && firstCompleted != -1 && firstFailed > firstCompleted {
		t.Error("Failed tasks should appear before completed tasks")
	}

	// Completed tasks should come before running tasks
	if firstCompleted != -1 && firstRunning != -1 && firstCompleted > firstRunning {
		t.Error("Completed tasks should appear before running tasks")
	}
}

// TestFlowProgressTracker_GetTaskDetails tests the getTaskDetails function
func TestFlowProgressTracker_GetTaskDetails(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")

	tests := []struct {
		name     string
		task     *TaskProgress
		expected string
	}{
		{
			name:     "nil task",
			task:     nil,
			expected: "",
		},
		{
			name: "task with no metadata",
			task: &TaskProgress{
				ID:       "task1",
				Name:     "Test Task",
				Status:   "running",
				Metadata: nil,
			},
			expected: "",
		},
		{
			name: "task with progress",
			task: &TaskProgress{
				ID:     "task1",
				Name:   "Test Task",
				Status: "running",
				Metadata: map[string]interface{}{
					"progress": 0.75,
				},
			},
			expected: "- 75%",
		},
		{
			name: "task with step",
			task: &TaskProgress{
				ID:     "task1",
				Name:   "Test Task",
				Status: "running",
				Metadata: map[string]interface{}{
					"step": "processing files",
				},
			},
			expected: "- processing files",
		},
		{
			name: "task with model",
			task: &TaskProgress{
				ID:     "task1",
				Name:   "Test Task",
				Status: "running",
				Metadata: map[string]interface{}{
					"model": "gpt-4",
				},
			},
			expected: "- (gpt-4)",
		},
		{
			name: "task with progress priority over step",
			task: &TaskProgress{
				ID:     "task1",
				Name:   "Test Task",
				Status: "running",
				Metadata: map[string]interface{}{
					"progress": 0.5,
					"step":     "processing",
				},
			},
			expected: "- 50%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.getTaskDetails(tt.task)
			if result != tt.expected {
				t.Errorf("getTaskDetails() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// TestFlowProgressTracker_GetTaskDuration tests the getTaskDuration function
func TestFlowProgressTracker_GetTaskDuration(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")
	now := time.Now()

	tests := []struct {
		name     string
		task     *TaskProgress
		expected string
	}{
		{
			name:     "nil task",
			task:     nil,
			expected: "",
		},
		{
			name: "task with no end time",
			task: &TaskProgress{
				ID:        "task1",
				StartTime: now,
				EndTime:   nil,
			},
			expected: "",
		},
		{
			name: "task with very short duration",
			task: &TaskProgress{
				ID:        "task1",
				StartTime: now,
				EndTime:   &now, // Same time - essentially 0 duration
			},
			expected: "",
		},
		{
			name: "task with millisecond duration",
			task: &TaskProgress{
				ID:        "task1",
				StartTime: now,
				EndTime:   func() *time.Time { t := now.Add(500 * time.Millisecond); return &t }(),
			},
			expected: "(500ms)",
		},
		{
			name: "task with second duration",
			task: &TaskProgress{
				ID:        "task1",
				StartTime: now,
				EndTime:   func() *time.Time { t := now.Add(2500 * time.Millisecond); return &t }(),
			},
			expected: "(2.5s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.getTaskDuration(tt.task)
			if result != tt.expected {
				t.Errorf("getTaskDuration() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// TestFlowProgressTracker_GetTaskError tests the getTaskError function
func TestFlowProgressTracker_GetTaskError(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")

	tests := []struct {
		name     string
		task     *TaskProgress
		expected string
	}{
		{
			name:     "nil task",
			task:     nil,
			expected: "",
		},
		{
			name: "task with no metadata",
			task: &TaskProgress{
				ID:       "task1",
				Metadata: nil,
			},
			expected: "",
		},
		{
			name: "task with short error",
			task: &TaskProgress{
				ID: "task1",
				Metadata: map[string]interface{}{
					"error": "connection failed",
				},
			},
			expected: "connection failed",
		},
		{
			name: "task with long error",
			task: &TaskProgress{
				ID: "task1",
				Metadata: map[string]interface{}{
					"error": "this is a very long error message that should be truncated because it exceeds the limit",
				},
			},
			expected: "this is a very long error message that should b...",
		},
		{
			name: "task with no error in metadata",
			task: &TaskProgress{
				ID: "task1",
				Metadata: map[string]interface{}{
					"other": "value",
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.getTaskError(tt.task)
			if result != tt.expected {
				t.Errorf("getTaskError() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// TestFlowProgressTracker_GetRetryInfo tests the getRetryInfo function
func TestFlowProgressTracker_GetRetryInfo(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")

	tests := []struct {
		name     string
		task     *TaskProgress
		expected string
	}{
		{
			name:     "nil task",
			task:     nil,
			expected: "",
		},
		{
			name: "task with no metadata",
			task: &TaskProgress{
				ID:       "task1",
				Metadata: nil,
			},
			expected: "",
		},
		{
			name: "task with attempt",
			task: &TaskProgress{
				ID: "task1",
				Metadata: map[string]interface{}{
					"attempt": 3,
				},
			},
			expected: "(attempt 3)",
		},
		{
			name: "task with delay",
			task: &TaskProgress{
				ID: "task1",
				Metadata: map[string]interface{}{
					"delay": 5 * time.Second,
				},
			},
			expected: "(in 5s)",
		},
		{
			name: "task with attempt priority over delay",
			task: &TaskProgress{
				ID: "task1",
				Metadata: map[string]interface{}{
					"attempt": 2,
					"delay":   3 * time.Second,
				},
			},
			expected: "(attempt 2)",
		},
		{
			name: "task with no retry info",
			task: &TaskProgress{
				ID: "task1",
				Metadata: map[string]interface{}{
					"other": "value",
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.getRetryInfo(tt.task)
			if result != tt.expected {
				t.Errorf("getRetryInfo() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// TestFlowProgressTracker_FinishFlow tests the finishFlow function
func TestFlowProgressTracker_FinishFlow(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow") // Silent mode to avoid race condition

	// Use proper mutex locking when setting isRunning
	tracker.mu.Lock()
	tracker.isRunning = true
	tracker.lastLines = 3 // Simulate some previous output
	tracker.mu.Unlock()

	// Test success case
	tracker.finishFlow(true)

	tracker.mu.RLock()
	isRunning := tracker.isRunning
	lastLines := tracker.lastLines
	tracker.mu.RUnlock()

	if isRunning {
		t.Error("Tracker should not be running after finishFlow")
	}

	if lastLines != 0 {
		t.Error("lastLines should be reset to 0 after finishFlow")
	}

	// Test failure case
	tracker.mu.Lock()
	tracker.isRunning = true
	tracker.lastLines = 2
	tracker.mu.Unlock()

	tracker.finishFlow(false)

	tracker.mu.RLock()
	isRunning = tracker.isRunning
	lastLines = tracker.lastLines
	tracker.mu.RUnlock()

	if isRunning {
		t.Error("Tracker should not be running after finishFlow with failure")
	}

	if lastLines != 0 {
		t.Error("lastLines should be reset to 0 after finishFlow with failure")
	}
} // TestFlowProgressTracker_SafeGetString tests the safeGetString function
func TestFlowProgressTracker_SafeGetString(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")

	tests := []struct {
		name     string
		metadata map[string]interface{}
		key      string
		expected string
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			key:      "test",
			expected: "",
		},
		{
			name:     "key not found",
			metadata: map[string]interface{}{"other": "value"},
			key:      "test",
			expected: "",
		},
		{
			name:     "string value",
			metadata: map[string]interface{}{"test": "hello"},
			key:      "test",
			expected: "hello",
		},
		{
			name:     "int value",
			metadata: map[string]interface{}{"test": 42},
			key:      "test",
			expected: "42",
		},
		{
			name:     "float value",
			metadata: map[string]interface{}{"test": 3.14},
			key:      "test",
			expected: "3.14",
		},
		{
			name:     "bool value",
			metadata: map[string]interface{}{"test": true},
			key:      "test",
			expected: "true",
		},
		{
			name:     "unsupported type",
			metadata: map[string]interface{}{"test": make(chan int)},
			key:      "test",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.safeGetString(tt.metadata, tt.key)
			if result != tt.expected {
				t.Errorf("safeGetString() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// TestFlowProgressTracker_SafeGetFloat64 tests the safeGetFloat64 function
func TestFlowProgressTracker_SafeGetFloat64(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")

	tests := []struct {
		name     string
		metadata map[string]interface{}
		key      string
		expected float64
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			key:      "test",
			expected: 0,
		},
		{
			name:     "key not found",
			metadata: map[string]interface{}{"other": "value"},
			key:      "test",
			expected: 0,
		},
		{
			name:     "float64 value",
			metadata: map[string]interface{}{"test": 3.14},
			key:      "test",
			expected: 3.14,
		},
		{
			name:     "float32 value",
			metadata: map[string]interface{}{"test": float32(2.5)},
			key:      "test",
			expected: 2.5,
		},
		{
			name:     "int value",
			metadata: map[string]interface{}{"test": 42},
			key:      "test",
			expected: 42.0,
		},
		{
			name:     "uint value",
			metadata: map[string]interface{}{"test": uint(100)},
			key:      "test",
			expected: 100.0,
		},
		{
			name:     "unsupported type",
			metadata: map[string]interface{}{"test": "not a number"},
			key:      "test",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.safeGetFloat64(tt.metadata, tt.key)
			if result != tt.expected {
				t.Errorf("safeGetFloat64() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestFlowProgressTracker_SafeGetInt tests the safeGetInt function
func TestFlowProgressTracker_SafeGetInt(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")

	tests := []struct {
		name     string
		metadata map[string]interface{}
		key      string
		expected int
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			key:      "test",
			expected: 0,
		},
		{
			name:     "key not found",
			metadata: map[string]interface{}{"other": "value"},
			key:      "test",
			expected: 0,
		},
		{
			name:     "int value",
			metadata: map[string]interface{}{"test": 42},
			key:      "test",
			expected: 42,
		},
		{
			name:     "int8 value",
			metadata: map[string]interface{}{"test": int8(10)},
			key:      "test",
			expected: 10,
		},
		{
			name:     "float64 value",
			metadata: map[string]interface{}{"test": 100.0},
			key:      "test",
			expected: 100,
		},
		{
			name:     "uint value",
			metadata: map[string]interface{}{"test": uint(200)},
			key:      "test",
			expected: 200,
		},
		{
			name:     "unsupported type",
			metadata: map[string]interface{}{"test": "not a number"},
			key:      "test",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.safeGetInt(tt.metadata, tt.key)
			if result != tt.expected {
				t.Errorf("safeGetInt() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestFlowProgressTracker_SafeGetDuration tests the safeGetDuration function
func TestFlowProgressTracker_SafeGetDuration(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")

	tests := []struct {
		name     string
		metadata map[string]interface{}
		key      string
		expected time.Duration
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			key:      "test",
			expected: 0,
		},
		{
			name:     "key not found",
			metadata: map[string]interface{}{"other": "value"},
			key:      "test",
			expected: 0,
		},
		{
			name:     "duration value",
			metadata: map[string]interface{}{"test": 5 * time.Second},
			key:      "test",
			expected: 5 * time.Second,
		},
		{
			name:     "int nanoseconds",
			metadata: map[string]interface{}{"test": int(1000000000)}, // 1 second in nanoseconds
			key:      "test",
			expected: time.Second,
		},
		{
			name:     "string duration",
			metadata: map[string]interface{}{"test": "2m30s"},
			key:      "test",
			expected: 2*time.Minute + 30*time.Second,
		},
		{
			name:     "invalid string",
			metadata: map[string]interface{}{"test": "invalid"},
			key:      "test",
			expected: 0,
		},
		{
			name:     "unsupported type",
			metadata: map[string]interface{}{"test": make(chan int)},
			key:      "test",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.safeGetDuration(tt.metadata, tt.key)
			if result != tt.expected {
				t.Errorf("safeGetDuration() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestRunFlowWithProgress tests the RunFlowWithProgress function
func TestRunFlowWithProgress(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		result, err := RunFlowWithProgress(true, "test-flow", func(tracker *FlowProgressTracker) (string, error) {
			if tracker == nil {
				return "", fmt.Errorf("tracker should not be nil")
			}
			return "success", nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if result != "success" {
			t.Errorf("Expected 'success', got %q", result)
		}
	})

	t.Run("error in action", func(t *testing.T) {
		_, err := RunFlowWithProgress(true, "test-flow", func(tracker *FlowProgressTracker) (string, error) {
			return "", fmt.Errorf("test error")
		})

		if err == nil {
			t.Error("Expected error, got nil")
		}

		if err.Error() != "test error" {
			t.Errorf("Expected 'test error', got %q", err.Error())
		}
	})
}

// TestRunFlowWithProgressAndContext tests the RunFlowWithProgressAndContext function
func TestRunFlowWithProgressAndContext(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		ctx := context.Background()
		result, err := RunFlowWithProgressAndContext(ctx, true, "test-flow", func(ctx context.Context, tracker *FlowProgressTracker) (int, error) {
			if ctx == nil {
				return 0, fmt.Errorf("context should not be nil")
			}
			if tracker == nil {
				return 0, fmt.Errorf("tracker should not be nil")
			}
			return 42, nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if result != 42 {
			t.Errorf("Expected 42, got %d", result)
		}
	})

	t.Run("nil action", func(t *testing.T) {
		ctx := context.Background()
		_, err := RunFlowWithProgressAndContext[string](ctx, true, "test-flow", nil)

		if err == nil {
			t.Error("Expected error for nil action, got nil")
		}

		if !strings.Contains(err.Error(), "action function cannot be nil") {
			t.Errorf("Expected 'action function cannot be nil' error, got %q", err.Error())
		}
	})

	t.Run("nil context", func(t *testing.T) {
		// Test the actual nil context handling in the function
		var nilCtx context.Context = nil
		_, err := RunFlowWithProgressAndContext[string](nilCtx, true, "test-flow", func(ctx context.Context, tracker *FlowProgressTracker) (string, error) {
			return "test", nil
		})

		if err == nil {
			t.Error("Expected error for nil context, got nil")
		}

		if !strings.Contains(err.Error(), "context cannot be nil") {
			t.Errorf("Expected 'context cannot be nil' error, got %q", err.Error())
		}
	})

	t.Run("invalid flow name", func(t *testing.T) {
		ctx := context.Background()
		_, err := RunFlowWithProgressAndContext(ctx, true, "invalid/flow*name", func(ctx context.Context, tracker *FlowProgressTracker) (string, error) {
			return "test", nil
		})

		if err == nil {
			t.Error("Expected error for invalid flow name, got nil")
		}

		if !strings.Contains(err.Error(), "invalid flow name") {
			t.Errorf("Expected 'invalid flow name' error, got %q", err.Error())
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := RunFlowWithProgressAndContext(ctx, true, "test-flow", func(ctx context.Context, tracker *FlowProgressTracker) (string, error) {
			return "test", nil
		})

		if err == nil {
			t.Error("Expected error for cancelled context, got nil")
		}

		if !strings.Contains(err.Error(), "operation cancelled before starting") {
			t.Errorf("Expected 'operation cancelled before starting' error, got %q", err.Error())
		}
	})
}

func TestFlowProgressTracker_UpdateDisplay(t *testing.T) {
	t.Run("empty tracker", func(t *testing.T) {
		tracker := NewFlowProgressTracker(true, "test-flow") // silent mode

		// Should not panic with empty tasks
		tracker.updateDisplay()

		// Verify no changes to internal state
		if tracker.lastLines != 0 {
			t.Errorf("Expected lastLines to be 0, got %d", tracker.lastLines)
		}
	})

	t.Run("with running tasks", func(t *testing.T) {
		tracker := NewFlowProgressTracker(true, "test-flow")
		now := time.Now()

		// Add a running task
		tracker.tasks["task1"] = &TaskProgress{
			ID:         "task1",
			Name:       "Running Task",
			Status:     "running",
			LastUpdate: now,
		}
		tracker.taskOrder = append(tracker.taskOrder, "task1")

		// Should not panic
		tracker.updateDisplay()

		// Verify lines were calculated
		if tracker.lastLines == 0 {
			t.Error("Expected lastLines to be greater than 0 with running tasks")
		}
	})

	t.Run("with completed tasks only", func(t *testing.T) {
		tracker := NewFlowProgressTracker(true, "test-flow")
		now := time.Now()

		// Add a completed task
		tracker.tasks["task1"] = &TaskProgress{
			ID:         "task1",
			Name:       "Completed Task",
			Status:     "completed",
			LastUpdate: now,
		}
		tracker.taskOrder = append(tracker.taskOrder, "task1")

		// Should not panic
		tracker.updateDisplay()

		// Should have calculated lines for task even without running tasks
		if tracker.lastLines == 0 {
			t.Error("Expected lastLines to be greater than 0 with completed tasks")
		}
	})

	t.Run("with mixed task states", func(t *testing.T) {
		tracker := NewFlowProgressTracker(true, "test-flow")
		now := time.Now()

		// Add tasks with different states
		tracker.tasks["running"] = &TaskProgress{
			ID:         "running",
			Name:       "Running Task",
			Status:     "running",
			LastUpdate: now,
		}
		tracker.tasks["completed"] = &TaskProgress{
			ID:         "completed",
			Name:       "Completed Task",
			Status:     "completed",
			LastUpdate: now.Add(-1 * time.Minute),
		}
		tracker.tasks["failed"] = &TaskProgress{
			ID:         "failed",
			Name:       "Failed Task",
			Status:     "failed",
			LastUpdate: now.Add(-2 * time.Minute),
		}
		tracker.tasks["retrying"] = &TaskProgress{
			ID:         "retrying",
			Name:       "Retrying Task",
			Status:     "retrying",
			LastUpdate: now.Add(-30 * time.Second),
		}
		tracker.taskOrder = []string{"running", "completed", "failed", "retrying"}

		// Should not panic
		tracker.updateDisplay()

		// Should have calculated lines for all visible tasks
		if tracker.lastLines == 0 {
			t.Error("Expected lastLines to be greater than 0 with mixed tasks")
		}
	})

	t.Run("updates lastLines correctly", func(t *testing.T) {
		tracker := NewFlowProgressTracker(true, "test-flow")

		// First update with no tasks
		tracker.updateDisplay()
		firstLines := tracker.lastLines

		// Add a task and update again
		tracker.tasks["task1"] = &TaskProgress{
			ID:         "task1",
			Name:       "Test Task",
			Status:     "running",
			LastUpdate: time.Now(),
		}
		tracker.taskOrder = append(tracker.taskOrder, "task1")

		tracker.updateDisplay()
		secondLines := tracker.lastLines

		// Should have more lines with tasks
		if secondLines <= firstLines {
			t.Errorf("Expected more lines after adding task: first=%d, second=%d", firstLines, secondLines)
		}
	})
}

func TestFlowProgressTracker_FormatTaskLine(t *testing.T) {
	tracker := NewFlowProgressTracker(true, "test-flow")
	now := time.Now()
	spinnerIndex := 0

	t.Run("nil task", func(t *testing.T) {
		result := tracker.formatTaskLine(nil, spinnerIndex)
		if result != "" {
			t.Errorf("Expected empty string for nil task, got %q", result)
		}
	})

	t.Run("running task", func(t *testing.T) {
		task := &TaskProgress{
			ID:         "task1",
			Name:       "Running Task",
			Status:     "running",
			LastUpdate: now,
		}

		result := tracker.formatTaskLine(task, spinnerIndex)

		if result == "" {
			t.Error("Expected non-empty result for running task")
		}

		if !strings.Contains(result, "Running Task") {
			t.Errorf("Expected task name in result, got %q", result)
		}

		if !strings.Contains(result, "...") {
			t.Errorf("Expected ellipsis for running task without details, got %q", result)
		}
	})

	t.Run("running task with details", func(t *testing.T) {
		task := &TaskProgress{
			ID:         "task1",
			Name:       "Running Task",
			Status:     "running",
			LastUpdate: now,
			Metadata: map[string]interface{}{
				"step":     "Processing files...",
				"progress": 0.75,
			},
		}

		result := tracker.formatTaskLine(task, spinnerIndex)

		if result == "" {
			t.Error("Expected non-empty result for running task with details")
		}

		if !strings.Contains(result, "Running Task") {
			t.Errorf("Expected task name in result, got %q", result)
		}

		// Should contain the step from metadata (but progress takes priority)
		if !strings.Contains(result, "75%") {
			t.Errorf("Expected progress percentage in result, got %q", result)
		}
	})

	t.Run("completed task", func(t *testing.T) {
		task := &TaskProgress{
			ID:         "task1",
			Name:       "Completed Task",
			Status:     "completed",
			LastUpdate: now,
			StartTime:  now.Add(-2 * time.Minute),
			EndTime:    &now,
		}

		result := tracker.formatTaskLine(task, spinnerIndex)

		if result == "" {
			t.Error("Expected non-empty result for completed task")
		}

		if !strings.Contains(result, "Completed Task") {
			t.Errorf("Expected task name in result, got %q", result)
		}

		if !strings.Contains(result, "✓") {
			t.Errorf("Expected checkmark for completed task, got %q", result)
		}
	})

	t.Run("completed task without duration", func(t *testing.T) {
		task := &TaskProgress{
			ID:         "task1",
			Name:       "Completed Task",
			Status:     "completed",
			LastUpdate: now,
		}

		result := tracker.formatTaskLine(task, spinnerIndex)

		if result == "" {
			t.Error("Expected non-empty result for completed task")
		}

		if !strings.Contains(result, "Completed Task") {
			t.Errorf("Expected task name in result, got %q", result)
		}

		if !strings.Contains(result, "✓") {
			t.Errorf("Expected checkmark for completed task, got %q", result)
		}
	})

	t.Run("failed task", func(t *testing.T) {
		task := &TaskProgress{
			ID:         "task1",
			Name:       "Failed Task",
			Status:     "failed",
			LastUpdate: now,
			Metadata: map[string]interface{}{
				"error": "connection timeout",
			},
		}

		result := tracker.formatTaskLine(task, spinnerIndex)

		if result == "" {
			t.Error("Expected non-empty result for failed task")
		}

		if !strings.Contains(result, "Failed Task") {
			t.Errorf("Expected task name in result, got %q", result)
		}

		if !strings.Contains(result, "✗") {
			t.Errorf("Expected X mark for failed task, got %q", result)
		}

		if !strings.Contains(result, "connection timeout") {
			t.Errorf("Expected error message in result, got %q", result)
		}
	})

	t.Run("failed task without error", func(t *testing.T) {
		task := &TaskProgress{
			ID:         "task1",
			Name:       "Failed Task",
			Status:     "failed",
			LastUpdate: now,
		}

		result := tracker.formatTaskLine(task, spinnerIndex)

		if result == "" {
			t.Error("Expected non-empty result for failed task")
		}

		if !strings.Contains(result, "Failed Task") {
			t.Errorf("Expected task name in result, got %q", result)
		}

		if !strings.Contains(result, "✗") {
			t.Errorf("Expected X mark for failed task, got %q", result)
		}
	})

	t.Run("retrying task", func(t *testing.T) {
		task := &TaskProgress{
			ID:         "task1",
			Name:       "Retrying Task",
			Status:     "retrying",
			LastUpdate: now,
			Metadata: map[string]interface{}{
				"attempt": 2,
				"delay":   30 * time.Second,
			},
		}

		result := tracker.formatTaskLine(task, spinnerIndex)

		if result == "" {
			t.Error("Expected non-empty result for retrying task")
		}

		if !strings.Contains(result, "Retrying") {
			t.Errorf("Expected 'Retrying' in result, got %q", result)
		}

		if !strings.Contains(result, "Retrying Task") {
			t.Errorf("Expected task name in result, got %q", result)
		}

		// Should contain attempt info from metadata
		if !strings.Contains(result, "attempt") {
			t.Errorf("Expected retry attempt info in result, got %q", result)
		}
	})

	t.Run("retrying task without retry info", func(t *testing.T) {
		task := &TaskProgress{
			ID:         "task1",
			Name:       "Retrying Task",
			Status:     "retrying",
			LastUpdate: now,
		}

		result := tracker.formatTaskLine(task, spinnerIndex)

		if result == "" {
			t.Error("Expected non-empty result for retrying task")
		}

		if !strings.Contains(result, "Retrying") {
			t.Errorf("Expected 'Retrying' in result, got %q", result)
		}

		if !strings.Contains(result, "...") {
			t.Errorf("Expected ellipsis for retrying task without info, got %q", result)
		}
	})

	t.Run("unknown status", func(t *testing.T) {
		task := &TaskProgress{
			ID:         "task1",
			Name:       "Unknown Task",
			Status:     "unknown",
			LastUpdate: now,
		}

		result := tracker.formatTaskLine(task, spinnerIndex)

		if result != "" {
			t.Errorf("Expected empty string for unknown status, got %q", result)
		}
	})

	t.Run("spinner index wrapping", func(t *testing.T) {
		task := &TaskProgress{
			ID:         "task1",
			Name:       "Running Task",
			Status:     "running",
			LastUpdate: now,
		}

		// Test with large spinner index that should wrap
		largeIndex := len(tracker.spinnerChars)*3 + 2
		result := tracker.formatTaskLine(task, largeIndex)

		if result == "" {
			t.Error("Expected non-empty result even with large spinner index")
		}

		if !strings.Contains(result, "Running Task") {
			t.Errorf("Expected task name in result, got %q", result)
		}
	})
}
