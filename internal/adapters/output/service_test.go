// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"bytes"
	"testing"

	"github.com/retran/meowg1k/internal/domain/output"
)

const outputServiceNilMessage = "output service is nil"

func TestNewServiceStdout(t *testing.T) {
	service := NewService(output.Stdout)
	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestNewServiceStderr(t *testing.T) {
	service := NewService(output.Stderr)
	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestNewServiceDiscard(t *testing.T) {
	service := NewService(output.Discard)
	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestNewServiceDefault(t *testing.T) {
	service := NewService("unknown")
	if service == nil {
		t.Error("NewService returned nil")
	}
}

// newTestService creates a non-TTY service backed by a buffer for unit tests.
func newTestService(buf *bytes.Buffer) *Service {
	return &Service{destination: buf, isTerminal: false}
}

func TestPrint(t *testing.T) {
	var buf bytes.Buffer
	service := newTestService(&buf)

	_ = service.Print("hello")
	_ = service.Print(" world")

	if err := service.Flush(); err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	if buf.String() != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", buf.String())
	}
}

func TestPrintLine(t *testing.T) {
	var buf bytes.Buffer
	service := newTestService(&buf)

	_ = service.PrintLine("hello")
	_ = service.PrintLine("world")

	if err := service.Flush(); err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	expected := "hello\nworld\n"
	if buf.String() != expected {
		t.Errorf("Expected %q, got %q", expected, buf.String())
	}
}

func TestPrintf(t *testing.T) {
	var buf bytes.Buffer
	service := newTestService(&buf)

	_ = service.Printf("count: %d", 42)
	_ = service.Printf(" name: %s", "test")

	if err := service.Flush(); err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	if buf.String() != "count: 42 name: test" {
		t.Errorf("Expected 'count: 42 name: test', got '%s'", buf.String())
	}
}

// TestStreamToken_NonTerminal verifies that StreamToken is a no-op on non-TTY
// (isTerminal=false), so the buffer stays empty and no panic occurs.
func TestStreamToken_NonTerminal(t *testing.T) {
	var buf bytes.Buffer
	service := newTestService(&buf)

	// Should be a no-op on non-TTY
	service.StreamToken("Hello", false)
	service.StreamToken(" world", true)

	if err := service.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Non-TTY: StreamToken is a no-op, nothing written to buf
	if buf.Len() != 0 {
		t.Errorf("Expected empty buffer on non-TTY, got %q", buf.String())
	}
}

func TestFlushEmpty(t *testing.T) {
	var buf bytes.Buffer
	service := newTestService(&buf)

	if err := service.Flush(); err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("Expected empty buffer, got %d bytes", buf.Len())
	}
}

func TestFlushMultipleTimes(t *testing.T) {
	var buf bytes.Buffer
	service := newTestService(&buf)

	_ = service.Print("first")
	if err := service.Flush(); err != nil {
		t.Errorf("First flush failed: %v", err)
	}

	_ = service.Print("second")
	if err := service.Flush(); err != nil {
		t.Errorf("Second flush failed: %v", err)
	}

	if buf.String() != "firstsecond" {
		t.Errorf("Expected 'firstsecond', got '%s'", buf.String())
	}
}

func TestPrint_NilService(t *testing.T) {
	var service *Service
	err := service.Print("test")
	if err == nil {
		t.Fatal("expected error for nil service, got nil")
	}
	if err.Error() != outputServiceNilMessage {
		t.Errorf("expected %q, got %q", outputServiceNilMessage, err.Error())
	}
}

func TestPrintLine_NilService(t *testing.T) {
	var service *Service
	err := service.PrintLine("test")
	if err == nil {
		t.Fatal("expected error for nil service, got nil")
	}
	if err.Error() != outputServiceNilMessage {
		t.Errorf("expected %q, got %q", outputServiceNilMessage, err.Error())
	}
}

func TestPrintf_NilService(t *testing.T) {
	var service *Service
	err := service.Printf("test %d", 42)
	if err == nil {
		t.Fatal("expected error for nil service, got nil")
	}
	if err.Error() != outputServiceNilMessage {
		t.Errorf("expected %q, got %q", outputServiceNilMessage, err.Error())
	}
}

func TestFlush_NilService(t *testing.T) {
	var service *Service
	err := service.Flush()
	if err == nil {
		t.Fatal("expected error for nil service, got nil")
	}
	if err.Error() != outputServiceNilMessage {
		t.Errorf("expected %q, got %q", outputServiceNilMessage, err.Error())
	}
}

// TestNewServiceWithOptions tests creating service with options.
func TestNewServiceWithOptions(t *testing.T) {
	t.Run("stdout with plain output", func(t *testing.T) {
		service := NewServiceWithOptions(output.Stdout, true, false)
		if service == nil {
			t.Fatal("Expected non-nil service")
		}
		if !service.plainOutput {
			t.Error("Expected plainOutput to be true")
		}
		if service.noColor {
			t.Error("Expected noColor to be false")
		}
	})

	t.Run("stderr with no color", func(t *testing.T) {
		service := NewServiceWithOptions(output.Stderr, false, true)
		if service == nil {
			t.Fatal("Expected non-nil service")
		}
		if service.plainOutput {
			t.Error("Expected plainOutput to be false")
		}
		if !service.noColor {
			t.Error("Expected noColor to be true")
		}
	})

	t.Run("discard with both options", func(t *testing.T) {
		service := NewServiceWithOptions(output.Discard, true, true)
		if service == nil {
			t.Fatal("Expected non-nil service")
		}
		if !service.plainOutput {
			t.Error("Expected plainOutput to be true")
		}
		if !service.noColor {
			t.Error("Expected noColor to be true")
		}
	})

	t.Run("unknown destination defaults to discard", func(t *testing.T) {
		service := NewServiceWithOptions("invalid", false, false)
		if service == nil {
			t.Fatal("Expected non-nil service")
		}
	})
}

// ---------------------------------------------------------------------------
// Nil-receiver guard tests for TurnWriter methods
// ---------------------------------------------------------------------------

func TestSendHeader_NilService(t *testing.T) {
	var s *Service
	// Should not panic
	s.SendHeader("hello")
}

func TestBeginUserTurn_NilService(t *testing.T) {
	var s *Service
	s.BeginUserTurn("prompt")
}

func TestBeginAssistantTurn_NilService(t *testing.T) {
	var s *Service
	s.BeginAssistantTurn()
}

func TestOpenStep_NilService(t *testing.T) {
	var s *Service
	id := s.OpenStep("step text")
	if id != "" {
		t.Errorf("expected empty id for nil service, got %q", id)
	}
}

func TestUpdateStep_NilService(t *testing.T) {
	var s *Service
	s.UpdateStep("id", "new text")
}

func TestAddStepInfo_NilService(t *testing.T) {
	var s *Service
	s.AddStepInfo("id", "info")
}

func TestCloseStep_NilService(t *testing.T) {
	var s *Service
	s.CloseStep("id", true, "summary")
}

func TestBeginSubTurn_NilService(t *testing.T) {
	var s *Service
	s.BeginSubTurn("label")
}

func TestEndSubTurn_NilService(t *testing.T) {
	var s *Service
	s.EndSubTurn()
}

func TestEndTurn_NilService(t *testing.T) {
	var s *Service
	s.EndTurn("summary")
}

func TestSetStatus_NilService(t *testing.T) {
	var s *Service
	s.SetStatus("status")
}

func TestSetCancel_NilService(t *testing.T) {
	var s *Service
	s.SetCancel(func() {})
}

// ---------------------------------------------------------------------------
// Non-TTY path tests (isTerminal=false so tuiActive()==false)
// ---------------------------------------------------------------------------

func TestSendHeader_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	// Non-TTY: no-op, should not panic
	svc.SendHeader("header text")
}

