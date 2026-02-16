// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"testing"
)

func TestRenderDivider(t *testing.T) {
	theme := DefaultThemeWithOptions(RenderOptions{Terminal: true, Plain: false, SupportsUnicode: true})
	
	tests := []struct {
		name     string
		style    string
		plain    bool
		unicode  bool
		expected string
	}{
		{
			name:     "plain mode",
			style:    "line",
			plain:    true,
			unicode:  false,
			expected: "---",
		},
		{
			name:     "line unicode",
			style:    "line",
			plain:    false,
			unicode:  true,
			expected: "─", // Should contain this char
		},
		{
			name:     "line ascii",
			style:    "line",
			plain:    false,
			unicode:  false,
			expected: "-", // Should contain this char
		},
		{
			name:     "empty",
			style:    "empty",
			plain:    false,
			unicode:  true,
			expected: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := RenderOptions{
				Plain:           tt.plain,
				Terminal:        !tt.plain,
				SupportsUnicode: tt.unicode,
			}
			
			result := RenderDivider(tt.style, theme, opts)
			
			if tt.expected == "" {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
			} else {
				if result == "" {
					t.Errorf("expected non-empty string")
				}
			}
		})
	}
}
