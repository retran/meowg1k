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
)

// ErrServiceIsNil indicates that the service is nil.
var ErrServiceIsNil = fmt.Errorf("service is nil")

// Writer defines the interface for output operations.
type Writer interface {
	Print(content string) error
	PrintLine(content string) error
	Printf(format string, args ...any) error
	Flush() error
}

// Service is the concrete implementation of the Writer interface.
type Service struct {
	destination io.Writer
	buffer      strings.Builder
}

// Destination represents where the output should be sent.
type Destination string

const (
	// Stdout sends output to standard output.
	Stdout Destination = "stdout"
	// Stderr sends output to standard error.
	Stderr Destination = "stderr"
	// Discard discards all output.
	Discard Destination = "discard"
)

// NewService creates a new instance of the buffered output service.
func NewService(destination Destination) *Service {
	var destWriter io.Writer
	switch destination {
	case Stdout:
		destWriter = os.Stdout
	case Stderr:
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
		return ErrServiceIsNil
	}

	s.buffer.WriteString(content)
	return nil
}

// PrintLine adds content with a newline to the buffer.
func (s *Service) PrintLine(content string) error {
	if s == nil {
		return ErrServiceIsNil
	}

	s.buffer.WriteString(content)
	s.buffer.WriteString("\n")
	return nil
}

// Printf adds formatted content to the buffer.
func (s *Service) Printf(format string, args ...any) error {
	if s == nil {
		return ErrServiceIsNil
	}

	_, err := fmt.Fprintf(&s.buffer, format, args...)

	// TODO proper error
	return err
}

// Flush writes all accumulated content from the buffer to the destination
// in a single write operation and then clears the buffer.
func (s *Service) Flush() error {
	if s == nil {
		return ErrServiceIsNil
	}

	if s.buffer.Len() == 0 {
		return nil
	}

	content := s.buffer.String()
	s.buffer.Reset()

	_, err := fmt.Fprint(s.destination, content)

	// TODO proper error
	return err
}
