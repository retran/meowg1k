// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package scanworkspacestate

import (
	"context"
	"errors"
	"testing"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockProjectStateService is a mock implementation of ports.ProjectStateService.
type mockProjectStateService struct {
	GetHeadStateFn    func(ctx context.Context) (map[string]domainindex.FileState, error)
	GetStagingStateFn func(ctx context.Context) (map[string]domainindex.FileState, error)
	GetWorkdirStateFn func(ctx context.Context) (map[string]domainindex.FileState, error)
}

func (m *mockProjectStateService) GetHeadState(ctx context.Context) (map[string]domainindex.FileState, error) {
	if m.GetHeadStateFn != nil {
		return m.GetHeadStateFn(ctx)
	}
	return nil, nil
}

func (m *mockProjectStateService) GetStagingState(ctx context.Context) (map[string]domainindex.FileState, error) {
	if m.GetStagingStateFn != nil {
		return m.GetStagingStateFn(ctx)
	}
	return nil, nil
}

func (m *mockProjectStateService) GetWorkdirState(ctx context.Context) (map[string]domainindex.FileState, error) {
	if m.GetWorkdirStateFn != nil {
		return m.GetWorkdirStateFn(ctx)
	}
	return nil, nil
}

func TestNewFactory(t *testing.T) {
	t.Run("should succeed with valid project state service", func(t *testing.T) {
		mockService := &mockProjectStateService{}
		factory, err := NewFactory(mockService)
		if err != nil {
			t.Fatalf("expected no error, but got %v", err)
		}
		if factory == nil {
			t.Fatal("factory should not be nil")
		}
	})

	t.Run("should fail with nil project state service", func(t *testing.T) {
		_, err := NewFactory(nil)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		expectedErr := "scanworkspacestate.NewFactory: projectStateSvc cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', but got '%s'", expectedErr, err.Error())
		}
	})
}

