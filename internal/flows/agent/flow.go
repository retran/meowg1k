// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package agent implements the multi-step agent workflow.
package do

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/agentstep"
	"github.com/retran/meowg1k/internal/activities/generatecontent"
	queryactivity "github.com/retran/meowg1k/internal/activities/query"
	agentconfig "github.com/retran/meowg1k/internal/core/agent"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// CommandParametersReader reads command-line parameters and flags.
type CommandParametersReader interface {
	GetTaskInput() (string, error)
	GetProfileFlag() (string, error)
	GetSystemPromptFlag() (string, error)
	GetSnapshotsFlag() ([]string, error)
	GetTopKFlag() (int, error)
	GetMinScoreFlag() (float32, error)
}

// ConfigReader provides access to agent configuration.
type ConfigReader interface {
	Get() (*agentconfig.ResolvedConfig, error)
}

// Factory creates instances of the agent flow.
type Factory struct {
	agentConfigService *agentconfig.Service
	stepFactory        *agentstep.Factory
	parametersReader   CommandParametersReader
	profileResolver    ports.ProfileResolver
	outputWriter       ports.OutputWriter
	workspaceService   ports.WorkspaceService
	filterService      ports.FilterService
	gitService         ports.GitToolingService
	queryFactory       executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output]
	invokeLLMFactory   executor.ActivityFactory[*generatecontent.Input, *generatecontent.Output]
	indexFlowBuilder   func() (executor.Flow, error)
}

// NewFactory creates a new agent flow factory.
func NewFactory(
	agentConfigService *agentconfig.Service,
	stepFactory *agentstep.Factory,
	parametersReader CommandParametersReader,
	profileResolver ports.ProfileResolver,
	outputWriter ports.OutputWriter,
	workspaceService ports.WorkspaceService,
	filterService ports.FilterService,
	gitService ports.GitToolingService,
	queryFactory executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output],
	invokeLLMFactory executor.ActivityFactory[*generatecontent.Input, *generatecontent.Output],
	indexFlowBuilder func() (executor.Flow, error),
) (*Factory, error) {
	if agentConfigService == nil {
		return nil, fmt.Errorf("agent config service is nil")
	}
	if stepFactory == nil {
		return nil, fmt.Errorf("step factory is nil")
	}
	if parametersReader == nil {
		return nil, fmt.Errorf("parameters reader is nil")
	}
	if profileResolver == nil {
		return nil, fmt.Errorf("profile resolver is nil")
	}
	if outputWriter == nil {
		return nil, fmt.Errorf("output writer is nil")
	}
	if workspaceService == nil {
		return nil, fmt.Errorf("workspace service is nil")
	}
	if queryFactory == nil {
		return nil, fmt.Errorf("query activity factory is nil")
	}
	if invokeLLMFactory == nil {
		return nil, fmt.Errorf("invokeLLMFactory is nil")
	}

	return &Factory{
		agentConfigService: agentConfigService,
		stepFactory:        stepFactory,
		parametersReader:   parametersReader,
		profileResolver:    profileResolver,
		outputWriter:       outputWriter,
		workspaceService:   workspaceService,
		filterService:      filterService,
		gitService:         gitService,
		queryFactory:       queryFactory,
		invokeLLMFactory:   invokeLLMFactory,
		indexFlowBuilder:   indexFlowBuilder,
	}, nil
}

// NewFlow creates and returns the agent flow function.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		return f.runAgentFlow(ctx, flowCtx)
	}
}

