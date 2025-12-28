// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package agent provides configuration resolution for agent mode.
package agent

import (
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/ports"
)

// Service resolves agent configuration with defaults applied.
type Service struct {
	configResolver ports.ConfigResolver
}

// NewService creates a new agent config service.
func NewService(configResolver ports.ConfigResolver) (*Service, error) {
	if configResolver == nil {
		return nil, fmt.Errorf("config resolver is nil")
	}

	return &Service{configResolver: configResolver}, nil
}

// ResolvedConfig contains the agent configuration with defaults applied.
type ResolvedConfig struct {
	Steps    map[string]*StepConfig
	Defaults Defaults
	Tools    Tools
}

// Defaults are the base defaults for agent steps.
type Defaults struct {
	Profile      string
	SystemPrompt string
}

// Tools are default tool settings for agent mode.
type Tools struct {
	SearchDefaults SearchDefaults
}

// SearchDefaults are defaults for embeddings search.
type SearchDefaults struct {
	Snapshots []string
	TopK      int
	MinScore  float32
}

// StepConfig defines a resolved agent step configuration.
type StepConfig struct {
	ToolModes    map[string]map[string]bool
	Profile      string
	SystemPrompt string
	Tools        []string
	Index        int
}

// AllowsToolMode checks whether a tool and mode are allowed for this step.
func (s *StepConfig) AllowsToolMode(tool, mode string) bool {
	if s == nil {
		return false
	}

	allowedTools := make(map[string]bool, len(s.Tools))
	for _, name := range s.Tools {
		allowedTools[strings.ToLower(name)] = true
	}

	if !allowedTools[strings.ToLower(tool)] {
		return false
	}

	modeSet, ok := s.ToolModes[strings.ToLower(tool)]
	if !ok || len(modeSet) == 0 {
		return true
	}

	return modeSet[strings.ToLower(mode)]
}

// StepOrder defines the execution order for agent steps.
var StepOrder = []string{"research", "plan", "execute", "verify"}

// Get resolves the agent configuration with defaults applied.
func (s *Service) Get() (*ResolvedConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("agent config service is nil")
	}

	cfg, err := s.configResolver.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	agentCfg := cfg.Agent
	if agentCfg == nil {
		agentCfg = defaultAgentConfig()
	}

	defaults := applyDefaults(agentCfg.Defaults)
	tools := applyToolDefaults(agentCfg.Tools)
	steps := applyStepDefaults(agentCfg.Steps)

	return &ResolvedConfig{
		Defaults: defaults,
		Tools:    tools,
		Steps:    steps,
	}, nil
}

func applyDefaults(cfg *config.AgentDefaults) Defaults {
	if cfg == nil {
		cfg = defaultAgentConfig().Defaults
	}

	profile := cfg.Profile
	if profile == "" {
		profile = "gemini-pro"
	}

	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a multi-step agent that works in four steps: research, plan, execute, verify.\nResearch gathers context without changes. Plan turns findings into ordered tasks. Execute applies the changes. Verify checks outcomes and reports gaps.\nUse the memory tool to keep context between steps: call memory.list at the start of each step, and call memory.add at the end of each step to store key findings, decisions, and outputs."
	}

	return Defaults{
		Profile:      profile,
		SystemPrompt: systemPrompt,
	}
}

func applyToolDefaults(cfg *config.AgentToolsConfig) Tools {
	if cfg == nil || cfg.SearchDefaults == nil {
		cfg = defaultAgentConfig().Tools
	}

	search := cfg.SearchDefaults
	if search == nil {
		search = &config.AgentSearchDefaults{}
	}

	snapshots := search.Snapshots
	if len(snapshots) == 0 {
		snapshots = []string{"_workdir_", "_stage_", "_head_"}
	}

	topK := search.TopK
	if topK <= 0 {
		topK = 8
	}

	minScore := search.MinScore
	if minScore <= 0 {
		minScore = 0.6
	}

	return Tools{
		SearchDefaults: SearchDefaults{
			Snapshots: snapshots,
			TopK:      topK,
			MinScore:  minScore,
		},
	}
}

func applyStepDefaults(cfg map[string]*config.AgentStepConfig) map[string]*StepConfig {
	defaultCfg := defaultAgentConfig()
	resolved := make(map[string]*StepConfig, len(StepOrder))

	for _, step := range StepOrder {
		base := defaultCfg.Steps[step]
		override := cfg[step]
		merged := mergeStepConfig(step, base, override)
		resolved[step] = merged
	}

	return resolved
}

func mergeStepConfig(name string, base *config.AgentStepConfig, override *config.AgentStepConfig) *StepConfig {
	result := &config.AgentStepConfig{}
	if base != nil {
		*result = *base
	}
	if override != nil {
		applyStepOverride(result, override)
	}

	return buildStepConfig(name, result)
}

func applyStepOverride(target *config.AgentStepConfig, override *config.AgentStepConfig) {
	if override.Profile != nil {
		target.Profile = override.Profile
	}
	if override.SystemPrompt != nil {
		target.SystemPrompt = override.SystemPrompt
	}
	if len(override.Tools) > 0 {
		target.Tools = override.Tools
	}
	if len(override.ToolModes) > 0 {
		target.ToolModes = override.ToolModes
	}
}

