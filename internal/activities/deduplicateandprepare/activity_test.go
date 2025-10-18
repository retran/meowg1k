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

package deduplicateandprepare

import (
	"context"
	"errors"
	"testing"

	"github.com/retran/meowg1k/internal/ports"
)

// mockIndexService is a mock implementation of the ports.IndexService.
type mockIndexService struct {
	PrepareForProcessingFn func(ctx context.Context, workspaceState interface{}) (interface{}, error)
}

// Ensure mockIndexService implements ports.IndexService
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
		expectedErr := "deduplicateandprepare.NewFactory: indexService cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("expected error message '%s', but got '%s'", expectedErr, err.Error())
		}
	})
}
