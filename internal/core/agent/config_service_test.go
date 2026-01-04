package agent

import (
	"errors"
	"testing"

	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConfigResolver struct {
	cfg *config.Config
	err error
}

func (m *mockConfigResolver) Get() (*config.Config, error) {
	return m.cfg, m.err
}

func TestNewService(t *testing.T) {
	s, err := NewService(&mockConfigResolver{})
	require.NoError(t, err)
	assert.NotNil(t, s)

	s, err = NewService(nil)
	assert.Error(t, err)
	assert.Nil(t, s)
}

func TestService_Get_Errors(t *testing.T) {
	// 1. Resolver error
	resolver := &mockConfigResolver{err: errors.New("resolver error")}
	s, _ := NewService(resolver)
	cfg, err := s.Get()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "resolver error")

	// 2. Resolver returns nil config
	resolver = &mockConfigResolver{cfg: nil}
	s, _ = NewService(resolver)
	cfg, err = s.Get()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "config is nil")

	// 3. Missing agent config
	resolver = &mockConfigResolver{cfg: &config.Config{}}
	s, _ = NewService(resolver)
	cfg, err = s.Get()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "agent configuration missing")
	
	// 4. Service is nil
	var nilService *Service
	cfg, err = nilService.Get()
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestService_Get_ValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		mod      func(*config.AgentConfig)
		errMatch string
	}{
		{
			name: "missing system prompt",
			mod: func(c *config.AgentConfig) { c.SystemPrompt = "" },
			errMatch: "system_prompt is required",
		},
		{
			name: "missing tools config",
			mod: func(c *config.AgentConfig) { c.Tools = nil },
			errMatch: "tools is required",
		},
		{
			name: "missing search defaults",
			mod: func(c *config.AgentConfig) { c.Tools.SearchDefaults = nil },
			errMatch: "search_defaults is required",
		},
		{
			name: "missing search snapshots",
			mod: func(c *config.AgentConfig) { c.Tools.SearchDefaults.Snapshots = nil },
			errMatch: "snapshots is required",
		},
		{
			name: "invalid top k",
			mod: func(c *config.AgentConfig) { c.Tools.SearchDefaults.TopK = 0 },
			errMatch: "top_k must be > 0",
		},
		{
			name: "invalid min score",
			mod: func(c *config.AgentConfig) { c.Tools.SearchDefaults.MinScore = 0 },
			errMatch: "min_score must be > 0",
		},
		{
			name: "empty pipelines",
			mod: func(c *config.AgentConfig) { c.Pipelines = nil },
			errMatch: "pipelines is required",
		},
		{
			name: "missing default pipeline",
			mod: func(c *config.AgentConfig) { 
				delete(c.Pipelines, "default")
				c.Pipelines["other"] = &config.AgentPipelineConfig{Steps: []string{"s"}}
			},
			errMatch: "default is required",
		},
		{
			name: "pipeline nil",
			mod: func(c *config.AgentConfig) { c.Pipelines["default"] = nil },
			errMatch: "is nil",
		},
		{
			name: "pipeline empty steps",
			mod: func(c *config.AgentConfig) { c.Pipelines["default"].Steps = nil },
			errMatch: "steps is required",
		},
		{
			name: "pipeline empty name",
			mod: func(c *config.AgentConfig) { c.Pipelines[""] = &config.AgentPipelineConfig{Steps: []string{"s"}} },
			errMatch: "empty name",
		},
		{
			name: "missing personas",
			mod: func(c *config.AgentConfig) { c.Personas = nil },
			errMatch: "personas is required",
		},
		{
			name: "persona nil",
			mod: func(c *config.AgentConfig) { c.Personas["discover"] = nil },
			errMatch: "is nil",
		},
		{
			name: "persona empty name",
			mod: func(c *config.AgentConfig) { c.Personas[""] = &config.PersonaConfig{} },
			errMatch: "empty name",
		},
		{
			name: "persona missing preset",
			mod: func(c *config.AgentConfig) { c.Personas["discover"].Preset = "" },
			errMatch: "preset is required",
		},
		{
			name: "persona missing tools",
			mod: func(c *config.AgentConfig) { c.Personas["discover"].Tools = nil },
			errMatch: "tools is required",
		},
		{
			name: "persona missing system persona",
			mod: func(c *config.AgentConfig) { c.Personas["discover"].SystemPersona = "" },
			errMatch: "system_persona is required",
		},
		{
			name: "persona missing instructions",
			mod: func(c *config.AgentConfig) { c.Personas["discover"].UserInstructions = "" },
			errMatch: "user_instructions is required",
		},
		{
			name: "missing required persona",
			mod: func(c *config.AgentConfig) { delete(c.Personas, "discover") },
			errMatch: "discover is required",
		},
		{
			name: "missing safety",
			mod: func(c *config.AgentConfig) { c.Safety = nil },
			errMatch: "safety is required",
		},
		{
			name: "missing circuit breaker",
			mod: func(c *config.AgentConfig) { c.Safety.CircuitBreaker = nil },
			errMatch: "circuit_breaker is required",
		},
		{
			name: "invalid circuit breaker",
			mod: func(c *config.AgentConfig) { c.Safety.CircuitBreaker.MaxRestarts = 0 },
			errMatch: "max_restarts must be > 0",
		},
		{
			name: "invalid max steps",
			mod: func(c *config.AgentConfig) { c.Safety.MaxSteps = -1 },
			errMatch: "max_steps must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Deep copy or re-construct for isolation
			// Re-constructing is easier given the structure size
			currentCfg := &config.Config{
				Agent: &config.AgentConfig{
					SystemPrompt: "sys",
					Tools: &config.AgentToolsConfig{
						SearchDefaults: &config.AgentSearchDefaults{
							Snapshots: []string{"main"},
							TopK:      1,
							MinScore:  0.1,
						},
					},
					Pipelines: map[string]*config.AgentPipelineConfig{
						"default": {Steps: []string{"plan"}},
					},
					Personas: map[string]*config.PersonaConfig{
						"discover": {Preset: "p", Tools: []string{}, SystemPersona: "s", UserInstructions: "u"},
						"plan":     {Preset: "p", Tools: []string{}, SystemPersona: "s", UserInstructions: "u"},
						"execute":  {Preset: "p", Tools: []string{}, SystemPersona: "s", UserInstructions: "u"},
						"verify":   {Preset: "p", Tools: []string{}, SystemPersona: "s", UserInstructions: "u"},
					},
					Safety: &config.AgentSafetyConfig{
						CircuitBreaker: &config.CircuitBreakerConfig{MaxRestarts: 1},
					},
				},
			}
			
			tt.mod(currentCfg.Agent)
			
			resolver := &mockConfigResolver{cfg: currentCfg}
			s, _ := NewService(resolver)
			_, err := s.Get()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMatch)
		})
	}
}

