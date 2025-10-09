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

// Package app contains the main application struct and orchestrates cross-cutting services.
package app

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/ui"
)

// FlowRunner provides a unified way to execute flows with proper tracker and output handling.
type FlowRunner struct {
	container *Container
}

// NewFlowRunner creates a new FlowRunner with the given container.
func NewFlowRunner(container *Container) (*FlowRunner, error) {
	if container == nil {
		return nil, fmt.Errorf("container cannot be nil")
	}

	return &FlowRunner{container: container}, nil
}

// RunFlow executes a flow with proper tracker initialization and cleanup.
func (r *FlowRunner) RunFlow(
	ctx context.Context,
	flowName string,
	flow executor.Flow,
) error {
	if r == nil {
		return fmt.Errorf("flow runner is nil")
	}

	silent, err := r.container.CommandService.GetSilentFlag()
	if err != nil {
		return fmt.Errorf("failed to get silent flag: %w", err)
	}

	executionTracker := ui.NewExecutionTracker(silent)
	executionTracker.Start()

	exec := executor.NewExecutor().
		WithRetryPolicy(executor.DefaultRetryPolicy()).
		WithFeedbackHandler(executionTracker.FeedbackHandler())

	flowErr := exec.RunFlow(ctx, flowName, flow)

	executionTracker.Stop()

	if flushErr := r.container.OutputService.Flush(); flushErr != nil {
		if flowErr != nil {
			return fmt.Errorf("flow error: %w, flush error: %v", flowErr, flushErr)
		}

		return fmt.Errorf("failed to flush output: %w", flushErr)
	}

	return flowErr
}
