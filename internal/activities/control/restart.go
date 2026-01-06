// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package control provides activities for flow control operations.
package control

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

// RestartInput is the input for restarting a flow with a new instruction.
type RestartInput struct {
	Instruction string
}

// Output is the result of a control operation.
type Output struct {
	Message string
}

// RestartFactory creates restart activities.
type RestartFactory struct{}

var _ executor.ActivityFactory[*RestartInput, *Output] = (*RestartFactory)(nil)

// NewRestartFactory creates a new RestartFactory.
func NewRestartFactory() *RestartFactory {
	return &RestartFactory{}
}

// NewActivity creates a new restart activity that triggers a flow restart with a new instruction.
func (f *RestartFactory) NewActivity() executor.Activity[*RestartInput, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *RestartInput) (*Output, error) {
		s, err := state.GetFlowState(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get flow state: %w", err)
		}

		flowCtx.SendRunningWithDetails("Requesting flow restart", fmt.Sprintf("len=%d", len(input.Instruction)))

		s.SetRestartRequest(input.Instruction)

		flowCtx.SendCompleted("Restart requested")

		return &Output{Message: "Flow will restart with new instruction."}, nil
	}
}
