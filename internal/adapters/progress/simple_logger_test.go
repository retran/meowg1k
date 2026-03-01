// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSimpleLogger_NilWriter(t *testing.T) {
	// Should not panic; falls back to os.Stderr
	l := NewSimpleLogger(nil)
	require.NotNil(t, l)
}

func TestNewSimpleLogger_WithWriter(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	require.NotNil(t, l)
}

func TestSimpleLogger_Thought(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.Thought("thinking hard")
	assert.Contains(t, buf.String(), "thinking hard")
	assert.Contains(t, buf.String(), "Thought:")
}

func TestSimpleLogger_Action(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.Action("search", `"query"`)
	out := buf.String()
	assert.Contains(t, out, "search")
	assert.Contains(t, out, "Action:")
}

func TestSimpleLogger_ActionResult_Success(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.ActionResult(true, "done", 1500*time.Millisecond)
	out := buf.String()
	assert.Contains(t, out, "done")
	assert.Contains(t, out, "✓")
}

func TestSimpleLogger_ActionResult_Failure(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.ActionResult(false, "failed", 500*time.Millisecond)
	out := buf.String()
	assert.Contains(t, out, "failed")
	assert.Contains(t, out, "✗")
}

func TestSimpleLogger_StartOperation(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.StartOperation("loading")
	assert.Contains(t, buf.String(), "loading")
}

func TestSimpleLogger_CompleteOperation(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.CompleteOperation("loaded", 200*time.Millisecond)
	out := buf.String()
	assert.Contains(t, out, "loaded")
	assert.Contains(t, out, "✓")
}

func TestSimpleLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.Info("some info")
	assert.Contains(t, buf.String(), "some info")
}

func TestSimpleLogger_Success(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.Success("all good")
	out := buf.String()
	assert.Contains(t, out, "all good")
	assert.Contains(t, out, "✓")
}

func TestSimpleLogger_Warning(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.Warning("watch out")
	out := buf.String()
	assert.Contains(t, out, "watch out")
	assert.Contains(t, out, "Warning:")
}

func TestSimpleLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.Error(errors.New("something broke"))
	out := buf.String()
	assert.Contains(t, out, "something broke")
	assert.Contains(t, out, "✗")
}

func TestSimpleLogger_StartProgress(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.StartProgress("files", 10)
	out := buf.String()
	assert.Contains(t, out, "files")
	assert.Contains(t, out, "10")
}

func TestSimpleLogger_UpdateProgress_WithDetail(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.UpdateProgress(3, "file.go")
	assert.Contains(t, buf.String(), "file.go")
}

func TestSimpleLogger_UpdateProgress_NoDetail(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.UpdateProgress(3, "")
	// Nothing should be written for empty detail
	assert.Empty(t, buf.String())
}

func TestSimpleLogger_FinishProgress(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.FinishProgress("complete")
	out := buf.String()
	assert.Contains(t, out, "complete")
	assert.Contains(t, out, "✓")
}

func TestSimpleLogger_StartSpinner(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.StartSpinner("processing")
	assert.Contains(t, buf.String(), "processing")
}

func TestSimpleLogger_StopSpinner_Success(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.StopSpinner(true, "done")
	out := buf.String()
	assert.Contains(t, out, "done")
	assert.Contains(t, out, "✓")
}

func TestSimpleLogger_StopSpinner_Failure(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	l.StopSpinner(false, "aborted")
	out := buf.String()
	assert.Contains(t, out, "aborted")
	assert.Contains(t, out, "✗")
}

func TestSimpleLogger_Flush(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	err := l.Flush()
	assert.NoError(t, err)
}

func TestSimpleLogger_Close(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimpleLogger(&buf)
	err := l.Close()
	assert.NoError(t, err)
}

func TestNew_Silent(t *testing.T) {
	var buf bytes.Buffer
	l := New(true, &buf)
	require.NotNil(t, l)
	// noopLogger — all writes are suppressed
	l.Info("this should not appear")
	assert.Empty(t, buf.String())
}

func TestNew_NonTTY_Writer(t *testing.T) {
	var buf bytes.Buffer
	l := New(false, &buf)
	require.NotNil(t, l)
	// Non-TTY writer → SimpleLogger
	l.Info("hello from simple")
	assert.True(t, strings.Contains(buf.String(), "hello from simple"))
}

func TestNew_NilWriter_NonSilent(t *testing.T) {
	// Should not panic; falls back to os.Stderr
	l := New(false, nil)
	require.NotNil(t, l)
}
