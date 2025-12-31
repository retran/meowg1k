// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package listfiles

import (
	"context"
	"fmt"
	"sort"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct{}

type Output struct {
	Files []string
}

type Factory struct {
	projectStateSvc ports.ProjectStateService
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory(projectStateSvc ports.ProjectStateService) *Factory {
	return &Factory{projectStateSvc: projectStateSvc}
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		flowCtx.SendRunning("Listing files")

		state, err := f.projectStateSvc.GetWorkdirState(ctx)
		if err != nil {
			return nil, err
		}

		files := make([]string, 0, len(state))
		for path := range state {
			files = append(files, path)
		}
		sort.Strings(files)

		flowCtx.SendCompleted(fmt.Sprintf("Listed %d files", len(files)))

		return &Output{Files: files}, nil
	}
}
