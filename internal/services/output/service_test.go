/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package output

import (
	"bytes"
	"testing"

	"github.com/retran/meowg1k/internal/core/output"
)

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
