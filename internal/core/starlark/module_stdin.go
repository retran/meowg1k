// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// createStdinModule creates the stdin built-in module.
func (r *Runtime) createStdinModule() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"is_piped":  starlark.NewBuiltin("is_piped", r.stdinIsPiped),
		"read":      starlark.NewBuiltin("read", r.stdinRead),
		"read_line": starlark.NewBuiltin("read_line", r.stdinReadLine),
	})
}

// stdinIsPiped checks if stdin is piped.
func (r *Runtime) stdinIsPiped(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}

	// Check if stdin is a pipe or redirect by examining the file mode
	fi, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat stdin: %w", err)
	}

	// If stdin is not a character device, it's piped/redirected
	isPiped := (fi.Mode() & os.ModeCharDevice) == 0

	return starlark.Bool(isPiped), nil
}

// stdinRead reads all available input from stdin.
func (r *Runtime) stdinRead(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}

	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdin: %w", err)
	}

	return starlark.String(string(content)), nil
}

// stdinReadLine reads a single line from stdin.
func (r *Runtime) stdinReadLine(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}

	reader := r.getStdinReader()
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read line from stdin: %w", err)
	}

	line = strings.TrimSuffix(line, "\n")

	return starlark.String(line), nil
}

// getStdinReader returns a persistent bufio.Reader bound to os.Stdin.
// This ensures successive calls to stdin.read_line() read subsequent lines
// without losing buffered data to a discarded reader instance.
func (r *Runtime) getStdinReader() *bufio.Reader {
	if r.stdinReader == nil {
		r.stdinReader = bufio.NewReader(os.Stdin)
	}
	return r.stdinReader
}
