package state

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFlowState(t *testing.T) {
	s := NewFlowState()
	assert.NotNil(t, s)
	assert.Empty(t, s.Facts)
	assert.Empty(t, s.Tasks)
	assert.Nil(t, s.RestartRequest)
	assert.Equal(t, 0, s.RestartCount)
}

func TestRestartRequest(t *testing.T) {
	s := NewFlowState()

	// Initial state
	req, ok := s.GetRestartRequest()
	assert.False(t, ok)
	assert.Empty(t, req)

	// Set request
	instruction := "restart now"
	s.SetRestartRequest(instruction)

	// Get request (should clear it)
	req, ok = s.GetRestartRequest()
	assert.True(t, ok)
	assert.Equal(t, instruction, req)

	// Get again (should be empty)
	req, ok = s.GetRestartRequest()
	assert.False(t, ok)
	assert.Empty(t, req)
}

func TestIncrementRestartCount(t *testing.T) {
	s := NewFlowState()
	assert.Equal(t, 0, s.RestartCount)

	count := s.IncrementRestartCount()
	assert.Equal(t, 1, count)
	assert.Equal(t, 1, s.RestartCount)

	count = s.IncrementRestartCount()
	assert.Equal(t, 2, count)
	assert.Equal(t, 2, s.RestartCount)
}

func TestFactManagement(t *testing.T) {
	s := NewFlowState()

	// Add facts
	s.AddFact("The sky is blue")
	s.AddFact("Grass is green")

	// Get facts
	facts := s.GetFacts()
	assert.Len(t, facts, 2)
	assert.Equal(t, "The sky is blue", facts[0].Content)
	assert.Equal(t, "Grass is green", facts[1].Content)

	// Modify returned slice, should not affect state
	facts[0].Content = "Modified"
	assert.Equal(t, "The sky is blue", s.GetFacts()[0].Content)

	// Search facts
	results := s.SearchFacts("blue")
	assert.Len(t, results, 1)
	assert.Equal(t, "The sky is blue", results[0].Content)

	results = s.SearchFacts("is")
	assert.Len(t, results, 2)

	results = s.SearchFacts("red")
	assert.Empty(t, results)
}

func TestTaskManagement(t *testing.T) {
	s := NewFlowState()

	tasks := []Task{
		{ID: "1", Description: "Task 1", Status: StatusPending},
		{ID: "2", Description: "Task 2", Status: StatusPending},
	}

	// Set tasks
	s.SetTasks(tasks)
	assert.Len(t, s.GetTasks(), 2)

	// Update task status
	err := s.UpdateTaskStatus("1", StatusDone)
	require.NoError(t, err)

	currentTasks := s.GetTasks()
	assert.Equal(t, StatusDone, currentTasks[0].Status)
	assert.Equal(t, StatusPending, currentTasks[1].Status)

	// Update non-existent task
	err = s.UpdateTaskStatus("999", StatusDone)
	assert.Error(t, err)

	// Reset plan
	s.ResetPlan()
	assert.Empty(t, s.GetTasks())
	// Facts should remain (if any were added)
	s.AddFact("Fact")
	s.ResetPlan()
	assert.NotEmpty(t, s.GetFacts())
	assert.Empty(t, s.GetTasks())
}

func TestContextOperations(t *testing.T) {
	s := NewFlowState()
	ctx := context.Background()

	// Get from empty context
	retrieved, err := GetFlowState(ctx)
	assert.Error(t, err)
	assert.Nil(t, retrieved)

	// With context
	ctx = WithFlowState(ctx, s)
	retrieved, err = GetFlowState(ctx)
	require.NoError(t, err)
	assert.Equal(t, s, retrieved)

	// With invalid type in context (using internal key since we are in same package)
	ctxInvalid := context.WithValue(context.Background(), flowStateKey, "invalid string")
	retrieved, err = GetFlowState(ctxInvalid)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid flow state type")
	assert.Nil(t, retrieved)
}
