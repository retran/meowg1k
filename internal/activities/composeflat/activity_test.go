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

package composeflat

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

// Mock activity factory for content generation
type mockContentGenerationFactory struct {
	response string
	err      error
}

func (m *mockContentGenerationFactory) NewActivity() executor.Activity[*invokellm.Input, *invokellm.Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *invokellm.Input) (*invokellm.Output, error) {
		if m.err != nil {
			return nil, m.err
		}
		return &invokellm.Output{Content: m.response}, nil
	}
}

func TestNewFactory(t *testing.T) {
	t.Run("valid factory creation", func(t *testing.T) {
		mockFactory := &mockContentGenerationFactory{response: "test"}
		factory, err := NewFactory(mockFactory, "test activity", "test content")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if factory == nil {
			t.Fatal("expected factory to be non-nil")
		}
	})

	t.Run("nil content generation factory", func(t *testing.T) {
		factory, err := NewFactory(nil, "test", "test")
		if err == nil {
			t.Fatal("expected error for nil factory")
		}
		if factory != nil {
			t.Fatal("expected nil factory")
		}
	})

	t.Run("default values for empty strings", func(t *testing.T) {
		mockFactory := &mockContentGenerationFactory{response: "test"}
		factory, err := NewFactory(mockFactory, "", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if factory.activityName == "" {
			t.Error("expected default activity name")
		}
		if factory.contentType == "" {
			t.Error("expected default content type")
		}
	})
}

func TestComposeFlat(t *testing.T) {
	t.Run("successful composition", func(t *testing.T) {
		mockFactory := &mockContentGenerationFactory{response: "Generated content"}
		factory, err := NewFactory(mockFactory, "Test activity", "test content")
		if err != nil {
			t.Fatalf("failed to create factory: %v", err)
		}

		activity := factory.NewActivity()
		exec := executor.NewExecutor(0)
		execCtx := executor.NewContext("test", nil, exec)

		input := &Input{
			Profile: &profile.ResolvedProfile{
				MaxInputTokens: 10000,
			},
			SystemPrompt: "Test system prompt",
			Changes: []*git.FileChange{
				{
					Filename: "test.go",
					Change:   "+func test() {}",
				},
			},
			Intent: "Test intent",
		}

		output, err := activity(context.Background(), execCtx, input)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if output.Content != "Generated content" {
			t.Errorf("expected 'Generated content', got %q", output.Content)
		}
	})

	t.Run("nil factory", func(t *testing.T) {
		var factory *Factory
		activity := factory.NewActivity()
		exec := executor.NewExecutor(0)
		execCtx := executor.NewContext("test", nil, exec)

		input := &Input{
			Profile: &profile.ResolvedProfile{},
		}

		_, err := activity(context.Background(), execCtx, input)
		if err == nil {
			t.Fatal("expected error for nil factory")
		}
	})

	t.Run("nil input", func(t *testing.T) {
		mockFactory := &mockContentGenerationFactory{response: "test"}
		factory, _ := NewFactory(mockFactory, "test", "test")
		activity := factory.NewActivity()
		exec := executor.NewExecutor(0)
		execCtx := executor.NewContext("test", nil, exec)

		_, err := activity(context.Background(), execCtx, nil)
		if err == nil {
			t.Fatal("expected error for nil input")
		}
	})

	t.Run("diff too large", func(t *testing.T) {
		mockFactory := &mockContentGenerationFactory{response: "test"}
		factory, _ := NewFactory(mockFactory, "test", "test content")
		activity := factory.NewActivity()
		exec := executor.NewExecutor(0)
		execCtx := executor.NewContext("test", nil, exec)

		// Create a large change
		largeChange := ""
		for i := 0; i < 10000; i++ {
			largeChange += "+line\n"
		}

		input := &Input{
			Profile: &profile.ResolvedProfile{
				MaxInputTokens: 100, // Very small limit
			},
			SystemPrompt: "Test",
			Changes: []*git.FileChange{
				{
					Filename: "test.go",
					Change:   largeChange,
				},
			},
		}

		_, err := activity(context.Background(), execCtx, input)
		if err == nil {
			t.Fatal("expected error for diff too large")
		}
		if !contains(err.Error(), "diff is too large") {
			t.Errorf("expected 'diff is too large' error, got %v", err)
		}
	})

	t.Run("multiple files with intent", func(t *testing.T) {
		mockFactory := &mockContentGenerationFactory{response: "Multi-file content"}
		factory, _ := NewFactory(mockFactory, "test", "test content")
		activity := factory.NewActivity()
		exec := executor.NewExecutor(0)
		execCtx := executor.NewContext("test", nil, exec)

		input := &Input{
			Profile: &profile.ResolvedProfile{
				MaxInputTokens: 50000,
			},
			SystemPrompt: "Test",
			Changes: []*git.FileChange{
				{
					Filename: "file1.go",
					Change:   "+func test1() {}",
				},
				{
					Filename: "file2.go",
					Change:   "+func test2() {}",
				},
			},
			Intent: "Add new functions",
		}

		output, err := activity(context.Background(), execCtx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output.Content != "Multi-file content" {
			t.Errorf("expected 'Multi-file content', got %q", output.Content)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
