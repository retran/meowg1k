// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package getplan provides activities for retrieving the current plan/task board.
package getplan

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input is the input for getting the plan (currently empty).
type Input struct{}

// Task represents a single task from the plan.
type Task struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// Output contains the list of tasks from the plan.
type Output struct {
	Tasks []Task `json:"tasks"`
}

// Factory creates getplan activities.
type Factory struct{}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new Factory.
func NewFactory() *Factory {
	return &Factory{}
}

// NewActivity creates a new activity that retrieves the current plan/task board.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, _ *Input) (*Output, error) {
		s, err := state.GetFlowState(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get flow state: %w", err)
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
