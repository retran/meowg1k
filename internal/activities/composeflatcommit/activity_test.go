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

package composeflatcommit_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/retran/meowg1k/internal/activities/composeflatcommit"
	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/domain/provider"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockInvokeLLMFactory is a mock implementation of the invokeLLM factory for testing.
type mockInvokeLLMFactory struct {
	response string
	err      error
}

func (m *mockInvokeLLMFactory) NewActivity() executor.Activity[*invokellm.Input, *invokellm.Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *invokellm.Input) (*invokellm.Output, error) {
		if m.err != nil {
			return nil, m.err
		}
		return &invokellm.Output{
			Content: m.response,
		}, nil
	}
}

func TestComposeFlatCommit_Success(t *testing.T) {
	mockLLM := &mockInvokeLLMFactory{
		response: "feat: add new feature\n\nAdded support for flat strategy",
	}

	factory, err := composeflatcommit.NewFactory(mockLLM)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	activity := factory.NewActivity()

	testProfile := &profile.ResolvedProfile{
		Name:            "test",
		Provider:        provider.Gemini,
		Model:           "test-model",
		MaxInputTokens:  10000,
		MaxOutputTokens: 1000,
		Timeout:         5 * time.Minute,
	}

	changes := []*git.FileChange{
		{
			Filename: "test.go",
			Change:   "+func Test() {}\n-func OldTest() {}",
		},
	}

	input := &composeflatcommit.Input{
		Profile:      testProfile,
		SystemPrompt: "Generate a commit message",
		Changes:      changes,
		Intent:       "",
	}

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	executorCtx := executor.NewContext("test", nil, mockExec)

	output, err := activity(ctx, executorCtx, input)
	if err != nil {
		t.Fatalf("Activity failed: %v", err)
	}

	if output.CommitMessage != mockLLM.response {
		t.Errorf("Expected commit message %q, got %q", mockLLM.response, output.CommitMessage)
	}
}

func TestComposeFlatCommit_WithIntent(t *testing.T) {
	mockLLM := &mockInvokeLLMFactory{
		response: "feat: implement user request",
	}

	factory, err := composeflatcommit.NewFactory(mockLLM)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	activity := factory.NewActivity()

	testProfile := &profile.ResolvedProfile{
		Name:            "test",
		Provider:        provider.Gemini,
		Model:           "test-model",
		MaxInputTokens:  10000,
		MaxOutputTokens: 1000,
		Timeout:         5 * time.Minute,
	}

	changes := []*git.FileChange{
		{
			Filename: "feature.go",
			Change:   "+func NewFeature() {}",
		},
	}

	input := &composeflatcommit.Input{
		Profile:      testProfile,
		SystemPrompt: "Generate a commit message",
		Changes:      changes,
		Intent:       "Add support for flat strategy",
	}

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	executorCtx := executor.NewContext("test", nil, mockExec)

	output, err := activity(ctx, executorCtx, input)
	if err != nil {
		t.Fatalf("Activity failed: %v", err)
	}

	if output.CommitMessage != mockLLM.response {
		t.Errorf("Expected commit message %q, got %q", mockLLM.response, output.CommitMessage)
	}
}

func TestComposeFlatCommit_DiffTooLarge(t *testing.T) {
	mockLLM := &mockInvokeLLMFactory{
		response: "feat: add feature",
	}

	factory, err := composeflatcommit.NewFactory(mockLLM)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	activity := factory.NewActivity()

	testProfile := &profile.ResolvedProfile{
		Name:            "test",
		Provider:        provider.Gemini,
		Model:           "test-model",
		MaxInputTokens:  100, // Very small limit
		MaxOutputTokens: 1000,
		Timeout:         5 * time.Minute,
	}

	// Create a large diff that exceeds the token limit
	largeChange := strings.Repeat("a", 500) // ~125 tokens (500 chars / 4)
	changes := []*git.FileChange{
		{
			Filename: "large.go",
			Change:   largeChange,
		},
	}

	input := &composeflatcommit.Input{
		Profile:      testProfile,
		SystemPrompt: "Generate a commit message",
		Changes:      changes,
		Intent:       "",
	}

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	executorCtx := executor.NewContext("test", nil, mockExec)

	_, err = activity(ctx, executorCtx, input)
	if err == nil {
		t.Fatal("Expected error for oversized diff, got nil")
	}

	expectedErrSubstring := "diff is too large for 'flat' strategy"
	if !strings.Contains(err.Error(), expectedErrSubstring) {
		t.Errorf("Expected error to contain %q, got %q", expectedErrSubstring, err.Error())
	}
}

func TestComposeFlatCommit_NilFactory(t *testing.T) {
	_, err := composeflatcommit.NewFactory(nil)
	if err == nil {
		t.Fatal("Expected error for nil factory, got nil")
	}
}

func TestComposeFlatCommit_NilInput(t *testing.T) {
	mockLLM := &mockInvokeLLMFactory{
		response: "feat: add feature",
	}

	factory, err := composeflatcommit.NewFactory(mockLLM)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	activity := factory.NewActivity()

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	executorCtx := executor.NewContext("test", nil, mockExec)

	_, err = activity(ctx, executorCtx, nil)
	if err == nil {
		t.Fatal("Expected error for nil input, got nil")
	}
}

func TestComposeFlatCommit_MultipleFiles(t *testing.T) {
	mockLLM := &mockInvokeLLMFactory{
		response: "feat: update multiple files",
	}

	factory, err := composeflatcommit.NewFactory(mockLLM)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}

	activity := factory.NewActivity()

	testProfile := &profile.ResolvedProfile{
		Name:            "test",
		Provider:        provider.Gemini,
		Model:           "test-model",
		MaxInputTokens:  10000,
		MaxOutputTokens: 1000,
		Timeout:         5 * time.Minute,
	}

	changes := []*git.FileChange{
		{
			Filename: "file1.go",
			Change:   "+func File1() {}",
		},
		{
			Filename: "file2.go",
			Change:   "+func File2() {}",
		},
		{
			Filename: "file3.go",
			Change:   "+func File3() {}",
		},
	}

	input := &composeflatcommit.Input{
		Profile:      testProfile,
		SystemPrompt: "Generate a commit message",
		Changes:      changes,
		Intent:       "",
	}

	ctx := context.Background()
	mockExec := executor.NewExecutor(0)
	executorCtx := executor.NewContext("test", nil, mockExec)

	output, err := activity(ctx, executorCtx, input)
	if err != nil {
		t.Fatalf("Activity failed: %v", err)
	}

	if output.CommitMessage != mockLLM.response {
		t.Errorf("Expected commit message %q, got %q", mockLLM.response, output.CommitMessage)
	}
}
