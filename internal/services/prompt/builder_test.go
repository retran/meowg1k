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

package prompt

import (
	"os"
	"testing"
)

func TestBuilder_CombinePrompts(t *testing.T) {
	builder := NewBuilder()

	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{
			name:     "empty parts",
			parts:    []string{},
			expected: "",
		},
		{
			name:     "single part",
			parts:    []string{"Hello"},
			expected: "Hello",
		},
		{
			name:     "multiple parts",
			parts:    []string{"Hello", "World", "!"},
			expected: "Hello\n\nWorld\n\n!",
		},
		{
			name:     "parts with empty strings",
			parts:    []string{"Hello", "", "World", "   ", "!"},
			expected: "Hello\n\nWorld\n\n!",
		},
		{
			name:     "parts with whitespace",
			parts:    []string{" Hello ", " World "},
			expected: "Hello\n\nWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.CombinePrompts(tt.parts...)
			if result != tt.expected {
				t.Errorf("CombinePrompts() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuilder_BuildUserPrompt_NoStdin(t *testing.T) {
	builder := NewBuilder()

	// Test with base prompt but no stdin (since we can't easily mock stdin in tests)
	result, err := builder.BuildUserPrompt("base prompt", "\n\n```\n%s\n```")
	if err != nil {
		t.Errorf("BuildUserPrompt() error = %v", err)
	}

	// Without stdin, should return base prompt unchanged
	expected := "base prompt"
	if result != expected {
		t.Errorf("BuildUserPrompt() = %q, want %q", result, expected)
	}
}

func TestBuilder_BuildUserPrompt_WithStdin(t *testing.T) {
	builder := NewBuilder()

	// Save original stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create a pipe and replace stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdin = r

	// Write content to stdin in a goroutine
	stdinContent := "content from stdin"
	go func() {
		defer w.Close()
		w.WriteString(stdinContent)
	}()

	// Test BuildUserPrompt with stdin content
	result, err := builder.BuildUserPrompt("base prompt", "\n\n```\n%s\n```")
	if err != nil {
		t.Errorf("BuildUserPrompt() error = %v", err)
	}

	// Should combine base prompt with stdin content using the template
	expected := "base prompt\n\n```\ncontent from stdin\n```"
	if result != expected {
		t.Errorf("BuildUserPrompt() = %q, want %q", result, expected)
	}
}
