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

// Package filter provides functionality to filter files based on configured ignore patterns.
package filter

import (
	"fmt"

	"github.com/retran/meowg1k/internal/core/ports"
	"github.com/retran/meowg1k/pkg/gitignore"
)

// Service implements the Service interface.
type Service struct {
	matcher *gitignore.Matcher
}

// NewService creates a file filter service with ignore patterns from configuration.
func NewService(configResolver ports.ConfigResolver) (*Service, error) {
	if configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	var patterns []string
	if cfg, err := configResolver.Get(); cfg != nil && cfg.Filter != nil {
		patterns = cfg.Filter.Ignore
	} else if err != nil {
		return nil, fmt.Errorf("failed to get cfg: %w", err)
	}
	matcher := gitignore.NewMatcher(patterns)

	return &Service{
		matcher: matcher,
	}, nil
}

// IsIgnoredFile checks if the given file path matches any of the ignore patterns.
func (s *Service) IsIgnoredFile(path string) bool {
	if s == nil || s.matcher == nil {
		return false
	}

	return s.matcher.Match(path, false)
}
