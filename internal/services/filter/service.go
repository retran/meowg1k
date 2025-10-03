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
	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/pkg/gitignore"
)

// Service defines the interface for filtering files.
type Service interface {
	// IsIgnoredFile checks if the given file path matches any of the ignore patterns.
	IsIgnoredFile(path string) bool
}

// serviceImpl is the concrete implementation of the Service interface.
type serviceImpl struct {
	matcher *gitignore.Matcher
}

// NewService creates a new instance of the filter service.
func NewService(configService config.Service) Service {
	var patterns []string
	if config := configService.GetConfig(); config.Filter != nil {
		patterns = config.Filter.Ignore
	}
	matcher := gitignore.NewMatcher(patterns)

	return &serviceImpl{
		matcher: matcher,
	}
}

// IsIgnoredFile checks if the given file path matches any of the ignore patterns.
func (s *serviceImpl) IsIgnoredFile(path string) bool {
	return s.matcher.Match(path, false)
}
