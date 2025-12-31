// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package tracktask

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	ID     string
	Status string // pending, done, failed, skipped
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
			return nil, err
		}

		flowCtx.SendCompleted("Task updated")

		return &Output{Success: true}, nil
	}
}
