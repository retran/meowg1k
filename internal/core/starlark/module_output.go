// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// OutputWriter is the interface for buffered plain-text output.
// It is a simple write buffer; no markdown rendering or streaming preview.
// StreamToken is used by the ui module (ui.stream) for live TUI preview.
type OutputWriter interface {
	Print(content string) error
	PrintLine(content string) error
	Printf(format string, args ...any) error
	// StreamToken forwards a token delta to the TUI StreamBlock on TTY.
	// On non-TTY implementations this is a no-op.
	StreamToken(delta string, done bool)
}

// NewOutputModule creates the output module for buffered writing.
func NewOutputModule(writer OutputWriter) *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "output",
		Members: starlark.StringDict{
			"write":     starlark.NewBuiltin("output.write", makeOutputWrite(writer)),
			"writeline": starlark.NewBuiltin("output.writeline", makeOutputWriteLine(writer)),
			"writef":    starlark.NewBuiltin("output.writef", makeOutputWritef(writer)),
		},
	}
}

// makeOutputWrite creates the output.write function
func makeOutputWrite(writer OutputWriter) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content string
		if err := starlark.UnpackPositionalArgs("output.write", args, kwargs, 1, &content); err != nil {
			return nil, err
		}

		if err := writer.Print(content); err != nil {
			return nil, fmt.Errorf("output.write failed: %w", err)
		}

		return starlark.None, nil
	}
}

// makeOutputWriteLine creates the output.writeline function
func makeOutputWriteLine(writer OutputWriter) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var content string
		if err := starlark.UnpackPositionalArgs("output.writeline", args, kwargs, 1, &content); err != nil {
			return nil, err
		}

		if err := writer.PrintLine(content); err != nil {
			return nil, fmt.Errorf("output.writeline failed: %w", err)
		}

		return starlark.None, nil
	}
}

// makeOutputWritef creates the output.writef function
func makeOutputWritef(writer OutputWriter) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
