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
	"github.com/retran/meowg1k/internal/activities/listfiles"
	"github.com/retran/meowg1k/internal/activities/memorize"
	"github.com/retran/meowg1k/internal/activities/plan"
	"github.com/retran/meowg1k/internal/activities/readfile"
	"github.com/retran/meowg1k/internal/activities/recall"
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

		effectiveProfile := cfg.Defaults.Profile
		baseSystemPrompt := strings.TrimSpace(cfg.Defaults.SystemPrompt)

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
			Recall:     recall.NewFactory(),
			Plan:       plan.NewFactory(),
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
		flowSteps, ok := cfg.Flows[flowName]
		if !ok {
			return fmt.Errorf("flow %s not found", flowName)
		}

		// 7. Execute Flow Loop (with Circuit Breaker)
		maxRestarts := 5
		if cfg.Safety != nil && cfg.Safety.CircuitBreaker != nil {
			maxRestarts = cfg.Safety.CircuitBreaker.MaxRestarts
		}

		currentGoal := goal

		for {
			restartReq, err := f.executeSteps(ctx, flowCtx, flowSteps, cfg, registry, currentGoal, baseSystemPrompt)
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
) (string, error) {

	exec := flowCtx.GetExecutor()

	for _, personaName := range steps {
		personaCfg, ok := cfg.Personas[personaName]
		if !ok {
			return "", fmt.Errorf("persona %s not found", personaName)
		}

		stepCtx := flowCtx.Child(strings.Title(personaName))
		stepCtx.SendRunning(fmt.Sprintf("Agent %s is working", personaName))

		personaProfile := strings.TrimSpace(personaCfg.Profile)
		prof, err := f.profileResolver.Get(profile.Profile(personaProfile))
		if err != nil {
			return "", fmt.Errorf("failed to resolve profile %s: %w", personaProfile, err)
		}

		// Check if we need to restart BEFORE running the step?
		// No, usually restart comes from tool execution within the step.

		// Run Agent Loop for this Persona
		agentAct := f.agentLoopFactory.NewActivity()
		systemPrompt := strings.TrimSpace(personaCfg.Instructions)
		if strings.TrimSpace(baseSystemPrompt) != "" {
			if systemPrompt != "" {
				systemPrompt = baseSystemPrompt + "\n\n" + systemPrompt
			} else {
				systemPrompt = baseSystemPrompt
			}
		}
		input := &agentloop.Input{
			ToolRegistry: registry,
			AllowedTools: personaCfg.Tools,
			StepName:     personaName,
			Profile:      prof,
			Goal:         &goal,
			SystemPrompt: &systemPrompt,
		}

		_, err = executor.ExecuteActivity(ctx, exec, stepCtx, "AgentLoop", agentAct, input)
		if err != nil {
			return "", fmt.Errorf("agent loop failed for %s: %w", personaName, err)
		}

		// Check for restart request after step completion
		flowState, _ := state.GetFlowState(ctx) // Error checked in loop
		if req, ok := flowState.GetRestartRequest(); ok {
			return req, nil
		}
	}

	return "", nil
}
