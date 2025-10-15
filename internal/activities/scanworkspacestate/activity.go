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

// Package scanworkspacestate implements an activity that scans workspace state (working directory, stage, or HEAD) for files.
package scanworkspacestate

import (
	"context"
	"fmt"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Output struct {
	HeadState    map[string]domainindex.FileState
	StageState   map[string]domainindex.FileState
	WorkdirState map[string]domainindex.FileState
}

type Factory struct {
	projectStateSvc ports.ProjectStateService
}

var _ executor.ActivityFactory[struct{}, *Output] = (*Factory)(nil)

func NewFactory(projectStateSvc ports.ProjectStateService) (executor.ActivityFactory[struct{}, *Output], error) {
	if projectStateSvc == nil {
		return nil, fmt.Errorf("scanworkspacestate.NewFactory: projectStateSvc cannot be nil")
	}

	return &Factory{
		projectStateSvc: projectStateSvc,
	}, nil
}

func (f *Factory) NewActivity() executor.Activity[struct{}, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, _ struct{}) (*Output, error) {
		executorCtx.SendRunning("Scanning workspace")

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

		executorCtx.SendCompleted("Scanned workspace")
		return result, nil
	}
}
