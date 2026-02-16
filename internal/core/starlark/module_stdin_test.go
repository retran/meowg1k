// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"io"
	"os"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// withStdin temporarily replaces os.Stdin for the duration of the function and restores it afterwards.
func withStdin(f *os.File, fn func()) {
	old := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = old }()
	fn()
}

func TestStdinIsPiped_DevNullIsNotPiped(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("failed to open devnull: %v", err)
	}
	defer devnull.Close()

	withStdin(devnull, func() {
		v, err := r.stdinIsPiped(thread, starlark.NewBuiltin("is_piped", r.stdinIsPiped), starlark.Tuple{}, nil)
		if err != nil {
			t.Fatalf("stdinIsPiped error: %v", err)
		}
		if got := bool(v.(starlark.Bool)); got != false {
			t.Fatalf("expected not piped for /dev/null, got %v", got)
		}
	})
}

func TestStdinIsPiped_PipeIsPiped(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	defer pw.Close()
	defer pr.Close()

	withStdin(pr, func() {
		v, err := r.stdinIsPiped(thread, starlark.NewBuiltin("is_piped", r.stdinIsPiped), starlark.Tuple{}, nil)
		if err != nil {
			t.Fatalf("stdinIsPiped error: %v", err)
		}
		if got := bool(v.(starlark.Bool)); got != true {
			t.Fatalf("expected piped for os.Pipe, got %v", got)
		}
	})
}

func TestStdinRead_ReadsAll(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	defer pw.Close()
	defer pr.Close()

	// Write content then close writer to signal EOF
	data := "hello\nworld"
	if _, err := io.WriteString(pw, data); err != nil {
		t.Fatalf("write to pipe error: %v", err)
	}
	pw.Close()

	withStdin(pr, func() {
		v, err := r.stdinRead(thread, starlark.NewBuiltin("read", r.stdinRead), starlark.Tuple{}, nil)
		if err != nil {
			t.Fatalf("stdinRead error: %v", err)
		}
		if got := string(v.(starlark.String)); got != data {
			t.Fatalf("stdinRead mismatch: got %q want %q", got, data)
		}
	})
}

func TestStdinReadLine_ReadsLines(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	defer pw.Close()
	defer pr.Close()

	data := "first line\nsecond line\n"
	if _, err := io.WriteString(pw, data); err != nil {
		t.Fatalf("write to pipe error: %v", err)
	}
	pw.Close()

	withStdin(pr, func() {
		v1, err := r.stdinReadLine(thread, starlark.NewBuiltin("read_line", r.stdinReadLine), starlark.Tuple{}, nil)
		if err != nil {
			t.Fatalf("stdinReadLine #1 error: %v", err)
		}
		if got := string(v1.(starlark.String)); got != "first line" {
			t.Fatalf("stdinReadLine #1 mismatch: got %q want %q", got, "first line")
		}

		v2, err := r.stdinReadLine(thread, starlark.NewBuiltin("read_line", r.stdinReadLine), starlark.Tuple{}, nil)
		if err != nil {
			t.Fatalf("stdinReadLine #2 error: %v", err)
		}
		if got := string(v2.(starlark.String)); got != "second line" {
			t.Fatalf("stdinReadLine #2 mismatch: got %q want %q", got, "second line")
		}
	})
}

// TestStdinIsPipedErrors tests error cases for stdin.is_piped()
func TestStdinIsPipedErrors(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	// Test with unexpected arguments
	args := starlark.Tuple{starlark.String("unexpected")}
	_, err := r.stdinIsPiped(thread, starlark.NewBuiltin("is_piped", r.stdinIsPiped), args, nil)
	if err == nil {
		t.Fatal("expected error for unexpected arguments")
	}
}

// TestStdinReadErrors tests error cases for stdin.read()
func TestStdinReadErrors(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	// Test with unexpected arguments
	args := starlark.Tuple{starlark.String("unexpected")}
	_, err := r.stdinRead(thread, starlark.NewBuiltin("read", r.stdinRead), args, nil)
	if err == nil {
		t.Fatal("expected error for unexpected arguments")
	}
}

// TestStdinReadLineErrors tests error cases for stdin.read_line()
func TestStdinReadLineErrors(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	// Test with unexpected arguments
	args := starlark.Tuple{starlark.String("unexpected")}
	_, err := r.stdinReadLine(thread, starlark.NewBuiltin("read_line", r.stdinReadLine), args, nil)
	if err == nil {
		t.Fatal("expected error for unexpected arguments")
	}
}

