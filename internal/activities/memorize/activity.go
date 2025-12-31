// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package memorize

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	Fact string
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

		flowCtx.SendRunningWithDetails("Memorizing fact", fmt.Sprintf("len=%d", len(input.Fact)))
		s.AddFact(input.Fact)
		flowCtx.SendCompleted("Memorized fact")

		return &Output{Success: true}, nil
	}
}
