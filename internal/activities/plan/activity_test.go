// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestPlanActivity_CreatesTasks(t *testing.T) {
	flowState := state.NewFlowState()
	ctx := state.WithFlowState(context.Background(), flowState)

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory().NewActivity()
	input := &Input{
		Tasks: []TaskInput{
			{ID: "1", Description: "first"},
			{ID: "2", Description: "second"},
		},
	}
	output, err := activity(ctx, flowCtx, input)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.True(t, output.Success)
	assert.Len(t, output.Tasks, 2)
	assert.Equal(t, state.StatusPending, output.Tasks[0].Status)

	tasks := flowState.GetTasks()
	require.Len(t, tasks, 2)
	assert.Equal(t, "1", tasks[0].ID)
	assert.Equal(t, state.StatusPending, tasks[0].Status)
}

func TestPlanActivity_UnchangedPlan(t *testing.T) {
	flowState := state.NewFlowState()
	flowState.SetTasks([]state.Task{
		{ID: "1", Description: "first", Status: state.StatusPending},
	})
	ctx := state.WithFlowState(context.Background(), flowState)

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory().NewActivity()
	_, err := activity(ctx, flowCtx, &Input{Tasks: []TaskInput{{ID: "1", Description: "first"}}})
	require.NoError(t, err)
	assert.Len(t, flowState.GetTasks(), 1)
}

func TestEqualTasks(t *testing.T) {
	base := []state.Task{{ID: "1", Description: "a", Status: state.StatusPending}}
	assert.True(t, equalTasks(base, []state.Task{{ID: "1", Description: "a", Status: state.StatusPending}}))
	assert.False(t, equalTasks(base, []state.Task{}))
	assert.False(t, equalTasks(base, []state.Task{{ID: "1", Description: "b", Status: state.StatusPending}}))
	assert.False(t, equalTasks(base, []state.Task{{ID: "1", Description: "a", Status: state.StatusDone}}))
}

func TestFormatPlanDetails(t *testing.T) {
	assert.Equal(t, "(no tasks)", formatPlanDetails(nil))
	assert.Equal(t, "(no tasks)", formatPlanDetails([]state.Task{}))

	tasks := []state.Task{
		{ID: "1", Description: "first", Status: state.StatusPending},
		{ID: "2", Description: "second", Status: state.StatusDone},
	}
	expected := "- [pending] 1: first\n- [done] 2: second"
	assert.Equal(t, expected, formatPlanDetails(tasks))
}
