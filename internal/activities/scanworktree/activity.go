// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package scanworktree implements an activity that scans workspace state (working directory, stage, or HEAD) for files.
package scanworktree

import (
	"context"
	"fmt"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Output contains file states for workdir, stage, and head snapshots.
type Output struct {
	HeadState    map[string]domainindex.FileState
	StageState   map[string]domainindex.FileState
	WorkdirState map[string]domainindex.FileState
}

// Factory builds scanworktree activities.
type Factory struct {
	projectStateSvc ports.ProjectStateService
}

var _ executor.ActivityFactory[struct{}, *Output] = (*Factory)(nil)

// NewFactory creates a scanworktree activity factory.
func NewFactory(projectStateSvc ports.ProjectStateService) (executor.ActivityFactory[struct{}, *Output], error) {
	if projectStateSvc == nil {
		return nil, fmt.Errorf("scanworktree.NewFactory: projectStateSvc cannot be nil")
	}

	return &Factory{
		projectStateSvc: projectStateSvc,
	}, nil
}

// NewActivity returns the activity implementation.
func (f *Factory) NewActivity() executor.Activity[struct{}, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, _ struct{}) (*Output, error) {
		executorCtx.SendRunningWithDetails("I'm scanning the workspace state", "snapshots=_head_,_stage_,_workdir_")

		result := &Output{}

		headState, err := f.projectStateSvc.GetHeadState(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get HEAD state: %w", err)
		}
		result.HeadState = headState

		stageState, err := f.projectStateSvc.GetStagingState(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get staging state: %w", err)
		}
		result.StageState = stageState

		workdirState, err := f.projectStateSvc.GetWorkdirState(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory state: %w", err)
		}
		result.WorkdirState = workdirState

		executorCtx.SendCompletedWithDetails(
			"I've scanned the workspace",
			fmt.Sprintf("head=%d staged=%d working=%d", len(headState), len(stageState), len(workdirState)),
		)
		return result, nil
	}
}
