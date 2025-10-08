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

package applyfilters

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/internal/services/filter"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockFilterService is a mock implementation of filter.Service for testing.
type mockFilterService struct {
	ignoredFiles map[string]bool
}

// Compile-time check that mockFilterService implements filter.Service
var _ filter.Service = (*mockFilterService)(nil)

func (m *mockFilterService) IsIgnoredFile(path string) bool {
	return m.ignoredFiles[path]
}

func TestNewFactory(t *testing.T) {
	filterSvc := &mockFilterService{}
	factory := NewFactory(filterSvc)

	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestNewActivity(t *testing.T) {
	filterSvc := &mockFilterService{
		ignoredFiles: map[string]bool{
			"ignored.txt": true,
			"*.tmp":       false, // mock doesn't handle patterns, just exact matches
		},
	}
	factory := NewFactory(filterSvc)
	activity := factory.NewActivity()

	if activity == nil {
		t.Error("NewActivity returned nil")
	}
}

func TestActivityExecute(t *testing.T) {
	filterSvc := &mockFilterService{
		ignoredFiles: map[string]bool{
			"ignored.txt": true,
			"keep.txt":    false,
		},
	}
	factory := NewFactory(filterSvc)
	activity := factory.NewActivity()

	input := &Input{
		Files: []string{"file1.txt", "ignored.txt", "file2.go", "keep.txt"},
	}

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Errorf("Activity execution failed: %v", err)
	}

	expected := []string{"file1.txt", "file2.go", "keep.txt"}
	if len(output.Files) != len(expected) {
		t.Errorf("Expected %d files, got %d", len(expected), len(output.Files))
	}

	for i, file := range expected {
		if i >= len(output.Files) || output.Files[i] != file {
			t.Errorf("Expected file %s at position %d, got %v", file, i, output.Files)
		}
	}
}

func TestActivityExecuteNilInput(t *testing.T) {
	filterSvc := &mockFilterService{}
	factory := NewFactory(filterSvc)
	activity := factory.NewActivity()

	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	_, err := activity(ctx, execCtx, nil)
	if err != executor.ErrInputCannotBeNil {
		t.Errorf("Expected ErrInputCannotBeNil, got %v", err)
	}
}
