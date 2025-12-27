//go:build doflow
// +build doflow

// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package do

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/retran/meowg1k/internal/activities/generatecontent"
	queryactivity "github.com/retran/meowg1k/internal/activities/query"
	agentconfig "github.com/retran/meowg1k/internal/core/agent"
	"github.com/retran/meowg1k/internal/core/retrieval"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type noopFilter struct{}

func (n *noopFilter) IsIgnoredFile(_ string) bool { return false }

type ignoreFilter struct{}

func (i *ignoreFilter) IsIgnoredFile(path string) bool {
	return path == "ignored.txt"
}

type mockGitTooling struct {
	status string
	diff   string
	show   string
	log    string
	branch string
}

func (m *mockGitTooling) Status() (string, error)          { return m.status, nil }
func (m *mockGitTooling) Diff(_, _ string) (string, error) { return m.diff, nil }
func (m *mockGitTooling) Show(_ string) (string, error)    { return m.show, nil }
func (m *mockGitTooling) Log(_ int, _ string) (string, error) {
	return m.log, nil
}
func (m *mockGitTooling) Branches() ([]string, error) { return []string{"main"}, nil }
func (m *mockGitTooling) CurrentBranch() (string, error) {
	return m.branch, nil
}
func (m *mockGitTooling) Stage(_ []string) (string, error) { return "staged", nil }
func (m *mockGitTooling) Commit(_ string) (string, error)  { return "committed", nil }
func (m *mockGitTooling) HeadHash() (string, error)        { return "abc123", nil }

type mockQueryFactory struct {
	output *queryactivity.Output
}

func (m *mockQueryFactory) NewActivity() executor.Activity[*queryactivity.Input, *queryactivity.Output] {
	return func(_ context.Context, _ *executor.Context, _ *queryactivity.Input) (*queryactivity.Output, error) {
		return m.output, nil
	}
}

type mockInvokeFactory struct {
	content string
}

func (m *mockInvokeFactory) NewActivity() executor.Activity[*generatecontent.Input, *generatecontent.Output] {
	return func(_ context.Context, _ *executor.Context, _ *generatecontent.Input) (*generatecontent.Output, error) {
		return &generatecontent.Output{Content: m.content}, nil
	}
}

func TestWorkspaceReplaceSingle(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("alpha beta alpha"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	runner := NewToolRunner(tmpDir, &noopFilter{}, nil, nil, nil, nil, agentconfig.SearchDefaults{})

	params := map[string]interface{}{
		"path":       "sample.txt",
		"old_text":   "alpha",
		"new_text":   "ALPHA",
		"occurrence": "single",
	}
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("failed to marshal params: %v", err)
	}

	_, err = runner.RunTool(context.Background(), nil, "workspace", "replace", raw, nil, "")
	if err == nil {
		t.Fatal("expected error for multiple matches with single occurrence")
	}

	if err := os.WriteFile(filePath, []byte("alpha beta"), 0o600); err != nil {
		t.Fatalf("failed to rewrite file: %v", err)
	}

	_, err = runner.RunTool(context.Background(), nil, "workspace", "replace", raw, nil, "")
	if err != nil {
		t.Fatalf("expected replace to succeed, got error: %v", err)
	}
}

func TestWorkspaceReplaceFirst(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("one one"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	runner := NewToolRunner(tmpDir, &noopFilter{}, nil, nil, nil, nil, agentconfig.SearchDefaults{})
	params := map[string]interface{}{
		"path":       "sample.txt",
		"old_text":   "one",
		"new_text":   "two",
		"occurrence": "first",
	}
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("failed to marshal params: %v", err)
	}

	_, err = runner.RunTool(context.Background(), nil, "workspace", "replace", raw, nil, "")
	if err != nil {
		t.Fatalf("replace failed: %v", err)
	}
}

func TestWorkspaceWriteReadListExists(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewToolRunner(tmpDir, &noopFilter{}, nil, nil, nil, nil, agentconfig.SearchDefaults{})

	writeParams := map[string]interface{}{
		"path":    "dir/file.txt",
		"content": "hello world",
	}
	writeRaw, err := json.Marshal(writeParams)
	if err != nil {
		t.Fatalf("failed to marshal write params: %v", err)
	}

	_, err = runner.RunTool(context.Background(), nil, "workspace", "write", writeRaw, nil, "")
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	readParams := map[string]interface{}{
		"path":       "dir/file.txt",
		"start_line": 1,
		"end_line":   1,
	}
	readRaw, err := json.Marshal(readParams)
	if err != nil {
		t.Fatalf("failed to marshal read params: %v", err)
	}

	readResult, err := runner.RunTool(context.Background(), nil, "workspace", "read", readRaw, nil, "")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if readResult == nil {
		t.Fatal("expected read result")
	}

	existsParams := map[string]interface{}{
		"path": "dir/file.txt",
	}
	existsRaw, err := json.Marshal(existsParams)
	if err != nil {
		t.Fatalf("failed to marshal exists params: %v", err)
	}

	existsResult, err := runner.RunTool(context.Background(), nil, "workspace", "exists", existsRaw, nil, "")
	if err != nil {
		t.Fatalf("exists failed: %v", err)
	}
	if existsResult == nil {
		t.Fatal("expected exists result")
	}

	listParams := map[string]interface{}{
		"path":  "dir",
		"depth": 1,
	}
	listRaw, err := json.Marshal(listParams)
	if err != nil {
		t.Fatalf("failed to marshal list params: %v", err)
	}

	listResult, err := runner.RunTool(context.Background(), nil, "workspace", "list", listRaw, nil, "")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if listResult == nil {
		t.Fatal("expected list result")
	}
}

