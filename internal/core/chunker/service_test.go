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

package chunker

import (
	"testing"
)

func TestNewService(t *testing.T) {
	maxChunkRunes := 1000
	overlapRunes := 100

	service := NewService(maxChunkRunes, overlapRunes)

	if service == nil {
		t.Fatal("Expected service to be non-nil")
	}

	if service.strategies == nil {
		t.Error("Expected strategies map to be initialized")
	}

	if len(service.strategies) == 0 {
		t.Error("Expected strategies map to have entries")
	}
}

func TestService_Chunk_WithTxtExtension(t *testing.T) {
	service := NewService(100, 10)
	content := []byte("This is a test file with some content.")
	filePath := "test.txt"

	chunks, err := service.Chunk(content, filePath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}
}

func TestService_Chunk_WithMdExtension(t *testing.T) {
	service := NewService(100, 10)
	content := []byte("# Heading\n\nThis is markdown content.")
	filePath := "test.md"

	chunks, err := service.Chunk(content, filePath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}
}

func TestService_Chunk_WithUnknownExtension(t *testing.T) {
	service := NewService(100, 10)
	content := []byte("Content with unknown extension.")
	filePath := "test.xyz"

	chunks, err := service.Chunk(content, filePath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk (using default strategy)")
	}
}

func TestService_Chunk_WithNoExtension(t *testing.T) {
	service := NewService(100, 10)
	content := []byte("Content without extension.")
	filePath := "README"

	chunks, err := service.Chunk(content, filePath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk (using default strategy)")
	}
}

func TestService_Chunk_WithUpperCaseExtension(t *testing.T) {
	service := NewService(100, 10)
	content := []byte("Content with uppercase extension.")
	filePath := "test.TXT"

	chunks, err := service.Chunk(content, filePath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}
}

func TestService_Chunk_EmptyContent(t *testing.T) {
	service := NewService(100, 10)
	content := []byte("")
	filePath := "test.txt"

	chunks, err := service.Chunk(content, filePath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Empty content should result in zero chunks
	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestService_Chunk_LargeContent(t *testing.T) {
	service := NewService(50, 5)

	// Create content that will definitely need multiple chunks
	content := []byte("Line 1\n\nLine 2\n\nLine 3\n\nLine 4\n\nLine 5\n\nLine 6\n\nLine 7\n\nLine 8")
	filePath := "test.txt"

	chunks, err := service.Chunk(content, filePath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(chunks) <= 1 {
		t.Error("Expected multiple chunks for large content")
	}
}

func TestService_StrategySelection(t *testing.T) {
	service := NewService(100, 10)

	testCases := []struct {
		name     string
		filePath string
	}{
		{"txt file", "document.txt"},
		{"md file", "readme.md"},
		{"go file (default)", "main.go"},
		{"py file (default)", "script.py"},
		{"no extension (default)", "LICENSE"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := []byte("Test content")
			chunks, err := service.Chunk(content, tc.filePath)
			if err != nil {
				t.Fatalf("Expected no error for %s, got %v", tc.name, err)
			}
			if len(chunks) == 0 {
				t.Errorf("Expected at least one chunk for %s", tc.name)
			}
		})
	}
}
