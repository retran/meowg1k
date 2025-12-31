// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package getplan

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct{}

type Task struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type Output struct {
	Tasks []Task `json:"tasks"`
}

type Factory struct{}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, _ *Input) (*Output, error) {
		s, err := state.GetFlowState(ctx)
		if err != nil {
			return nil, err
		}

		tasks := s.GetTasks()
		flowCtx.SendRunningWithDetails("Getting plan", fmt.Sprintf("tasks=%d", len(tasks)))

		out := make([]Task, 0, len(tasks))
		for _, t := range tasks {
			out = append(out, Task{ID: t.ID, Description: t.Description, Status: string(t.Status)})
		}

		flowCtx.SendCompleted("Plan loaded")
		return &Output{Tasks: out}, nil
	}
}
