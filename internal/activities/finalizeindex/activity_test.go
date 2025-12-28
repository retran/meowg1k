// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package finalizeindex

import (
	"testing"

	"github.com/retran/meowg1k/internal/core/index"
)

// Note: The tests for `Activity` are commented out because they depend on `index.Service`,
// which is a concrete type and cannot be easily mocked. For these tests to work,
// `index.Service` should be refactored to an interface (e.g., `ports.IndexService`)
// so that a mock implementation can be provided for testing.

/*
// mockIndexService would be a mock implementation of the `ports.IndexService` interface.
// Uncomment this once index.Service is refactored to an interface.
type mockIndexService struct {
    FinalizeLiveSnapshotsFn func(ctx context.Context, input *index.FinalizeInput) error
}

func (m *mockIndexService) FinalizeLiveSnapshots(ctx context.Context, input *index.FinalizeInput) error {
    if m.FinalizeLiveSnapshotsFn != nil {
        return m.FinalizeLiveSnapshotsFn(ctx, input)
    }
    return nil
}
*/

func TestNewFactory(t *testing.T) {
	t.Run("should fail with a nil service", func(t *testing.T) {
		_, err := NewFactory(nil)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		expectedErr := "finalizeindex.NewFactory: indexService cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', but got '%s'", expectedErr, err.Error())
		}
	})

	// NOTE: Cannot test successful factory creation because index.Service
	// is a concrete type that cannot be easily mocked. This would be
	// testable once index.Service is refactored to an interface.
	// Silence unused import warning:
	_ = (*index.Service)(nil)
}

// NOTE: TestActivity is commented out because it requires index.Service to be refactored
// to an interface (e.g., ports.IndexService) to enable proper mocking.
// The tests below show the correct pattern with feedback handlers instead of GetMessages().

/*
func TestActivity(t *testing.T) {
    mockSvc := &mockIndexService{}

    ctx := context.Background()

    t.Run("should succeed when service call is successful", func(t *testing.T) {
        // Arrange
        var feedbackMessages []*executor.Feedback
        feedbackHandler := func(feedback *executor.Feedback) {
            feedbackMessages = append(feedbackMessages, feedback)
        }

        executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))
        input := &Input{
            ScanResult:       &scanworktree.Output{},
            ExistingVersions: map[string]int64{"hash1": 1},
            NewVersions:      map[string]int64{"hash2": 2},
        }
        mockSvc.FinalizeLiveSnapshotsFn = func(ctx context.Context, in *index.FinalizeInput) error {
            return nil
        }
        // This line would work once NewFactory accepts an interface
        factory, _ := NewFactory(mockSvc)
        activity := factory.NewActivity()

        // Act
        _, err := activity(ctx, executorCtx, input)

        // Assert
        if err != nil {
            t.Fatalf("activity failed: %v", err)
        }
        // Use feedback handler instead of GetMessages()
        if len(feedbackMessages) != 2 || feedbackMessages[1].Message != "Finalized snapshots (existing: 1, new: 1)" {
            t.Errorf("unexpected completion message: %+v", feedbackMessages)
        }
    })

    t.Run("should fail when service returns an error", func(t *testing.T) {
        // Arrange
        executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))
        input := &Input{}
        serviceErr := errors.New("db finalize error")
        mockSvc.FinalizeLiveSnapshotsFn = func(ctx context.Context, in *index.FinalizeInput) error {
            return serviceErr
        }
        // This line would work once NewFactory accepts an interface
        factory, _ := NewFactory(mockSvc)
        activity := factory.NewActivity()

        // Act
        _, err := activity(ctx, executorCtx, input)

        // Assert
        if err == nil {
            t.Fatal("expected an error, but got nil")
        }
        if !errors.Is(err, serviceErr) {
            t.Errorf("expected error to wrap '%v'", serviceErr)
        }
    })
}
*/

// TestActivity_Placeholder serves as a reminder that the activity logic is currently untestable
// due to the direct dependency on the concrete `index.Service` type.
func TestActivity_Placeholder(t *testing.T) {
	t.Log("Skipping TestActivity for finalizeindex. Refactor index.Service to an interface to enable testing.")
}