func TestBeginUserTurn_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	svc.BeginUserTurn("user turn")
}

func TestBeginAssistantTurn_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	svc.BeginAssistantTurn()
}

func TestOpenStep_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	id := svc.OpenStep("step")
	if id != "" {
		t.Errorf("expected empty id on non-TTY, got %q", id)
	}
}

func TestUpdateStep_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	svc.UpdateStep("id", "text")
}

func TestUpdateStep_EmptyID_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	svc.UpdateStep("", "text")
}

func TestAddStepInfo_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	svc.AddStepInfo("id", "info")
}

func TestCloseStep_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	svc.CloseStep("id", true, "ok")
}

func TestBeginSubTurn_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	svc.BeginSubTurn("label")
}

func TestEndSubTurn_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	svc.EndSubTurn()
}

func TestEndTurn_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	svc.EndTurn("summary")
}

func TestSetStatus_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	svc.SetStatus("loading...")
}

func TestSetCancel_NonNil(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf}
	called := false
	svc.SetCancel(func() { called = true })
	if svc.cancel == nil {
		t.Fatal("expected cancel to be set")
	}
	svc.cancel()
	if !called {
		t.Fatal("expected cancel to be called")
	}
}

func TestIsTTY_False(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	if svc.IsTTY() {
		t.Fatal("expected IsTTY to be false")
	}
}

func TestIsTTY_NilService(t *testing.T) {
	var svc *Service
	if svc.IsTTY() {
		t.Fatal("expected IsTTY to be false for nil service")
	}
}

func TestLogWriter_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	svc := &Service{destination: &buf, isTerminal: false}
	w := svc.LogWriter()
	if w == nil {
		t.Fatal("expected non-nil writer")
	}
	// On non-TTY, LogWriter returns destination directly
	_, err := w.Write([]byte("test output"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
}

func TestLogWriter_NilService(t *testing.T) {
	var svc *Service
	w := svc.LogWriter()
	if w == nil {
		t.Fatal("expected non-nil writer (should be io.Discard)")
	}
}

func TestNextStepID_Increments(t *testing.T) {
	a := nextStepID()
	b := nextStepID()
	if b <= a {
		t.Errorf("expected nextStepID to increment: got a=%d b=%d", a, b)
	}
}
