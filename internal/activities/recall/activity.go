// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package recall

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	Query string
}

type Output struct {
	Facts []string
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

		flowCtx.SendRunningWithDetails("Recalling facts", fmt.Sprintf("query=%s", input.Query))
		facts := s.SearchFacts(input.Query)

		factStrings := make([]string, len(facts))
		for i, f := range facts {
			factStrings[i] = f.Content
		}

		flowCtx.SendCompletedWithDetails("Recalled facts", fmt.Sprintf("count=%d", len(facts)))

		return &Output{Facts: factStrings}, nil
	}
}
