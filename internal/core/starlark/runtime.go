// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package starlark provides the Starlark script runtime and module system.
package starlark

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"

	"github.com/retran/meowg1k/internal/domain/session"
	"github.com/retran/meowg1k/internal/ports"
)

// Runtime manages Starlark script execution.
type Runtime struct {
	sessionService ports.SessionService
	outputService  ports.UIWriter
	ctx            context.Context
	registry       *Registry
	predeclared    starlark.StringDict
	llmServices    *LLMServices
	indexServices  *IndexServices
	stdinReader    *bufio.Reader
	providers      map[string]ProviderConfig
	models         map[string]ModelConfig
	presets        map[string]PresetConfig
	workingDir     string
}

// ProviderConfig stores provider configuration from Starlark.
type ProviderConfig struct {
	ExtraOpts            map[string]interface{}
	Type                 string
	BaseURL              string
	APIKey               string //nolint:gosec // API key is user-provided configuration, not a hardcoded secret
	Tokenizer            string
	AppID                string
	EditorVersion        string
	EditorPluginVersion  string
	UserAgent            string
	CopilotIntegrationID string
	OpenAIOrganization   string
	RetryCount           int
}

// ModelConfig stores model configuration from Starlark.
type ModelConfig struct {
	ExtraOpts       map[string]interface{}
	Provider        string
	Model           string
	MaxInputTokens  int
	MaxOutputTokens int
	RateLimitRPM    int
	RateLimitTPM    int
	RateLimitRPD    int
}

// PresetConfig stores preset configuration from Starlark.
type PresetConfig struct {
	ExtraOpts        map[string]interface{}
	Model            string
	Extends          string
	Temperature      float64
	MaxTokens        int
	TopP             float64
	TopK             int
	FrequencyPenalty float64
	PresencePenalty  float64
}

// NewRuntime creates a new Starlark runtime.
func NewRuntime(workingDir string) *Runtime {
	r := &Runtime{
		workingDir:  workingDir,
		registry:    NewRegistry(),
		predeclared: make(starlark.StringDict),
		providers:   make(map[string]ProviderConfig),
		models:      make(map[string]ModelConfig),
		presets:     make(map[string]PresetConfig),
		ctx:         context.Background(),
	}

	r.initModules()
	return r
}

// SetContext sets the context used for LLM and other cancellable operations.
// Call this with the shutdown context before executing any handlers.
func (r *Runtime) SetContext(ctx context.Context) {
	r.ctx = ctx
}

// initModules registers all built-in modules.
func (r *Runtime) initModules() {
	r.predeclared["meow"] = r.createMeowModule()
	r.predeclared["env"] = NewEnvModule()

	// Note: All other operational modules (git, fs, llm, shell, index, json,
	// ui, path, crypto, time, regexp, stdin) are NOT exposed globally.
	// They are provided exclusively via the handler context `ctx`.
}

// LoadScript executes a Starlark script file.
func (r *Runtime) LoadScript(path string) error {
	content, err := os.ReadFile(path) //nolint:gosec // user-provided script path
	if err != nil {
		return fmt.Errorf("failed to read script %s: %w", path, err)
	}

	thread := &starlark.Thread{
		Name: "meow",
		Load: r.makeLoadFunc(path),
	}

	_, err = starlark.ExecFileOptions(&syntax.FileOptions{}, thread, path, content, r.predeclared)
	if err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	return nil
}

// makeLoadFunc creates a load function for Starlark's load() statement.
// It resolves relative paths based on the current file being executed.
func (r *Runtime) makeLoadFunc(currentFile string) func(*starlark.Thread, string) (starlark.StringDict, error) { //nolint:gocognit // complexity inherent in resolving and caching load paths across module types
	// Cache for loaded modules to avoid re-loading
	cache := make(map[string]starlark.StringDict)

	return func(_ *starlark.Thread, module string) (starlark.StringDict, error) {
		if dict, ok := cache[module]; ok {
			return dict, nil
		}

		var modulePath string
		switch {
		case strings.HasPrefix(module, "//"):
			// Bazel-style absolute path from workspace root: //packages/foo/bar.star
			// Try workspace first, then fallback to system config
			modulePath = filepath.Join(r.workingDir, ".meowg1k", module[2:])
		case filepath.IsAbs(module):
			modulePath = module
		default:
			// Relative to current file
			modulePath = filepath.Join(filepath.Dir(currentFile), module)
		}

		content, err := os.ReadFile(modulePath) //nolint:gosec // user-provided module path
		if err != nil {                         //nolint:nestif // nested fallback path for system config directory resolution
			// If not found and using // prefix, try system config directory
			if strings.HasPrefix(module, "//") {
				homeDir, homeErr := os.UserHomeDir()
				if homeErr == nil {
					systemPath := filepath.Join(homeDir, ".config", "meowg1k", module[2:])
					content, err = os.ReadFile(systemPath) //nolint:gosec // user-provided module path
					if err == nil {
						modulePath = systemPath
					}
				}
			}
			if err != nil {
				return nil, fmt.Errorf("failed to read module %s: %w", module, err)
			}
		}

		moduleThread := &starlark.Thread{
			Name: "load:" + modulePath,
			Load: r.makeLoadFunc(modulePath), // Allow nested loads
		}

		globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, moduleThread, modulePath, content, r.predeclared)
		if err != nil {
			return nil, fmt.Errorf("failed to execute module %s: %w", module, err)
		}

		cache[module] = globals

		return globals, nil
	}
}

