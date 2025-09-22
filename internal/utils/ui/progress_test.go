package ui

import (
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
