// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package control

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

type RestartInput struct {
	Instruction string
}

type Output struct {
	Message string
}

type RestartFactory struct{}

var _ executor.ActivityFactory[*RestartInput, *Output] = (*RestartFactory)(nil)

func NewRestartFactory() *RestartFactory {
	return &RestartFactory{}
}

func (f *RestartFactory) NewActivity() executor.Activity[*RestartInput, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *RestartInput) (*Output, error) {
		s, err := state.GetFlowState(ctx)
		if err != nil {
			return nil, err
		}

		flowCtx.SendRunningWithDetails("Requesting flow restart", fmt.Sprintf("len=%d", len(input.Instruction)))

		s.SetRestartRequest(input.Instruction)

		flowCtx.SendCompleted("Restart requested")

		return &Output{Message: "Flow will restart with new instruction."}, nil
	}
}
