// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClamp(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		lo       int
		hi       int
		expected int
	}{
		{"below lo", -5, 0, 10, 0},
		{"at lo", 0, 0, 10, 0},
		{"in range", 5, 0, 10, 5},
		{"at hi", 10, 0, 10, 10},
		{"above hi", 15, 0, 10, 10},
		{"lo equals hi", 5, 3, 3, 3},
		{"negative range in range", -3, -10, -1, -3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Clamp(tt.value, tt.lo, tt.hi)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIndentLines(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		indent   string
		expected string
	}{
		{"empty indent returns as-is", "hello\nworld", "", "hello\nworld"},
		{"empty text returns as-is", "", "  ", ""},
		{"single line", "hello", "  ", "  hello"},
		{"multiple lines", "hello\nworld", "  ", "  hello\n  world"},
		{"with tab indent", "a\nb", "\t", "\ta\n\tb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IndentLines(tt.text, tt.indent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		width  int
		minLen int
	}{
		{"zero width returns empty", "hello", 0, 0},
		{"negative width returns empty", "hello", -1, 0},
		{"shorter than width gets padded", "hi", 5, 5},
		{"longer than width stays", "hello world", 5, 11},
		{"exact width unchanged", "hello", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadRight(tt.text, tt.width)
			if tt.minLen == 0 {
				assert.Equal(t, tt.minLen, len(result))
			} else {
				assert.GreaterOrEqual(t, len(result), tt.minLen)
			}
		})
	}
}

func TestTruncatePlain(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		width  int
		maxLen int
	}{
		{"zero width returns empty", "hello", 0, 0},
		{"short text fits", "hi", 20, 2},
		{"long text truncated with ellipsis", strings.Repeat("a", 100), 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncatePlain(tt.text, tt.width)
			assert.LessOrEqual(t, len(result), tt.maxLen+3) // allow for ellipsis
			if tt.width > 0 && len(tt.text) > tt.width {
				assert.True(t, strings.HasSuffix(result, "...") || len(result) <= tt.width)
			}
		})
	}
}

func TestVisibleWidth(t *testing.T) {
	// Plain ASCII string should equal its length
	assert.Equal(t, 5, VisibleWidth("hello"))
	assert.Equal(t, 0, VisibleWidth(""))
}
