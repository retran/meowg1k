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

package io

import (
	"os"
	"strings"
	"testing"
)

func TestFinalizeOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "\n",
		},
		{
			name:     "string with leading/trailing whitespace",
			input:    "  hello world  ",
			expected: "hello world\n",
		},
		{
			name:     "string with newlines",
			input:    "\nhello\nworld\n",
			expected: "hello\nworld\n",
		},
		{
			name:     "string without trailing newline",
			input:    "hello world",
			expected: "hello world\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FinalizeOutput(tt.input)
			if result != tt.expected {
				t.Errorf("FinalizeOutput(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestReadFromStdin(t *testing.T) {
	// Test when stdin is not piped (no input)
	result, err := ReadFromStdin()
	if err != nil {
		t.Errorf("ReadFromStdin() error = %v", err)
		return
	}
	if result != "" {
		t.Errorf("ReadFromStdin() = %q, want empty string when not piped", result)
	}

	// Test with piped input by temporarily replacing os.Stdin
	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdin = r

	testInput := "hello from pipe\nwith multiple lines"
	_, err = w.WriteString(testInput)
	if err != nil {
		t.Fatalf("Failed to write to pipe: %v", err)
	}
	w.Close()

	result, err = ReadFromStdin()
	if err != nil {
		t.Errorf("ReadFromStdin() error = %v", err)
		return
	}

	expected := strings.TrimSpace(testInput)
	if result != expected {
		t.Errorf("ReadFromStdin() = %q, want %q", result, expected)
	}
}
