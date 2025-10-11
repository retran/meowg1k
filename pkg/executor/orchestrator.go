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

// OutputSink defines an interface for flushing output.
type OutputSink interface {
	Flush() error
}

// TraceLogger defines the interface for trace logging feedback.
type TraceLogger interface {
	FeedbackHandler(inner FeedbackHandler) FeedbackHandler
}

// Orchestrator provides a unified way to execute flows with proper tracker and output handling.
type Orchestrator struct {
	outputSink  OutputSink
	traceLogger TraceLogger
}

// NewOrchestrator creates a new FlowRunner with the given container.
// traceLogger can be nil if trace logging is not needed.
func NewOrchestrator(outputSink OutputSink, traceLogger TraceLogger) (*Orchestrator, error) {
	if outputSink == nil {
		return nil, fmt.Errorf("flusher is nil")
	}

	return &Orchestrator{
		outputSink:  outputSink,
		traceLogger: traceLogger,
	}, nil
}

// Execute executes a flow with proper tracker initialization and cleanup.
func (o *Orchestrator) Execute(
	ctx context.Context,
	flowName string,
	flow Flow,
	silent bool,
) error {
	if o == nil {
		return fmt.Errorf("flow runner is nil")
	}

	executionTracker := NewTracker(silent)
	executionTracker.Start()

	// Wrap the feedback handler with trace logging if available
	feedbackHandler := executionTracker.FeedbackHandler()
	if o.traceLogger != nil {
		feedbackHandler = o.traceLogger.FeedbackHandler(feedbackHandler)
	}

	exec := NewExecutor().
		WithRetryPolicy(DefaultRetryPolicy()).
		WithFeedbackHandler(feedbackHandler)

	err := exec.ExecuteFlow(ctx, flowName, flow)

	executionTracker.Stop()

	if flushErr := o.outputSink.Flush(); flushErr != nil {
		if err != nil {
			return fmt.Errorf("flow error: %w, flush error: %v", err, flushErr)
		}

		return fmt.Errorf("failed to flush output: %w", flushErr)
	}

	return err
}
