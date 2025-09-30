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

// Package gitignore provides functionality to match paths against a set of
// rules compatible with the .gitignore format.
package gitignore

import (
	"path/filepath"
	"regexp"
	"strings"
)

// pattern is the internal representation of a single compiled .gitignore rule.
type pattern struct {
	raw      string         // The original raw pattern string.
	regex    *regexp.Regexp // The compiled regular expression.
	negation bool           // Whether this is a negation pattern (!).
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
	p := &pattern{raw: line}

	if strings.HasPrefix(line, "!") {
		p.negation = true
		line = line[1:]
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

	p.regex, _ = regexp.Compile(regex.String())
	return p
}

// Match checks if a given path should be ignored.
// path is the relative path from the root.
func (m *Matcher) Match(path string, isDir bool) bool {
	path = filepath.ToSlash(path)

	var finalMatch *pattern
	for _, p := range m.patterns {
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