func TestService_Get_Success(t *testing.T) {
	validCfg := &config.Config{
		Agent: &config.AgentConfig{
			SystemPrompt: "sys",
			Tools: &config.AgentToolsConfig{
				SearchDefaults: &config.AgentSearchDefaults{
					Snapshots: []string{"main"},
					TopK:      1,
					MinScore:  0.1,
				},
				ToolDescriptions: map[string]string{"t": "d"},
			},
			Pipelines: map[string]*config.AgentPipelineConfig{
				"default": {Steps: []string{"plan"}},
			},
			Personas: map[string]*config.PersonaConfig{
				"discover": {Preset: "p", Tools: []string{}, SystemPersona: "s", UserInstructions: "u"},
				"plan":     {Preset: "p", Tools: []string{}, SystemPersona: "s", UserInstructions: "u"},
				"execute":  {Preset: "p", Tools: []string{}, SystemPersona: "s", UserInstructions: "u"},
				"verify":   {Preset: "p", Tools: []string{}, SystemPersona: "s", UserInstructions: "u"},
			},
			Safety: &config.AgentSafetyConfig{
				CircuitBreaker: &config.CircuitBreakerConfig{MaxRestarts: 1},
				MaxSteps: 10,
			},
		},
	}
	
	resolver := &mockConfigResolver{cfg: validCfg}
	s, _ := NewService(resolver)
	resolved, err := s.Get()
	require.NoError(t, err)
	assert.NotNil(t, resolved)
	
	assert.Equal(t, "sys", resolved.SystemPrompt)
	assert.Equal(t, "d", resolved.Tools.ToolDescriptions["t"])
	assert.Equal(t, []string{"main"}, resolved.Tools.SearchDefaults.Snapshots)
}