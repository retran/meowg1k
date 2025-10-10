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

package executor

import (
	"context"
	"fmt"
)

type Flusher interface {
	Flush() error
}

// FlowRunner provides a unified way to execute flows with proper tracker and output handling.
type FlowRunner struct {
	flusher Flusher
}

// NewFlowRunner creates a new FlowRunner with the given container.
func NewFlowRunner(flusher Flusher) (*FlowRunner, error) {
	if flusher == nil {
		return nil, fmt.Errorf("flusher is nil")
	}

	return &FlowRunner{flusher: flusher}, nil
}

// RunFlow executes a flow with proper tracker initialization and cleanup.
func (r *FlowRunner) RunFlow(
	ctx context.Context,
	flowName string,
	flow Flow,
	silent bool,
) error {
	if r == nil {
		return fmt.Errorf("flow runner is nil")
	}

	executionTracker := NewExecutionTracker(silent)
	executionTracker.Start()

	exec := NewExecutor().
		WithRetryPolicy(DefaultRetryPolicy()).
		WithFeedbackHandler(executionTracker.FeedbackHandler())

	flowErr := exec.RunFlow(ctx, flowName, flow)

	executionTracker.Stop()

	if flushErr := r.flusher.Flush(); flushErr != nil {
		if flowErr != nil {
			return fmt.Errorf("flow error: %w, flush error: %v", flowErr, flushErr)
		}

		return fmt.Errorf("failed to flush output: %w", flushErr)
	}

	return flowErr
}
