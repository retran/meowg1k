/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cleanupstaledata

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockSnapshotRepository is a mock implementation of ports.SnapshotRepository.
type mockSnapshotRepository struct {
	mu                          sync.Mutex
	ClearSnapshotLinksFn        func(ctx context.Context, name string) error
	LinkVersionToSnapshotFn     func(ctx context.Context, commitHash string, versionID int64) error
	UnlinkVersionFromSnapshotFn func(ctx context.Context, commitHash string, versionID int64) error
	GetVersionIDsForSnapshotFn  func(ctx context.Context, commitHash string) ([]int64, error)
}

// Ensure mockSnapshotRepository implements ports.SnapshotRepository
var _ ports.SnapshotRepository = (*mockSnapshotRepository)(nil)

func (m *mockSnapshotRepository) ClearSnapshotLinks(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ClearSnapshotLinksFn != nil {
		return m.ClearSnapshotLinksFn(ctx, name)
	}
	return nil
}

func (m *mockSnapshotRepository) LinkVersionToSnapshot(ctx context.Context, commitHash string, versionID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.LinkVersionToSnapshotFn != nil {
		return m.LinkVersionToSnapshotFn(ctx, commitHash, versionID)
	}
	return nil
}

func (m *mockSnapshotRepository) UnlinkVersionFromSnapshot(ctx context.Context, commitHash string, versionID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.UnlinkVersionFromSnapshotFn != nil {
		return m.UnlinkVersionFromSnapshotFn(ctx, commitHash, versionID)
	}
	return nil
}

func (m *mockSnapshotRepository) GetVersionIDsForSnapshot(ctx context.Context, commitHash string) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetVersionIDsForSnapshotFn != nil {
		return m.GetVersionIDsForSnapshotFn(ctx, commitHash)
	}
	return []int64{}, nil
}

// mockMetaRepository is a mock implementation of ports.MetaRepository.
type mockMetaRepository struct {
	mu            sync.Mutex
	DeleteValueFn func(ctx context.Context, key string) error
	GetValueFn    func(ctx context.Context, key string) ([]byte, error)
	SetValueFn    func(ctx context.Context, key string, value []byte) error
}

// Ensure mockMetaRepository implements ports.MetaRepository
var _ ports.MetaRepository = (*mockMetaRepository)(nil)

func (m *mockMetaRepository) DeleteValue(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.DeleteValueFn != nil {
		return m.DeleteValueFn(ctx, key)
	}
	return nil
}

func (m *mockMetaRepository) GetValue(ctx context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetValueFn != nil {
		return m.GetValueFn(ctx, key)
	}
	return nil, nil
}

func (m *mockMetaRepository) SetValue(ctx context.Context, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SetValueFn != nil {
		return m.SetValueFn(ctx, key, value)
	}
	return nil
}

func TestNewFactory(t *testing.T) {
	snapshotRepo := &mockSnapshotRepository{}
	metaRepo := &mockMetaRepository{}

	t.Run("should succeed with valid repositories", func(t *testing.T) {
		factory, err := NewFactory(snapshotRepo, metaRepo)
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}
		if factory == nil {
			t.Fatal("factory should not be nil")
		}
	})

	t.Run("should fail with nil snapshotRepo", func(t *testing.T) {
		_, err := NewFactory(nil, metaRepo)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if err.Error() != "cleanupstaledata.NewFactory: snapshotRepo cannot be nil" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("should fail with nil metaRepo", func(t *testing.T) {
		_, err := NewFactory(snapshotRepo, nil)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		if err.Error() != "cleanupstaledata.NewFactory: metaRepo cannot be nil" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestActivity(t *testing.T) {
	t.Run("should succeed when all dependencies succeed", func(t *testing.T) {
		// Arrange
		snapshotRepo := &mockSnapshotRepository{}
		metaRepo := &mockMetaRepository{}
		factory, _ := NewFactory(snapshotRepo, metaRepo)
		activity := factory.NewActivity()
		ctx := context.Background()

		// Capture feedback messages
		var feedbackMessages []*executor.Feedback
		feedbackHandler := func(feedback *executor.Feedback) {
			feedbackMessages = append(feedbackMessages, feedback)
		}

		executorCtx := executor.NewContext("test", feedbackHandler, executor.NewExecutor(0))

		// Act
		_, err := activity(ctx, executorCtx, struct{}{})

		// Assert
		if err != nil {
			t.Fatalf("activity returned an unexpected error: %v", err)
		}

		if len(feedbackMessages) != 2 {
			t.Fatalf("expected 2 feedback messages, but got %d", len(feedbackMessages))
		}
		if feedbackMessages[0].Message != "Cleaning stale data" || feedbackMessages[1].Message != "Cleaned stale data" {
			t.Errorf("unexpected messages: got %+v and %+v", feedbackMessages[0], feedbackMessages[1])
		}
	})

	failureCases := []struct {
		name          string
		setupMocks    func(*mockSnapshotRepository, *mockMetaRepository)
		expectedError string
	}{
		{
			name: "snapshotRepo.ClearSnapshotLinks(_head_) fails",
			setupMocks: func(sr *mockSnapshotRepository, mr *mockMetaRepository) {
				sr.ClearSnapshotLinksFn = func(ctx context.Context, name string) error {
					if name == "_head_" {
						return errors.New("db error")
					}
					return nil
				}
			},
			expectedError: "failed to clear _head_ snapshot links: db error",
		},
		{
			name: "snapshotRepo.ClearSnapshotLinks(_stage_) fails",
			setupMocks: func(sr *mockSnapshotRepository, mr *mockMetaRepository) {
				sr.ClearSnapshotLinksFn = func(ctx context.Context, name string) error {
					if name == "_stage_" {
						return errors.New("db error")
					}
					return nil
				}
			},
			expectedError: "failed to clear _stage_ snapshot links: db error",
		},
		{
			name: "metaRepo.DeleteValue(idx_dump_head) fails",
			setupMocks: func(sr *mockSnapshotRepository, mr *mockMetaRepository) {
				mr.DeleteValueFn = func(ctx context.Context, key string) error {
					if key == "idx_dump_head" {
						return errors.New("kv error")
					}
					return nil
				}
			},
			expectedError: "failed to delete idx_dump_head: kv error",
		},
	}

	for _, tc := range failureCases {
		t.Run(fmt.Sprintf("should fail when %s", tc.name), func(t *testing.T) {
			// Arrange
			snapshotRepo := &mockSnapshotRepository{}
			metaRepo := &mockMetaRepository{}
			tc.setupMocks(snapshotRepo, metaRepo)

			factory, _ := NewFactory(snapshotRepo, metaRepo)
			activity := factory.NewActivity()
			ctx := context.Background()
			executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))

			// Act
			_, err := activity(ctx, executorCtx, struct{}{})

			// Assert
			if err == nil {
				t.Fatal("expected an error, but got nil")
			}
			if err.Error() != tc.expectedError {
				t.Errorf("unexpected error message.\nExpected: %s\nGot:      %s", tc.expectedError, err.Error())
			}
		})
	}
}
