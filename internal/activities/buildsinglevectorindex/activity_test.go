/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUTHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package buildsinglevectorindex

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockVectorIndexService is a mock implementation of the ports.VectorIndexService.
type mockVectorIndexService struct {
	mu             sync.Mutex
	BuildAndSaveFn func(snapshotName string) error
	calls          map[string][][]interface{}
}

// Ensure mockVectorIndexService implements ports.VectorIndexService
var _ ports.VectorIndexService = (*mockVectorIndexService)(nil)

func newMockVectorIndexService() *mockVectorIndexService {
	return &mockVectorIndexService{
		calls: make(map[string][][]interface{}),
	}
}

func (m *mockVectorIndexService) BuildAndSave(snapshotName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls["BuildAndSave"] = append(m.calls["BuildAndSave"], []interface{}{snapshotName})

	if m.BuildAndSaveFn != nil {
		return m.BuildAndSaveFn(snapshotName)
	}
	return nil
}

func (m *mockVectorIndexService) AssertCalled(t *testing.T, methodName string, expectedArgs ...interface{}) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	calls, ok := m.calls[methodName]
	if !ok {
		t.Errorf("expected method '%s' to be called, but it was not", methodName)
		return
	}

	for _, actualArgs := range calls {
		if reflect.DeepEqual(actualArgs, expectedArgs) {
			return // A matching call was found.
		}
	}

	t.Errorf("method '%s' was called, but not with the expected arguments.\nExpected: %v\nGot:      %v", methodName, expectedArgs, calls)
}

func TestNewFactory(t *testing.T) {
	t.Run("should return factory when service is not nil", func(t *testing.T) {
		mockSvc := newMockVectorIndexService()

		factory, err := NewFactory(mockSvc)
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}
		if factory == nil {
			t.Fatal("expected factory to be not nil")
		}
	})

	t.Run("should return error when service is nil", func(t *testing.T) {
		factory, err := NewFactory(nil)

		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if factory != nil {
			t.Fatal("expected factory to be nil")
		}
		expectedErr := "buildsinglevectorindex.NewFactory: vectorIndexSvc cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', but got '%s'", expectedErr, err.Error())
		}
	})
}

func TestActivity(t *testing.T) {
	t.Run("should succeed and send correct messages", func(t *testing.T) {
		mockSvc := newMockVectorIndexService()
		mockSvc.BuildAndSaveFn = func(snapshotName string) error {
			return nil
		}
		factory, _ := NewFactory(mockSvc)
		activity := factory.NewActivity()
		ctx := context.Background()

		// Capture feedback messages
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}

		executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))
		snapshotName := "test_snapshot_success"

		result, err := activity(ctx, executorCtx, snapshotName)
		if err != nil {
			t.Fatalf("activity returned an unexpected error: %v", err)
		}
		if !reflect.DeepEqual(result, struct{}{}) {
			t.Errorf("expected empty struct, but got: %+v", result)
		}

		mockSvc.AssertCalled(t, "BuildAndSave", snapshotName)

		if len(feedbackMessages) != 2 {
			t.Fatalf("expected 2 feedback messages, but got %d", len(feedbackMessages))
		}

		// Check messages without comparing timestamps.
		if feedbackMessages[0].Status != executor.StatusRunning || feedbackMessages[0].Message != fmt.Sprintf("Building index: %s", snapshotName) {
			t.Errorf("unexpected running message.\nExpected Status: %v, Message: %s\nGot Status:      %v, Message: %s",
				executor.StatusRunning, fmt.Sprintf("Building index: %s", snapshotName), feedbackMessages[0].Status, feedbackMessages[0].Message)
		}
		if feedbackMessages[1].Status != executor.StatusCompleted || feedbackMessages[1].Message != fmt.Sprintf("Built index: %s", snapshotName) {
			t.Errorf("unexpected completed message.\nExpected Status: %v, Message: %s\nGot Status:      %v, Message: %s",
				executor.StatusCompleted, fmt.Sprintf("Built index: %s", snapshotName), feedbackMessages[1].Status, feedbackMessages[1].Message)
		}
	})

	t.Run("should fail and return error from service", func(t *testing.T) {
		serviceErr := errors.New("build failed")
		mockSvc := newMockVectorIndexService()
		mockSvc.BuildAndSaveFn = func(snapshotName string) error {
			return serviceErr
		}
		factory, _ := NewFactory(mockSvc)
		activity := factory.NewActivity()
		ctx := context.Background()

		// Capture feedback messages
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}

		executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))
		snapshotName := "test_snapshot_fail"

		_, err := activity(ctx, executorCtx, snapshotName)

		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if !errors.Is(err, serviceErr) {
			t.Fatalf("expected error to wrap '%v', but it did not", serviceErr)
		}

		expectedErrMsg := fmt.Sprintf("failed to build vector index for %s: %s", snapshotName, serviceErr.Error())
		if err.Error() != expectedErrMsg {
			t.Errorf("unexpected error message.\nExpected: %s\nGot:      %s", expectedErrMsg, err.Error())
		}

		mockSvc.AssertCalled(t, "BuildAndSave", snapshotName)

		if len(feedbackMessages) != 1 {
			t.Fatalf("expected 1 feedback message, but got %d", len(feedbackMessages))
		}

		// Check message without comparing timestamp.
		if feedbackMessages[0].Status != executor.StatusRunning || feedbackMessages[0].Message != fmt.Sprintf("Building index: %s", snapshotName) {
			t.Errorf("unexpected running message.\nExpected Status: %v, Message: %s\nGot Status:      %v, Message: %s",
				executor.StatusRunning, fmt.Sprintf("Building index: %s", snapshotName), feedbackMessages[0].Status, feedbackMessages[0].Message)
		}
	})
}
