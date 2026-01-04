// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package memorize

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestMemorizeActivity_AddsFact(t *testing.T) {
	flowState := state.NewFlowState()
	ctx := state.WithFlowState(context.Background(), flowState)

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory().NewActivity()
	output, err := activity(ctx, flowCtx, &Input{Fact: "rain is wet"})
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.True(t, output.Success)

	facts := flowState.GetFacts()
	require.Len(t, facts, 1)
	assert.Equal(t, "rain is wet", facts[0].Content)
}

func TestMemorizeActivity_MissingFlowState(t *testing.T) {
	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	activity := NewFactory().NewActivity()
	_, err := activity(context.Background(), flowCtx, &Input{Fact: "note"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get flow state")
}
