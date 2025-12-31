// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

type TaskInput struct {
	ID          string
	Description string
}

type Input struct {
	Tasks []TaskInput
}

type Output struct {
	Success bool
}

type Factory struct{}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		s, err := state.GetFlowState(ctx)
		if err != nil {
			return nil, err
		}

		flowCtx.SendRunningWithDetails("Creating plan", fmt.Sprintf("tasks=%d", len(input.Tasks)))

		tasks := make([]state.Task, len(input.Tasks))
		for i, t := range input.Tasks {
			tasks[i] = state.Task{
				ID:          t.ID,
				Description: t.Description,
				Status:      state.StatusPending,
			}
		}

		s.SetTasks(tasks)
		flowCtx.SendCompleted("Plan created")

		return &Output{Success: true}, nil
	}
}
