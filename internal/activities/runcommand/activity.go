// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package runcommand

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input for running a command.
type Input struct {
	Command string
	Args    []string
}

// Output defines the output of the command execution.
type Output struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Factory builds runcommand activities.
type Factory struct {
	workspaceService ports.WorkspaceService
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new runcommand activity factory.
func NewFactory(workspaceService ports.WorkspaceService) *Factory {
	return &Factory{
		workspaceService: workspaceService,
	}
}

// NewActivity creates the activity.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, flowCtx *executor.Context, input *Input) (*Output, error) {
		workspaceRoot, err := f.workspaceService.Get()
		if err != nil {
			return nil, fmt.Errorf("failed to get workspace root: %w", err)
		}

		flowCtx.SendRunningWithDetails("Running command", fmt.Sprintf("cmd=%s args=%v", input.Command, input.Args))

		cmd := exec.CommandContext(ctx, input.Command, input.Args...)
		cmd.Dir = workspaceRoot

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				// Command failed to start or was killed
				return nil, fmt.Errorf("failed to run command: %w", err)
			}
		}

		outStr := stdout.String()
		errStr := stderr.String()

		flowCtx.SendCompletedWithDetails("Ran command", fmt.Sprintf("exitCode=%d stdout_len=%d stderr_len=%d", exitCode, len(outStr), len(errStr)))

		return &Output{
			Stdout:   outStr,
			Stderr:   errStr,
			ExitCode: exitCode,
		}, nil
	}
}
