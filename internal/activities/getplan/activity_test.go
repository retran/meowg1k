// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package getplan

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestGetPlanActivity_ReturnsTasks(t *testing.T) {
	flowState := state.NewFlowState()
	flowState.SetTasks([]state.Task{
		{ID: "1", Description: "First", Status: state.StatusPending},
		{ID: "2", Description: "Second", Status: state.StatusDone},
	})

	ctx := state.WithFlowState(context.Background(), flowState)
	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory().NewActivity()
	output, err := activity(ctx, flowCtx, &Input{})
	require.NoError(t, err)
	require.NotNil(t, output)
	require.Len(t, output.Tasks, 2)
	assert.Equal(t, "1", output.Tasks[0].ID)
	assert.Equal(t, "First", output.Tasks[0].Description)
	assert.Equal(t, "pending", output.Tasks[0].Status)
	assert.Equal(t, "2", output.Tasks[1].ID)
	assert.Equal(t, "done", output.Tasks[1].Status)
}

func TestGetPlanActivity_MissingFlowState(t *testing.T) {
	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory().NewActivity()
	_, err := activity(context.Background(), flowCtx, &Input{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get flow state")
}