func (f *Factory) runAgentFlow(ctx context.Context, flowCtx *executor.Context) error {
	flowCtx.SendRunning("Do Flow")

	initialGoal, err := f.readGoal()
	if err != nil {
		return err
	}

	runtimeConfig, err := f.loadRuntimeConfig()
	if err != nil {
		return err
	}

	runner, err := f.buildToolRunner(runtimeConfig.Tools.SearchDefaults)
	if err != nil {
		return err
	}

	executorInstance := flowCtx.GetExecutor()
	if executorInstance == nil {
		return fmt.Errorf("executor not available")
	}

	finalContent, summaries, executeContent, err := f.executeSteps(ctx, flowCtx, initialGoal, runtimeConfig, runner, executorInstance)
	if err != nil {
		return err
	}

	finalContent = combineFinalOutput(finalContent, executeContent, summaries)

	flowCtx.SendCompleted("Do output ready")

	if err := f.outputWriter.PrintLine(strings.TrimSpace(finalContent)); err != nil {
		return fmt.Errorf("failed to print agent output: %w", err)
	}

	return nil
}

func (f *Factory) readGoal() (string, error) {
	goal, err := f.parametersReader.GetTaskInput()
	if err != nil {
		return "", fmt.Errorf("failed to read task input: %w", err)
	}
	return goal, nil
}

func (f *Factory) loadRuntimeConfig() (*agentconfig.ResolvedConfig, error) {
	cfg, err := f.agentConfigService.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to load agent config: %w", err)
	}

	runtimeConfig, err := f.applyOverrides(cfg)
	if err != nil {
		return nil, err
	}
	return runtimeConfig, nil
}

func (f *Factory) buildToolRunner(searchDefaults agentconfig.SearchDefaults) (*ToolRunner, error) {
	workspaceRoot, err := f.workspaceService.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve workspace root: %w", err)
	}

	return NewToolRunner(
		workspaceRoot,
		f.filterService,
		f.gitService,
		f.queryFactory,
		f.invokeLLMFactory,
		f.indexFlowBuilder,
		searchDefaults,
	), nil
}

func (f *Factory) executeSteps(ctx context.Context, flowCtx *executor.Context, goal string, runtimeConfig *agentconfig.ResolvedConfig, runner *ToolRunner, executorInstance executor.Executor) (finalContent string, summaries []string, executeContent string, err error) {
	currentGoal := goal
	maxRetries := 2

	for attempt := 1; ; attempt++ {
		finalContent, summaries, executeContent, err = f.executeStepCycle(ctx, flowCtx, currentGoal, runtimeConfig, runner, executorInstance, attempt)
		if err != nil {
			return "", nil, "", err
		}

		result := parseVerificationResult(finalContent)
		if result.Passed || attempt > maxRetries {
			return finalContent, summaries, executeContent, nil
		}

		currentGoal = buildRetryGoal(goal, result.Tasks, finalContent)
		runner.ResetPlanMemory()
	}
}

func (f *Factory) executeStepCycle(ctx context.Context, flowCtx *executor.Context, goal string, runtimeConfig *agentconfig.ResolvedConfig, runner *ToolRunner, executorInstance executor.Executor, attempt int) (finalContent string, summaries []string, executeContent string, err error) {
	summaries = make([]string, 0, len(agentconfig.StepOrder))
	finalContent = ""
	executeContent = ""

	for _, stepName := range agentconfig.StepOrder {
		step := runtimeConfig.Steps[stepName]
		if step == nil {
			return "", nil, "", fmt.Errorf("missing step config for %s", stepName)
		}
		output, err := f.runStep(ctx, flowCtx, goal, runner, executorInstance, stepName, step, &summaries, attempt)
		if err != nil {
			return "", nil, "", err
		}

		applyStepOutput(stepName, output, &summaries, &executeContent, &finalContent)
	}

	return finalContent, summaries, executeContent, nil
}

