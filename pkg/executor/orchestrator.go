// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

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
	concurrency int
}

// NewOrchestrator creates a new FlowRunner with the given container.
// traceLogger can be nil if trace logging is not needed.
// concurrency limits the number of in-flight activities (0 means no limit).
func NewOrchestrator(outputSink OutputSink, traceLogger TraceLogger, concurrency int) (*Orchestrator, error) {
	if outputSink == nil {
		return nil, fmt.Errorf("flusher is nil")
	}

	return &Orchestrator{
		outputSink:  outputSink,
		traceLogger: traceLogger,
		concurrency: concurrency,
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

	exec := NewExecutor(o.concurrency).
		WithRetryPolicy(DefaultRetryPolicy()).
		WithFeedbackHandler(feedbackHandler)

	err := exec.ExecuteFlow(ctx, flowName, flow)
	if err != nil {
		err = fmt.Errorf("failed to execute flow: %w", err)
	}

	executionTracker.Stop()

	if flushErr := o.outputSink.Flush(); flushErr != nil {
		if err != nil {
			return fmt.Errorf("flow error: %w, flush error: %w", err, flushErr)
		}

		return fmt.Errorf("failed to flush output: %w", flushErr)
	}

	return err
}