func TestWorkspaceMkdirStatDelete(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewToolRunner(tmpDir, &noopFilter{}, nil, nil, nil, nil, agentconfig.SearchDefaults{})

	mkdirParams := map[string]interface{}{
		"path":    "nested/dir",
		"parents": true,
	}
	mkdirRaw, err := json.Marshal(mkdirParams)
	if err != nil {
		t.Fatalf("failed to marshal mkdir params: %v", err)
	}

	_, err = runner.RunTool(context.Background(), nil, "workspace", "mkdir", mkdirRaw, nil, "")
	if err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	statParams := map[string]interface{}{
		"path": "nested/dir",
	}
	statRaw, err := json.Marshal(statParams)
	if err != nil {
		t.Fatalf("failed to marshal stat params: %v", err)
	}

	statResult, err := runner.RunTool(context.Background(), nil, "workspace", "stat", statRaw, nil, "")
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if statResult == nil {
		t.Fatal("expected stat result")
	}

	deleteParams := map[string]interface{}{
		"path":      "nested",
		"recursive": true,
	}
	deleteRaw, err := json.Marshal(deleteParams)
	if err != nil {
		t.Fatalf("failed to marshal delete params: %v", err)
	}

	_, err = runner.RunTool(context.Background(), nil, "workspace", "delete", deleteRaw, nil, "")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}

func TestPlanToolAddCompleteList(t *testing.T) {
	runner := NewToolRunner(t.TempDir(), &noopFilter{}, nil, nil, nil, nil, agentconfig.SearchDefaults{})

	addParams := map[string]interface{}{
		"text": "do the thing",
	}
	addRaw, err := json.Marshal(addParams)
	if err != nil {
		t.Fatalf("failed to marshal add params: %v", err)
	}

	_, err = runner.RunTool(context.Background(), nil, "plan", "add", addRaw, nil, "")
	if err != nil {
		t.Fatalf("plan add failed: %v", err)
	}

	completeParams := map[string]interface{}{
		"task_id": 1,
	}
	completeRaw, err := json.Marshal(completeParams)
	if err != nil {
		t.Fatalf("failed to marshal complete params: %v", err)
	}

	_, err = runner.RunTool(context.Background(), nil, "plan", "complete", completeRaw, nil, "")
	if err != nil {
		t.Fatalf("plan complete failed: %v", err)
	}

	_, err = runner.RunTool(context.Background(), nil, "plan", "list", json.RawMessage(`{}`), nil, "")
	if err != nil {
		t.Fatalf("plan list failed: %v", err)
	}
}

func TestGitToolModes(t *testing.T) {
	gitSvc := &mockGitTooling{
		status: "M file.txt",
		diff:   "diff --git a/file b/file",
		show:   "commit abc123",
		log:    "abc123 fix",
		branch: "main",
	}
	runner := NewToolRunner(t.TempDir(), &noopFilter{}, gitSvc, nil, nil, nil, agentconfig.SearchDefaults{})

	_, err := runner.RunTool(context.Background(), nil, "git", "status", json.RawMessage(`{}`), nil, "")
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	_, err = runner.RunTool(context.Background(), nil, "git", "diff", json.RawMessage(`{"ref":"HEAD"}`), nil, "")
	if err != nil {
		t.Fatalf("git diff failed: %v", err)
	}
	_, err = runner.RunTool(context.Background(), nil, "git", "show", json.RawMessage(`{"ref":"HEAD"}`), nil, "")
	if err != nil {
		t.Fatalf("git show failed: %v", err)
	}
	_, err = runner.RunTool(context.Background(), nil, "git", "log", json.RawMessage(`{"limit":5}`), nil, "")
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	_, err = runner.RunTool(context.Background(), nil, "git", "branch", json.RawMessage(`{}`), nil, "")
	if err != nil {
		t.Fatalf("git branch failed: %v", err)
	}
	_, err = runner.RunTool(context.Background(), nil, "git", "current_branch", json.RawMessage(`{}`), nil, "")
	if err != nil {
		t.Fatalf("git current_branch failed: %v", err)
	}
}