func buildStepConfig(name string, step *config.AgentStepConfig) *StepConfig {
	profileValue := ""
	if step.Profile != nil {
		profileValue = *step.Profile
	}
	systemPromptValue := ""
	if step.SystemPrompt != nil {
		systemPromptValue = *step.SystemPrompt
	}

	return &StepConfig{
		Profile:      profileValue,
		SystemPrompt: systemPromptValue,
		Tools:        step.Tools,
		ToolModes:    normalizeToolModes(step.ToolModes),
		Index:        stepIndex(name),
	}
}

func stepIndex(name string) int {
	for idx, stepName := range StepOrder {
		if stepName == name {
			return idx
		}
	}
	return -1
}

func normalizeToolModes(modes map[string][]string) map[string]map[string]bool {
	normalized := make(map[string]map[string]bool, len(modes))
	for tool, list := range modes {
		toolKey := strings.ToLower(tool)
		if normalized[toolKey] == nil {
			normalized[toolKey] = make(map[string]bool)
		}
		for _, mode := range list {
			if mode == "" {
				continue
			}
			normalized[toolKey][strings.ToLower(mode)] = true
		}
	}
	return normalized
}

func defaultAgentConfig() *config.AgentConfig {
	return &config.AgentConfig{
		Defaults: &config.AgentDefaults{
			Profile:      "gemini-pro",
			SystemPrompt: "You are a multi-step agent that works in four steps: research, plan, execute, verify.\nResearch gathers context without changes. Plan turns findings into ordered tasks. Execute applies the changes. Verify checks outcomes and reports gaps.\nUse the memory tool to keep context between steps: call memory.list at the start of each step, and call memory.add at the end of each step to store key findings, decisions, and outputs.",
		},
		Tools: &config.AgentToolsConfig{
			SearchDefaults: &config.AgentSearchDefaults{
				Snapshots: []string{"_workdir_", "_stage_", "_head_"},
				TopK:      8,
				MinScore:  0.6,
			},
		},
		Steps: map[string]*config.AgentStepConfig{
			"research": {
				Profile:      stringPtr("gemini-flash"),
				SystemPrompt: stringPtr("Research step: discover context and constraints without modifying files.\nBest practices: start with memory.list, inspect relevant files and configs, note assumptions and risks, avoid destructive commands, and finish by calling memory.add with key findings and references."),
				Tools:        []string{"workspace", "search", "summarize", "git", "plan", "memory", "command", "patch"},
				ToolModes: map[string][]string{
					"workspace": {"list", "read", "stat", "exists"},
					"search":    {"embeddings"},
					"summarize": {"text", "file", "diff"},
					"git":       {"status", "log", "show", "diff", "branch", "current_branch"},
					"plan":      {"list"},
					"memory":    {"add", "list"},
				},
			},
			"plan": {
				Profile:      stringPtr("gemini-pro"),
				SystemPrompt: stringPtr("Plan step: build a clear, minimal task list to satisfy the goal.\nBest practices: review memory.list, identify dependencies and tests, keep steps actionable and ordered, register tasks with plan.add, and finish by calling memory.add with the plan and notable risks."),
				Tools:        []string{"workspace", "summarize", "git", "plan", "memory"},
				ToolModes: map[string][]string{
					"workspace": {"list", "read", "stat", "exists"},
					"summarize": {"text", "file", "diff"},
					"git":       {"status", "diff"},
					"plan":      {"add", "list"},
					"memory":    {"add", "list"},
				},
			},
			"execute": {
				Profile:      stringPtr("gemini-pro"),
				SystemPrompt: stringPtr("Execute step: implement the planned changes safely.\nBest practices: review memory.list and plan tasks, make focused edits, keep diffs small, update plan task completion, and call memory.add with changes made, files touched, and any follow-ups."),
				Tools:        []string{"workspace", "search", "summarize", "git", "plan", "memory", "command", "patch"},
				ToolModes: map[string][]string{
					"workspace": {"list", "read", "write", "replace", "delete", "mkdir", "stat", "exists"},
					"search":    {"embeddings"},
					"summarize": {"text", "file", "diff"},
					"git":       {"status", "diff", "show", "log", "branch", "current_branch", "stage", "commit"},
					"plan":      {"complete", "list"},
					"memory":    {"add", "list"},
				},
			},
			"verify": {
				Profile:      stringPtr("gemini-flash"),
				SystemPrompt: stringPtr("Verify step: validate changes and report gaps.\nBest practices: review memory.list, check git status/diff and run tests if needed, confirm requirements are met, call out missing verification, and finish with memory.add summarizing results and next steps.\nOutput format: include a line `VerificationResult: pass|fail`. If fail, include a `FailureTasks:` section with bullet tasks to fix."),
				Tools:        []string{"workspace", "search", "summarize", "git", "plan", "memory"},
				ToolModes: map[string][]string{
					"workspace": {"list", "read", "stat", "exists"},
					"search":    {"embeddings"},
					"summarize": {"text", "file", "diff"},
					"git":       {"status", "diff"},
					"plan":      {"list"},
					"memory":    {"add", "list"},
				},
			},
		},
	}
}

func stringPtr(value string) *string {
	return &value
}
