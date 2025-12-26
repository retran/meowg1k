// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package gitignore provides gitignore pattern matching functionality for filtering files.
package gitignore

import (
	"fmt"
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

	parsedLine, negated, ok := stripNegation(line)
	if !ok {
		return nil
	}
	p.negation = negated
	line = stripEscapedPrefix(parsedLine)
	line = strings.TrimPrefix(line, "/")

	rgx, err := buildRegex(line, p.raw)
	if err == nil {
		p.regex = rgx
	}
	return p
}

func stripNegation(line string) (pattern string, negated bool, ok bool) {
	if strings.HasPrefix(line, "!") {
		line = line[1:]
		if line == "" {
			return "", false, false
		}
		return line, true, true
	}
	return line, false, true
}

func stripEscapedPrefix(line string) string {
	if strings.HasPrefix(line, `\`) && len(line) > 1 && (line[1] == '!' || line[1] == '#') {
		return line[1:]
	}
	return line
}

func buildRegex(line, raw string) (*regexp.Regexp, error) {
	var regex strings.Builder
	regex.WriteString("^")

	if !strings.Contains(line, "/") {
		regex.WriteString("(?:.*/)?")
	}

	segments := strings.Split(line, "/")
	appendSegments(&regex, segments)

	if strings.HasSuffix(raw, "/") {
		regex.WriteString("(?:/.*)?$")
	} else {
		regex.WriteString("(?:$|/.*)")
	}

	compiled, err := regexp.Compile(regex.String())
	if err != nil {
		return nil, fmt.Errorf("failed to compile gitignore regex: %w", err)
	}
	return compiled, nil
}

func appendSegments(builder *strings.Builder, segments []string) {
	for i, segment := range segments {
		isLastSegment := i == len(segments)-1
		if segment == "**" {
			appendGlobstar(builder, isLastSegment)
		} else {
			appendSegment(builder, segment)
		}

		if !isLastSegment && segment != "**" {
			builder.WriteString("/")
		}
	}
}

func appendGlobstar(builder *strings.Builder, isLastSegment bool) {
	if isLastSegment {
		builder.WriteString(".*")
	} else {
		builder.WriteString("(?:.*[/])?")
	}
}

func appendSegment(builder *strings.Builder, segment string) {
	for i := 0; i < len(segment); i++ {
		appendSegmentChar(builder, segment[i])
	}
}

func appendSegmentChar(builder *strings.Builder, char byte) {
	switch char {
	case '*':
		builder.WriteString("[^/]*")
	case '?':
		builder.WriteString("[^/]")
	case '.', '(', ')', '+', '|', '{', '}', '[', ']', '^', '$':
		builder.WriteByte('\\')
		builder.WriteRune(rune(char))
	default:
		builder.WriteRune(rune(char))
	}
}

// Match checks if a given path should be ignored.
// path is the relative path from the root.
func (m *Matcher) Match(path string, isDir bool) bool {
	if m == nil {
		return false
	}

	path = normalizePath(path, isDir)
	finalMatch := m.findFinalMatch(path, isDir)
	if finalMatch == nil {
		return false
	}

	if finalMatch.negation {
		return m.hasNonNegatedParentMatch(path)
	}

	return true
}

func normalizePath(path string, isDir bool) string {
	path = filepath.ToSlash(path)
	if isDir && !strings.HasSuffix(path, "/") {
		return path + "/"
	}
	return path
}

func (m *Matcher) findFinalMatch(path string, isDir bool) *pattern {
	var finalMatch *pattern
	for _, p := range m.patterns {
		if p == nil || p.regex == nil {
			continue
		}
		if !p.regex.MatchString(path) {
			continue
		}
		if strings.HasSuffix(p.raw, "/") && !isDir {
			continue
		}
		finalMatch = p
	}
	return finalMatch
}

func (m *Matcher) hasNonNegatedParentMatch(path string) bool {
	parent := path
	for {
		parent = filepath.Dir(parent)
		if parent == "." || parent == "/" || parent == "" {
			break
		}

		parentMatch := m.findFinalMatch(parent, true)
		if parentMatch != nil && !parentMatch.negation {
			return true
		}
	}

	return false
}
