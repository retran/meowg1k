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
type mockOutputService struct {
	flushError error
}

func (m *mockOutputService) Print(content string)              {}
func (m *mockOutputService) PrintLine(content string)          {}
func (m *mockOutputService) Printf(format string, args ...any) {}
func (m *mockOutputService) Flush() error                      { return m.flushError }

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

func TestFlowRunner_RunFlowWithFlushError(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("silent", true, "silent flag")

	commandService, err := command.NewService(cmd)
	if err != nil {
		t.Fatalf("Failed to create command service: %v", err)
	}

	// Create a mock output service that will fail on flush
	mockOutput := &mockOutputService{
		flushError: context.DeadlineExceeded,
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
		t.Error("RunFlow() expected error from flush, got nil")
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

	// Create a mock output service that will fail on flush
	mockOutput := &mockOutputService{
		flushError: context.DeadlineExceeded,
	}

	container := &Container{
		Logger:         slog.Default(),
		CommandService: commandService,
		OutputService:  mockOutput,
	}

	runner := NewFlowRunner(container)

	// Use a flow that also fails
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
