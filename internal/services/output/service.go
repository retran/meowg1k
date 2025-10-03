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

// Service provides functionality for outputting content.
// All output is buffered in memory and will only be written to the
// destination when Flush() is explicitly called.
type Service interface {
	// Print adds content to the in-memory buffer.
	Print(content string)
	// PrintLine adds content with a trailing newline to the in-memory buffer.
	PrintLine(content string)
	// Printf adds formatted content to the in-memory buffer.
	Printf(format string, args ...any)
	// Flush writes all buffered content to the configured destination.
	Flush() error
}

// serviceImpl is the concrete implementation of the Service interface.
type serviceImpl struct {
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
func NewService(destination Destination) Service {
	var destWriter io.Writer
	switch destination {
	case Stdout:
		destWriter = os.Stdout
	case Stderr:
		destWriter = os.Stderr
	default:
		destWriter = io.Discard
	}

	return &serviceImpl{
		destination: destWriter,
	}
}

// Print adds content to the buffer.
func (s *serviceImpl) Print(content string) {
	s.buffer.WriteString(content)
}

// PrintLine adds content with a newline to the buffer.
func (s *serviceImpl) PrintLine(content string) {
	s.buffer.WriteString(content)
	s.buffer.WriteString("\n")
}

// Printf adds formatted content to the buffer.
func (s *serviceImpl) Printf(format string, args ...any) {
	fmt.Fprintf(&s.buffer, format, args...)
}

// Flush writes all accumulated content from the buffer to the destination
// in a single write operation and then clears the buffer.
func (s *serviceImpl) Flush() error {
	if s.buffer.Len() == 0 {
		return nil
	}

	content := s.buffer.String()
	s.buffer.Reset()

	_, err := fmt.Fprint(s.destination, content)
	return err
}
