// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestStep_Basic(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{
		Terminal:        true,
		SupportsUnicode: true,
	}
	
	step := NewStep("Test Step", "✦", 0, theme, opts, &buf)
	step.Write("Operation 1")
	step.Write("Operation 2")
	step.Done("Completed")
	
	output := buf.String()
	
	if !strings.Contains(output, "Test Step") {
		t.Error("Expected step title in output")
	}
	
	if !strings.Contains(output, "Operation 1") {
		t.Error("Expected operation 1 in output")
	}
	
	if !strings.Contains(output, "╭─") {
		t.Error("Expected top border")
	}
	
	if !strings.Contains(output, "╰─") {
		t.Error("Expected bottom border")
	}
}

func TestStep_Plain(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{
		Plain:    true,
		Terminal: false,
	}
	
	step := NewStep("Test Step", "", 0, theme, opts, &buf)
	step.Write("Content")
	step.Done("Done")
	
	output := buf.String()
	
	// Plain mode should not have fancy borders
	if strings.Contains(output, "╭") || strings.Contains(output, "╰") {
		t.Error("Plain mode should not have Unicode borders")
	}
	
	if !strings.Contains(output, "Test Step") {
		t.Error("Expected step title")
	}
	
	if !strings.Contains(output, "Content") {
		t.Error("Expected content")
	}
}

func TestStep_ASCII(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{
		Terminal:        true,
		SupportsUnicode: false, // ASCII mode
	}
	
	step := NewStep("ASCII Step", "*", 0, theme, opts, &buf)
	step.Done("")
	
	output := buf.String()
	
	// Should use ASCII borders
	if strings.Contains(output, "╭") {
		t.Error("ASCII mode should not have Unicode characters")
	}
	
	if !strings.Contains(output, "+-") {
		t.Error("Expected ASCII border (+-)") 
	}
}

func TestStep_Nesting(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{
		Terminal:        true,
		SupportsUnicode: true,
	}
	
	// Parent step (indent 0)
	parent := NewStep("Parent", "📦", 0, theme, opts, &buf)
	
	// Child step (indent 1)
	child := NewStep("Child", "🔧", 1, theme, opts, &buf)
	child.Write("Nested operation")
	child.Done("Child done")
	
	parent.Done("Parent done")
	
	output := buf.String()
	
	// Check for nested structure
	lines := strings.Split(output, "\n")
	hasNestedLine := false
	for _, line := range lines {
		// Child lines should have more indentation
		if strings.Contains(line, "│  │") {
			hasNestedLine = true
			break
		}
	}
	
	if !hasNestedLine {
		t.Error("Expected nested indentation (│  │)")
	}
}

func TestStep_Fail(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{
		Terminal:        true,
		SupportsUnicode: true,
	}
	
	step := NewStep("Failed Step", "⚠", 0, theme, opts, &buf)
	step.Write("Attempting operation")
	step.Fail("Something went wrong")
	
	output := buf.String()
	
	if !strings.Contains(output, "Something went wrong") {
		t.Error("Expected error message")
	}
}

func TestStep_MultilineContent(t *testing.T) {
	var buf bytes.Buffer
	theme := DefaultTheme()
	opts := RenderOptions{
		Terminal:        true,
		SupportsUnicode: true,
	}
	
	step := NewStep("Test", "", 0, theme, opts, &buf)
	step.Write("Line 1\nLine 2\nLine 3")
	step.Done("")
	
	output := buf.String()
	
	if !strings.Contains(output, "Line 1") || 
	   !strings.Contains(output, "Line 2") || 
	   !strings.Contains(output, "Line 3") {
		t.Error("Expected all lines in multiline content")
	}
}
