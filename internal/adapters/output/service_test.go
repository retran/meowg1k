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

func TestPrintMarkdown(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{destination: &buf}

	err := service.PrintMarkdown("# Title\n\nBody")
	if err != nil {
		t.Fatalf("PrintMarkdown failed: %v", err)
	}

	err = service.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Expected rendered markdown output, got empty buffer")
	}
}

func TestStreamMarkdown(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{
		destination: &buf,
		isTerminal:  true, // Emulate terminal for this test
	}

	err := service.StreamMarkdown("Hello", false)
	if err != nil {
		t.Fatalf("StreamMarkdown failed: %v", err)
	}
	err = service.StreamMarkdown(" world", true)
	if err != nil {
		t.Fatalf("StreamMarkdown failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Expected streamed markdown output, got empty buffer")
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

func TestPrintMarkdown_NilService(t *testing.T) {
	var service *Service
	err := service.PrintMarkdown("test")
	if err == nil {
		t.Fatal("expected error for nil service, got nil")
	}
	expectedMsg := outputServiceNilMessage
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestStreamMarkdown_NilService(t *testing.T) {
	var service *Service
	err := service.StreamMarkdown("test", true)
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

// Test NewServiceWithOptions

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
		// Should use io.Discard as destination
	})
}

// Test PrintMarkdown with different modes

func TestPrintMarkdown_PlainOutput(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{
		destination: &buf,
		plainOutput: true,
		isTerminal:  true,
	}

	content := "# Title\n\nBody"
	err := service.PrintMarkdown(content)
	if err != nil {
		t.Fatalf("PrintMarkdown failed: %v", err)
	}

	err = service.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// In plain mode, content should be passed through unrendered
	if buf.String() != content {
		t.Errorf("Expected plain content %q, got %q", content, buf.String())
	}
}

func TestPrintMarkdown_NonTerminal(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{
		destination: &buf,
		plainOutput: false,
		isTerminal:  false,
	}

	content := "# Title\n\nBody"
	err := service.PrintMarkdown(content)
	if err != nil {
		t.Fatalf("PrintMarkdown failed: %v", err)
	}

	err = service.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Non-terminal output should be plain
	if buf.String() != content {
		t.Errorf("Expected plain content %q, got %q", content, buf.String())
	}
}

// Test StreamMarkdown with different modes

func TestStreamMarkdown_PlainOutput(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{
		destination: &buf,
		plainOutput: true,
		isTerminal:  true,
	}

	err := service.StreamMarkdown("Hello", false)
	if err != nil {
		t.Fatalf("StreamMarkdown failed: %v", err)
	}

	err = service.StreamMarkdown(" world", true)
	if err != nil {
		t.Fatalf("StreamMarkdown failed: %v", err)
	}

	// In plain mode, content should be buffered and not written until done
	// After done=true, it should be in buffer ready to flush
	err = service.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	expected := "Hello world"
	if buf.String() != expected {
		t.Errorf("Expected %q, got %q", expected, buf.String())
	}
}

func TestStreamMarkdown_NonTerminal(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{
		destination: &buf,
		plainOutput: false,
		isTerminal:  false,
	}

	err := service.StreamMarkdown("Hello", false)
	if err != nil {
		t.Fatalf("StreamMarkdown failed: %v", err)
	}

	err = service.StreamMarkdown(" world", true)
	if err != nil {
		t.Fatalf("StreamMarkdown failed: %v", err)
	}

	err = service.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	expected := "Hello world"
	if buf.String() != expected {
		t.Errorf("Expected %q, got %q", expected, buf.String())
	}
}

// Test Flush with active live writer

func TestFlush_WithActiveLiveWriter(t *testing.T) {
	var buf bytes.Buffer
	service := &Service{
		destination: &buf,
		isTerminal:  true,
		liveActive:  false,
	}

	// Start streaming to activate live writer
	err := service.StreamMarkdown("Hello", false)
	if err != nil {
		t.Fatalf("StreamMarkdown failed: %v", err)
	}

	// Verify live writer is active
	if !service.liveActive {
		t.Error("Expected liveActive to be true after StreamMarkdown")
	}

	// Add regular content to buffer
	err = service.Print("buffered content")
	if err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	// Flush should stop live writer and write buffered content
	err = service.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Live writer should be stopped
	if service.liveActive {
		t.Error("Expected liveActive to be false after Flush")
	}

	// Buffer should contain the regular buffered content
	if !bytes.Contains(buf.Bytes(), []byte("buffered content")) {
		t.Error("Expected buffer to contain 'buffered content'")
	}
}
