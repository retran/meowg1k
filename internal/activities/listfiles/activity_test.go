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
















}	}		t.Fatalf("unexpected files: %#v", out.Files)	if len(out.Files) != 2 || out.Files[0] != "internal/a.go" || out.Files[1] != "internal/b.go" {	}		t.Fatalf("expected output")	if out == nil {	}		t.Fatalf("unexpected error: %v", err)	if err != nil {	out, err := f.NewActivity()(ctx, execCtx, &Input{Dir: "internal"})	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))	ctx := context.Background()	f := NewFactory(svc)	}}		"cmd/root.go":             {},		"internal/sub/deeper/d.go": {},		"internal/sub/c.go":       {},		"internal/b.go":           {},		"internal/a.go":           {},	svc := &mockProjectStateService{workdir: map[string]domainindex.FileState{func TestListFiles_NonRecursiveSubdir(t *testing.T) {}	}		t.Fatalf("unexpected files: %#v", out.Files)	if len(out.Files) != 1 || out.Files[0] != "README.md" {	}		t.Fatalf("expected output")	if out == nil {	}		t.Fatalf("unexpected error: %v", err)	if err != nil {	out, err := f.NewActivity()(ctx, execCtx, &Input{Dir: "."})	execCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, executor.NewExecutor(0))	ctx := context.Background()	f := NewFactory(svc)	}}		"internal/ignored_nested.go": {},		"cmd/sub/ignored_nested.go":  {},		"cmd/root.go":               {},		"internal/activities/b.go":   {},		"internal/activities/a.go":   {},		"README.md":                 {},	svc := &mockProjectStateService{workdir: map[string]domainindex.FileState{func TestListFiles_NonRecursiveRoot(t *testing.T) {}	return m.workdir, nil	_ = ctxfunc (m *mockProjectStateService) GetWorkdirState(ctx context.Context) (map[string]domainindex.FileState, error) {}	return nil, nil	_ = ctxfunc (m *mockProjectStateService) GetStagingState(ctx context.Context) (map[string]domainindex.FileState, error) {}	return nil, nil	_ = ctxfunc (m *mockProjectStateService) GetHeadState(ctx context.Context) (map[string]domainindex.FileState, error) {}	workdir map[string]domainindex.FileStatetype mockProjectStateService struct {)	"github.com/retran/meowg1k/pkg/executor"	domainindex "github.com/retran/meowg1k/internal/domain/index"
import (
	"context"
	"testing"