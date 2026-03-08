// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Note: TTYLogger methods that launch background goroutines (StartSpinner,
// StopSpinner, Action, StartOperation, StartProgress, UpdateProgress,
// FinishProgress) have a known race condition in their goroutine shutdown
// logic and cannot be exercised under -race without triggering a panic.
// Only the non-goroutine methods are tested here.

package progress

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTTYLogger(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	require.NotNil(t, l)
	_ = l.Close()
}

func TestTTYLogger_Thought(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	defer func() { _ = l.Close() }()
	l.Thought("my thought")
	_ = l.Flush()
	assert.Contains(t, buf.String(), "Thought:")
	assert.Contains(t, buf.String(), "my thought")
}

func TestTTYLogger_ActionResult_Success(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	defer func() { _ = l.Close() }()
	l.ActionResult(true, "done", 500*time.Millisecond)
	_ = l.Flush()
	assert.Contains(t, buf.String(), "done")
	assert.Contains(t, buf.String(), symbolSuccess)
}

func TestTTYLogger_ActionResult_Failure(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	defer func() { _ = l.Close() }()
	l.ActionResult(false, "failed", 200*time.Millisecond)
	_ = l.Flush()
	assert.Contains(t, buf.String(), "failed")
	assert.Contains(t, buf.String(), symbolFailure)
}

func TestTTYLogger_CompleteOperation(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	defer func() { _ = l.Close() }()
	l.CompleteOperation("loaded", 300*time.Millisecond)
	_ = l.Flush()
	assert.Contains(t, buf.String(), "loaded")
	assert.Contains(t, buf.String(), symbolSuccess)
}

func TestTTYLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	defer func() { _ = l.Close() }()
	l.Info("some info message")
	_ = l.Flush()
	assert.Contains(t, buf.String(), "some info message")
}

func TestTTYLogger_Success(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	defer func() { _ = l.Close() }()
	l.Success("all done")
	_ = l.Flush()
	assert.Contains(t, buf.String(), "all done")
	assert.Contains(t, buf.String(), symbolSuccess)
}

func TestTTYLogger_Warning(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	defer func() { _ = l.Close() }()
	l.Warning("something is off")
	_ = l.Flush()
	assert.Contains(t, buf.String(), "something is off")
	assert.Contains(t, buf.String(), "Warning:")
}

func TestTTYLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	defer func() { _ = l.Close() }()
	l.Error(errors.New("something broke"))
	_ = l.Flush()
	assert.Contains(t, buf.String(), "something broke")
	assert.Contains(t, buf.String(), symbolFailure)
}

func TestTTYLogger_Flush_NoError(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	defer func() { _ = l.Close() }()
	l.Info("hello")
	err := l.Flush()
	assert.NoError(t, err)
}

func TestTTYLogger_Flush_ReturnsAccumulatedError(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	defer func() { _ = l.Close() }()
	injected := errors.New("injected error")
	l.mu.Lock()
	l.err = injected
	l.mu.Unlock()
	err := l.Flush()
	assert.Equal(t, injected, err)
	// Second call: error should be cleared.
	err2 := l.Flush()
	assert.NoError(t, err2)
}

func TestTTYLogger_Close_NoError(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	l.Info("before close")
	err := l.Close()
	assert.NoError(t, err)
}

func TestTTYLogger_Close_ReturnsAccumulatedError(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	injected := errors.New("close error")
	l.mu.Lock()
	l.err = injected
	l.mu.Unlock()
	err := l.Close()
	assert.Equal(t, injected, err)
}

func TestTTYLogger_MultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	l := NewTTYLogger(&buf)
	defer func() { _ = l.Close() }()
	l.Info("first")
	l.Info("second")
	l.Success("done")
	_ = l.Flush()
	out := buf.String()
	assert.Contains(t, out, "first")
	assert.Contains(t, out, "second")
	assert.Contains(t, out, "done")
}

// TestNoopLogger_AllMethods ensures the noopLogger satisfies the Logger
// interface and none of its methods panic.
func TestNoopLogger_AllMethods(t *testing.T) {
	l := &noopLogger{}
	l.Thought("t")
	l.Action("a", "b")
	l.ActionResult(true, "r", time.Second)
	l.StartOperation("s")
	l.CompleteOperation("c", time.Second)
	l.Info("i")
	l.Success("s")
	l.Warning("w")
	l.Error(errors.New("e"))
	l.StartProgress("p", 10)
	l.UpdateProgress(5, "d")
	l.FinishProgress("f")
	l.StartSpinner("sp")
	l.StopSpinner(true, "done")
	assert.NoError(t, l.Flush())
	assert.NoError(t, l.Close())
}
