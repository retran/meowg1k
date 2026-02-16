// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

// TestRegexpMatch tests regexp.match() function
func TestRegexpMatch(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		text        string
		expected    bool
		expectError bool
	}{
		{
			name:     "match simple pattern",
			pattern:  "hello",
			text:     "hello world",
			expected: true,
		},
		{
			name:     "no match",
			pattern:  "goodbye",
			text:     "hello world",
			expected: false,
		},
		{
			name:     "match with regex",
			pattern:  "^[0-9]+$",
			text:     "12345",
			expected: true,
		},
		{
			name:     "regex no match",
			pattern:  "^[0-9]+$",
			text:     "12345abc",
			expected: false,
		},
		{
			name:        "invalid regex",
			pattern:     "[invalid",
			text:        "test",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regexpModule := NewRegexpModule()
			matchFunc := regexpModule.Members["match"]

			thread := &starlark.Thread{Name: "test"}
			args := starlark.Tuple{
				starlark.String(tt.pattern),
				starlark.String(tt.text),
			}

			result, err := starlark.Call(thread, matchFunc, args, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			matched, ok := result.(starlark.Bool)
			require.True(t, ok, "result should be a bool")
			assert.Equal(t, tt.expected, bool(matched))
		})
	}
}

// TestRegexpFindAll tests regexp.find_all() function
func TestRegexpFindAll(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		text        string
		limit       *int
		expected    []string
		expectError bool
	}{
		{
			name:     "find all words",
			pattern:  "\\w+",
			text:     "hello world test",
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "find all digits",
			pattern:  "\\d+",
			text:     "abc123def456ghi789",
			expected: []string{"123", "456", "789"},
		},
		{
			name:     "find with limit",
			pattern:  "\\d+",
			text:     "1 2 3 4 5",
			limit:    regexpIntPtr(2),
			expected: []string{"1", "2"},
		},
		{
			name:     "no matches",
			pattern:  "\\d+",
			text:     "abcdef",
			expected: []string{},
		},
		{
			name:        "invalid regex",
			pattern:     "[invalid",
			text:        "test",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regexpModule := NewRegexpModule()
			findAllFunc := regexpModule.Members["find_all"]

			thread := &starlark.Thread{Name: "test"}
			kwargs := []starlark.Tuple{
				{starlark.String("pattern"), starlark.String(tt.pattern)},
				{starlark.String("text"), starlark.String(tt.text)},
			}
			if tt.limit != nil {
				kwargs = append(kwargs, starlark.Tuple{
					starlark.String("limit"),
					starlark.MakeInt(*tt.limit),
				})
			}

			result, err := starlark.Call(thread, findAllFunc, starlark.Tuple{}, kwargs)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			list, ok := result.(*starlark.List)
			require.True(t, ok, "result should be a list")

			assert.Equal(t, len(tt.expected), list.Len())
			for i := 0; i < list.Len(); i++ {
				match, ok := list.Index(i).(starlark.String)
				require.True(t, ok, "list element should be a string")
				assert.Equal(t, tt.expected[i], string(match))
			}
		})
	}
}

// TestRegexpReplace tests regexp.replace() function
func TestRegexpReplace(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		text        string
		replacement string
		expected    string
		expectError bool
	}{
		{
			name:        "replace digits",
			pattern:     "\\d+",
			text:        "abc123def456",
			replacement: "X",
			expected:    "abcXdefX",
		},
		{
			name:        "replace words",
			pattern:     "\\bworld\\b",
			text:        "hello world",
			replacement: "universe",
			expected:    "hello universe",
		},
		{
			name:        "replace with backreference",
			pattern:     "(\\w+)@(\\w+)",
			text:        "user@example",
			replacement: "$1 at $2",
			expected:    "user at example",
		},
		{
			name:        "no matches",
			pattern:     "\\d+",
			text:        "abcdef",
			replacement: "X",
			expected:    "abcdef",
		},
		{
			name:        "invalid regex",
			pattern:     "[invalid",
			text:        "test",
			replacement: "X",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regexpModule := NewRegexpModule()
			replaceFunc := regexpModule.Members["replace"]

			thread := &starlark.Thread{Name: "test"}
			args := starlark.Tuple{
				starlark.String(tt.pattern),
				starlark.String(tt.text),
				starlark.String(tt.replacement),
			}

			result, err := starlark.Call(thread, replaceFunc, args, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			replaced, ok := result.(starlark.String)
			require.True(t, ok, "result should be a string")
			assert.Equal(t, tt.expected, string(replaced))
		})
	}
}

// TestRegexpSplit tests regexp.split() function
func TestRegexpSplit(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		text        string
		limit       *int
		expected    []string
		expectError bool
	}{
		{
			name:     "split by comma",
			pattern:  ",",
			text:     "a,b,c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "split by whitespace",
			pattern:  "\\s+",
			text:     "hello   world  test",
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "split with limit",
			pattern:  ",",
			text:     "a,b,c,d,e",
			limit:    regexpIntPtr(3),
			expected: []string{"a", "b", "c,d,e"},
		},
		{
			name:     "no matches",
			pattern:  ",",
			text:     "abcdef",
			expected: []string{"abcdef"},
		},
		{
			name:        "invalid regex",
			pattern:     "[invalid",
			text:        "test",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regexpModule := NewRegexpModule()
			splitFunc := regexpModule.Members["split"]

			thread := &starlark.Thread{Name: "test"}
			kwargs := []starlark.Tuple{
				{starlark.String("pattern"), starlark.String(tt.pattern)},
				{starlark.String("text"), starlark.String(tt.text)},
			}
			if tt.limit != nil {
				kwargs = append(kwargs, starlark.Tuple{
					starlark.String("limit"),
					starlark.MakeInt(*tt.limit),
				})
			}

			result, err := starlark.Call(thread, splitFunc, starlark.Tuple{}, kwargs)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			list, ok := result.(*starlark.List)
			require.True(t, ok, "result should be a list")

			assert.Equal(t, len(tt.expected), list.Len())
			for i := 0; i < list.Len(); i++ {
				part, ok := list.Index(i).(starlark.String)
				require.True(t, ok, "list element should be a string")
				assert.Equal(t, tt.expected[i], string(part))
			}
		})
	}
}

// Helper function to create int pointers
func regexpIntPtr(val int) *int {
	return &val
}
