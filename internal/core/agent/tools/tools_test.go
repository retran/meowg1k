package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/activities/control"
	"github.com/retran/meowg1k/internal/activities/deletefile"
	"github.com/retran/meowg1k/internal/activities/editfile"
	"github.com/retran/meowg1k/internal/activities/getdiff"
	"github.com/retran/meowg1k/internal/activities/getplan"
	"github.com/retran/meowg1k/internal/activities/gitundo"
	"github.com/retran/meowg1k/internal/activities/listfiles"
	"github.com/retran/meowg1k/internal/activities/memorize"
	"github.com/retran/meowg1k/internal/activities/movefile"
	"github.com/retran/meowg1k/internal/activities/plan"
	"github.com/retran/meowg1k/internal/activities/readfile"
	"github.com/retran/meowg1k/internal/activities/runshell"
	"github.com/retran/meowg1k/internal/activities/searchindex"
	"github.com/retran/meowg1k/internal/activities/summarize"
	"github.com/retran/meowg1k/internal/activities/tracktask"
	"github.com/retran/meowg1k/internal/activities/writefile"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/pkg/executor"
)

// MockExecutor matches the Executor interface.
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) ExecuteActivity(
	ctx context.Context,
	parentCtx *executor.Context,
	name string,
	activity executor.Activity[any, any],
	input any,
) (any, error) {
	args := m.Called(ctx, parentCtx, name, activity, input)
	return args.Get(0), args.Error(1)
}

func (m *MockExecutor) ExecuteFlow(ctx context.Context, name string, flow executor.Flow) error {
	args := m.Called(ctx, name, flow)
	return args.Error(0)
}

func (m *MockExecutor) WithRetryPolicy(policy *executor.RetryPolicy) executor.Executor {
	m.Called(policy)
	return m
}

func (m *MockExecutor) WithFeedbackHandler(handler executor.FeedbackHandler) executor.Executor {
	m.Called(handler)
	return m
}

// simpleMockFactory is a helper to create an ActivityFactory that returns a specific activity.
type simpleMockFactory[I any, O any] struct {
	activity executor.Activity[I, O]
}

func (f *simpleMockFactory[I, O]) NewActivity() executor.Activity[I, O] {
	return f.activity
}

func newMockFactory[I any, O any](activity executor.Activity[I, O]) executor.ActivityFactory[I, O] {
	if activity == nil {
		// Default dummy activity if none provided
		activity = func(ctx context.Context, activityCtx *executor.Context, input I) (O, error) {
			var zero O
			return zero, nil
		}
	}
	return &simpleMockFactory[I, O]{activity: activity}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	// 1. Test Register and Get
	mockHandler := func(ctx context.Context, execCtx *executor.Context, args map[string]any) (any, error) {
		return "result", nil
	}
	toolDef := gateway.ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
	}
	r.Register(Tool{Definition: toolDef, Handler: mockHandler})

	tool, ok := r.Get("test_tool")
	assert.True(t, ok)
	assert.Equal(t, "test_tool", tool.Definition.Name)

	_, ok = r.Get("non_existent")
	assert.False(t, ok)

	// 2. Test GetDefinitions
	defs := r.GetDefinitions([]string{"test_tool", "non_existent"})
	assert.Len(t, defs, 1)
	assert.Equal(t, "test_tool", defs[0].Name)

	// 3. Test ExecuteTool
	execCtx := executor.NewContext("test", nil, &MockExecutor{})
	res, err := r.ExecuteTool(context.Background(), execCtx, "test_tool", nil)
	require.NoError(t, err)
	assert.Equal(t, "result", res)

	_, err = r.ExecuteTool(context.Background(), execCtx, "non_existent", nil)
	assert.Error(t, err)
}

func TestSuggestToolName(t *testing.T) {
	r := NewRegistry()

	// Register run_shell
	r.Register(Tool{Definition: gateway.ToolDefinition{Name: "run_shell"}})

	suggestion := r.suggestToolName("execute_shell")
	assert.Equal(t, "run_shell", suggestion)

	suggestion = r.suggestToolName("run_terminal")
	assert.Equal(t, "run_shell", suggestion)

	suggestion = r.suggestToolName("unknown")
	assert.Equal(t, "", suggestion)

	// ExecuteTool with suggestion error message
	execCtx := executor.NewContext("test", nil, &MockExecutor{})
	_, err := r.ExecuteTool(context.Background(), execCtx, "execute_shell", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "did you mean run_shell")
}

func TestBindArgs(t *testing.T) {
	type TestArgs struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	input := map[string]any{
		"name": "Alice",
		"age":  30,
	}

	var target TestArgs
	err := BindArgs(input, &target)
	require.NoError(t, err)
	assert.Equal(t, "Alice", target.Name)
	assert.Equal(t, 30, target.Age)

	// Invalid input (json marshal fail - practically hard with map[string]any unless cycles or bad types)
	// We can test unmarshal fail
	inputBad := map[string]any{
		"age": "not an int",
	}
	var targetBad TestArgs
	err = BindArgs(inputBad, &targetBad)
	assert.Error(t, err)
}

