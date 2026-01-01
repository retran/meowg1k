// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package do

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/agentloop"
	"github.com/retran/meowg1k/internal/activities/control"
	"github.com/retran/meowg1k/internal/activities/editfile"
	"github.com/retran/meowg1k/internal/activities/getdiff"
	"github.com/retran/meowg1k/internal/activities/getplan"
	"github.com/retran/meowg1k/internal/activities/listfiles"
	"github.com/retran/meowg1k/internal/activities/memorize"
	"github.com/retran/meowg1k/internal/activities/plan"
	"github.com/retran/meowg1k/internal/activities/readfile"
	"github.com/retran/meowg1k/internal/activities/runcommand"
	"github.com/retran/meowg1k/internal/activities/searchindex"
	"github.com/retran/meowg1k/internal/activities/summarize"
	"github.com/retran/meowg1k/internal/activities/tracktask"
	"github.com/retran/meowg1k/internal/activities/writefile"
	agentconfig "github.com/retran/meowg1k/internal/core/agent"
	"github.com/retran/meowg1k/internal/core/agent/state"
	"github.com/retran/meowg1k/internal/core/agent/tools"
	"github.com/retran/meowg1k/internal/core/retrieval"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type Factory struct {
	configService       *agentconfig.Service
	profileResolver     ports.ProfileResolver
	workspaceService    ports.WorkspaceService
	gitToolingService   ports.GitToolingService
	projectStateService ports.ProjectStateService
	retrievalService    retrieval.Retriever
	gatewayFactory      ports.GenerationGatewayFactory
	agentLoopFactory    *agentloop.Factory
	parametersReader    CommandParametersReader
	outputWriter        ports.OutputWriter
}

type CommandParametersReader interface {
	GetTaskInput() (string, error)
	GetStdIn() (string, error)
	GetDryRunFlag() (bool, error)
}

