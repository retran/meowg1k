// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package do implements the 'do' flow which orchestrates agent execution.
package do

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/retran/meowg1k/internal/activities/agentloop"
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
	agentconfig "github.com/retran/meowg1k/internal/core/agent"
	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/internal/core/agent/tools"
	"github.com/retran/meowg1k/internal/core/retrieval"
	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// Factory builds 'do' flows.
type Factory struct {
	configService       *agentconfig.Service
	presetResolver      ports.PresetResolver
	workspaceService    ports.WorkspaceService
	gitToolingService   ports.GitToolingService
	projectStateService ports.ProjectStateService
	retrievalService    retrieval.Retriever
	gatewayFactory      ports.GenerationGatewayFactory
	agentLoopFactory    *agentloop.Factory
	parametersReader    CommandParametersReader
	outputWriter        ports.OutputWriter
}

// CommandParametersReader reads command parameters and flags.
type CommandParametersReader interface {
	GetTaskInput() (string, error)
	GetStdIn() (string, error)
	GetDryRunFlag() (bool, error)
}

// NewFactory creates a new do flow factory.
func NewFactory(
	configService *agentconfig.Service,
	presetResolver ports.PresetResolver,
	workspaceService ports.WorkspaceService,
	gitToolingService ports.GitToolingService,
	projectStateService ports.ProjectStateService,
	retrievalService retrieval.Retriever,
	gatewayFactory ports.GenerationGatewayFactory,
	agentLoopFactory *agentloop.Factory,
	parametersReader CommandParametersReader,
	outputWriter ports.OutputWriter,
) *Factory {
	return &Factory{
		configService:       configService,
		presetResolver:      presetResolver,
		workspaceService:    workspaceService,
		gitToolingService:   gitToolingService,
		projectStateService: projectStateService,
		retrievalService:    retrievalService,
		gatewayFactory:      gatewayFactory,
		agentLoopFactory:    agentLoopFactory,
		parametersReader:    parametersReader,
		outputWriter:        outputWriter,
	}
}

// NewFlow creates a new do flow.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		cfg, err := f.configService.Get()
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		goal, err := f.readGoal()
		if err != nil {
			return err
		}

		flowCtx.SendRunningWithDetails("Starting agent flow", fmt.Sprintf("goal_len=%d", len(goal)))

		flowState := state.NewFlowState()
		ctx = state.WithFlowState(ctx, flowState)

		dryRunOverride, err := f.parametersReader.GetDryRunFlag()
		if err != nil {
			return fmt.Errorf("failed to read dry-run flag: %w", err)
		}

		baseSystemPrompt := strings.TrimSpace(cfg.SystemPrompt)
		effectivePreset := f.resolveEffectivePreset(cfg)
		registry := f.initTools(cfg, dryRunOverride, effectivePreset)

		pipelineCfg, err := f.resolvePipeline(cfg)
		if err != nil {
			return err
		}

		return f.runPipelineLoop(ctx, flowCtx, flowState, pipelineCfg, registry, goal, baseSystemPrompt, cfg)
	}
}

func (f *Factory) readGoal() (string, error) {
	goal, err := f.parametersReader.GetTaskInput()
	if err != nil {
		return "", fmt.Errorf("failed to get task input: %w", err)
	}
	stdin, err := f.parametersReader.GetStdIn()
	if err == nil && stdin != "" && !strings.Contains(goal, stdin) {
		goal = goal + "\n\n" + stdin
	}

	goal = strings.TrimSpace(goal)
	if goal == "" {
		return "", fmt.Errorf("no goal provided")
	}
	return goal, nil
}

func (f *Factory) resolvePipeline(cfg *agentconfig.ResolvedConfig) (*config.AgentPipelineConfig, error) {
	pipelineName := "default" // TODO: allow config override
	pipelineCfg, ok := cfg.Pipelines[pipelineName]
	if !ok {
		return nil, fmt.Errorf("pipeline %s not found", pipelineName)
	}
	if pipelineCfg == nil {
		return nil, fmt.Errorf("pipeline %s is nil", pipelineName)
	}
	if len(pipelineCfg.Steps) == 0 {
		return nil, fmt.Errorf("pipeline %s has no steps", pipelineName)
	}
	return pipelineCfg, nil
}

