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
	Personas     map[string]*config.PersonaConfig
	Pipelines    map[string]*config.AgentPipelineConfig
	Safety       *config.AgentSafetyConfig
	SystemPrompt string
	Tools        Tools
}

// Tools are default tool settings for agent mode.
type Tools struct {
	ToolDescriptions map[string]string
	SearchDefaults   SearchDefaults
}

// SearchDefaults are defaults for embeddings search.
type SearchDefaults struct {
	Snapshots []string
	TopK      int
	MinScore  float32
}

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

	if cfg.Agent == nil {
		return nil, fmt.Errorf("agent configuration missing (no 'agent' section in config)")
	}

	resolved, err := resolveStrict(cfg.Agent)
	if err != nil {
		return nil, err
	}

	return resolved, nil
}

func resolveStrict(agentCfg *config.AgentConfig) (*ResolvedConfig, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("agent configuration is nil")
	}

	systemPrompt := strings.TrimSpace(agentCfg.SystemPrompt)
	if systemPrompt == "" {
		return nil, fmt.Errorf("flows.agent.system_prompt is required")
	}

	if err := validateToolsConfig(agentCfg.Tools); err != nil {
		return nil, err
	}

	if err := validatePipelinesConfig(agentCfg.Pipelines); err != nil {
		return nil, err
	}

	if err := validatePersonasConfig(agentCfg.Personas); err != nil {
		return nil, err
	}

	if err := validateSafetyConfig(agentCfg.Safety); err != nil {
		return nil, err
	}

	toolDescriptions := agentCfg.Tools.ToolDescriptions
	if toolDescriptions == nil {
		toolDescriptions = map[string]string{}
	}

	return &ResolvedConfig{
		SystemPrompt: systemPrompt,
		Tools: Tools{
			SearchDefaults: SearchDefaults{
				Snapshots: agentCfg.Tools.SearchDefaults.Snapshots,
				TopK:      agentCfg.Tools.SearchDefaults.TopK,
				MinScore:  agentCfg.Tools.SearchDefaults.MinScore,
			},
			ToolDescriptions: toolDescriptions,
		},
		Personas:  agentCfg.Personas,
		Pipelines: agentCfg.Pipelines,
		Safety:    agentCfg.Safety,
	}, nil
}

func validateToolsConfig(toolsCfg *config.AgentToolsConfig) error {
	if toolsCfg == nil {
		return fmt.Errorf("flows.agent.tools is required")
	}
	if toolsCfg.SearchDefaults == nil {
		return fmt.Errorf("flows.agent.tools.search_defaults is required")
	}
	if len(toolsCfg.SearchDefaults.Snapshots) == 0 {
		return fmt.Errorf("flows.agent.tools.search_defaults.snapshots is required")
	}
	if toolsCfg.SearchDefaults.TopK <= 0 {
		return fmt.Errorf("flows.agent.tools.search_defaults.top_k must be > 0")
	}
	if toolsCfg.SearchDefaults.MinScore <= 0 {
		return fmt.Errorf("flows.agent.tools.search_defaults.min_score must be > 0")
	}
	return nil
}

func validatePipelinesConfig(pipelines map[string]*config.AgentPipelineConfig) error {
	if len(pipelines) == 0 {
		return fmt.Errorf("flows.agent.pipelines is required")
	}
	for name, pipeline := range pipelines {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("flows.agent.pipelines contains an empty name")
		}
		if pipeline == nil {
			return fmt.Errorf("flows.agent.pipelines.%s is nil", name)
		}
		if len(pipeline.Steps) == 0 {
			return fmt.Errorf("flows.agent.pipelines.%s.steps is required", name)
		}
	}
	if pipelines["default"] == nil {
		return fmt.Errorf("flows.agent.pipelines.default is required")
	}
	return nil
}

func validatePersonasConfig(personas map[string]*config.PersonaConfig) error {
	if len(personas) == 0 {
		return fmt.Errorf("flows.agent.personas is required")
	}
	for name, p := range personas {
		if err := validatePersona(name, p); err != nil {
			return err
		}
	}
	for _, required := range []string{"discover", "plan", "execute", "verify"} {
		if personas[required] == nil {
			return fmt.Errorf("flows.agent.personas.%s is required", required)
		}
	}
	return nil
}

func validatePersona(name string, p *config.PersonaConfig) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("flows.agent.personas contains an empty name")
	}
	if p == nil {
		return fmt.Errorf("flows.agent.personas.%s is nil", name)
	}
	if strings.TrimSpace(p.Preset) == "" {
		return fmt.Errorf("flows.agent.personas.%s.preset is required", name)
	}
	// Tools must be explicitly present in config (can be empty for no-tools steps).
	if p.Tools == nil {
		return fmt.Errorf("flows.agent.personas.%s.tools is required", name)
	}
	if strings.TrimSpace(p.SystemPersona) == "" {
		return fmt.Errorf("flows.agent.personas.%s.system_persona is required", name)
	}
	if strings.TrimSpace(p.UserInstructions) == "" {
		return fmt.Errorf("flows.agent.personas.%s.user_instructions is required", name)
	}
	return nil
}

func validateSafetyConfig(safety *config.AgentSafetyConfig) error {
	if safety == nil {
		return fmt.Errorf("flows.agent.safety is required")
	}
	if safety.CircuitBreaker == nil {
		return fmt.Errorf("flows.agent.safety.circuit_breaker is required")
	}
	if safety.CircuitBreaker.MaxRestarts <= 0 {
		return fmt.Errorf("flows.agent.safety.circuit_breaker.max_restarts must be > 0")
	}
	if safety.MaxSteps < 0 {
		return fmt.Errorf("flows.agent.safety.max_steps must be >= 0")
	}
	return nil
}
