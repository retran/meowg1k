// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package do

import (
	"context"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/agentstep"
	"github.com/retran/meowg1k/internal/activities/generatecontent"
	agentconfig "github.com/retran/meowg1k/internal/core/agent"
	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type mockParams struct {
	profile      string
	systemPrompt string
	snapshots    []string
	topK         int
	minScore     float32
}

func (m *mockParams) GetTaskInput() (string, error)        { return "task", nil }
func (m *mockParams) GetProfileFlag() (string, error)      { return m.profile, nil }
func (m *mockParams) GetSystemPromptFlag() (string, error) { return m.systemPrompt, nil }
func (m *mockParams) GetSnapshotsFlag() ([]string, error)  { return m.snapshots, nil }
func (m *mockParams) GetTopKFlag() (int, error)            { return m.topK, nil }
func (m *mockParams) GetMinScoreFlag() (float32, error)    { return m.minScore, nil }

func TestApplyOverrides(t *testing.T) {
	steps := make(map[string]*agentconfig.StepConfig)
	for idx, name := range agentconfig.StepOrder {
		steps[name] = &agentconfig.StepConfig{
			Index:     idx,
			Tools:     []string{"plan"},
			ToolModes: map[string]map[string]bool{"plan": {"list": true}},
		}
	}

	cfg := &agentconfig.ResolvedConfig{
		Defaults: agentconfig.Defaults{
			Profile:      "smart",
			SystemPrompt: "default",
		},
		Tools: agentconfig.Tools{
			SearchDefaults: agentconfig.SearchDefaults{
				Snapshots: []string{"_head_"},
				TopK:      3,
				MinScore:  0.2,
			},
		},
		Steps: steps,
	}

	params := &mockParams{
		profile:      "fast",
		systemPrompt: "override",
		snapshots:    []string{"_workdir_"},
		topK:         7,
		minScore:     0.5,
	}

	factory := &Factory{parametersReader: params}
	updated, err := factory.applyOverrides(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if updated.Defaults.Profile != "fast" {
		t.Fatalf("expected profile override, got %q", updated.Defaults.Profile)
	}
	if updated.Tools.SearchDefaults.TopK != 7 {
		t.Fatalf("expected topK override, got %d", updated.Tools.SearchDefaults.TopK)
	}
}

type mockConfigResolver struct {
	cfg *config.Config
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	return m.cfg, nil
}

type mockProfileResolver struct{}

func (m *mockProfileResolver) Get(_ profile.Profile) (*profile.ResolvedProfile, error) {
	return &profile.ResolvedProfile{Model: "test"}, nil
}

type mockOutputWriter struct {
	lines []string
}

func (m *mockOutputWriter) PrintLine(line string) error {
	m.lines = append(m.lines, line)
	return nil
}

type mockWorkspaceService struct {
	root string
}

func (m *mockWorkspaceService) Get() (string, error) {
	return m.root, nil
}

type mockInvokeFactoryFlow struct {
	content string
}

func (m *mockInvokeFactoryFlow) NewActivity() executor.Activity[*generatecontent.Input, *generatecontent.Output] {
	return func(_ context.Context, _ *executor.Context, _ *generatecontent.Input) (*generatecontent.Output, error) {
		return &generatecontent.Output{Content: m.content}, nil
	}
}

func TestRunAgentFlowFinalOutput(t *testing.T) {
	cfgService, err := agentconfig.NewService(&mockConfigResolver{cfg: &config.Config{}})
	if err != nil {
		t.Fatalf("failed to create agent config service: %v", err)
	}

	invokeFactory := &mockInvokeFactoryFlow{content: `{"type":"final","content":"done","summary":"ok"}`}
	stepFactory, err := agentstep.NewFactory(invokeFactory)
	if err != nil {
		t.Fatalf("failed to create step factory: %v", err)
	}

	output := &mockOutputWriter{}
	params := &mockParams{snapshots: []string{"_workdir_"}}

	factory := &Factory{
		agentConfigService: cfgService,
		stepFactory:        stepFactory,
		parametersReader:   params,
		profileResolver:    &mockProfileResolver{},
		outputWriter:       output,
		workspaceService:   &mockWorkspaceService{root: t.TempDir()},
		filterService:      nil,
		gitService:         nil,
		queryFactory:       nil,
		invokeLLMFactory:   invokeFactory,
		indexFlowBuilder:   nil,
	}

	exec := executor.NewExecutor(2)
	flowCtx := executor.NewContext("DoFlow", executor.NoOpFeedbackHandler, exec)
	flow := factory.NewFlow()
	if err := flow(context.Background(), flowCtx); err != nil {
		t.Fatalf("flow failed: %v", err)
	}

	if len(output.lines) == 0 {
		t.Fatal("expected output to be written")
	}
}

var _ ports.OutputWriter = (*mockOutputWriter)(nil)

func TestParseVerificationStatusLine(t *testing.T) {
	passed, ok := parseVerificationStatusLine("VerificationResult: PASS")
	if !ok || !passed {
		t.Fatalf("expected PASS to be recognized, got ok=%v passed=%v", ok, passed)
	}

	passed, ok = parseVerificationStatusLine("verification: fail - missing tests")
	if !ok || passed {
		t.Fatalf("expected FAIL to be recognized, got ok=%v passed=%v", ok, passed)
	}

	_, ok = parseVerificationStatusLine("verification: maybe")
	if ok {
		t.Fatal("expected unrecognized verification value to be ignored")
	}

	_, ok = parseVerificationStatusLine("status: pass")
	if ok {
		t.Fatal("expected missing prefix to be ignored")
	}
}

func TestParseVerificationStatus(t *testing.T) {
	content := "note\nVerificationResult: FAIL\nmore"
	passed, ok := parseVerificationStatus(content)
	if !ok || passed {
		t.Fatalf("expected FAIL in content, got ok=%v passed=%v", ok, passed)
	}

	passed, ok = parseVerificationStatus("no status line")
	if ok || !passed {
		t.Fatalf("expected no status to return ok=false passed=true, got ok=%v passed=%v", ok, passed)
	}
}

func TestExtractFailureTasks(t *testing.T) {
	content := "VerificationResult: FAIL\nFailureTasks:\n- add tests\n* fix lint\n  document behavior\n\nOther"
	tasks := extractFailureTasks(content)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	if tasks[0] != "add tests" || tasks[1] != "fix lint" || tasks[2] != "document behavior" {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
}

func TestBuildRetryGoal(t *testing.T) {
	goal := "Ship it"
	output := "VerificationResult: FAIL"
	updated := buildRetryGoal(goal, nil, output)
	if !strings.Contains(updated, "Follow-up task: address verification failures.") {
		t.Fatalf("expected follow-up task line, got %q", updated)
	}
	if !strings.Contains(updated, output) {
		t.Fatalf("expected verification output in retry goal, got %q", updated)
	}

	tasks := []string{"add tests", "update docs"}
	updated = buildRetryGoal(goal, tasks, output)
	if !strings.Contains(updated, "Follow-up tasks from verification:") {
		t.Fatalf("expected tasks header, got %q", updated)
	}
	if !strings.Contains(updated, "- add tests") || !strings.Contains(updated, "- update docs") {
		t.Fatalf("expected task list, got %q", updated)
	}
}

func TestParseVerificationResult(t *testing.T) {
	result := parseVerificationResult("VerificationResult: PASS\nFailureTasks:\n- should ignore")
	if !result.Passed || len(result.Tasks) != 0 {
		t.Fatalf("expected pass with no tasks, got passed=%v tasks=%v", result.Passed, result.Tasks)
	}

	content := "Verification: FAIL\nFailureTasks:\n- fix issue\n"
	result = parseVerificationResult(content)
	if result.Passed || len(result.Tasks) != 1 || result.Tasks[0] != "fix issue" {
		t.Fatalf("unexpected fail result: passed=%v tasks=%v", result.Passed, result.Tasks)
	}
}
