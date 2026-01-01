// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package getdiff

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	Staged bool
}

type Output struct {
	Diff string
}

type Factory struct {
	gitTooling ports.GitToolingService
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory(gitTooling ports.GitToolingService) *Factory {
	return &Factory{gitTooling: gitTooling}
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(_ context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		ref := ""
		desc := "workdir"
		if input.Staged {
			ref = "--staged"
			desc = "staged"
		}

		flowCtx.SendRunningWithDetails("Getting diff", fmt.Sprintf("type=%s", desc))

		diff, err := f.gitTooling.Diff(ref, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get diff: %w", err)
		}

		flowCtx.SendCompletedWithDetails("Got diff", fmt.Sprintf("len=%d", len(diff)))

		return &Output{Diff: diff}, nil
	}
}
