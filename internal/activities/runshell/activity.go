// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package runshell implements an activity for executing shell commands.
package runshell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

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

// Factory builds runshell activities.
type Factory struct {
	workspaceService ports.WorkspaceService
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new runshell activity factory.
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

		cmdLine := input.Command
		if len(input.Args) > 0 {
			cmdLine = cmdLine + " " + strings.Join(input.Args, " ")
		}
		flowCtx.SendRunning(fmt.Sprintf("Running: %s", cmdLine))

		cmd := exec.CommandContext(ctx, input.Command, input.Args...) // #nosec G204
		cmd.Dir = workspaceRoot

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		exitCode := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				return nil, fmt.Errorf("failed to run command: %w", err)
			}
		}

		outStr := stdout.String()
		errStr := stderr.String()

		if exitCode == 0 {
			flowCtx.SendCompleted(fmt.Sprintf("Ran: %s", cmdLine))
		} else {
			flowCtx.SendCompleted(fmt.Sprintf("Ran: %s (exit %d)", cmdLine, exitCode))
		}

		return &Output{
			Stdout:   outStr,
			Stderr:   errStr,
			ExitCode: exitCode,
		}, nil
	}
}
