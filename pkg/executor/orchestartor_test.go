// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/retran/meowg1k/internal/adapters/output"
	domainOutput "github.com/retran/meowg1k/internal/domain/output"
)

func mockFlow(err error) Flow {
	return func(ctx context.Context, flowCtx *Context) error {
		return err
	}
}

func TestFlowRunner_RunFlow(t *testing.T) {
	tests := []struct {
		flowError error
		name      string
		wantError bool
		silent    bool
	}{
		{
			name:      "successful flow execution",
			flowError: nil,
			wantError: false,
			silent:    true,
		},
		{
			name:      "successful flow execution with output",
			flowError: nil,
			wantError: false,
			silent:    false,
		},
		{
			name:      "flow execution with error",
			flowError: context.DeadlineExceeded,
			wantError: false, // Errors are reported via tracker and suppressed
			silent:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, err := NewOrchestrator(output.NewService(domainOutput.Stdout), nil, 0)
			if err != nil {
				t.Fatalf("NewFlowRunner() returned error: %v", err)
			}

			flow := mockFlow(tt.flowError)

			err = runner.Execute(context.Background(), "TestFlow", flow, false)
			if (err != nil) != tt.wantError {
				t.Errorf("RunFlow() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// mockOutputService is a mock implementation of output.Service for testing.
func TestNewFlowRunner(t *testing.T) {
	runner, err := NewOrchestrator(output.NewService(domainOutput.Stdout), nil, 0)
	if err != nil {
		t.Fatalf("NewFlowRunner() returned error: %v", err)
	}

	if runner == nil {
		t.Fatal("NewFlowRunner() returned nil")
	}
}

func TestNewFlowRunnerNil(t *testing.T) {
	runner, err := NewOrchestrator(nil, nil, 0)
	if err == nil {
		t.Error("Expected error when NewFlowRunner called with nil")
	}
	if runner != nil {
		t.Error("Expected nil runner when error returned")
	}
}

// mockOutputWriter is a mock implementation of output.Writer for testing that can return errors on Flush.
type mockOutputWriter struct {
	flushError error
}

func (m *mockOutputWriter) Print(content string) error {
	// No-op
	return nil
}

func (m *mockOutputWriter) PrintLine(content string) error {
	// No-op
	return nil
}

func (m *mockOutputWriter) Printf(format string, args ...any) error {
	// No-op
	return nil
}

func (m *mockOutputWriter) Flush() error {
	return m.flushError
}

func TestFlowRunner_RunFlowWithFlushError(t *testing.T) {
	// Create a mock output writer that will return an error on Flush
	mockOutput := &mockOutputWriter{
		flushError: context.Canceled,
	}

	runner, err := NewOrchestrator(mockOutput, nil, 0)
	if err != nil {
		t.Fatalf("NewFlowRunner() returned error: %v", err)
	}

	// Use a flow that succeeds, but flush will fail
	flow := mockFlow(nil)

	err = runner.Execute(context.Background(), "TestFlow", flow, false)

	if err == nil {
		t.Error("RunFlow() expected error from Flush, got nil")
	}

	// Error is wrapped, so we check with errors.Is
	if !errors.Is(err, context.Canceled) {
		t.Errorf("RunFlow() expected wrapped flush error (context.Canceled), got: %v", err)
	}
}

func TestFlowRunner_RunFlowWithBothErrors(t *testing.T) {
	// Create an output service for testing
	mockOutput := output.NewService(domainOutput.Discard)

	runner, err := NewOrchestrator(mockOutput, nil, 0)
	if err != nil {
		t.Fatalf("NewFlowRunner() returned error: %v", err)
	}

	// Use a flow that fails
	flow := mockFlow(context.Canceled)

	err = runner.Execute(context.Background(), "TestFlow", flow, false)

	// Flow errors are now reported via tracker and suppressed
	if err != nil {
		t.Errorf("RunFlow() expected nil (error reported via tracker), got: %v", err)
	}
}
