// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package agent provides configuration resolution for agent mode.
package agent

import (
	"errors"
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
	SystemPrompt string
	Tools        Tools

	Personas map[string]*config.PersonaConfig
	Flows    map[string]*config.AgentFlowConfig
	Safety   *config.AgentSafetyConfig
}

// Tools are default tool settings for agent mode.
type Tools struct {
	SearchDefaults   SearchDefaults
	ToolDescriptions map[string]string
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

	agentCfg := cfg.Agent
	if agentCfg == nil {
		return nil, fmt.Errorf("agent configuration missing (no 'agent' section in config)")
	}

	resolved, err := resolveStrict(agentCfg)
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
		return nil, fmt.Errorf("agent.system_prompt is required")
	}

	toolsCfg := agentCfg.Tools
	if toolsCfg == nil {
		return nil, fmt.Errorf("agent.tools is required")
	}
	if toolsCfg.SearchDefaults == nil {
		return nil, fmt.Errorf("agent.tools.searchDefaults is required")
	}
	if len(toolsCfg.SearchDefaults.Snapshots) == 0 {
		return nil, fmt.Errorf("agent.tools.searchDefaults.snapshots is required")
	}
	if toolsCfg.SearchDefaults.TopK <= 0 {
		return nil, fmt.Errorf("agent.tools.searchDefaults.topK must be > 0")
	}
	if toolsCfg.SearchDefaults.MinScore <= 0 {
		return nil, fmt.Errorf("agent.tools.searchDefaults.minScore must be > 0")
	}

	flows := agentCfg.Flows
	if len(flows) == 0 {
		return nil, fmt.Errorf("agent.flows is required")
	}
	for name, flow := range flows {
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("agent.flows contains an empty name")
		}
		if flow == nil {
			return nil, fmt.Errorf("agent.flows.%s is nil", name)
		}
		if len(flow.Steps) == 0 {
			return nil, fmt.Errorf("agent.flows.%s.steps is required", name)
		}
	}
	if flows["default"] == nil {
		return nil, fmt.Errorf("agent.flows.default is required")
	}

	personas := agentCfg.Personas
	if len(personas) == 0 {
		return nil, fmt.Errorf("agent.personas is required")
	}
	for name, p := range personas {
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("agent.personas contains an empty name")
		}
		if p == nil {
			return nil, fmt.Errorf("agent.personas.%s is nil", name)
		}
		if strings.TrimSpace(p.Profile) == "" {
			return nil, fmt.Errorf("agent.personas.%s.profile is required", name)
		}
		// Tools must be explicitly present in config (can be empty for no-tools steps).
		if p.Tools == nil {
			return nil, fmt.Errorf("agent.personas.%s.tools is required", name)
		}
		if strings.TrimSpace(p.SystemPersona) == "" {
			return nil, fmt.Errorf("agent.personas.%s.system_persona is required", name)
		}
		if strings.TrimSpace(p.UserInstructions) == "" {
			return nil, fmt.Errorf("agent.personas.%s.user_instructions is required", name)
		}
	}
	for _, required := range []string{"discover", "plan", "execute", "verify"} {
		if personas[required] == nil {
			return nil, fmt.Errorf("agent.personas.%s is required", required)
		}
	}

	safety := agentCfg.Safety
	if safety == nil {
		return nil, fmt.Errorf("agent.safety is required")
	}
	if safety.CircuitBreaker == nil {
		return nil, fmt.Errorf("agent.safety.circuit_breaker is required")
	}
	if safety.CircuitBreaker.MaxRestarts <= 0 {
		return nil, fmt.Errorf("agent.safety.circuit_breaker.max_restarts must be > 0")
	}
	if safety.MaxSteps < 0 {
		return nil, fmt.Errorf("agent.safety.max_steps must be >= 0")
	}

	toolDescriptions := toolsCfg.ToolDescriptions
	if toolDescriptions == nil {
		toolDescriptions = map[string]string{}
	}

	// Ensure maps are non-nil to simplify downstream code.
	if agentCfg.Flows == nil || agentCfg.Personas == nil {
		return nil, errors.New("agent configuration must provide flows and personas")
	}

	return &ResolvedConfig{
		SystemPrompt: systemPrompt,
		Tools: Tools{
			SearchDefaults: SearchDefaults{
				Snapshots: toolsCfg.SearchDefaults.Snapshots,
				TopK:      toolsCfg.SearchDefaults.TopK,
				MinScore:  toolsCfg.SearchDefaults.MinScore,
			},
			ToolDescriptions: toolDescriptions,
		},
		Personas: agentCfg.Personas,
		Flows:    agentCfg.Flows,
		Safety:   safety,
	}, nil
}
