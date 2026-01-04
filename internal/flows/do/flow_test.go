// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package do

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/activities/agentloop"
	"github.com/retran/meowg1k/internal/activities/draftcontent"
	agentconfig "github.com/retran/meowg1k/internal/core/agent"
	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/internal/core/agent/tools"
	"github.com/retran/meowg1k/internal/core/retrieval"
	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/pkg/executor"
)

type stubParamsReader struct {
	taskInput string
	stdin     string
	taskErr   error
	stdinErr  error
	dryRun    bool
	dryRunErr error
}

func (s *stubParamsReader) GetTaskInput() (string, error) { return s.taskInput, s.taskErr }
func (s *stubParamsReader) GetStdIn() (string, error)     { return s.stdin, s.stdinErr }
func (s *stubParamsReader) GetDryRunFlag() (bool, error)  { return s.dryRun, s.dryRunErr }

type stubPresetResolver struct {
	preset *preset.ResolvedPreset
	err    error
}

func (s *stubPresetResolver) Get(p preset.Preset) (*preset.ResolvedPreset, error) {
	_ = p
	if s.err != nil {
		return nil, s.err
	}
	return s.preset, nil
}

type stubOutputWriter struct {
	lines []string
	err   error
}

func (s *stubOutputWriter) PrintLine(line string) error {
	if s.err != nil {
		return s.err
	}
	s.lines = append(s.lines, line)
	return nil
}

type fakeDraftFactory struct {
	lastInput *draftcontent.Input
	err       error
}

func (f *fakeDraftFactory) NewActivity() executor.Activity[*draftcontent.Input, *draftcontent.Output] {
	return func(ctx context.Context, execCtx *executor.Context, input *draftcontent.Input) (*draftcontent.Output, error) {
		_ = ctx
		_ = execCtx
		f.lastInput = input
		if f.err != nil {
			return nil, f.err
		}
		return &draftcontent.Output{
			Response: &gateway.GenerateContentResponse{
				Blocks: []gateway.ContentBlock{{Kind: gateway.ContentBlockText, Text: "ok"}},
			},
		}, nil
	}
}

type stubRetriever struct{}

func (s *stubRetriever) RetrieveContext(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) (string, error) {
	_ = ctx
	_ = queryText
	_ = snapshotPriority
	_ = topK
	_ = minScore
	return "", nil
}

func (s *stubRetriever) Search(ctx context.Context, queryText string, snapshotPriority []string, topK int, minScore float32) ([]retrieval.SearchResult, error) {
	_ = ctx
	_ = queryText
	_ = snapshotPriority
	_ = topK
	_ = minScore
	return nil, nil
}
func TestReadGoal(t *testing.T) {
	factory := &Factory{
		parametersReader: &stubParamsReader{
			taskInput: "do it",
			stdin:     "extra",
		},
	}
	goal, err := factory.readGoal()
	require.NoError(t, err)
	assert.Equal(t, "do it\n\nextra", goal)

	factory.parametersReader = &stubParamsReader{
		taskInput: "   ",
	}
	_, err = factory.readGoal()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no goal provided")
}

func TestResolvePipeline(t *testing.T) {
	factory := &Factory{}

	_, err := factory.resolvePipeline(&agentconfig.ResolvedConfig{
		Pipelines: map[string]*config.AgentPipelineConfig{},
	})
	assert.Error(t, err)

	_, err = factory.resolvePipeline(&agentconfig.ResolvedConfig{
		Pipelines: map[string]*config.AgentPipelineConfig{"default": nil},
	})
	assert.Error(t, err)

	_, err = factory.resolvePipeline(&agentconfig.ResolvedConfig{
		Pipelines: map[string]*config.AgentPipelineConfig{"default": {Steps: nil}},
	})
	assert.Error(t, err)

	cfg := &agentconfig.ResolvedConfig{
		Pipelines: map[string]*config.AgentPipelineConfig{"default": {Steps: []string{"discover"}}},
	}
	pipeline, err := factory.resolvePipeline(cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"discover"}, pipeline.Steps)
}

func TestResolveEffectivePreset(t *testing.T) {
	factory := &Factory{}

	cfg := &agentconfig.ResolvedConfig{
		Personas: map[string]*config.PersonaConfig{
			"discover": {Preset: "discover-preset"},
			"plan":     {Preset: "plan-preset"},
		},
	}
	assert.Equal(t, "discover-preset", factory.resolveEffectivePreset(cfg))

	cfg.Personas["discover"].Preset = "  "
	assert.Equal(t, "plan-preset", factory.resolveEffectivePreset(cfg))

	cfg.Personas["plan"].Preset = "   "
	assert.Equal(t, "", factory.resolveEffectivePreset(cfg))
}

