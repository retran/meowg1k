// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/pkg/executor"
)

type flowBuilder func(container *app.Container) (executor.Flow, error)

// validateInputOrStdin checks if input is provided via args or stdin.
func validateInputOrStdin(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return nil
	}

	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		return nil
	}

	return fmt.Errorf("input required: provide as argument or via stdin")
}

func runFlowCommand(cmd *cobra.Command, flowName string, build flowBuilder) error {
	ctx := cmd.Context()

	container, ok := ctx.Value(app.AppContainerKey).(*app.Container)
	if !ok || container == nil {
		return fmt.Errorf("application not initialized")
	}

	flow, err := build(container)
	if err != nil {
		return fmt.Errorf("failed to create %s flow: %w", flowName, err)
	}

	concurrency := runtime.NumCPU() * 2
	orchestrator, err := executor.NewOrchestrator(container.OutputService, container.TraceLogger, concurrency)
	if err != nil {
		return fmt.Errorf("failed to create flow runner: %w", err)
	}

	silent, err := container.CommandService.GetSilentFlag()
	if err != nil {
		return fmt.Errorf("failed to get command silent flag: %w", err)
	}

	if err := orchestrator.Execute(ctx, flowName, flow, silent); err != nil {
		return fmt.Errorf("failed to execute %s flow: %w", flowName, err)
	}
	return nil
}