func TestRegisterStandardTools(t *testing.T) {
	r := NewRegistry()
	deps := &ToolDependencies{
		ReadFile:   newMockFactory[*readfile.Input, *readfile.Output](nil),
		WriteFile:  newMockFactory[*writefile.Input, *writefile.Output](nil),
		EditFile:   newMockFactory[*editfile.Input, *editfile.Output](nil),
		MoveFile:   newMockFactory[*movefile.Input, *movefile.Output](nil),
		DeleteFile: newMockFactory[*deletefile.Input, *deletefile.Output](nil),
		GitUndo:    newMockFactory[*gitundo.Input, *gitundo.Output](nil),
		RunShell:   newMockFactory[*runshell.Input, *runshell.Output](nil),
		ListFiles:  newMockFactory[*listfiles.Input, *listfiles.Output](nil),
		SearchCode: newMockFactory[*searchindex.Input, *searchindex.Output](nil),
		GetDiff:    newMockFactory[*getdiff.Input, *getdiff.Output](nil),
		Memorize:   newMockFactory[*memorize.Input, *memorize.Output](nil),
		Plan:       newMockFactory[*plan.Input, *plan.Output](nil),
		GetPlan:    newMockFactory[*getplan.Input, *getplan.Output](nil),
		TrackTask:  newMockFactory[*tracktask.Input, *tracktask.Output](nil),
		Summarize:  newMockFactory[*summarize.Input, *summarize.Output](nil),
		Restart:    newMockFactory[*control.RestartInput, *control.Output](nil),

		SearchSnapshots: []string{"master"},
		SearchTopK:      5,
		SearchMinScore:  0.7,
	}

	RegisterStandardTools(r, deps)

	expectedTools := []string{
		"file_read", "file_write", "file_edit", "file_move", "file_delete", "git_undo",
		"dir_list", "shell_exec", "search_text", "search_semantic", "git_diff",
		"plan_init", "plan_read", "plan_update_task", "mem_store", "util_summarize", "agent_restart",
	}

	for _, name := range expectedTools {
		_, ok := r.Get(name)
		assert.True(t, ok, "Tool %s should be registered", name)
	}
}

func TestSearchTextHandler(t *testing.T) {
	r := NewRegistry()

	mockShellActivity := func(ctx context.Context, activityCtx *executor.Context, input *runshell.Input) (*runshell.Output, error) {
		return &runshell.Output{Stdout: "found"}, nil
	}

	deps := &ToolDependencies{
		RunShell: newMockFactory[*runshell.Input, *runshell.Output](mockShellActivity),
	}

	// Manually register since search_text is private-ish/helper registration
	registerSearchTextTool(r, deps)

	tool, ok := r.Get("search_text")
	require.True(t, ok)

	// Mock Executor
	mockExecutor := &MockExecutor{}
	mockExecutor.On("ExecuteActivity", mock.Anything, mock.Anything, "search_text", mock.Anything, mock.Anything).
		Return(&runshell.Output{Stdout: "found"}, nil)

	execCtx := executor.NewContext("test", nil, mockExecutor)

	// Test basic execution
	args := map[string]any{
		"pattern": "foo",
		"path":    "src",
	}

	_, err := tool.Handler(context.Background(), execCtx, args)
	require.NoError(t, err)

	// Verify input
	// Since we are mocking ExecuteActivity, the mockShellActivity won't actually be called via ExecuteActivity
	// because MockExecutor intercepts it. But we can verify what was passed to ExecuteActivity.

	// Argument capture from mock
	call := mockExecutor.Calls[0]
	passedInput, ok := call.Arguments[4].(*runshell.Input)
	require.True(t, ok)

	// We expect grep or rg depending on system.
	// NOTE: This test might be flaky if `rg` is installed or not.
	// We can check if Args contains "foo" and "src".
	assert.Contains(t, passedInput.Args, "foo")
	assert.Contains(t, passedInput.Args, "src")
}

func TestSearchSemanticTool(t *testing.T) {
	r := NewRegistry()

	mockSearchActivity := func(ctx context.Context, activityCtx *executor.Context, input *searchindex.Input) (*searchindex.Output, error) {
		return &searchindex.Output{}, nil
	}

	deps := &ToolDependencies{
		SearchCode:      newMockFactory[*searchindex.Input, *searchindex.Output](mockSearchActivity),
		SearchSnapshots: []string{"main"},
		SearchTopK:      10,
		SearchMinScore:  0.5,
	}

	registerSearchSemanticTool(r, deps)
	tool, ok := r.Get("search_semantic")
	require.True(t, ok)

	mockExecutor := &MockExecutor{}
	mockExecutor.On("ExecuteActivity", mock.Anything, mock.Anything, "search_semantic", mock.Anything, mock.Anything).
		Return(&searchindex.Output{}, nil)

	execCtx := executor.NewContext("test", nil, mockExecutor)

	args := map[string]any{
		"query": "how to test",
	}

	_, err := tool.Handler(context.Background(), execCtx, args)
	require.NoError(t, err)

	call := mockExecutor.Calls[0]
	passedInput, ok := call.Arguments[4].(*searchindex.Input)
	require.True(t, ok)
	assert.Equal(t, "how to test", passedInput.QueryText)
	assert.Equal(t, []string{"main"}, passedInput.SnapshotPriority)
}