func TestBuildMemoryFactsSection(t *testing.T) {
	assert.Equal(t, "", buildMemoryFactsSection(context.Background()))

	flowState := state.NewFlowState()
	ctx := state.WithFlowState(context.Background(), flowState)
	assert.Equal(t, "MEMORY FACTS:\n(none yet)", buildMemoryFactsSection(ctx))

	flowState.AddFact("alpha")
	flowState.AddFact(" ")
	section := buildMemoryFactsSection(ctx)
	assert.True(t, strings.Contains(section, "MEMORY FACTS:"))
	assert.True(t, strings.Contains(section, "- alpha"))
}

func TestBuildStepSystemPrompt(t *testing.T) {
	factory := &Factory{}
	steps := []string{"discover", "plan"}
	prompt := factory.buildStepSystemPrompt("base", "flow", steps, 0, "discover", "persona")
	assert.Contains(t, prompt, "base")
	assert.Contains(t, prompt, "flow")
	assert.Contains(t, prompt, "persona")
	assert.Contains(t, prompt, "Flow steps (in order):")
}

func TestBuildStepGoal(t *testing.T) {
	factory := &Factory{}
	flowState := state.NewFlowState()
	flowState.AddFact("remember this")
	ctx := state.WithFlowState(context.Background(), flowState)

	goal := factory.buildStepGoal(ctx, "goal", []string{"[Step]\noutput"}, "do it")
	assert.Contains(t, goal, "MEMORY FACTS:")
	assert.Contains(t, goal, "PREVIOUS STEP OUTPUTS")
	assert.Contains(t, goal, "INSTRUCTIONS")
}

func TestBuildFlowRoleLine(t *testing.T) {
	line := buildFlowRoleLine([]string{"discover", "plan"}, 0, "discover")
	assert.Contains(t, line, "Flow steps (in order):")
	assert.Contains(t, line, "Current step: Discover")

	assert.Equal(t, "", buildFlowRoleLine(nil, 0, ""))
}

func TestFormatStepOutputForNextStep(t *testing.T) {
	output := &agentloop.Output{FinalMessage: "done"}
	formatted := formatStepOutputForNextStep("discover", output)
	assert.True(t, strings.HasPrefix(formatted, "[Discover]"))
	assert.Contains(t, formatted, "done")

	output.FinalMessage = "  "
	formatted = formatStepOutputForNextStep("discover", output)
	assert.Contains(t, formatted, "(no content)")
}

func TestBuildRestartGoal(t *testing.T) {
	assert.Equal(t, "new", buildRestartGoal("old", " new "))
	assert.Equal(t, "old", buildRestartGoal("old", "  "))
}

func TestReadGoalErrors(t *testing.T) {
	factory := &Factory{
		parametersReader: &stubParamsReader{taskErr: errors.New("no task")},
	}
	_, err := factory.readGoal()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get task input")
}

func TestExecuteSingleStepSuccess(t *testing.T) {
	draftFactory := &fakeDraftFactory{}
	agentFactory, err := agentloop.NewFactory(draftFactory)
	require.NoError(t, err)

	resolver := &stubPresetResolver{preset: &preset.ResolvedPreset{Model: "model"}}
	factory := &Factory{
		presetResolver:   resolver,
		agentLoopFactory: agentFactory,
	}

	cfg := &agentconfig.ResolvedConfig{
		Personas: map[string]*config.PersonaConfig{
			"discover": {
				Preset:           "p1",
				Tools:            []string{},
				SystemPersona:    "system",
				UserInstructions: "instructions",
			},
		},
		Tools: agentconfig.Tools{ToolDescriptions: map[string]string{}},
	}

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
	ctx := state.WithFlowState(context.Background(), state.NewFlowState())
	registry := tools.NewRegistry()

	out, err := factory.executeSingleStep(ctx, flowCtx, 0, "discover", []string{"discover"}, cfg, registry, "goal", "base", "flow", nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "ok", out.FinalMessage)
	require.NotNil(t, draftFactory.lastInput)
	assert.Contains(t, draftFactory.lastInput.SystemPrompt, "base")
	assert.True(t, len(draftFactory.lastInput.Messages) > 0)
}

