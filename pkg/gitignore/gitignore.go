// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package gitignore provides gitignore pattern matching functionality for filtering files.
package gitignore

import (
	"path/filepath"
	"regexp"
	"strings"
)

// pattern is the internal representation of a single compiled .gitignore rule.
type pattern struct {
	regex    *regexp.Regexp
	raw      string
	negation bool
}

// Matcher stores the compiled rules and performs the matching.
type Matcher struct {
	patterns []*pattern
}

// NewMatcher creates and initializes a matcher with a given set of rules.
// Empty lines and comments are automatically ignored.
func NewMatcher(lines []string) *Matcher {
	matcher := &Matcher{}
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}
		matcher.patterns = append(matcher.patterns, parsePattern(trimmedLine))
	}
	return matcher
}

// parsePattern converts a single rule string into a compiled pattern struct.
func parsePattern(line string) *pattern {
	// Return nil for empty patterns
	if line == "" {
		return nil
	}

	p := &pattern{raw: line}

	if strings.HasPrefix(line, "!") {
		p.negation = true
		line = line[1:]
		// After removing negation, check if pattern is empty
		if line == "" {
			return nil
		}
	}

	if strings.HasPrefix(line, `\`) && (len(line) > 1 && (line[1] == '!' || line[1] == '#')) {
		line = line[1:]
	}

	line = strings.TrimPrefix(line, "/")

	var regex strings.Builder
	regex.WriteString("^")

	if !strings.Contains(line, "/") {
		regex.WriteString("(?:.*/)?")
	}

	segments := strings.Split(line, "/")
	isLastSegment := false

	for i, segment := range segments {
		if i == len(segments)-1 {
			isLastSegment = true
		}

		if segment == "**" {
			if isLastSegment {
				regex.WriteString(".*")
			} else {
				regex.WriteString("(?:.*[/])?")
			}
			continue
		}

		for j := 0; j < len(segment); j++ {
			char := segment[j]
			switch char {
			case '*':
				regex.WriteString("[^/]*")
			case '?':
				regex.WriteString("[^/]")
			case '.', '(', ')', '+', '|', '{', '}', '[', ']', '^', '$':
				regex.WriteRune('\\')
				regex.WriteRune(rune(char))
			default:
				regex.WriteRune(rune(char))
			}
		}

		if !isLastSegment {
			regex.WriteString("/")
		}
	}

	if strings.HasSuffix(p.raw, "/") {
		regex.WriteString("(?:/.*)?$")
	} else {
		regex.WriteString("(?:$|/.*)")
	}

	rgx, err := regexp.Compile(regex.String())
	if err == nil {
		p.regex = rgx
	}
	return p
}

// Match checks if a given path should be ignored.
// path is the relative path from the root.
func (m *Matcher) Match(path string, isDir bool) bool {
	if m == nil {
		return false
	}

	path = filepath.ToSlash(path)

	if isDir && !strings.HasSuffix(path, "/") {
		path += "/"
	}

	var finalMatch *pattern
	for _, p := range m.patterns {
		if p == nil {
			continue
		}
		if p.regex != nil && p.regex.MatchString(path) {
			if strings.HasSuffix(p.raw, "/") && !isDir {
				continue
			}
			finalMatch = p
		}
	}

	if finalMatch == nil {
		return false
	}

	if finalMatch.negation {
		parent := path
		for {
			parent = filepath.Dir(parent)
			if parent == "." || parent == "/" || parent == "" {
				break
			}

			var parentMatch *pattern
			for _, p := range m.patterns {
				if p == nil {
					continue
				}
				if p.regex != nil && p.regex.MatchString(parent) {
					parentMatch = p
				}
			}

			if parentMatch != nil && !parentMatch.negation {
				return true
			}
		}

		return false
	}

	return true
}