// Registry returns the command registry.
func (r *Runtime) Registry() *Registry {
	return r.registry
}

// RegisterProvider registers a provider configuration.
func (r *Runtime) RegisterProvider(name string, config ProviderConfig) { //nolint:gocritic // hugeParam: config passed by value intentionally for immutability
	r.providers[name] = config
}

// RegisterModel registers a model configuration.
func (r *Runtime) RegisterModel(name string, config ModelConfig) { //nolint:gocritic // hugeParam: config passed by value intentionally for immutability
	r.models[name] = config
}

// RegisterPreset registers a preset configuration.
func (r *Runtime) RegisterPreset(name string, config PresetConfig) { //nolint:gocritic // hugeParam: config passed by value intentionally for immutability
	r.presets[name] = config
}

// WorkingDir returns the runtime working directory (workspace root).
func (r *Runtime) WorkingDir() string {
	return r.workingDir
}

// CreateFSModuleForCtx returns a fs module instance for ctx.
func (r *Runtime) CreateFSModuleForCtx() starlark.Value { return r.createFSModule() }

// CreateGitModuleForCtx returns a git module instance for ctx.
func (r *Runtime) CreateGitModuleForCtx() starlark.Value { return r.createGitModule() }

// CreateLLMModuleForCtx returns an llm module instance for ctx.
// currentSession is the session this context belongs to (can be nil if no session).
func (r *Runtime) CreateLLMModuleForCtx(currentSession *session.Session) starlark.Value {
	return r.createLLMModule(currentSession)
}

// CreateShellModuleForCtx returns a shell module instance for ctx.
func (r *Runtime) CreateShellModuleForCtx() starlark.Value { return r.createShellModule() }

// CreateIndexModuleForCtx returns an index module instance for ctx.
func (r *Runtime) CreateIndexModuleForCtx() starlark.Value { return r.createIndexModule() }

// CreateStdinModuleForCtx returns a stdin module instance for ctx.
func (r *Runtime) CreateStdinModuleForCtx() starlark.Value { return r.createStdinModule() }

// SetOutputService sets the output service for the runtime.
func (r *Runtime) SetOutputService(service ports.UIWriter) {
	r.outputService = service
}

// CreateOutputModuleForCtx returns an output module instance for ctx.
func (r *Runtime) CreateOutputModuleForCtx() starlark.Value {
	if r.outputService == nil {
		// Return a no-op module if service is not set
		return NewOutputModule(&noopOutputWriter{})
	}
	return NewOutputModule(r.outputService)
}

// CreateUIModuleForCtx returns a ui module wired to the output service.
// On TTY, turn/step messages go through the BubbleTea program.
// On non-TTY, all ui functions are no-ops.
func (r *Runtime) CreateUIModuleForCtx(depth int) *starlarkstruct.Module {
	return NewUIModuleWithUIWriter(depth, r.outputService)
}

// SetSessionService sets the session service for the runtime.
func (r *Runtime) SetSessionService(service ports.SessionService) {
	r.sessionService = service
}

// CreateSessionModuleForCtx returns a session module instance for ctx.
// currentSession is the session this context belongs to (can be nil if no session).
func (r *Runtime) CreateSessionModuleForCtx(currentSession *session.Session) starlark.Value {
	if r.sessionService == nil {
		// Return a no-op builtin if service is not set
		return starlark.NewBuiltin("session", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
			return starlark.None, fmt.Errorf("session service not initialized")
		})
	}
	return NewSessionModule(r.sessionService, currentSession)
}

// noopOutputWriter is a no-op implementation of ports.OutputWriter for when outputService is not set.
type noopOutputWriter struct{}

func (n *noopOutputWriter) Print(_ string) error            { return nil }
func (n *noopOutputWriter) PrintLine(_ string) error        { return nil }
func (n *noopOutputWriter) Printf(_ string, _ ...any) error { return nil }