func TestExecuteSingleStepErrors(t *testing.T) {
	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
	ctx := state.WithFlowState(context.Background(), state.NewFlowState())
	registry := tools.NewRegistry()

	factory := &Factory{}
	cfg := &agentconfig.ResolvedConfig{
		Personas: map[string]*config.PersonaConfig{},
		Tools:    agentconfig.Tools{ToolDescriptions: map[string]string{}},
	}
	_, err := factory.executeSingleStep(ctx, flowCtx, 0, "missing", []string{"missing"}, cfg, registry, "goal", "", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "persona missing")

	draftFactory := &fakeDraftFactory{}
	agentFactory, err := agentloop.NewFactory(draftFactory)
	require.NoError(t, err)

	factory = &Factory{
		presetResolver:   &stubPresetResolver{err: errors.New("preset err")},
		agentLoopFactory: agentFactory,
	}
	cfg.Personas["discover"] = &config.PersonaConfig{
		Preset:           "p1",
		Tools:            []string{},
		SystemPersona:    "system",
		UserInstructions: "instructions",
	}
	_, err = factory.executeSingleStep(ctx, flowCtx, 0, "discover", []string{"discover"}, cfg, registry, "goal", "", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve preset")
}

func TestExecuteStepsOutputAndRestart(t *testing.T) {
	draftFactory := &fakeDraftFactory{}
	agentFactory, err := agentloop.NewFactory(draftFactory)
	require.NoError(t, err)

	outputWriter := &stubOutputWriter{}
	factory := &Factory{
		presetResolver:   &stubPresetResolver{preset: &preset.ResolvedPreset{Model: "model"}},
		agentLoopFactory: agentFactory,
		outputWriter:     outputWriter,
	}

	cfg := &agentconfig.ResolvedConfig{
		Personas: map[string]*config.PersonaConfig{
			"discover": {
				Preset:           "p1",
				Tools:            []string{},
				SystemPersona:    "system",
				UserInstructions: "instructions",
			},
			"plan": {
				Preset:           "p1",
				Tools:            []string{},
				SystemPersona:    "system",
				UserInstructions: "instructions",
			},
		},
		Tools: agentconfig.Tools{ToolDescriptions: map[string]string{}},
	}

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
	flowState := state.NewFlowState()
	ctx := state.WithFlowState(context.Background(), flowState)
	registry := tools.NewRegistry()

	restart, err := factory.executeSteps(ctx, flowCtx, []string{"discover", "plan"}, cfg, registry, "goal", "base", "")
	require.NoError(t, err)
	assert.Equal(t, "", restart)
	assert.Len(t, outputWriter.lines, 1)

	flowState.SetRestartRequest("restart now")
	restart, err = factory.executeSteps(ctx, flowCtx, []string{"discover"}, cfg, registry, "goal", "base", "")
	require.NoError(t, err)
	assert.Equal(t, "restart now", restart)
}

func TestExecuteStepsOutputError(t *testing.T) {
	draftFactory := &fakeDraftFactory{}
	agentFactory, err := agentloop.NewFactory(draftFactory)
	require.NoError(t, err)

	outputWriter := &stubOutputWriter{err: errors.New("write error")}
	factory := &Factory{
		presetResolver:   &stubPresetResolver{preset: &preset.ResolvedPreset{Model: "model"}},
		agentLoopFactory: agentFactory,
		outputWriter:     outputWriter,
	}

	cfg := &agentconfig.ResolvedConfig{
		Personas: map[string]*config.PersonaConfig{
			"discover": {
				Preset:           "p1",
				Tools:            []string{},
				SystemPersona:    "system",
				UserInstructions: "instructions",
			},
		},
		Tools: agentconfig.Tools{ToolDescriptions: map[string]string{}},
	}

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)
	ctx := state.WithFlowState(context.Background(), state.NewFlowState())
	registry := tools.NewRegistry()

	_, err = factory.executeSteps(ctx, flowCtx, []string{"discover"}, cfg, registry, "goal", "base", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to print final output")
}

func TestFactoryInitTools(t *testing.T) {
	factory := &Factory{
		presetResolver:  &stubPresetResolver{preset: &preset.ResolvedPreset{Model: "model"}},
		retrievalService: &stubRetriever{},
	}
	cfg := &agentconfig.ResolvedConfig{
		Tools: agentconfig.Tools{
			SearchDefaults: agentconfig.SearchDefaults{
				Snapshots: []string{"head"},
				TopK:      1,
				MinScore:  0.5,
			},
		},
		Safety: &config.AgentSafetyConfig{DryRun: true},
	}
	reg := factory.initTools(cfg, true, "preset")
	assert.NotNil(t, reg)
}
