// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package summarizechanges

import (
	"context"
	"testing"

	"github.com/retran/meowg1k/internal/activities/summarizefilechanges"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestNewFactory(t *testing.T) {
	mockFactory := (*summarizefilechanges.Factory)(nil)
	factory, err := NewFactory(mockFactory)
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestNewFactoryNil(t *testing.T) {
	factory, err := NewFactory(nil)
	if err == nil {
		t.Error("Expected error when NewFactory called with nil")
	}
	if factory != nil {
		t.Error("Expected nil factory when error returned")
	}
}

func TestActivityNilInput(t *testing.T) {
	factory, err := NewFactory((*summarizefilechanges.Factory)(nil))
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}
	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	_, err = activity(ctx, execCtx, nil)
	if err == nil {
		t.Error("Expected error for nil input, got nil")
	}
}

func TestActivitySuccess(t *testing.T) {
	factory, err := NewFactory((*summarizefilechanges.Factory)(nil))
	if err != nil {
		t.Fatalf("NewFactory failed: %v", err)
	}

	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)

	input := &Input{
		Changes: []*git.FileChange{},
	}

	output, err := activity(ctx, execCtx, input)
	if err != nil {
		t.Errorf("Activity failed: %v", err)
	}

	if output.Summaries == nil {
		t.Error("Expected summaries to be non-nil")
	}
}
