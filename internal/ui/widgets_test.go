// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"os"
	"strings"
	"testing"
)

func TestRenderPanel_Plain(t *testing.T) {
	theme := DefaultTheme()
	content := "Test content"
	title := "Test Title"
	
	// Plain mode: no borders
	opts := RenderOptions{Plain: true}
	result := RenderPanel(content, title, "", theme, opts)
	
	if strings.Contains(result, "+") || strings.Contains(result, "-") || strings.Contains(result, "|") {
		t.Error("Plain mode should not contain border characters")
	}
	
	if !strings.Contains(result, content) {
		t.Error("Result should contain content")
	}
	
	if !strings.Contains(result, title) {
		t.Error("Result should contain title")
	}
}

func TestRenderPanel_Terminal(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, SupportsUnicode: false}
	theme := DefaultThemeWithOptions(opts)
	content := "Test content"
	title := "Test Title"
	
	// Terminal mode with ASCII: should have ASCII borders
	result := RenderPanel(content, title, "", theme, opts)
	
	if !strings.Contains(result, "+") || !strings.Contains(result, "-") {
		t.Error("ASCII mode should contain ASCII border characters")
	}
}

func TestRenderPanel_Unicode(t *testing.T) {
	opts := RenderOptions{Terminal: true, Plain: false, SupportsUnicode: true}
	theme := DefaultThemeWithOptions(opts)
	content := "Test content"
	title := "Test Title"
	
	// Terminal mode with Unicode: should have Unicode borders
	result := RenderPanel(content, title, "", theme, opts)
	
	// Check for Unicode box-drawing characters
	if !strings.Contains(result, "╭") && !strings.Contains(result, "┌") {
		t.Error("Unicode mode should contain Unicode border characters")
	}
}

func TestRenderTable_Plain(t *testing.T) {
	theme := DefaultTheme()
	columns := []string{"Name", "Value"}
	rows := []map[string]string{
		{"Name": "foo", "Value": "bar"},
		{"Name": "baz", "Value": "qux"},
	}
	
	opts := TableOptions{
		Theme: theme,
		Opts:  RenderOptions{Plain: true},
	}
	
	result := RenderTable(rows, columns, opts)
	
	// Should be TSV format
	if strings.Contains(result, "+") || strings.Contains(result, "|") {
		t.Error("Plain mode should not contain border characters")
	}
	
	// Should contain tab-separated values
	if !strings.Contains(result, "\t") {
		t.Error("Plain mode should contain tabs")
	}
	
	if !strings.Contains(result, "foo") || !strings.Contains(result, "bar") {
		t.Error("Plain mode should contain data")
	}
}

func TestRenderTable_Terminal(t *testing.T) {
	opts := TableOptions{
		Theme: DefaultThemeWithOptions(RenderOptions{Terminal: true, SupportsUnicode: false}),
		Opts:  RenderOptions{Terminal: true, Plain: false, SupportsUnicode: false},
	}
	columns := []string{"Name", "Value"}
	rows := []map[string]string{
		{"Name": "foo", "Value": "bar"},
	}
	
	result := RenderTable(rows, columns, opts)
	
	// Should have borders
	if !strings.Contains(result, "+") || !strings.Contains(result, "|") {
		t.Error("Terminal mode should contain border characters")
	}
}

func TestRenderTable_Unicode(t *testing.T) {
	opts := TableOptions{
		Theme: DefaultThemeWithOptions(RenderOptions{Terminal: true, SupportsUnicode: true}),
		Opts:  RenderOptions{Terminal: true, Plain: false, SupportsUnicode: true},
	}
	columns := []string{"Name", "Value"}
	rows := []map[string]string{
		{"Name": "foo", "Value": "bar"},
	}
	
	result := RenderTable(rows, columns, opts)
	
	// Should have Unicode box-drawing characters
	if !strings.Contains(result, "│") && !strings.Contains(result, "┼") {
		t.Error("Unicode mode should contain Unicode border characters")
	}
}

func TestRenderDiff_Plain(t *testing.T) {
	theme := DefaultTheme()
	diff := `diff --git a/file.txt b/file.txt
--- a/file.txt
+++ b/file.txt
@@ -1,1 +1,1 @@
-old line
+new line`
	
	opts := RenderOptions{Plain: true}
	result := RenderDiff(diff, theme, opts)
	
	// Plain mode: no ANSI codes
	if strings.Contains(result, "\x1b[") {
		t.Error("Plain mode should not contain ANSI escape codes")
	}
	
	// Should be unchanged
	if result != diff {
		t.Error("Plain mode should return diff as-is")
	}
}

func TestSupportsUnicode(t *testing.T) {
	// Save original env vars
	origLang := os.Getenv("LANG")
	origLcAll := os.Getenv("LC_ALL")
	origLcCtype := os.Getenv("LC_CTYPE")
	origTerm := os.Getenv("TERM")
	
	defer func() {
		os.Setenv("LANG", origLang)
		os.Setenv("LC_ALL", origLcAll)
		os.Setenv("LC_CTYPE", origLcCtype)
		os.Setenv("TERM", origTerm)
	}()
	
	tests := []struct {
		name     string
		lang     string
		lcAll    string
		lcCtype  string
		term     string
		expected bool
	}{
		{"UTF-8 locale", "en_US.UTF-8", "", "", "", true},
		{"UTF8 locale", "en_US.UTF8", "", "", "", true},
		{"ASCII locale", "C", "", "", "", false},
		{"xterm", "", "", "", "xterm-256color", true},
		{"kitty", "", "", "", "xterm-kitty", true},
		{"default", "", "", "", "", true}, // Default to true for modern systems
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all locale vars first
			os.Unsetenv("LANG")
			os.Unsetenv("LC_ALL")
			os.Unsetenv("LC_CTYPE")
			os.Unsetenv("TERM")
			
			// Set specific values for this test
			if tt.lang != "" {
				os.Setenv("LANG", tt.lang)
			}
			if tt.lcAll != "" {
				os.Setenv("LC_ALL", tt.lcAll)
			}
			if tt.lcCtype != "" {
				os.Setenv("LC_CTYPE", tt.lcCtype)
			}
			if tt.term != "" {
				os.Setenv("TERM", tt.term)
			}
			
			result := SupportsUnicode()
			if result != tt.expected {
				t.Errorf("SupportsUnicode() = %v, want %v (LANG=%s, TERM=%s)", 
					result, tt.expected, tt.lang, tt.term)
			}
		})
	}
}

func TestRenderOptions_AutoDetect(t *testing.T) {
	opts := NewRenderOptions()
	
	// Just verify it doesn't panic and has reasonable defaults
	if opts.Plain && opts.Terminal {
		t.Error("Can't be both plain and terminal")
	}
	
	// SupportsUnicode should be set based on environment
	t.Logf("Auto-detected SupportsUnicode: %v", opts.SupportsUnicode)
}

func TestPanelColorResolution(t *testing.T) {
	theme := DefaultTheme()
	
	tests := []struct {
		style    string
		expected string
	}{
		{"success", "success"},
		{"error", "error"},
		{"warn", "warn"},
		{"info", "info"},
	}
	
	for _, tt := range tests {
		opts := RenderOptions{Terminal: true}
		result := RenderPanel("test", "", tt.style, theme, opts)
		if result == "" {
			t.Errorf("Failed to render panel with style %s", tt.style)
		}
	}
}
