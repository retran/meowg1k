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

import "testing"

func TestFinalizeOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic text",
			input:    "hello world",
			expected: "hello world\n",
		},
		{
			name:     "text with leading whitespace",
			input:    "  hello world",
			expected: "hello world\n",
		},
		{
			name:     "text with trailing whitespace",
			input:    "hello world  ",
			expected: "hello world\n",
		},
		{
			name:     "text with both leading and trailing whitespace",
			input:    "  hello world  ",
			expected: "hello world\n",
		},
		{
			name:     "text with newlines at the end",
			input:    "hello world\n\n",
			expected: "hello world\n",
		},
		{
			name:     "text already with single newline",
			input:    "hello world\n",
			expected: "hello world\n",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "\n",
		},
		{
			name:     "only whitespace",
			input:    "   \n\t  ",
			expected: "\n",
		},
		{
			name:     "multiline text",
			input:    "line1\nline2\nline3",
			expected: "line1\nline2\nline3\n",
		},
		{
			name:     "multiline text with trailing whitespace",
			input:    "line1\nline2\nline3  \n  ",
			expected: "line1\nline2\nline3\n",
		},
		{
			name:     "text with tabs",
			input:    "\thello\tworld\t",
			expected: "hello\tworld\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FinalizeOutput(tt.input)
			if result != tt.expected {
				t.Errorf("FinalizeOutput(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFinalizeOutputEnsuresNewline(t *testing.T) {
	inputs := []string{
		"no newline",
		"has newline\n",
		"multiple newlines\n\n\n",
		"",
		"   ",
	}

	for _, input := range inputs {
		result := FinalizeOutput(input)
		if len(result) == 0 || result[len(result)-1] != '\n' {
			t.Errorf("FinalizeOutput(%q) should always end with newline, got %q", input, result)
		}
	}
}