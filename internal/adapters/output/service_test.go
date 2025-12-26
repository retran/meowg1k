// Copyright © 2025 The meowg1k Authors
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

func TestPrint(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{destination: &buf}

	service.Print("hello")
	service.Print(" world")

	err := service.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	if buf.String() != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", buf.String())
	}
}

func TestPrintLine(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{destination: &buf}

	service.PrintLine("hello")
	service.PrintLine("world")

	err := service.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	expected := "hello\nworld\n"
	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}

func TestPrintf(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{destination: &buf}

	service.Printf("count: %d", 42)
	service.Printf(" name: %s", "test")

	err := service.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	if buf.String() != "count: 42 name: test" {
		t.Errorf("Expected 'count: 42 name: test', got '%s'", buf.String())
	}
}

func TestFlushEmpty(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{destination: &buf}

	err := service.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("Expected empty buffer, got %d bytes", buf.Len())
	}
}

func TestFlushMultipleTimes(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{destination: &buf}

	service.Print("first")
	err := service.Flush()
	if err != nil {
		t.Errorf("First flush failed: %v", err)
	}

	service.Print("second")
	err = service.Flush()
	if err != nil {
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
	expectedMsg := outputServiceNilMessage
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestPrintLine_NilService(t *testing.T) {
	var service *Service
	err := service.PrintLine("test")
	if err == nil {
		t.Fatal("expected error for nil service, got nil")
	}
	expectedMsg := outputServiceNilMessage
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestPrintf_NilService(t *testing.T) {
	var service *Service
	err := service.Printf("test %d", 42)
	if err == nil {
		t.Fatal("expected error for nil service, got nil")
	}
	expectedMsg := outputServiceNilMessage
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestFlush_NilService(t *testing.T) {
	var service *Service
	err := service.Flush()
	if err == nil {
		t.Fatal("expected error for nil service, got nil")
	}
	expectedMsg := outputServiceNilMessage
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}
