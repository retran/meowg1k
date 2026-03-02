// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package starlark provides the Starlark script runtime and module system.
package starlark

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/retran/meowg1k/internal/domain/session"
	"github.com/retran/meowg1k/internal/ports"
)

// Runtime manages Starlark script execution.
type Runtime struct {
	workingDir     string
	registry       *Registry
	predeclared    starlark.StringDict
	llmServices    *LLMServices
	indexServices  *IndexServices
	sessionService ports.SessionService
	outputService  OutputWriter
	stdinReader    *bufio.Reader

	// Configuration storage - providers, models and presets defined via Starlark scripts
	// These configurations are applied to LLM and Index services during initialization
	providers map[string]ProviderConfig
	models    map[string]ModelConfig
	presets   map[string]PresetConfig
}

// ProviderConfig stores provider configuration from Starlark
type ProviderConfig struct {
	Type       string
	BaseURL    string
	APIKey     string
	Tokenizer  string
	RetryCount int
	ExtraOpts  map[string]interface{}
}

// ModelConfig stores model configuration from Starlark
type ModelConfig struct {
	Provider        string
	Model           string
	MaxInputTokens  int
	MaxOutputTokens int
	RateLimitRPM    int
	RateLimitTPM    int
	RateLimitRPD    int
	ExtraOpts       map[string]interface{}
}

// PresetConfig stores preset configuration from Starlark
type PresetConfig struct {
	Model            string
	Extends          string
	Temperature      float64
	MaxTokens        int
	TopP             float64
	TopK             int
	FrequencyPenalty float64
	PresencePenalty  float64
	ExtraOpts        map[string]interface{}
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
	}

	r.initModules()
	return r
}

// initModules registers all built-in modules.
func (r *Runtime) initModules() {
	// meow module for command registration
	r.predeclared["meow"] = r.createMeowModule()

	// env module is exposed globally for configuration
	r.predeclared["env"] = NewEnvModule()

	// Note: All other operational modules (git, fs, llm, shell, index, json,
	// ui, path, crypto, time, regexp, stdin) are NOT exposed globally.
	// They are provided exclusively via the handler context `ctx`.
}

// LoadScript executes a Starlark script file.
func (r *Runtime) LoadScript(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read script %s: %w", path, err)
	}

	thread := &starlark.Thread{
		Name: "meow",
		Load: r.makeLoadFunc(path),
	}

	_, err = starlark.ExecFile(thread, path, content, r.predeclared)
	if err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	return nil
}

// makeLoadFunc creates a load function for Starlark's load() statement.
// It resolves relative paths based on the current file being executed.
func (r *Runtime) makeLoadFunc(currentFile string) func(*starlark.Thread, string) (starlark.StringDict, error) {
	// Cache for loaded modules to avoid re-loading
	cache := make(map[string]starlark.StringDict)

	return func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		// Check cache first
		if dict, ok := cache[module]; ok {
			return dict, nil
		}

		// Resolve module path
		var modulePath string
		if strings.HasPrefix(module, "//") {
			// Bazel-style absolute path from workspace root: //packages/foo/bar.star
			// Try workspace first, then fallback to system config
			modulePath = filepath.Join(r.workingDir, ".meowg1k", module[2:])
		} else if filepath.IsAbs(module) {
			modulePath = module
		} else {
			// Relative to current file
			modulePath = filepath.Join(filepath.Dir(currentFile), module)
		}

		// Read module content
		content, err := os.ReadFile(modulePath)
		if err != nil {
			// If not found and using // prefix, try system config directory
			if strings.HasPrefix(module, "//") {
				homeDir, homeErr := os.UserHomeDir()
				if homeErr == nil {
					systemPath := filepath.Join(homeDir, ".config", "meowg1k", module[2:])
					content, err = os.ReadFile(systemPath)
					if err == nil {
						modulePath = systemPath
					}
				}
			}
			if err != nil {
				return nil, fmt.Errorf("failed to read module %s: %w", module, err)
			}
		}

		// Create a new thread for the loaded module with recursive load support
		moduleThread := &starlark.Thread{
			Name: "load:" + modulePath,
			Load: r.makeLoadFunc(modulePath), // Allow nested loads
		}

		// Execute module and return its globals
		globals, err := starlark.ExecFile(moduleThread, modulePath, content, r.predeclared)
		if err != nil {
			return nil, fmt.Errorf("failed to execute module %s: %w", module, err)
		}

		// Cache the result
		cache[module] = globals

		return globals, nil
	}
}

// Registry returns the command registry.
func (r *Runtime) Registry() *Registry {
	return r.registry
}

// RegisterProvider registers a provider configuration.
func (r *Runtime) RegisterProvider(name string, config ProviderConfig) {
	r.providers[name] = config
}

// RegisterModel registers a model configuration.
func (r *Runtime) RegisterModel(name string, config ModelConfig) {
	r.models[name] = config
}

// RegisterPreset registers a preset configuration.
func (r *Runtime) RegisterPreset(name string, config PresetConfig) {
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
func (r *Runtime) SetOutputService(service OutputWriter) {
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
// On TTY, chrome writes go through LogWriter() and stream tokens go through
// StreamToken() so everything is routed through the same BubbleTea program.
// On non-TTY, all ui functions are no-ops.
func (r *Runtime) CreateUIModuleForCtx(depth int) *starlarkstruct.Module {
	if r.outputService == nil {
		return NewUIModuleWithWriter(depth, false, io.Discard, &noopOutputWriter{})
	}
	// OutputWriter also satisfies StreamSender (has StreamToken method).
	isTTY := false
	var logWriter io.Writer = io.Discard
	if svc, ok := r.outputService.(interface {
		IsTTY() bool
		LogWriter() io.Writer
	}); ok {
		isTTY = svc.IsTTY()
		logWriter = svc.LogWriter()
	}
	return NewUIModuleWithWriter(depth, isTTY, logWriter, r.outputService)
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
		return starlark.NewBuiltin("session", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return starlark.None, fmt.Errorf("session service not initialized")
		})
	}
	return NewSessionModule(r.sessionService, currentSession)
}

// noopOutputWriter is a no-op implementation for when outputService is not set
type noopOutputWriter struct{}

func (n *noopOutputWriter) Print(content string) error              { return nil }
func (n *noopOutputWriter) PrintLine(content string) error          { return nil }
func (n *noopOutputWriter) Printf(format string, args ...any) error { return nil }
func (n *noopOutputWriter) StreamToken(delta string, done bool)     {}
