// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"testing"

	"github.com/retran/meowg1k/internal/domain/config"
)

type mockConfigResolver struct {
	cfg *config.Config
	err error
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	return m.cfg, m.err
}

func minimalAgentConfig() *config.AgentConfig {
	return &config.AgentConfig{
		SystemPrompt: "system",
		Tools: &config.AgentToolsConfig{
			SearchDefaults: &config.AgentSearchDefaults{
				Snapshots: []string{"_workdir_"},
				TopK:      8,
				MinScore:  0.6,
			},
			ToolDescriptions: map[string]string{},
		},
		Flows: map[string]*config.AgentFlowConfig{
			"default": {Instructions: "flow", Steps: []string{"discover", "plan", "execute", "verify"}},
		},
		Personas: map[string]*config.PersonaConfig{
			"discover": {Role: "r", Profile: "p", Tools: []string{"t"}, SystemPersona: "sp", UserInstructions: "ui"},
			"plan":     {Role: "r", Profile: "p", Tools: []string{"t"}, SystemPersona: "sp", UserInstructions: "ui"},
			"execute":  {Role: "r", Profile: "p", Tools: []string{"t"}, SystemPersona: "sp", UserInstructions: "ui"},
			"verify":   {Role: "r", Profile: "p", Tools: []string{"t"}, SystemPersona: "sp", UserInstructions: "ui"},
		},
		Safety: &config.AgentSafetyConfig{
			MaxSteps: 0,
			CircuitBreaker: &config.CircuitBreakerConfig{
				MaxRestarts: 1,
			},
			DryRun: false,
		},
	}
}

func TestService_Get_MissingAgentConfigErrors(t *testing.T) {
	service, err := NewService(&mockConfigResolver{cfg: &config.Config{}})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	_, err = service.Get()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestService_Get_Overrides(t *testing.T) {
	cfg := &config.Config{
		Agent: &config.AgentConfig{
			SystemPrompt: minimalAgentConfig().SystemPrompt,
			Tools: &config.AgentToolsConfig{
				SearchDefaults: minimalAgentConfig().Tools.SearchDefaults,
				ToolDescriptions: map[string]string{
					"read_file": "Custom read file description",
				},
			},
			Flows:    minimalAgentConfig().Flows,
			Personas: minimalAgentConfig().Personas,
			Safety:   minimalAgentConfig().Safety,
		},
	}

	service, err := NewService(&mockConfigResolver{cfg: cfg})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	resolved, err := service.Get()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resolved.Tools.ToolDescriptions["read_file"] != "Custom read file description" {
		t.Fatalf("expected tool description override to be applied")
	}
}
