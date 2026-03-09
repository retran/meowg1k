// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/retran/meowg1k/internal/ports"
)

// NewOutputModule creates the output module for buffered writing.
func NewOutputModule(writer ports.OutputWriter) *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "output",
		Members: starlark.StringDict{
			"write":     starlark.NewBuiltin("output.write", makeOutputWrite(writer)),
			"writeline": starlark.NewBuiltin("output.writeline", makeOutputWriteLine(writer)),
			"writef":    starlark.NewBuiltin("output.writef", makeOutputWritef(writer)),
		},
	}
}

// makeOutputFunc creates a Starlark function that writes a string using the provided write function.
func makeOutputFunc(name string, write func(string) error) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content string
		if err := starlark.UnpackPositionalArgs(name, args, kwargs, 1, &content); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if err := write(content); err != nil {
			return nil, fmt.Errorf("%s failed: %w", name, err)
		}
		return starlark.None, nil
	}
}

// makeOutputWrite creates the output.write function.
func makeOutputWrite(writer ports.OutputWriter) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return makeOutputFunc("output.write", writer.Print)
}

// makeOutputWriteLine creates the output.writeline function.
func makeOutputWriteLine(writer ports.OutputWriter) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return makeOutputFunc("output.writeline", writer.PrintLine)
}

// makeOutputWritef creates the output.writef function.
func makeOutputWritef(writer ports.OutputWriter) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("output.writef requires at least one argument (format string)")
		}

		formatStr, ok := starlark.AsString(args[0])
		if !ok {
			return nil, fmt.Errorf("output.writef first argument must be a string")
		}

		// Convert remaining args to []any for Printf
		formatArgs := make([]any, len(args)-1)
		for i := 1; i < len(args); i++ {
			formatArgs[i-1] = args[i]
		}

		if err := writer.Printf(formatStr, formatArgs...); err != nil {
			return nil, fmt.Errorf("output.writef failed: %w", err)
		}

		return starlark.None, nil
	}
}