func TestResolveMaxResults(t *testing.T) {
	// Since resolveMaxResults is unexported, we can test it via searchTextHandler arguments
	// or rely on unit testing internal functions if we are in the same package (which we are).
	assert.Equal(t, 10, resolveMaxResults(10))
	assert.Equal(t, 5, resolveMaxResults(5.0))
	assert.Equal(t, 0, resolveMaxResults("invalid"))
	assert.Equal(t, 0, resolveMaxResults(nil))
}

func TestGenericToolHandler(t *testing.T) {
	r := NewRegistry()

	mockReadActivity := func(ctx context.Context, activityCtx *executor.Context, input *readfile.Input) (*readfile.Output, error) {
		return &readfile.Output{Content: "content"}, nil
	}

	deps := &ToolDependencies{
		ReadFile: newMockFactory[*readfile.Input, *readfile.Output](mockReadActivity),
	}

	registerFileTools(r, deps)
	tool, ok := r.Get("file_read")
	require.True(t, ok)

	mockExecutor := &MockExecutor{}
	mockExecutor.On("ExecuteActivity", mock.Anything, mock.Anything, "file_read", mock.Anything, mock.Anything).
		Return(&readfile.Output{Content: "content"}, nil)

	execCtx := executor.NewContext("test", nil, mockExecutor)

	args := map[string]any{
		"path": "test.go",
	}

	_, err := tool.Handler(context.Background(), execCtx, args)
	require.NoError(t, err)

	// Test BindArgs failure path in handler
	_, err = tool.Handler(context.Background(), execCtx, map[string]any{"path": 123}) // Invalid type
	assert.Error(t, err)
}

func TestPlanReadTool(t *testing.T) {
	r := NewRegistry()
	deps := &ToolDependencies{
		GetPlan: newMockFactory[*getplan.Input, *getplan.Output](nil),
	}

	registerPlanTools(r, deps)
	tool, ok := r.Get("plan_read")
	require.True(t, ok)

	mockExecutor := &MockExecutor{}
	mockExecutor.On("ExecuteActivity", mock.Anything, mock.Anything, "plan_read", mock.Anything, mock.Anything).
		Return(&getplan.Output{}, nil)

	execCtx := executor.NewContext("test", nil, mockExecutor)
	_, err := tool.Handler(context.Background(), execCtx, map[string]any{})
	require.NoError(t, err)
}

func TestSummarizeTool(t *testing.T) {
	r := NewRegistry()
	deps := &ToolDependencies{
		Summarize: newMockFactory[*summarize.Input, *summarize.Output](nil),
	}

	registerAgentTools(r, deps)
	tool, ok := r.Get("util_summarize")
	require.True(t, ok)

	mockExecutor := &MockExecutor{}
	mockExecutor.On("ExecuteActivity", mock.Anything, mock.Anything, "util_summarize", mock.Anything, mock.Anything).
		Return(&summarize.Output{}, nil)

	execCtx := executor.NewContext("test", nil, mockExecutor)

	args := map[string]any{
		"content": "some content",
	}
	_, err := tool.Handler(context.Background(), execCtx, args)
	require.NoError(t, err)

	// Verify input defaults
	call := mockExecutor.Calls[0]
	passedInput, ok := call.Arguments[4].(*summarize.Input)
	require.True(t, ok)
	assert.Equal(t, "text", passedInput.Type) // Default should be applied
}

func TestRunShellTool(t *testing.T) {
	r := NewRegistry()
	deps := &ToolDependencies{
		RunShell: newMockFactory[*runshell.Input, *runshell.Output](nil),
	}

	registerSystemTools(r, deps)
	tool, ok := r.Get("shell_exec")
	require.True(t, ok)

	mockExecutor := &MockExecutor{}
	mockExecutor.On("ExecuteActivity", mock.Anything, mock.Anything, "shell_exec", mock.Anything, mock.Anything).
		Return(&runshell.Output{}, nil)

	execCtx := executor.NewContext("test", nil, mockExecutor)

	args := map[string]any{
		"command": "echo",
		"args":    []string{"hello"},
	}
	_, err := tool.Handler(context.Background(), execCtx, args)
	require.NoError(t, err)
}
