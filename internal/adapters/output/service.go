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

// Package output provides an abstraction for output destinations.
package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/retran/meowg1k/internal/domain/output"
)

// Service is the concrete implementation of the Writer interface.
type Service struct {
	destination io.Writer
	buffer      strings.Builder
}

// NewService creates a new instance of the buffered output service.
func NewService(destination output.Destination) *Service {
	var destWriter io.Writer
	switch destination {
	case output.Stdout:
		destWriter = os.Stdout
	case output.Stderr:
		destWriter = os.Stderr
	default:
		destWriter = io.Discard
	}

	return &Service{
		destination: destWriter,
	}
}

// Print adds content to the buffer.
func (s *Service) Print(content string) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}

	s.buffer.WriteString(content)
	return nil
}

// PrintLine adds content with a newline to the buffer.
func (s *Service) PrintLine(content string) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}

	s.buffer.WriteString(content)
	s.buffer.WriteString("\n")
	return nil
}

// Printf adds formatted content to the buffer.
func (s *Service) Printf(format string, args ...any) error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}

	_, err := fmt.Fprintf(&s.buffer, format, args...)
	if err != nil {
		return fmt.Errorf("failed to write to buffer: %w", err)
	}

	return nil
}

// Flush writes all accumulated content from the buffer to the destination
// in a single write operation and then clears the buffer.
func (s *Service) Flush() error {
	if s == nil {
		return fmt.Errorf("output service is nil")
	}

	if s.buffer.Len() == 0 {
		return nil
	}

	content := s.buffer.String()
	s.buffer.Reset()

	_, err := fmt.Fprint(s.destination, content)
	if err != nil {
		return fmt.Errorf("failed to write to destination: %w", err)
	}

	return nil
}