func (f *Factory) runStep(ctx context.Context, flowCtx *executor.Context, goal string, runner *ToolRunner, executorInstance executor.Executor, stepName string, step *agentconfig.StepConfig, summaries *[]string, attempt int) (*agentstep.Output, error) {
	stepProfile, err := f.profileResolver.Get(profile.Profile(step.Profile))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile %q: %w", step.Profile, err)
	}

	activity := f.stepFactory.NewActivity()
	input := &agentstep.Input{
		Goal:           stringPtr(goal),
		Profile:        stepProfile,
		SystemPrompt:   stringPtr(step.SystemPrompt),
		StepConfig:     step,
		PriorSummaries: summaries,
		ToolRunner:     runner,
	}

	activityName := fmt.Sprintf("AgentStep:%s", stepName)
	if attempt > 1 {
		activityName = fmt.Sprintf("AgentStep:%s#%d", stepName, attempt)
	}

	output, err := executor.ExecuteActivity(ctx, executorInstance, flowCtx, activityName, activity, input)
	if err != nil {
		return nil, fmt.Errorf("agent step %s failed: %w", stepName, err)
	}

	return output, nil
}

func applyStepOutput(stepName string, output *agentstep.Output, summaries *[]string, executeContent *string, finalContent *string) {
	if output == nil {
		return
	}

	summary := strings.TrimSpace(output.Summary)
	content := strings.TrimSpace(output.Content)
	if summary != "" {
		*summaries = append(*summaries, output.Summary)
	}

	switch stepName {
	case "execute":
		if content != "" {
			*executeContent = output.Content
		} else if summary != "" {
			*executeContent = output.Summary
		}
	case "verify":
		*finalContent = output.Content
	}
}

func stringPtr(value string) *string {
	return &value
}

type verificationResult struct {
	Tasks  []string
	Passed bool
}

func parseVerificationResult(content string) verificationResult {
	result := verificationResult{Passed: true}
	if passed, ok := parseVerificationStatus(content); ok {
		result.Passed = passed
	}
	if result.Passed {
		return result
	}

	result.Tasks = extractFailureTasks(content)
	return result
}

func parseVerificationStatus(content string) (passed bool, ok bool) {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if passed, ok := parseVerificationStatusLine(trimmed); ok {
			return passed, true
		}
	}
	return true, false
}

func parseVerificationStatusLine(line string) (passed bool, ok bool) {
	lower := strings.ToLower(line)
	if !strings.HasPrefix(lower, "verificationresult:") && !strings.HasPrefix(lower, "verification:") {
		return false, false
	}
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return false, false
	}
	value := strings.ToLower(strings.TrimSpace(parts[1]))
	if strings.HasPrefix(value, "fail") {
		return false, true
	}
	if strings.HasPrefix(value, "pass") {
		return true, true
	}
	return false, false
}

func extractFailureTasks(content string) []string {
	lines := strings.Split(content, "\n")
	inSection := false
	tasks := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "failuretasks:") {
			inSection = true
			continue
		}
		if inSection {
			if trimmed == "" {
				break
			}
			clean := strings.TrimPrefix(trimmed, "-")
			clean = strings.TrimPrefix(clean, "*")
			clean = strings.TrimSpace(clean)
			if clean != "" {
				tasks = append(tasks, clean)
			}
		}
	}
	return tasks
}

func buildRetryGoal(baseGoal string, tasks []string, verificationOutput string) string {
	if len(tasks) == 0 {
		return fmt.Sprintf("%s\n\nFollow-up task: address verification failures.\nVerification output:\n%s", strings.TrimSpace(baseGoal), strings.TrimSpace(verificationOutput))
	}

	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(baseGoal))
	builder.WriteString("\n\nFollow-up tasks from verification:\n")
	for _, task := range tasks {
		builder.WriteString("- ")
		builder.WriteString(task)
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func (f *Factory) applyOverrides(cfg *agentconfig.ResolvedConfig) (*agentconfig.ResolvedConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("agent config is nil")
	}

	overrides, err := f.readOverrides()
	if err != nil {
		return nil, err
	}

	updated := *cfg
	updated.Defaults = cfg.Defaults
	updated.Tools = cfg.Tools
	updated.Steps = make(map[string]*agentconfig.StepConfig, len(cfg.Steps))

	defaults := applyDefaultOverrides(cfg.Defaults, &overrides)
	updated.Defaults = defaults

	searchDefaults := cfg.Tools.SearchDefaults
	if len(overrides.snapshots) > 0 {
		searchDefaults.Snapshots = overrides.snapshots
	}
	if overrides.topK > 0 {
		searchDefaults.TopK = overrides.topK
	}
	if overrides.minScore > 0 {
		searchDefaults.MinScore = overrides.minScore
	}
	updated.Tools.SearchDefaults = searchDefaults

	applyStepOverrides(cfg, &updated, defaults)

	return &updated, nil
}

