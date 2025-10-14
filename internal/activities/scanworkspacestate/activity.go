/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package scanworkspacestate provides an activity to scan workspace state.
package scanworkspacestate

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Output contains the workspace state for all three contexts.
type Output struct {
	HeadState    map[string]domainindex.FileState
	StageState   map[string]domainindex.FileState
	WorkdirState map[string]domainindex.FileState
}

// Factory creates instances of the ScanWorkspaceState activity with injected dependencies.
type Factory struct {
	projectStateSvc ports.ProjectStateService
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[struct{}, *Output] = (*Factory)(nil)

// NewFactory creates a new ScanWorkspaceState activity factory.
func NewFactory(projectStateSvc ports.ProjectStateService) (executor.ActivityFactory[struct{}, *Output], error) {
	if projectStateSvc == nil {
		return nil, fmt.Errorf("scanworkspacestate.NewFactory: projectStateSvc cannot be nil")
	}

	return &Factory{
		projectStateSvc: projectStateSvc,
	}, nil
}

// NewActivity creates and returns the ScanWorkspaceState activity function.
func (f *Factory) NewActivity() executor.Activity[struct{}, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, _ struct{}) (*Output, error) {
		executorCtx.SendRunning("Scanning workspace state...")

		result := &Output{}
		g, gctx := errgroup.WithContext(ctx)

		// Scan HEAD state
		g.Go(func() error {
			headState, err := f.projectStateSvc.GetHeadState()
			if err != nil {
				return fmt.Errorf("failed to get HEAD state: %w", err)
			}
			result.HeadState = headState
			return nil
		})

		// Scan staging area state
		g.Go(func() error {
			stageState, err := f.projectStateSvc.GetStagingState()
			if err != nil {
				return fmt.Errorf("failed to get staging state: %w", err)
			}
			result.StageState = stageState
			return nil
		})

		// Scan working directory state
		g.Go(func() error {
			workdirState, err := f.projectStateSvc.GetWorkdirState()
			if err != nil {
				return fmt.Errorf("failed to get working directory state: %w", err)
			}
			result.WorkdirState = workdirState
			return nil
		})

		// Wait for all scans to complete
		if err := g.Wait(); err != nil {
			return nil, err
		}

		// Check context cancellation
		if gctx.Err() != nil {
			return nil, gctx.Err()
		}

		executorCtx.SendCompleted("Workspace scan complete")
		return result, nil
	}
}
