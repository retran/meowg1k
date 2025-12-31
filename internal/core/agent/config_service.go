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
	Steps    map[string]*StepConfig // Legacy support
	Defaults Defaults
	Tools    Tools

	Personas map[string]*config.PersonaConfig
	Flows    map[string][]string
	Safety   *config.AgentSafetyConfig
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

	personas := agentCfg.Personas
	if len(personas) == 0 {
		personas = defaultPersonas()
	}

	flows := agentCfg.Flows
	if len(flows) == 0 {
		flows = defaultFlows()
	}

	safety := agentCfg.Safety
	if safety == nil {
		safety = defaultSafety()
	}

	return &ResolvedConfig{
		Defaults: defaults,
		Tools:    tools,
		Steps:    steps,
		Personas: personas,
		Flows:    flows,
		Safety:   safety,
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
		systemPrompt = "You are a multi-step agent."
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
			SystemPrompt: "You are a multi-step agent.",
		},
		Tools: &config.AgentToolsConfig{
			SearchDefaults: &config.AgentSearchDefaults{
				Snapshots: []string{"_workdir_", "_stage_", "_head_"},
				TopK:      8,
				MinScore:  0.6,
			},
		},
		// Legacy defaults
		Steps: map[string]*config.AgentStepConfig{},
	}
}

func defaultPersonas() map[string]*config.PersonaConfig {
	return map[string]*config.PersonaConfig{
		"discover": {
			Role:             "Code Discovery Agent",
			Profile:          "gemini-flash",
			Tools:            []string{"list_files", "search_code", "read_file", "summarize", "memorize_fact", "delegate"},
			Instructions:     "Discover the context for the user goal.\nList files and search for relevant logic.\nMemorize key findings. Do not edit files.",
			AllowedDelegates: []string{"research_assistant"},
		},
		"plan": {
			Role:         "Planning Agent",
			Profile:      "gemini-pro",
			Tools:        []string{"recall_facts", "read_file", "create_plan"},
			Instructions: "Create a step-by-step task board based on discovered facts.\nVerify assumptions before finalizing.",
		},
		"execute": {
			Role:         "Execution Agent",
			Profile:      "gemini-pro",
			Tools:        []string{"recall_facts", "read_file", "write_file", "edit_file", "run_shell", "update_task", "run_task"},
			Instructions: "Execute the task board. Use edit_file for targeted changes.\nUpdate task status immediately.",
			AllowedTasks: []string{"doc", "test", "review"},
		},
		"verify": {
			Role:         "Verification Agent",
			Profile:      "gemini-flash",
			Tools:        []string{"run_shell", "get_diff", "summarize", "update_task", "restart_with_instruction"},
			Instructions: "Verify changes via tests/lints.\nIf failures exist, use restart_with_instruction to trigger a fix cycle.",
		},
	}
}

func defaultFlows() map[string][]string {
	return map[string][]string{
		"default": {"discover", "plan", "execute", "verify"},
		"quick":   {"discover", "execute", "verify"},
	}
}

func defaultSafety() *config.AgentSafetyConfig {
	return &config.AgentSafetyConfig{
		MaxSteps: 0,
		CircuitBreaker: &config.CircuitBreakerConfig{
			MaxRestarts: 5,
		},
		DryRun: false,
	}
}

func stringPtr(value string) *string {
	return &value
}
