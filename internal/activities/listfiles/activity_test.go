// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package listfiles

import (
	"context"
	"testing"

	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/pkg/executor"
)

type mockProjectStateService struct {
	workdir map[string]domainindex.FileState
}

func (m *mockProjectStateService) GetHeadState(ctx context.Context) (map[string]domainindex.FileState, error) {
	_ = ctx
	return nil, nil
}

func (m *mockProjectStateService) GetStagingState(ctx context.Context) (map[string]domainindex.FileState, error) {
	_ = ctx
	return nil, nil
}

func (m *mockProjectStateService) GetWorkdirState(ctx context.Context) (map[string]domainindex.FileState, error) {
	_ = ctx
	return m.workdir, nil
}

func TestListFiles_NonRecursiveRoot_IncludesDirs(t *testing.T) {
	svc := &mockProjectStateService{workdir: map[string]domainindex.FileState{
		"README.md":                {},
		"internal/activities/a.go": {},
		"internal/activities/b.go": {},
		"cmd/root.go":              {},
		"cmd/sub/nested.go":        {},
	}}

	f := NewFactory(svc)
	ctx := context.Background()
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))

	out, err := f.NewActivity()(ctx, execCtx, &Input{Dir: "."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatalf("expected output")
	}

	want := []string{"README.md", "cmd/", "internal/"}
	if len(out.Files) != len(want) {
		t.Fatalf("unexpected files: %#v", out.Files)
	}
	for i := range want {
		if out.Files[i] != want[i] {
			t.Fatalf("unexpected files: %#v", out.Files)
		}
	}
}

func TestListFiles_NonRecursiveSubdir_IncludesDirsAndFiles(t *testing.T) {
	svc := &mockProjectStateService{workdir: map[string]domainindex.FileState{
		"internal/a.go":             {},
		"internal/b.go":             {},
		"internal/sub/c.go":         {},
		"internal/sub/deeper/d.go":  {},
		"internal/sub2/x.go":        {},
		"cmd/root.go":               {},
		"cmd/sub/ignored_nested.go": {},
	}}

	f := NewFactory(svc)
	ctx := context.Background()
	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))

	out, err := f.NewActivity()(ctx, execCtx, &Input{Dir: "internal"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatalf("expected output")
	}

	want := []string{"internal/a.go", "internal/b.go", "internal/sub/", "internal/sub2/"}
	if len(out.Files) != len(want) {
		t.Fatalf("unexpected files: %#v", out.Files)
	}
	for i := range want {
		if out.Files[i] != want[i] {
			t.Fatalf("unexpected files: %#v", out.Files)
		}
	}
}
