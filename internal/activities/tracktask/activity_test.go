// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package tracktask

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestTrackTaskActivity_UpdatesStatus(t *testing.T) {
	flowState := state.NewFlowState()
	flowState.SetTasks([]state.Task{
		{ID: "1", Description: "first", Status: state.StatusPending},
	})
	ctx := state.WithFlowState(context.Background(), flowState)

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory().NewActivity()
	output, err := activity(ctx, flowCtx, &Input{ID: "1", Status: "done"})
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.True(t, output.Success)

	tasks := flowState.GetTasks()
	require.Len(t, tasks, 1)
	assert.Equal(t, state.StatusDone, tasks[0].Status)
}

func TestTrackTaskActivity_InvalidStatus(t *testing.T) {
	flowState := state.NewFlowState()
	flowState.SetTasks([]state.Task{{ID: "1", Description: "first", Status: state.StatusPending}})
	ctx := state.WithFlowState(context.Background(), flowState)

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory().NewActivity()
	_, err := activity(ctx, flowCtx, &Input{ID: "1", Status: "unknown"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestTrackTaskActivity_MissingFlowState(t *testing.T) {
	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory().NewActivity()
	_, err := activity(context.Background(), flowCtx, &Input{ID: "1", Status: "done"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get flow state")
}
