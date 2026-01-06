// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package memorize provides activities for storing facts in flow memory.
package memorize

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input is the input for memorizing a fact.
type Input struct {
	Fact string
}

// Output is the result of memorizing a fact.
type Output struct {
	Success bool
}

// Factory creates memorize activities.
type Factory struct{}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new Factory.
func NewFactory() *Factory {
	return &Factory{}
}

// NewActivity creates a new activity that stores a fact in the flow memory.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		s, err := state.GetFlowState(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get flow state: %w", err)
		}

		flowCtx.SendRunningWithDetails("Memorizing fact", fmt.Sprintf("len=%d", len(input.Fact)))
		s.AddFact(input.Fact)
		flowCtx.SendCompleted("Memorized fact")

		return &Output{Success: true}, nil
	}
}