func TestActivity(t *testing.T) {
	mockService := &mockProjectStateService{}
	factory, _ := NewFactory(mockService)
	activity := factory.NewActivity()

	// Mock file states for different workspace states
	headState := map[string]domainindex.FileState{
		"file1.go": {
			Content:     []byte("package main\nfunc main() {}"),
			ContentHash: "head123",
		},
		"file2.go": {
			Content:     []byte("package test\nfunc test() {}"),
			ContentHash: "head456",
		},
	}

	stageState := map[string]domainindex.FileState{
		"file1.go": {
			Content:     []byte("package main\nfunc main() {\n    // staged change\n}"),
			ContentHash: "stage123",
		},
		"file3.go": {
			Content:     []byte("package new\nfunc new() {}"),
			ContentHash: "stage789",
		},
	}

	workdirState := map[string]domainindex.FileState{
		"file1.go": {
			Content:     []byte("package main\nfunc main() {\n    // workdir change\n}"),
			ContentHash: "workdir123",
		},
		"file4.go": {
			Content:     []byte("package workdir\nfunc workdir() {}"),
			ContentHash: "workdir999",
		},
	}

	t.Run("should succeed with valid workspace states", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}
		executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))

		getHeadCallCount := 0
		getStagingCallCount := 0
		getWorkdirCallCount := 0

		mockService.GetHeadStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			getHeadCallCount++
			return headState, nil
		}

		mockService.GetStagingStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			getStagingCallCount++
			return stageState, nil
		}

		mockService.GetWorkdirStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			getWorkdirCallCount++
			return workdirState, nil
		}

		// Act
		output, err := activity(ctx, executorCtx, struct{}{})
		// Assert
		if err != nil {
			t.Fatalf("activity failed: %v", err)
		}
		if output == nil {
			t.Fatal("output should not be nil")
		}

		// Verify HEAD state
		if len(output.HeadState) != 2 {
			t.Errorf("expected 2 files in HeadState, got %d", len(output.HeadState))
		}
		if output.HeadState["file1.go"].ContentHash != "head123" {
			t.Errorf("unexpected HEAD file1.go content hash: %s", output.HeadState["file1.go"].ContentHash)
		}

		// Verify stage state
		if len(output.StageState) != 2 {
			t.Errorf("expected 2 files in StageState, got %d", len(output.StageState))
		}
		if output.StageState["file1.go"].ContentHash != "stage123" {
			t.Errorf("unexpected stage file1.go content hash: %s", output.StageState["file1.go"].ContentHash)
		}

		// Verify workdir state
		if len(output.WorkdirState) != 2 {
			t.Errorf("expected 2 files in WorkdirState, got %d", len(output.WorkdirState))
		}
		if output.WorkdirState["file1.go"].ContentHash != "workdir123" {
			t.Errorf("unexpected workdir file1.go content hash: %s", output.WorkdirState["file1.go"].ContentHash)
		}

		// Verify service calls
		if getHeadCallCount != 1 {
			t.Errorf("expected GetHeadState to be called once, got %d", getHeadCallCount)
		}
		if getStagingCallCount != 1 {
			t.Errorf("expected GetStagingState to be called once, got %d", getStagingCallCount)
		}
		if getWorkdirCallCount != 1 {
			t.Errorf("expected GetWorkdirState to be called once, got %d", getWorkdirCallCount)
		}

		// Check feedback messages
		if len(feedbackMessages) != 2 {
			t.Fatalf("expected 2 feedback messages, got %d", len(feedbackMessages))
		}
		if feedbackMessages[0].Message != "Scanning workspace" {
			t.Errorf("unexpected running message: expected 'Scanning workspace', got '%s'", feedbackMessages[0].Message)
		}
		if feedbackMessages[1].Message != "head=2, stage=2, workdir=2" {
			t.Errorf("unexpected completion message: %s", feedbackMessages[1].Message)
		}
	})

	t.Run("should fail when GetHeadState returns error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))
		headErr := errors.New("head state error")

		mockService.GetHeadStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			return nil, headErr
		}

		// Act
		_, err := activity(ctx, executorCtx, struct{}{})

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if !errors.Is(err, headErr) {
			t.Errorf("expected error to wrap head error, got: %v", err)
		}
	})

	t.Run("should fail when GetStagingState returns error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))
		stagingErr := errors.New("staging state error")

		mockService.GetHeadStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			return headState, nil
		}

		mockService.GetStagingStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			return nil, stagingErr
		}

		// Act
		_, err := activity(ctx, executorCtx, struct{}{})

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if !errors.Is(err, stagingErr) {
			t.Errorf("expected error to wrap staging error, got: %v", err)
		}
	})

	t.Run("should fail when GetWorkdirState returns error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))
		workdirErr := errors.New("workdir state error")

		mockService.GetHeadStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			return headState, nil
		}

		mockService.GetStagingStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			return stageState, nil
		}

		mockService.GetWorkdirStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			return nil, workdirErr
		}

		// Act
		_, err := activity(ctx, executorCtx, struct{}{})

		// Assert
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if !errors.Is(err, workdirErr) {
			t.Errorf("expected error to wrap workdir error, got: %v", err)
		}
	})

	t.Run("should handle empty workspace states", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}
		executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))

		mockService.GetHeadStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			return map[string]domainindex.FileState{}, nil
		}

		mockService.GetStagingStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			return map[string]domainindex.FileState{}, nil
		}

		mockService.GetWorkdirStateFn = func(ctx context.Context) (map[string]domainindex.FileState, error) {
			return map[string]domainindex.FileState{}, nil
		}

		// Act
		output, err := activity(ctx, executorCtx, struct{}{})
		// Assert
		if err != nil {
			t.Fatalf("activity failed: %v", err)
		}
		if len(output.HeadState) != 0 {
			t.Errorf("expected 0 files in HeadState, got %d", len(output.HeadState))
		}
		if len(output.StageState) != 0 {
			t.Errorf("expected 0 files in StageState, got %d", len(output.StageState))
		}
		if len(output.WorkdirState) != 0 {
			t.Errorf("expected 0 files in WorkdirState, got %d", len(output.WorkdirState))
		}
		if len(feedbackMessages) != 2 {
			t.Fatalf("expected 2 feedback messages, got %d", len(feedbackMessages))
		}
		if feedbackMessages[1].Message != "head=0, stage=0, workdir=0" {
			t.Errorf("unexpected completion message: %s", feedbackMessages[1].Message)
		}
	})
}
