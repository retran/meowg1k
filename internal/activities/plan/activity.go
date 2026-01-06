// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package plan implements an activity for initializing task plans.
package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

// TaskInput represents a task to be added to the plan.
type TaskInput struct {
	ID          string
	Description string
}

// Input is the input for creating a plan.
type Input struct {
	Tasks []TaskInput
}

// Output contains the created plan.
type Output struct {
	Tasks   []state.Task
	Success bool
}

// Factory creates plan activities.
type Factory struct{}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new Factory.
func NewFactory() *Factory {
	return &Factory{}
}

// NewActivity creates a new activity that creates or updates a plan.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		s, err := state.GetFlowState(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get flow state: %w", err)
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

		prev := s.GetTasks()
		if equalTasks(prev, tasks) {
			flowCtx.SendCompletedWithDetails("Plan unchanged", formatPlanDetails(tasks))
			return &Output{Success: true, Tasks: tasks}, nil
		}

		s.SetTasks(tasks)
		flowCtx.SendCompletedWithDetails("Plan created", formatPlanDetails(tasks))

		return &Output{Success: true, Tasks: tasks}, nil
	}
}

func equalTasks(a, b []state.Task) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
		if a[i].Description != b[i].Description {
			return false
		}
		if a[i].Status != b[i].Status {
			return false
		}
	}
	return true
}

func formatPlanDetails(tasks []state.Task) string {
	if len(tasks) == 0 {
		return "(no tasks)"
	}
	var b strings.Builder
	for _, t := range tasks {
		b.WriteString("- [")
		b.WriteString(string(t.Status))
		b.WriteString("] ")
		b.WriteString(t.ID)
		b.WriteString(": ")
		b.WriteString(t.Description)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
