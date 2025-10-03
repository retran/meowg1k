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

func TestNewFlowRunner(t *testing.T) {
	container := &Container{
		Logger: slog.Default(),
	}

	runner := NewFlowRunner(container)

	if runner == nil {
		t.Fatal("NewFlowRunner() returned nil")
	}

	if runner == nil {
		t.Error("NewFlowRunner() did not properly initialize runner")
	}
}