// TestStdinReadLineEOF tests reading at EOF
func TestStdinReadLineEOF(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	defer pw.Close()
	defer pr.Close()

	// Write single line without newline and close
	data := "last line"
	if _, err := io.WriteString(pw, data); err != nil {
		t.Fatalf("write to pipe error: %v", err)
	}
	pw.Close()

	withStdin(pr, func() {
		v, err := r.stdinReadLine(thread, starlark.NewBuiltin("read_line", r.stdinReadLine), starlark.Tuple{}, nil)
		if err != nil {
			t.Fatalf("stdinReadLine error: %v", err)
		}
		if got := string(v.(starlark.String)); got != data {
			t.Fatalf("stdinReadLine mismatch: got %q want %q", got, data)
		}
	})
}

// TestStdinReadEmpty tests reading empty input
func TestStdinReadEmpty(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	pw.Close() // Close immediately to signal EOF
	defer pr.Close()

	withStdin(pr, func() {
		v, err := r.stdinRead(thread, starlark.NewBuiltin("read", r.stdinRead), starlark.Tuple{}, nil)
		if err != nil {
			t.Fatalf("stdinRead error: %v", err)
		}
		if got := string(v.(starlark.String)); got != "" {
			t.Fatalf("stdinRead mismatch: got %q want empty string", got)
		}
	})
}

// TestStdinReadLineEmpty tests reading empty line
func TestStdinReadLineEmpty(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	defer pw.Close()
	defer pr.Close()

	// Write newline only
	if _, err := io.WriteString(pw, "\n"); err != nil {
		t.Fatalf("write to pipe error: %v", err)
	}
	pw.Close()

	withStdin(pr, func() {
		v, err := r.stdinReadLine(thread, starlark.NewBuiltin("read_line", r.stdinReadLine), starlark.Tuple{}, nil)
		if err != nil {
			t.Fatalf("stdinReadLine error: %v", err)
		}
		if got := string(v.(starlark.String)); got != "" {
			t.Fatalf("stdinReadLine mismatch: got %q want empty string", got)
		}
	})
}

// TestStdinModuleCreation tests createStdinModule function
func TestStdinModuleCreation(t *testing.T) {
	r := NewRuntime("/tmp")
	module := r.createStdinModule()

	if module == nil {
		t.Fatal("module is nil")
	}

	// Test module is a struct
	moduleStruct, ok := module.(*starlarkstruct.Struct)
	if !ok {
		t.Fatal("module is not a struct")
	}

	// Verify all expected functions exist
	isPiped, err := moduleStruct.Attr("is_piped")
	if err != nil || isPiped == nil {
		t.Fatalf("is_piped function not found: %v", err)
	}

	read, err := moduleStruct.Attr("read")
	if err != nil || read == nil {
		t.Fatalf("read function not found: %v", err)
	}

	readLine, err := moduleStruct.Attr("read_line")
	if err != nil || readLine == nil {
		t.Fatalf("read_line function not found: %v", err)
	}
}

// TestGetStdinReader tests the getStdinReader helper
func TestGetStdinReader(t *testing.T) {
	r := &Runtime{}

	// First call should create reader
	reader1 := r.getStdinReader()
	if reader1 == nil {
		t.Fatal("getStdinReader returned nil")
	}

	// Second call should return same reader
	reader2 := r.getStdinReader()
	if reader2 != reader1 {
		t.Fatal("getStdinReader should return the same reader instance")
	}
}

// TestStdinReadMultipleLines tests reading multiple lines
func TestStdinReadMultipleLines(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	defer pw.Close()
	defer pr.Close()

	lines := []string{"line 1", "line 2", "line 3"}
	for _, line := range lines {
		if _, err := io.WriteString(pw, line+"\n"); err != nil {
			t.Fatalf("write to pipe error: %v", err)
		}
	}
	pw.Close()

	withStdin(pr, func() {
		for i, expected := range lines {
			v, err := r.stdinReadLine(thread, starlark.NewBuiltin("read_line", r.stdinReadLine), starlark.Tuple{}, nil)
			if err != nil {
				t.Fatalf("stdinReadLine #%d error: %v", i+1, err)
			}
			if got := string(v.(starlark.String)); got != expected {
				t.Fatalf("stdinReadLine #%d mismatch: got %q want %q", i+1, got, expected)
			}
		}
	})
}

// TestStdinReadUnicode tests reading Unicode content
func TestStdinReadUnicode(t *testing.T) {
	r := &Runtime{}
	thread := &starlark.Thread{Name: "test"}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	defer pw.Close()
	defer pr.Close()

	data := "Hello, 世界! 🌍\n"
	if _, err := io.WriteString(pw, data); err != nil {
		t.Fatalf("write to pipe error: %v", err)
	}
	pw.Close()

	withStdin(pr, func() {
		v, err := r.stdinReadLine(thread, starlark.NewBuiltin("read_line", r.stdinReadLine), starlark.Tuple{}, nil)
		if err != nil {
			t.Fatalf("stdinReadLine error: %v", err)
		}
		expected := "Hello, 世界! 🌍"
		if got := string(v.(starlark.String)); got != expected {
			t.Fatalf("stdinReadLine mismatch: got %q want %q", got, expected)
		}
	})
}
