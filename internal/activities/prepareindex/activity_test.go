// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package prepareindex

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/activities/scanworktree"
	"github.com/retran/meowg1k/internal/core/index"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// mockIndexService is a mock implementation of the ports.IndexService.
type mockIndexService struct {
	PrepareForProcessingFn func(ctx context.Context, workspaceState interface{}) (interface{}, error)
}

// Ensure mockIndexService implements ports.IndexService.
var _ ports.IndexService = (*mockIndexService)(nil)

// PrepareForProcessing is the mock method.
func (m *mockIndexService) PrepareForProcessing(ctx context.Context, workspaceState interface{}) (interface{}, error) {
	if m.PrepareForProcessingFn != nil {
		return m.PrepareForProcessingFn(ctx, workspaceState)
	}
	return nil, errors.New("PrepareForProcessingFn not implemented")
}

// SaveNewVersion is the mock method.
func (m *mockIndexService) SaveNewVersion(ctx context.Context, input interface{}) (interface{}, error) {
	return nil, errors.New("SaveNewVersion not implemented")
}

// FinalizeLiveSnapshots is the mock method.
func (m *mockIndexService) FinalizeLiveSnapshots(ctx context.Context, input interface{}) error {
	return errors.New("FinalizeLiveSnapshots not implemented")
}

func TestNewFactory(t *testing.T) {
	t.Run("should succeed with a valid service", func(t *testing.T) {
		mockSvc := &mockIndexService{}
		factory, err := NewFactory(mockSvc)
		if err != nil {
			t.Fatalf("expected no error, but got %v", err)
		}
		if factory == nil {
			t.Fatal("factory should not be nil")
		}
	})

	t.Run("should fail with a nil service", func(t *testing.T) {
		_, err := NewFactory(nil)
		if err == nil {
			t.Fatal("expected an error, but got nil")
		}
		expectedErr := "prepareindex.NewFactory: indexService cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', but got '%s'", expectedErr, err.Error())
		}
	})
}

func TestPrepareIndexActivitySuccess(t *testing.T) {
	mockSvc := &mockIndexService{
		PrepareForProcessingFn: func(ctx context.Context, workspaceState interface{}) (interface{}, error) {
			_ = ctx
			_ = workspaceState
			return &index.PrepareOutput{
				ExistingVersions: map[string]int64{"hash1": 1},
				ContentHashMap:   map[string]string{"a.txt": "hash1", "b.txt": "hash2"},
				FilesToProcess: []domainindex.FileToProcess{
					{FilePath: "b.txt", State: domainindex.FileState{ContentHash: "hash2", Content: []byte("b")}},
				},
			}, nil
		},
	}

	factory, err := NewFactory(mockSvc)
	require.NoError(t, err)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	output, err := activity(context.Background(), flowCtx, &Input{
		WorkspaceState: &scanworktree.Output{},
	})
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, int64(1), output.ExistingVersions["hash1"])
	assert.Equal(t, "hash2", output.ContentHashMap["b.txt"])
	assert.Len(t, output.FilesToProcess, 1)
}

func TestPrepareIndexActivityTypeError(t *testing.T) {
	mockSvc := &mockIndexService{
		PrepareForProcessingFn: func(ctx context.Context, workspaceState interface{}) (interface{}, error) {
			_ = ctx
			_ = workspaceState
			return struct{}{}, nil
		},
	}

	factory, err := NewFactory(mockSvc)
	require.NoError(t, err)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	_, err = activity(context.Background(), flowCtx, &Input{
		WorkspaceState: &scanworktree.Output{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected result type")
}

func TestPrepareIndexActivityServiceError(t *testing.T) {
	mockSvc := &mockIndexService{
		PrepareForProcessingFn: func(ctx context.Context, workspaceState interface{}) (interface{}, error) {
			_ = ctx
			_ = workspaceState
			return nil, errors.New("service error")
		},
	}

	factory, err := NewFactory(mockSvc)
	require.NoError(t, err)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	_, err = activity(context.Background(), flowCtx, &Input{
		WorkspaceState: &scanworktree.Output{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to prepare for processing")
}
