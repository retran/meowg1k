// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// createShellModule creates the shell built-in module.
func (r *Runtime) createShellModule() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"exec": starlark.NewBuiltin("exec", r.shellExec),
	})
}

// shellExec implements shell.exec().
func (r *Runtime) shellExec(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var command string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "command", &command); err != nil {
		return nil, fmt.Errorf("shell.exec: %w", err)
	}

	cmd := exec.CommandContext(context.Background(), "sh", "-c", command) //nolint:gosec // user-provided shell command is intentional
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to execute command: %w", err)
		}
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"stdout":    starlark.String(stdout.String()),
		"stderr":    starlark.String(stderr.String()),
		"exit_code": starlark.MakeInt(exitCode),
	}), nil
}
