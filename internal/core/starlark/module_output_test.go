// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

// MockOutputWriter is a test mock for OutputWriter
type MockOutputWriter struct {
	printCalls     []string
	printLineCalls []string
	printfCalls    []printfCall
	streamCalls    []streamCall
	shouldError    bool
}

type printfCall struct {
	format string
	args   []any
}

type streamCall struct {
	delta string
	done  bool
}

func (m *MockOutputWriter) Print(content string) error {
	if m.shouldError {
		return errors.New("mock print error")
	}
	m.printCalls = append(m.printCalls, content)
	return nil
}

func (m *MockOutputWriter) PrintLine(content string) error {
	if m.shouldError {
		return errors.New("mock printline error")
	}
	m.printLineCalls = append(m.printLineCalls, content)
	return nil
}

func (m *MockOutputWriter) Printf(format string, args ...any) error {
	if m.shouldError {
		return errors.New("mock printf error")
	}
	m.printfCalls = append(m.printfCalls, printfCall{format, args})
	return nil
}

func (m *MockOutputWriter) StreamToken(delta string, done bool) {
	m.streamCalls = append(m.streamCalls, streamCall{delta, done})
}

func TestOutputModuleWrite(t *testing.T) {
	t.Run("writes content", func(t *testing.T) {
		mock := &MockOutputWriter{}
		outputModule := NewOutputModule(mock)

		writeFunc := outputModule.Members["write"]
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{starlark.String("hello world")}

		result, err := starlark.Call(thread, writeFunc, args, nil)

		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
		assert.Equal(t, []string{"hello world"}, mock.printCalls)
	})

	t.Run("handles error from writer", func(t *testing.T) {
		mock := &MockOutputWriter{shouldError: true}
		outputModule := NewOutputModule(mock)

		writeFunc := outputModule.Members["write"]
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{starlark.String("test")}

		_, err := starlark.Call(thread, writeFunc, args, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "output.write failed")
	})

	t.Run("errors with missing argument", func(t *testing.T) {
		mock := &MockOutputWriter{}
		outputModule := NewOutputModule(mock)

		writeFunc := outputModule.Members["write"]
		thread := &starlark.Thread{Name: "test"}

		_, err := starlark.Call(thread, writeFunc, starlark.Tuple{}, nil)

		assert.Error(t, err)
	})
}

func TestOutputModuleWriteLine(t *testing.T) {
	t.Run("writes line", func(t *testing.T) {
		mock := &MockOutputWriter{}
		outputModule := NewOutputModule(mock)

		writeLineFunc := outputModule.Members["writeline"]
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{starlark.String("hello world")}

		result, err := starlark.Call(thread, writeLineFunc, args, nil)

		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
		assert.Equal(t, []string{"hello world"}, mock.printLineCalls)
	})

	t.Run("handles error from writer", func(t *testing.T) {
		mock := &MockOutputWriter{shouldError: true}
		outputModule := NewOutputModule(mock)

		writeLineFunc := outputModule.Members["writeline"]
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{starlark.String("test")}

		_, err := starlark.Call(thread, writeLineFunc, args, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "output.writeline failed")
	})

	t.Run("errors with missing argument", func(t *testing.T) {
		mock := &MockOutputWriter{}
		outputModule := NewOutputModule(mock)

		writeLineFunc := outputModule.Members["writeline"]
		thread := &starlark.Thread{Name: "test"}

		_, err := starlark.Call(thread, writeLineFunc, starlark.Tuple{}, nil)

		assert.Error(t, err)
	})
}