type overrideValues struct {
	profile      string
	systemPrompt string
	snapshots    []string
	topK         int
	minScore     float32
}

func (f *Factory) readOverrides() (overrideValues, error) {
	profileOverride, err := f.parametersReader.GetProfileFlag()
	if err != nil {
		return overrideValues{}, fmt.Errorf("failed to read profile flag: %w", err)
	}
	systemPromptOverride, err := f.parametersReader.GetSystemPromptFlag()
	if err != nil {
		return overrideValues{}, fmt.Errorf("failed to read system prompt flag: %w", err)
	}
	snapshotsOverride, err := f.parametersReader.GetSnapshotsFlag()
	if err != nil {
		return overrideValues{}, fmt.Errorf("failed to read snapshots flag: %w", err)
	}
	topKOverride, err := f.parametersReader.GetTopKFlag()
	if err != nil {
		return overrideValues{}, fmt.Errorf("failed to read top_k flag: %w", err)
	}
	minScoreOverride, err := f.parametersReader.GetMinScoreFlag()
	if err != nil {
		return overrideValues{}, fmt.Errorf("failed to read min_score flag: %w", err)
	}

	return overrideValues{
		profile:      profileOverride,
		systemPrompt: systemPromptOverride,
		snapshots:    snapshotsOverride,
		topK:         topKOverride,
		minScore:     minScoreOverride,
	}, nil
}

func applyDefaultOverrides(defaults agentconfig.Defaults, overrides *overrideValues) agentconfig.Defaults {
	if overrides == nil {
		return defaults
	}
	if strings.TrimSpace(overrides.profile) != "" {
		defaults.Profile = overrides.profile
	}
	if strings.TrimSpace(overrides.systemPrompt) != "" {
		defaults.SystemPrompt = overrides.systemPrompt
	}
	return defaults
}

func applyStepOverrides(cfg *agentconfig.ResolvedConfig, updated *agentconfig.ResolvedConfig, defaults agentconfig.Defaults) {
	sharedPrompt := strings.TrimSpace(defaults.SystemPrompt)
	for _, stepName := range agentconfig.StepOrder {
		step := cfg.Steps[stepName]
		if step == nil {
			continue
		}

		stepCopy := *step
		if strings.TrimSpace(stepCopy.Profile) == "" {
			stepCopy.Profile = defaults.Profile
		}
		stepCopy.SystemPrompt = combineSystemPrompt(sharedPrompt, stepCopy.SystemPrompt)

		updated.Steps[stepName] = &stepCopy
	}
}

func combineSystemPrompt(sharedPrompt, stepPrompt string) string {
	shared := strings.TrimSpace(sharedPrompt)
	step := strings.TrimSpace(stepPrompt)
	if shared == "" && step == "" {
		return ""
	}
	if step == "" {
		return shared
	}
	if shared == "" {
		return step
	}
	return shared + "\n\n" + step
}

func combineFinalOutput(finalContent string, executeContent string, summaries []string) string {
	trimmedFinal := strings.TrimSpace(finalContent)
	trimmedExecute := strings.TrimSpace(executeContent)
	if trimmedFinal == "" {
		if trimmedExecute != "" {
			return trimmedExecute
		}
		return strings.TrimSpace(strings.Join(summaries, "\n"))
	}
	if trimmedExecute != "" && !strings.Contains(trimmedFinal, trimmedExecute) {
		return fmt.Sprintf("%s\n\nResult:\n%s", trimmedFinal, trimmedExecute)
	}
	return trimmedFinal
}
