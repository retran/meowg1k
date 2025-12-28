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

func TestService_Get_Defaults(t *testing.T) {
	service, err := NewService(&mockConfigResolver{cfg: &config.Config{}})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	resolved, err := service.Get()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resolved.Defaults.Profile == "" {
		t.Fatal("expected default profile to be set")
	}
	if resolved.Tools.SearchDefaults.TopK == 0 {
		t.Fatal("expected search defaults to be set")
	}

	research := resolved.Steps["research"]
	if research == nil {
		t.Fatal("expected research step to be present")
	}
	if research.Profile == "" {
		t.Fatal("expected research profile to be set")
	}
	if len(research.Tools) == 0 {
		t.Fatal("expected research tools to be set")
	}
}

func TestService_Get_Overrides(t *testing.T) {
	cfg := &config.Config{
		Agent: &config.AgentConfig{
			Defaults: &config.AgentDefaults{
				Profile:      "gemini-flash",
				SystemPrompt: "Custom prompt",
			},
			Steps: map[string]*config.AgentStepConfig{
				"research": {
					Profile:      stringPtr("gemini-pro"),
					SystemPrompt: stringPtr("Step prompt"),
					Tools:        []string{"workspace"},
					ToolModes: map[string][]string{
						"workspace": {"read"},
					},
				},
			},
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

	if resolved.Defaults.Profile != "gemini-flash" {
		t.Fatalf("expected defaults profile 'gemini-flash', got %q", resolved.Defaults.Profile)
	}
	research := resolved.Steps["research"]
	if research.Profile != "gemini-pro" {
		t.Fatalf("expected research profile 'gemini-pro', got %q", research.Profile)
	}
	if !research.AllowsToolMode("workspace", "read") {
		t.Fatal("expected workspace/read to be allowed")
	}
}