func TestOutputModuleWritef(t *testing.T) {
	t.Run("writes formatted string", func(t *testing.T) {
		mock := &MockOutputWriter{}
		outputModule := NewOutputModule(mock)

		writefFunc := outputModule.Members["writef"]
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{
			starlark.String("Hello %s!"),
			starlark.String("World"),
		}

		result, err := starlark.Call(thread, writefFunc, args, nil)

		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
		assert.Equal(t, 1, len(mock.printfCalls))
		assert.Equal(t, "Hello %s!", mock.printfCalls[0].format)
		assert.Equal(t, 1, len(mock.printfCalls[0].args))
	})

	t.Run("handles error from writer", func(t *testing.T) {
		mock := &MockOutputWriter{shouldError: true}
		outputModule := NewOutputModule(mock)

		writefFunc := outputModule.Members["writef"]
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{
			starlark.String("test %s"),
			starlark.String("value"),
		}

		_, err := starlark.Call(thread, writefFunc, args, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "output.writef failed")
	})

	t.Run("errors with missing argument", func(t *testing.T) {
		mock := &MockOutputWriter{}
		outputModule := NewOutputModule(mock)

		writefFunc := outputModule.Members["writef"]
		thread := &starlark.Thread{Name: "test"}

		_, err := starlark.Call(thread, writefFunc, starlark.Tuple{}, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least one argument")
	})

	t.Run("errors with non-string format", func(t *testing.T) {
		mock := &MockOutputWriter{}
		outputModule := NewOutputModule(mock)

		writefFunc := outputModule.Members["writef"]
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{
			starlark.MakeInt(123), // Not a string
		}

		_, err := starlark.Call(thread, writefFunc, args, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a string")
	})

	t.Run("works with no format arguments", func(t *testing.T) {
		mock := &MockOutputWriter{}
		outputModule := NewOutputModule(mock)

		writefFunc := outputModule.Members["writef"]
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{
			starlark.String("just a string"),
		}

		result, err := starlark.Call(thread, writefFunc, args, nil)

		require.NoError(t, err)
		assert.Equal(t, starlark.None, result)
		assert.Equal(t, 1, len(mock.printfCalls))
		assert.Equal(t, "just a string", mock.printfCalls[0].format)
		assert.Equal(t, 0, len(mock.printfCalls[0].args))
	})
}

// TestOutputModuleFunctions verifies all functions are available
func TestOutputModuleFunctions(t *testing.T) {
	mock := &MockOutputWriter{}
	outputModule := NewOutputModule(mock)

	expectedFunctions := []string{
		"write",
		"writeline",
		"writef",
	}

	for _, funcName := range expectedFunctions {
		t.Run(fmt.Sprintf("has_%s", funcName), func(t *testing.T) {
			_, ok := outputModule.Members[funcName]
			assert.True(t, ok, "module should have %s function", funcName)
		})
	}
}

// TestOutputModuleMultipleCalls verifies multiple calls work correctly
func TestOutputModuleMultipleCalls(t *testing.T) {
	mock := &MockOutputWriter{}
	outputModule := NewOutputModule(mock)

	writeFunc := outputModule.Members["write"]
	thread := &starlark.Thread{Name: "test"}

	// Make multiple calls
	for i := 0; i < 3; i++ {
		args := starlark.Tuple{starlark.String(fmt.Sprintf("message %d", i))}
		_, err := starlark.Call(thread, writeFunc, args, nil)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, len(mock.printCalls))
	assert.Equal(t, "message 0", mock.printCalls[0])
	assert.Equal(t, "message 1", mock.printCalls[1])
	assert.Equal(t, "message 2", mock.printCalls[2])
}

// TestOutputModuleUnicode tests unicode handling
func TestOutputModuleUnicode(t *testing.T) {
	mock := &MockOutputWriter{}
	outputModule := NewOutputModule(mock)

	writeFunc := outputModule.Members["write"]
	thread := &starlark.Thread{Name: "test"}
	args := starlark.Tuple{starlark.String("Hello 世界 🌍")}

	result, err := starlark.Call(thread, writeFunc, args, nil)

	require.NoError(t, err)
	assert.Equal(t, starlark.None, result)
	assert.Equal(t, []string{"Hello 世界 🌍"}, mock.printCalls)
}

func TestOutputModuleLongContent(t *testing.T) {
	mock := &MockOutputWriter{}
	outputModule := NewOutputModule(mock)

	writeFunc := outputModule.Members["write"]
	thread := &starlark.Thread{Name: "test"}
	longContent := strings.Repeat("a", 10000)
	args := starlark.Tuple{starlark.String(longContent)}

	result, err := starlark.Call(thread, writeFunc, args, nil)

	require.NoError(t, err)
	assert.Equal(t, starlark.None, result)
	assert.Equal(t, 1, len(mock.printCalls))
	assert.Equal(t, 10000, len(mock.printCalls[0]))
}
