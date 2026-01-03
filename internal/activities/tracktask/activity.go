// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package tracktask implements an activity for updating task status in a plan.
package tracktask

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input is the input for tracking a task.
type Input struct {
	ID     string
	Status string // pending, done, failed, skipped
}

// Output is the result of tracking a task.
type Output struct {
	Success bool
}

// Factory creates tracktask activities.
type Factory struct{}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new Factory.
func NewFactory() *Factory {
	return &Factory{}
}

// NewActivity creates a new activity that updates the status of a task in the plan.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		s, err := state.GetFlowState(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get flow state: %w", err)
		}

		statusStr := strings.ToLower(input.Status)
		var status state.TaskStatus
		switch statusStr {
		case "pending":
			status = state.StatusPending
		case "done", "completed":
			status = state.StatusDone
		case "failed", "fail":
			status = state.StatusFailed
		case "skipped", "skip":
			status = state.StatusSkipped
		default:
			return nil, fmt.Errorf("invalid status: %s", input.Status)
		}

		flowCtx.SendRunningWithDetails("Updating task", fmt.Sprintf("id=%s status=%s", input.ID, status))

		if err := s.UpdateTaskStatus(input.ID, status); err != nil {
			return nil, fmt.Errorf("failed to update task status: %w", err)
		}

		flowCtx.SendCompleted("Task updated")

		return &Output{Success: true}, nil
	}
}
