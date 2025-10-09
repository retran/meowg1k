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

package app

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/services/command"
	"github.com/retran/meowg1k/internal/services/output"
	"github.com/retran/meowg1k/pkg/executor"
)

func mockFlow(err error) executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		return err
	}
}

func TestFlowRunner_RunFlow(t *testing.T) {
	tests := []struct {
		name      string
		flowError error
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
			wantError: true,
			silent:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{
				Use: "test",
			}
			cmd.Flags().Bool("silent", tt.silent, "silent flag")

			commandService, err := command.NewService(cmd)
			if err != nil {
				t.Fatalf("Failed to create command service: %v", err)
			}

			container := &Container{
				Logger:         slog.Default(),
				CommandService: commandService,
				OutputService:  output.NewService(output.Stdout),
			}

			runner := NewFlowRunner(container)

			flow := mockFlow(tt.flowError)

			err = runner.RunFlow(context.Background(), "TestFlow", flow)

			if (err != nil) != tt.wantError {
				t.Errorf("RunFlow() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// mockOutputService is a mock implementation of output.Service for testing
func TestNewFlowRunner(t *testing.T) {
	container := &Container{
		Logger: slog.Default(),
	}

	runner := NewFlowRunner(container)

	if runner == nil {
		t.Fatal("NewFlowRunner() returned nil")
	}

	if runner.container != container {
		t.Error("NewFlowRunner() did not properly initialize runner")
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
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("silent", false, "silent flag") // Not silent so Flush() is called

	commandService, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Create a mock output writer that will return an error on Flush
	mockOutput := &mockOutputWriter{
		flushError: context.Canceled,
	}

	container := &Container{
		Logger:         slog.Default(),
		CommandService: commandService,
		OutputService:  mockOutput,
	}

	runner := NewFlowRunner(container)

	// Use a flow that succeeds, but flush will fail
	flow := mockFlow(nil)

	err = runner.RunFlow(context.Background(), "TestFlow", flow)

	if err == nil {
		t.Error("RunFlow() expected error from Flush, got nil")
	}

	// Error is wrapped, so we check with errors.Is
	if !errors.Is(err, context.Canceled) {
		t.Errorf("RunFlow() expected wrapped flush error (context.Canceled), got: %v", err)
	}
}

func TestFlowRunner_RunFlowWithBothErrors(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("silent", true, "silent flag")

	commandService, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Create an output service for testing
	mockOutput := output.NewService(output.Discard)

	container := &Container{
		Logger:         slog.Default(),
		CommandService: commandService,
		OutputService:  mockOutput,
	}

	runner := NewFlowRunner(container)

	// Use a flow that fails
	flow := mockFlow(context.Canceled)

	err = runner.RunFlow(context.Background(), "TestFlow", flow)

	if err == nil {
		t.Error("RunFlow() expected error, got nil")
	}
}

func TestFlowRunner_RunFlowWithGetSilentFlagError(t *testing.T) {
	// Create a command without the silent flag defined
	cmd := &cobra.Command{
		Use: "test",
	}
	// Don't define the silent flag to cause GetSilentFlag to fail

	commandService, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	container := &Container{
		Logger:         slog.Default(),
		CommandService: commandService,
		OutputService:  output.NewService(output.Stdout),
	}

	runner := NewFlowRunner(container)

	flow := mockFlow(nil)

	err = runner.RunFlow(context.Background(), "TestFlow", flow)

	if err == nil {
		t.Error("RunFlow() expected error from GetSilentFlag, got nil")
	}
}