func NewFactory(
	configService *agentconfig.Service,
	profileResolver ports.ProfileResolver,
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
		profileResolver:     profileResolver,
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

func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		// 1. Load Config
		cfg, err := f.configService.Get()
		if err != nil {
			return err
		}

		// 2. Read Input (Goal + Stdin)
		goal, err := f.parametersReader.GetTaskInput()
		if err != nil {
			return err
		}
		// GetTaskInput already includes stdin if it was piped.
		// But if we want to be explicit about concatenation:
		stdin, _ := f.parametersReader.GetStdIn()
		if stdin != "" && !strings.Contains(goal, stdin) {
			goal = goal + "\n\n" + stdin
		}

		goal = strings.TrimSpace(goal)
		if goal == "" {
			return fmt.Errorf("no goal provided")
		}

		flowCtx.SendRunningWithDetails("Starting agent flow", fmt.Sprintf("goal_len=%d", len(goal)))

		// 3. Initialize State
		flowState := state.NewFlowState()
		ctx = state.WithFlowState(ctx, flowState)

		// 4. Read CLI Overrides
		dryRunOverride, err := f.parametersReader.GetDryRunFlag()
		if err != nil {
			return fmt.Errorf("failed to read dry-run flag: %w", err)
		}

		baseSystemPrompt := strings.TrimSpace(cfg.SystemPrompt)
		effectiveProfile := ""
		if p, ok := cfg.Personas["discover"]; ok {
			effectiveProfile = strings.TrimSpace(p.Profile)
		}
		if effectiveProfile == "" {
			for _, p := range cfg.Personas {
				if p == nil {
					continue
				}
				candidate := strings.TrimSpace(p.Profile)
				if candidate != "" {
					effectiveProfile = candidate
					break
				}
			}
		}

		// 5. Initialize Tools Registry
		registry := tools.NewRegistry()

		dryRun := false
		if cfg.Safety != nil {
			dryRun = cfg.Safety.DryRun
		}
		if dryRunOverride {
			dryRun = true
		}

		searchSnapshots := cfg.Tools.SearchDefaults.Snapshots
		searchTopK := cfg.Tools.SearchDefaults.TopK
		searchMinScore := cfg.Tools.SearchDefaults.MinScore

		deps := tools.ToolDependencies{
			ReadFile:   readfile.NewFactory(f.workspaceService),
			WriteFile:  writefile.NewFactory(f.workspaceService, dryRun),
			EditFile:   editfile.NewFactory(f.workspaceService, dryRun),
			RunCommand: runcommand.NewFactory(f.workspaceService),
			ListFiles:  listfiles.NewFactory(f.projectStateService),
			SearchCode: searchindex.MustNewFactory(f.retrievalService),
			GetDiff:    getdiff.NewFactory(f.gitToolingService),
			Memorize:   memorize.NewFactory(),
			Plan:       plan.NewFactory(),
			GetPlan:    getplan.NewFactory(),
			TrackTask:  tracktask.NewFactory(),
			Summarize:  summarize.NewFactory(f.gatewayFactory, f.profileResolver, effectiveProfile),
			Restart:    control.NewRestartFactory(),

			SearchSnapshots: searchSnapshots,
			SearchTopK:      searchTopK,
			SearchMinScore:  searchMinScore,
		}
		tools.RegisterStandardTools(registry, deps)

		// 6. Determine Flow Steps
		flowName := "default" // TODO: allow config override
		flowCfg, ok := cfg.Flows[flowName]
		if !ok {
			return fmt.Errorf("flow %s not found", flowName)
		}
		if flowCfg == nil {
			return fmt.Errorf("flow %s is nil", flowName)
		}
		flowSteps := flowCfg.Steps
		if len(flowSteps) == 0 {
			return fmt.Errorf("flow %s has no steps", flowName)
		}

		// 7. Execute Flow Loop (with Circuit Breaker)
		maxRestarts := 5
		if cfg.Safety != nil && cfg.Safety.CircuitBreaker != nil {
			maxRestarts = cfg.Safety.CircuitBreaker.MaxRestarts
		}

		currentGoal := goal

		for {
			restartReq, err := f.executeSteps(ctx, flowCtx, flowSteps, cfg, registry, currentGoal, baseSystemPrompt, strings.TrimSpace(flowCfg.Instructions))
			if err != nil {
				return err
			}

			if restartReq == "" {
				break // Flow completed successfully
			}

			// Restart requested
			count := flowState.IncrementRestartCount()
			if count > maxRestarts {
				return fmt.Errorf("circuit breaker tripped: exceeded max restarts (%d)", maxRestarts)
			}

			flowCtx.SendRunningWithDetails("Restarting flow", fmt.Sprintf("attempt=%d instruction=%s", count+1, restartReq))

			// Update goal with restart instruction
			currentGoal = fmt.Sprintf("ORIGINAL GOAL: %s\n\nFEEDBACK/INSTRUCTION: %s", goal, restartReq)

			// Note: We keep the Memory and TaskBoard! The design says "Flow restarts".
			// Usually we want to keep memory (what we learned) but maybe reset Plan?
			// The Verifier feedback feeds into Explorer.
			// Explorer should see previous plan failure.
			// So keeping state is correct.
		}

		flowCtx.SendCompleted("Agent flow completed successfully")
		return nil
	}
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

	exec := flowCtx.GetExecutor()

	priorStepOutputs := make([]string, 0, len(steps))
	finalStepContent := ""

	for idx, personaName := range steps {
		personaCfg, ok := cfg.Personas[personaName]
		if !ok {
			return "", fmt.Errorf("persona %s not found", personaName)
		}

		stepCtx := flowCtx.Child(strings.Title(personaName))
		stepCtx.SendRunning(fmt.Sprintf("Starting %s step", strings.Title(personaName)))

		personaProfile := strings.TrimSpace(personaCfg.Profile)
		prof, err := f.profileResolver.Get(profile.Profile(personaProfile))
		if err != nil {
			return "", fmt.Errorf("failed to resolve profile %s: %w", personaProfile, err)
		}

		// Check if we need to restart BEFORE running the step?
		// No, usually restart comes from tool execution within the step.

		// Run Agent Loop for this Persona
		agentAct := f.agentLoopFactory.NewActivity()
		personaDescription := strings.TrimSpace(personaCfg.SystemPersona)
		sharedFlowPrompt := strings.TrimSpace(flowInstructions)
		roleLine := buildFlowRoleLine(steps, idx, personaName)

		// Required order:
		// 1) shared flow system prompt
		// 2) (user prompt contains prior step outputs)
		// 3) system prompt of current step
		systemPromptParts := make([]string, 0, 4)
		if strings.TrimSpace(baseSystemPrompt) != "" {
			systemPromptParts = append(systemPromptParts, strings.TrimSpace(baseSystemPrompt))
		}
		if sharedFlowPrompt != "" {
			systemPromptParts = append(systemPromptParts, sharedFlowPrompt)
		}
		if roleLine != "" {
			systemPromptParts = append(systemPromptParts, roleLine)
		}
		if personaDescription != "" {
			systemPromptParts = append(systemPromptParts, personaDescription)
		}
		systemPrompt := strings.Join(systemPromptParts, "\n\n")

		stepGoal := goal
		if mem := buildMemoryFactsSection(ctx); mem != "" {
			stepGoal = stepGoal + "\n\n" + mem
		}
		if len(priorStepOutputs) > 0 {
			stepGoal = stepGoal + "\n\nPREVIOUS STEP OUTPUTS (in order):\n\n" + strings.Join(priorStepOutputs, "\n\n")
		}
		if instructions := strings.TrimSpace(personaCfg.UserInstructions); instructions != "" {
			stepGoal = stepGoal + "\n\nINSTRUCTIONS:\n\n" + instructions
		}
		input := &agentloop.Input{
			ToolRegistry:             registry,
			AllowedTools:             personaCfg.Tools,
			ToolDescriptionOverrides: cfg.Tools.ToolDescriptions,
			StepName:                 personaName,
			Profile:                  prof,
			Goal:                     &stepGoal,
			SystemPrompt:             &systemPrompt,
		}

		out, err := executor.ExecuteActivity(ctx, exec, stepCtx, "AgentLoop", agentAct, input)
		if err != nil {
			return "", fmt.Errorf("agent loop failed for %s: %w", personaName, err)
		}
		stepCtx.SendCompleted(fmt.Sprintf("Finished %s step", strings.Title(personaName)))

		if out != nil {
			priorStepOutputs = append(priorStepOutputs, formatStepOutputForNextStep(personaName, out))
			if idx == len(steps)-1 {
				finalStepContent = out.FinalMessage
			}
		}

		// Check for restart request after step completion
		flowState, _ := state.GetFlowState(ctx) // Error checked in loop
		if req, ok := flowState.GetRestartRequest(); ok {
			return req, nil
		}
	}

	if f.outputWriter == nil {
		return "", fmt.Errorf("output writer is nil")
	}
	if err := f.outputWriter.PrintLine(strings.TrimSpace(finalStepContent)); err != nil {
		return "", fmt.Errorf("failed to print final output: %w", err)
	}

	return "", nil
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

	prettySteps := make([]string, 0, len(steps))
	for _, s := range steps {
		prettySteps = append(prettySteps, strings.Title(strings.TrimSpace(s)))
	}

	current := strings.Title(strings.TrimSpace(stepName))
	return fmt.Sprintf(
		"Flow steps (in order): %s. Current step: %s (%d/%d). Use PREVIOUS STEP OUTPUTS as required context.",
		strings.Join(prettySteps, " → "),
		current,
		stepIndex+1,
		len(steps),
	)
}

func formatStepOutputForNextStep(stepName string, out *agentloop.Output) string {
	label := strings.Title(strings.TrimSpace(stepName))
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
