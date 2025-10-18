// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package filter provides services for filtering files based on include/exclude patterns and gitignore rules.
package filter

import (
	"fmt"

	"github.com/retran/meowg1k/internal/ports"
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
