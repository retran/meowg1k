// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package control

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestRestartActivity_SetsRestartRequest(t *testing.T) {
	ctx := context.Background()
	flowState := state.NewFlowState()
	ctx = state.WithFlowState(ctx, flowState)

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewRestartFactory().NewActivity()
	output, err := activity(ctx, flowCtx, &RestartInput{Instruction: "restart now"})
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "Flow will restart with new instruction.", output.Message)

	request, ok := flowState.GetRestartRequest()
	assert.True(t, ok)
	assert.Equal(t, "restart now", request)
}

func TestRestartActivity_MissingFlowState(t *testing.T) {
	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewRestartFactory().NewActivity()
	_, err := activity(context.Background(), flowCtx, &RestartInput{Instruction: "restart"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get flow state")
}