func (f *Factory) runPipelineLoop(
	ctx context.Context,
	flowCtx *executor.Context,
	flowState *state.FlowState,
	pipelineCfg *config.AgentPipelineConfig,
	registry *tools.Registry,
	goal string,
	baseSystemPrompt string,
	cfg *agentconfig.ResolvedConfig,
) error {
	maxRestarts := 5
	if cfg.Safety != nil && cfg.Safety.CircuitBreaker != nil {
		maxRestarts = cfg.Safety.CircuitBreaker.MaxRestarts
	}

	currentGoal := goal
	for {
		restartReq, err := f.executeSteps(ctx, flowCtx, pipelineCfg.Steps, cfg, registry, currentGoal, baseSystemPrompt, strings.TrimSpace(pipelineCfg.Instructions))
		if err != nil {
			return err
		}

		if restartReq == "" {
			break
		}

		count := flowState.IncrementRestartCount()
		if count > maxRestarts {
			return fmt.Errorf("circuit breaker tripped: exceeded max restarts (%d)", maxRestarts)
		}

		flowCtx.SendRunningWithDetails("Restarting flow", fmt.Sprintf("attempt=%d instruction=%s", count+1, restartReq))
		currentGoal = buildRestartGoal(goal, restartReq)
		flowState.ResetPlan()
	}

	flowCtx.SendCompleted("Agent flow completed successfully")
	return nil
}