func TestSearchTool(t *testing.T) {
	queryOutput := &queryactivity.Output{
		Results: []retrieval.SearchResult{
			{
				FilePath:    "file.go",
				StartLine:   1,
				EndLine:     2,
				Score:       0.9,
				TextContent: "package main",
			},
		},
	}
	queryFactory := &mockQueryFactory{output: queryOutput}

	exec := executor.NewExecutor(1)
	execCtx := executor.NewContext("tool", executor.NoOpFeedbackHandler, exec)

	runner := NewToolRunner(t.TempDir(), &noopFilter{}, nil, queryFactory, nil, nil, agentconfig.SearchDefaults{
		Snapshots: []string{"_workdir_"},
		TopK:      5,
		MinScore:  0.4,
	})

	params := json.RawMessage(`{"query_text":"main"}`)
	_, err := runner.RunTool(context.Background(), execCtx, "search", "embeddings", params, nil, "")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
}

func TestSummarizeToolText(t *testing.T) {
	exec := executor.NewExecutor(1)
	execCtx := executor.NewContext("tool", executor.NoOpFeedbackHandler, exec)

	invokeFactory := &mockInvokeFactory{content: "summary"}
	runner := NewToolRunner(t.TempDir(), &noopFilter{}, nil, nil, invokeFactory, nil, agentconfig.SearchDefaults{})

	params := json.RawMessage(`{"text":"hello"}`)
	_, err := runner.RunTool(context.Background(), execCtx, "summarize", "text", params, &profile.ResolvedProfile{Model: "test"}, "Summarize")
	if err != nil {
		t.Fatalf("summarize failed: %v", err)
	}
}

func TestSearchToolTriggersIndex(t *testing.T) {
	queryOutput := &queryactivity.Output{
		Results: []retrieval.SearchResult{},
	}
	queryFactory := &mockQueryFactory{output: queryOutput}

	indexCalls := 0
	indexFlowBuilder := func() (executor.Flow, error) {
		return func(_ context.Context, _ *executor.Context) error {
			indexCalls++
			return nil
		}, nil
	}

	exec := executor.NewExecutor(1)
	execCtx := executor.NewContext("tool", executor.NoOpFeedbackHandler, exec)

	runner := NewToolRunner(t.TempDir(), &noopFilter{}, &mockGitTooling{}, queryFactory, nil, indexFlowBuilder, agentconfig.SearchDefaults{
		Snapshots: []string{"_workdir_"},
		TopK:      5,
		MinScore:  0.4,
	})
	runner.workspaceDirty = true

	params := json.RawMessage(`{"query_text":"main"}`)
	_, err := runner.RunTool(context.Background(), execCtx, "search", "embeddings", params, nil, "")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if indexCalls == 0 {
		t.Fatal("expected index flow to be invoked")
	}
}

func TestCommandToolRun(t *testing.T) {
	exec := executor.NewExecutor(1)
	execCtx := executor.NewContext("tool", executor.NoOpFeedbackHandler, exec)

	runner := NewToolRunner(t.TempDir(), &noopFilter{}, nil, nil, nil, nil, agentconfig.SearchDefaults{})
	params := json.RawMessage(`{"command":"echo hello"}`)
	_, err := runner.RunTool(context.Background(), execCtx, "command", "run", params, nil, "")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
}

func TestCommandToolMissingCommand(t *testing.T) {
	exec := executor.NewExecutor(1)
	execCtx := executor.NewContext("tool", executor.NoOpFeedbackHandler, exec)

	runner := NewToolRunner(t.TempDir(), &noopFilter{}, nil, nil, nil, nil, agentconfig.SearchDefaults{})
	_, err := runner.RunTool(context.Background(), execCtx, "command", "run", json.RawMessage(`{"command":""}`), nil, "")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestPatchToolInvalidDiff(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	exec := executor.NewExecutor(1)
	execCtx := executor.NewContext("tool", executor.NoOpFeedbackHandler, exec)
	runner := NewToolRunner(tmpDir, &noopFilter{}, nil, nil, nil, nil, agentconfig.SearchDefaults{})
	paramsMap := map[string]interface{}{"diff": "invalid diff"}
	raw, err := json.Marshal(paramsMap)
	if err != nil {
		t.Fatalf("failed to marshal patch params: %v", err)
	}
	_, err = runner.RunTool(context.Background(), execCtx, "patch", "apply", raw, nil, "")
	if err == nil {
		t.Fatal("expected patch apply to fail")
	}
}

func TestWorkspaceReadIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "ignored.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	runner := NewToolRunner(tmpDir, &ignoreFilter{}, nil, nil, nil, nil, agentconfig.SearchDefaults{})
	params := json.RawMessage(`{"path":"ignored.txt"}`)
	_, err := runner.RunTool(context.Background(), nil, "workspace", "read", params, nil, "")
	if err == nil {
		t.Fatal("expected error for ignored file")
	}
}

func TestResolveWorkspacePathOutsideRoot(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewToolRunner(tmpDir, &noopFilter{}, nil, nil, nil, nil, agentconfig.SearchDefaults{})
	_, err := runner.resolveWorkspacePath("../outside.txt")
	if err == nil {
		t.Fatal("expected error for path outside workspace")
	}
}

var _ ports.FilterService = (*noopFilter)(nil)
var _ ports.GitToolingService = (*mockGitTooling)(nil)
