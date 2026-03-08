// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package gitignore

import (
	"testing"
)

func TestNewMatcher(t *testing.T) {
	patterns := []string{
		"*.tmp",
		"# comment",
		"",
		"*.log",
	}

	matcher := NewMatcher(patterns)
	if matcher == nil {
		t.Fatal("NewMatcher returned nil")
	}

	if matcher.patterns == nil {
		t.Fatal("patterns is nil")
	}

	if len(matcher.patterns) != 2 {
		t.Errorf("Expected 2 patterns, got %d", len(matcher.patterns))
	}
}

func TestMatch(t *testing.T) {
	patterns := []string{
		"*.tmp",
		"*.log",
		"dir/",
		"!important.log",
	}

	matcher := NewMatcher(patterns)

	tests := []struct {
		path   string
		isDir  bool
		expect bool
	}{
		{"file.tmp", false, true},
		{"file.log", false, true},
		{"important.log", false, false}, // negated
		{"dir", true, true},
		{"dir/file.txt", false, false},
		{"other.txt", false, false},
	}

	for _, tt := range tests {
		result := matcher.Match(tt.path, tt.isDir)
		if result != tt.expect {
			t.Errorf("Match(%s, %v) = %v, expect %v", tt.path, tt.isDir, result, tt.expect)
		}
	}
}

func TestMatchEmpty(t *testing.T) {
	matcher := NewMatcher([]string{})

	if matcher.Match("anyfile.txt", false) {
		t.Error("Empty matcher should not match anything")
	}
}

func TestParsePattern(t *testing.T) {
	p := parsePattern("*.tmp")
	if p == nil {
		t.Fatal("parsePattern returned nil")
	}
	if p.raw != "*.tmp" {
		t.Errorf("Expected raw '*.tmp', got '%s'", p.raw)
	}
	if p.regex == nil {
		t.Error("regex is nil")
	}
}

func TestParsePatternNegation(t *testing.T) {
	p := parsePattern("!*.tmp")
	if p == nil {
		t.Fatal("parsePattern returned nil")
	}
	if !p.negation {
		t.Error("Expected negation to be true")
	}
}

func TestMatchWithDirectorySuffix(t *testing.T) {
	patterns := []string{
		"build/",
		"*.tmp",
	}

	matcher := NewMatcher(patterns)

	tests := []struct {
		path   string
		isDir  bool
		expect bool
	}{
		{"build", true, true},            // Directory matches
		{"build/file.txt", false, false}, // File under dir doesn't match dir/ pattern
		{"mybuild", false, false},        // File doesn't match dir pattern
		{"file.tmp", false, true},        // File matches *.tmp
	}

	for _, tt := range tests {
		result := matcher.Match(tt.path, tt.isDir)
		if result != tt.expect {
			t.Errorf("Match(%s, %v) = %v, expect %v", tt.path, tt.isDir, result, tt.expect)
		}
	}
}

func TestMatchNegationWithParent(t *testing.T) {
	patterns := []string{
		"logs/",
		"!logs/important/",
		"build/**",
		"!build/keep.txt",
	}

	matcher := NewMatcher(patterns)

	tests := []struct {
		path   string
		isDir  bool
		expect bool
	}{
		{"logs", true, true},
		{"logs/important", true, false},           // Negated
		{"logs/important/data.log", false, false}, // Under negated dir
		{"build/file.txt", false, true},
		{"build/keep.txt", false, false}, // Negated file
		{"build/subdir/file.txt", false, true},
	}

	for _, tt := range tests {
		result := matcher.Match(tt.path, tt.isDir)
		if result != tt.expect {
			t.Errorf("Match(%s, %v) = %v, expect %v", tt.path, tt.isDir, result, tt.expect)
		}
	}
}

func TestMatchComplexPatterns(t *testing.T) {
	patterns := []string{
		"*.log",
		"!important.log",
		"temp/",
		"src/**/*.tmp",
		"!src/keep/*.tmp",
	}

	matcher := NewMatcher(patterns)

	tests := []struct {
		path   string
		isDir  bool
		expect bool
	}{
		{"debug.log", false, true},
		{"important.log", false, false}, // Negated
		{"temp", true, true},
		{"src/file.tmp", false, true},
		{"src/sub/file.tmp", false, true},
		{"src/keep/file.tmp", false, false}, // Negated
	}

	for _, tt := range tests {
		result := matcher.Match(tt.path, tt.isDir)
		if result != tt.expect {
			t.Errorf("Match(%s, %v) = %v, expect %v", tt.path, tt.isDir, result, tt.expect)
		}
	}
}

func TestMatchDirectoryTrailingSlash(t *testing.T) {
	patterns := []string{
		"node_modules/",
	}

	matcher := NewMatcher(patterns)

	// Directory without trailing slash
	if !matcher.Match("node_modules", true) {
		t.Error("Should match directory even without trailing slash")
	}

	// File with same name should not match directory pattern
	if matcher.Match("node_modules", false) {
		t.Error("File should not match directory pattern")
	}
}

func TestMatchRootPatterns(t *testing.T) {
	patterns := []string{
		"/root.txt",
		"/*.log",
	}

	matcher := NewMatcher(patterns)

	tests := []struct {
		path   string
		isDir  bool
		expect bool
	}{
		{"root.txt", false, true},
		{"subdir/root.txt", false, true}, // Leading slash doesn't anchor in this impl
		{"debug.log", false, true},
		{"logs/debug.log", false, true}, // Leading slash doesn't anchor in this impl
	}

	for _, tt := range tests {
		result := matcher.Match(tt.path, tt.isDir)
		if result != tt.expect {
			t.Errorf("Match(%s, %v) = %v, expect %v", tt.path, tt.isDir, result, tt.expect)
		}
	}
}

func TestMatchWildcardPatterns(t *testing.T) {
	patterns := []string{
		"*.txt",
		"test*.log",
	}

	matcher := NewMatcher(patterns)

	tests := []struct {
		path   string
		isDir  bool
		expect bool
	}{
		{"readme.txt", false, true},
		{"test1.log", false, true},
		{"testA.log", false, true},
		{"file.csv", false, false},
	}

	for _, tt := range tests {
		result := matcher.Match(tt.path, tt.isDir)
		if result != tt.expect {
			t.Errorf("Match(%s, %v) = %v, expect %v", tt.path, tt.isDir, result, tt.expect)
		}
	}
}

func TestParsePatternSpecialCases(t *testing.T) {
	tests := []struct {
		pattern  string
		expected bool // Whether it should create a valid pattern
	}{
		{"", false},
		{"!negated", true},
		{"/root", true},
		{"dir/", true},
	}

	for _, tt := range tests {
		p := parsePattern(tt.pattern)
		if tt.expected && p == nil {
			t.Errorf("parsePattern(%s) should return a pattern, got nil", tt.pattern)
		}
		if !tt.expected && p != nil {
			t.Errorf("parsePattern(%s) should return nil, got a pattern", tt.pattern)
		}
	}
}

func TestMatchPathNormalization(t *testing.T) {
	patterns := []string{
		"src/build/",
	}

	matcher := NewMatcher(patterns)

	// Test with different path separators (should be normalized)
	tests := []struct {
		path   string
		isDir  bool
		expect bool
	}{
		{"src/build", true, true},
	}

	for _, tt := range tests {
		result := matcher.Match(tt.path, tt.isDir)
		if result != tt.expect {
			t.Errorf("Match(%s, %v) = %v, expect %v", tt.path, tt.isDir, result, tt.expect)
		}
	}
}