func (f *Factory) resolveEffectivePreset(cfg *agentconfig.ResolvedConfig) string {
	if p, ok := cfg.Personas["discover"]; ok {
		effectivePreset := strings.TrimSpace(p.Preset)
		if effectivePreset != "" {
			return effectivePreset
		}
	}
	for _, p := range cfg.Personas {
		if p == nil {
			continue
		}
		candidate := strings.TrimSpace(p.Preset)
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func (f *Factory) initTools(cfg *agentconfig.ResolvedConfig, dryRunOverride bool, effectivePreset string) *tools.Registry {
	registry := tools.NewRegistry()

	dryRun := false
	if cfg.Safety != nil {
		dryRun = cfg.Safety.DryRun
	}
	if dryRunOverride {
		dryRun = true
	}

	deps := tools.ToolDependencies{
		ReadFile:   readfile.NewFactory(f.workspaceService),
		WriteFile:  writefile.NewFactory(f.workspaceService, dryRun),
		EditFile:   editfile.NewFactory(f.workspaceService, dryRun),
		MoveFile:   movefile.NewFactory(f.workspaceService, dryRun),
		DeleteFile: deletefile.NewFactory(f.workspaceService, dryRun),
		GitUndo:    gitundo.NewFactory(f.workspaceService, dryRun),
		RunShell:   runshell.NewFactory(f.workspaceService),
		ListFiles:  listfiles.NewFactory(f.projectStateService),
		SearchCode: searchindex.MustNewFactory(f.retrievalService),
		GetDiff:    getdiff.NewFactory(f.gitToolingService),
		Memorize:   memorize.NewFactory(),
		Plan:       plan.NewFactory(),
		GetPlan:    getplan.NewFactory(),
		TrackTask:  tracktask.NewFactory(),
		Summarize:  summarize.NewFactory(f.gatewayFactory, f.presetResolver, effectivePreset),
		Restart:    control.NewRestartFactory(),

		SearchSnapshots: cfg.Tools.SearchDefaults.Snapshots,
		SearchTopK:      cfg.Tools.SearchDefaults.TopK,
		SearchMinScore:  cfg.Tools.SearchDefaults.MinScore,
	}
	tools.RegisterStandardTools(registry, &deps)
	return registry
}

func (f *Factory) executeSteps(
	ctx context.Context,
	flowCtx *executor.Context,
	steps []string,
	cfg *agentconfig.ResolvedConfig,
	registry *tools.Registry,
	goal string,
	baseSystemPrompt string,
	flowInstructions string,
) (string, error) {
	priorStepOutputs := make([]string, 0, len(steps))
	finalStepContent := ""

	for idx, personaName := range steps {
		out, err := f.executeSingleStep(ctx, flowCtx, idx, personaName, steps, cfg, registry, goal, baseSystemPrompt, flowInstructions, priorStepOutputs)
		if err != nil {
			return "", err
		}

		if out != nil {
			priorStepOutputs = append(priorStepOutputs, formatStepOutputForNextStep(personaName, out))
			if idx == len(steps)-1 {
				finalStepContent = out.FinalMessage
			}
		}

		// Check for restart request after step completion
		flowState, err := state.GetFlowState(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get flow state: %w", err)
		}
		if req, ok := flowState.GetRestartRequest(); ok {
			return req, nil
		}
	}

	if err := f.outputWriter.PrintLine(strings.TrimSpace(finalStepContent)); err != nil {
		return "", fmt.Errorf("failed to print final output: %w", err)
	}

	return "", nil
}

func (f *Factory) executeSingleStep(
	ctx context.Context,
	flowCtx *executor.Context,
	idx int,
	personaName string,
	steps []string,
	cfg *agentconfig.ResolvedConfig,
	registry *tools.Registry,
	goal string,
	baseSystemPrompt string,
	flowInstructions string,
	priorStepOutputs []string,
) (*agentloop.Output, error) {
	personaCfg, ok := cfg.Personas[personaName]
	if !ok {
		return nil, fmt.Errorf("persona %s not found", personaName)
	}

	caser := cases.Title(language.English)
	stepCtx := flowCtx.Child(caser.String(personaName))
	stepCtx.SendRunning(fmt.Sprintf("Starting %s step", caser.String(personaName)))

	personaPreset := strings.TrimSpace(personaCfg.Preset)
	prof, err := f.presetResolver.Get(preset.Preset(personaPreset))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve preset %s: %w", personaPreset, err)
	}

	systemPrompt := f.buildStepSystemPrompt(baseSystemPrompt, flowInstructions, steps, idx, personaName, personaCfg.SystemPersona)
	stepGoal := f.buildStepGoal(ctx, goal, priorStepOutputs, personaCfg.UserInstructions)

	input := &agentloop.Input{
		ToolRegistry:             registry,
		AllowedTools:             personaCfg.Tools,
		ToolDescriptionOverrides: cfg.Tools.ToolDescriptions,
		StepName:                 personaName,
		Preset:                   prof,
		Goal:                     &stepGoal,
		SystemPrompt:             &systemPrompt,
	}

	out, err := executor.ExecuteActivity(ctx, flowCtx.GetExecutor(), stepCtx, "AgentLoop", f.agentLoopFactory.NewActivity(), input)
	if err != nil {
		return nil, fmt.Errorf("agent loop failed for %s: %w", personaName, err)
	}

	stepCtx.SendCompleted(fmt.Sprintf("Finished %s step", caser.String(personaName)))
	return out, nil
}

func (f *Factory) buildStepSystemPrompt(base, flow string, steps []string, idx int, name, personaDesc string) string {
	roleLine := buildFlowRoleLine(steps, idx, name)
	parts := make([]string, 0, 4)
	if strings.TrimSpace(base) != "" {
		parts = append(parts, strings.TrimSpace(base))
	}
	if strings.TrimSpace(flow) != "" {
		parts = append(parts, strings.TrimSpace(flow))
	}
	if roleLine != "" {
		parts = append(parts, roleLine)
	}
	if strings.TrimSpace(personaDesc) != "" {
		parts = append(parts, strings.TrimSpace(personaDesc))
	}
	return strings.Join(parts, "\n\n")
}

func (f *Factory) buildStepGoal(ctx context.Context, goal string, priorOutputs []string, userInstructions string) string {
	stepGoal := goal
	if mem := buildMemoryFactsSection(ctx); mem != "" {
		stepGoal = stepGoal + "\n\n" + mem
	}
	if len(priorOutputs) > 0 {
		stepGoal = stepGoal + "\n\nPREVIOUS STEP OUTPUTS (in order):\n\n" + strings.Join(priorOutputs, "\n\n")
	}
	if instructions := strings.TrimSpace(userInstructions); instructions != "" {
		stepGoal = stepGoal + "\n\nINSTRUCTIONS:\n\n" + instructions
	}
	return stepGoal
}

func buildMemoryFactsSection(ctx context.Context) string {
	flowState, err := state.GetFlowState(ctx)
	if err != nil || flowState == nil {
		return ""
	}
	facts := flowState.GetFacts()
	if len(facts) == 0 {
		return "MEMORY FACTS:\n(none yet)"
	}
	var b strings.Builder
	b.WriteString("MEMORY FACTS:\n")
	for _, f := range facts {
		line := strings.TrimSpace(f.Content)
		if line == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func buildFlowRoleLine(steps []string, stepIndex int, stepName string) string {
	if len(steps) == 0 {
		return ""
	}

	caser := cases.Title(language.English)
	prettySteps := make([]string, 0, len(steps))
	for _, s := range steps {
		prettySteps = append(prettySteps, caser.String(strings.TrimSpace(s)))
	}

	current := caser.String(strings.TrimSpace(stepName))
	return fmt.Sprintf(
		"Flow steps (in order): %s. Current step: %s (%d/%d). Use PREVIOUS STEP OUTPUTS as required context.",
		strings.Join(prettySteps, " → "),
		current,
		stepIndex+1,
		len(steps),
	)
}

func formatStepOutputForNextStep(stepName string, out *agentloop.Output) string {
	caser := cases.Title(language.English)
	label := caser.String(strings.TrimSpace(stepName))
	if label == "" {
		label = "Step"
	}

	content := strings.TrimSpace(out.FinalMessage)

	header := fmt.Sprintf("[%s]", label)
	if content == "" {
		return header + "\n(no content)"
	}
	return header + "\n" + content
}

func buildRestartGoal(originalGoal, restartReq string) string {
	r := strings.TrimSpace(restartReq)
	if r == "" {
		return strings.TrimSpace(originalGoal)
	}
	return r
}
