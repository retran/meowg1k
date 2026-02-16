// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/retran/meowg1k/internal/app"
	starlarkpkg "github.com/retran/meowg1k/internal/core/starlark"
	"github.com/retran/meowg1k/internal/domain/session"
)

// Package-level runtime instance shared across command execution
var globalRuntime *starlarkpkg.Runtime

// BuildStarlarkCommands loads Starlark scripts and builds dynamic commands.
// This function is integrated into the CLI initialization flow via Execute().
// workspaceRoot is the resolved workspace directory from WorkspaceService.
func BuildStarlarkCommands(container *app.Container, workspaceRoot string) error {
	// Create runtime with workspace root (not just cwd)
	runtime := starlarkpkg.NewRuntime(workspaceRoot)
	globalRuntime = runtime // Store for later use

	// Load scripts first to gather configuration
	loader := starlarkpkg.NewLoaderService(runtime)
	if err := loader.LoadAll(); err != nil {
		// It's OK if configs don't exist yet
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to load scripts: %w", err)
		}
	}

	// Apply Starlark configuration to ConfigService
	if runtime.HasConfiguration() {
		// Convert Starlark config to domain config
		config, err := runtime.ApplyConfigToYAML(nil)
		if err != nil {
			return fmt.Errorf("failed to convert Starlark config: %w", err)
		}

		// Validate configuration
		if err := runtime.ValidateConfiguration(); err != nil {
			return fmt.Errorf("invalid Starlark configuration: %w", err)
		}

		// Update ConfigService with Starlark config
		if err := container.ConfigService.Override(config); err != nil {
			return fmt.Errorf("failed to apply Starlark config: %w", err)
		}

		// Note: LLM and Index services are NOT created here during initialization
		// They will be created later when commands are actually executed with a proper container
		// This avoids issues with the minimal bootstrap container
	}

	// Build commands from registry
	commands := runtime.Registry().List()
	for _, cmd := range commands {
		cobraCmd, err := buildCobraCommand(runtime, cmd)
		if err != nil {
			return fmt.Errorf("failed to build command %s: %w", cmd.Name, err)
		}
		rootCmd.AddCommand(cobraCmd)
	}

	return nil
}

// buildCobraCommand converts a Starlark command to a Cobra command
func buildCobraCommand(runtime *starlarkpkg.Runtime, cmd *starlarkpkg.Command) (*cobra.Command, error) {
	cobraCmd := &cobra.Command{
		Use:   cmd.Name,
		Short: cmd.Description,
		Long:  cmd.LongDescription,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return executeStarlarkHandler(runtime, runtime.Registry(), cmd, cobraCmd, args)
		},
	}

	// Add flags
	for flagName, flagDef := range cmd.Flags {
		switch flagDef.Type {
		case "bool":
			defaultVal := coerceBool(flagDef.Default)
			if flagDef.Short != "" {
				cobraCmd.Flags().BoolP(flagName, flagDef.Short, defaultVal, flagDef.Description)
			} else {
				cobraCmd.Flags().Bool(flagName, defaultVal, flagDef.Description)
			}
		case "int":
			defaultVal := coerceInt(flagDef.Default)
			if flagDef.Short != "" {
				cobraCmd.Flags().IntP(flagName, flagDef.Short, defaultVal, flagDef.Description)
			} else {
				cobraCmd.Flags().Int(flagName, defaultVal, flagDef.Description)
			}
		case "float":
			defaultVal := coerceFloat(flagDef.Default)
			if flagDef.Short != "" {
				cobraCmd.Flags().Float64P(flagName, flagDef.Short, defaultVal, flagDef.Description)
			} else {
				cobraCmd.Flags().Float64(flagName, defaultVal, flagDef.Description)
			}
		default: // string
			defaultVal := coerceString(flagDef.Default)
			if flagDef.Short != "" {
				cobraCmd.Flags().StringP(flagName, flagDef.Short, defaultVal, flagDef.Description)
			} else {
				cobraCmd.Flags().String(flagName, defaultVal, flagDef.Description)
			}
		}

		if strings.Contains(flagDef.Description, "(default)") {
			if flag := cobraCmd.Flags().Lookup(flagName); flag != nil {
				flag.DefValue = ""
			}
		}

		if flagDef.Required {
			if err := cobraCmd.MarkFlagRequired(flagName); err != nil {
				return nil, fmt.Errorf("required flag %s not found: %w", flagName, err)
			}
		}
	}

	return cobraCmd, nil
}

// executeStarlarkHandler executes a Starlark command handler
func executeStarlarkHandler(runtime *starlarkpkg.Runtime, registry *starlarkpkg.Registry, cmd *starlarkpkg.Command, cobraCmd *cobra.Command, args []string) error {
	// Get container from context (created in PersistentPreRunE)
	ctx := cobraCmd.Context()
	if ctx == nil {
		return fmt.Errorf("command context is nil")
	}

	container, ok := ctx.Value(app.AppContainerKey).(*app.Container)
	if !ok || container == nil {
		return fmt.Errorf("container not found in context")
	}

	// Reapply Starlark configuration to this container's ConfigService
	if globalRuntime != nil && globalRuntime.HasConfiguration() {
		config, err := globalRuntime.ApplyConfigToYAML(nil)
		if err != nil {
			return fmt.Errorf("failed to convert Starlark config: %w", err)
		}

		if err := container.ConfigService.Override(config); err != nil {
			return fmt.Errorf("failed to apply Starlark config: %w", err)
		}
	}

	// Set up LLM services
	llmServices, err := container.CreateLLMServices()
	if err != nil {
		return fmt.Errorf("failed to create LLM services: %w", err)
	}
	runtime.SetLLMServices(llmServices)

	// Set up output service for buffered writing
	runtime.SetOutputService(container.OutputService)

	// Set up Index services (optional - some commands don't need them)
	indexServices, err := container.CreateIndexServicesForStarlark()
	if err == nil {
		runtime.SetIndexServices(indexServices)
	}
	// If index services fail to initialize, that's OK - commands that need them will fail later

	// Set up Session service (optional - some commands don't need it)
	sessionService, err := container.CreateSessionService()
	if err == nil {
		runtime.SetSessionService(sessionService)
	}
	// If session service fails to initialize, that's OK - commands that need it will fail later

	// Create root session for this command execution
	var rootSession *session.Session
	if sessionService != nil {
		rootSession, err = sessionService.CreateSession(ctx, nil, cmd.Name)
		if err != nil {
			return fmt.Errorf("failed to create root session: %w", err)
		}
		// Ensure session is marked as completed or failed at the end
		defer func() {
			if err != nil {
				_ = sessionService.FailSession(ctx, rootSession.ID)
			} else {
				_ = sessionService.CompleteSession(ctx, rootSession.ID)
			}
		}()
	}

	originalStdin := os.Stdin
	var restoreStdin func()
	defer func() {
		if restoreStdin != nil {
			restoreStdin()
		}
	}()

	isEmptyValue := func(val starlark.Value) bool {
		switch v := val.(type) {
		case starlark.String:
			return v.GoString() == ""
		case starlark.NoneType:
			return true
		default:
			return false
		}
	}

	stdinChecked := false
	stdinContent := ""
	readStdin := func() {
		if stdinChecked {
			return
		}
		stdinChecked = true

		stat, err := os.Stdin.Stat()
		if err != nil {
			return
		}
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return
		}

		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return
		}

		stdinContent = strings.TrimSpace(string(data))
		if len(data) == 0 {
			return
		}

		r, w, err := os.Pipe()
		if err != nil {
			return
		}
		if _, err := w.Write(data); err != nil {
			_ = w.Close()
			_ = r.Close()
			return
		}
		_ = w.Close()

		os.Stdin = r
		restoreStdin = func() {
			_ = r.Close()
			os.Stdin = originalStdin
		}
	}

	// Add flags as struct for dot-access (ctx.flags.name instead of ctx.flags.get("name"))
	// If flag wasn't provided by user, use default from flag definition
	flagsMembers := starlark.StringDict{}
	paramsMembers := make(map[string]starlark.Value) // For direct ctx.param_name access

	for flagName, flagDef := range cmd.Flags {
		var value starlark.Value
		flag := cobraCmd.Flags().Lookup(flagName)

		var paramDef *starlarkpkg.Param
		if cmd.Tool != nil {
			if p, ok := cmd.Tool.Params[flagName]; ok {
				paramDef = p
			}
		}

		// If flag wasn't changed by user and has a default, use it
		if flag != nil && !flag.Changed && flagDef.Default != nil {
			value = convertToStarlarkValue(flagDef.Default)
		} else {
			// Use value from cobra (either user-provided or cobra's default)
			switch flagDef.Type {
			case "bool":
				val, _ := cobraCmd.Flags().GetBool(flagName)
				value = starlark.Bool(val)
			case "int":
				val, _ := cobraCmd.Flags().GetInt(flagName)
				value = starlark.MakeInt(val)
			case "float":
				val, _ := cobraCmd.Flags().GetFloat64(flagName)
				value = starlark.Float(val)
			default: // string
				val, _ := cobraCmd.Flags().GetString(flagName)
				value = starlark.String(val)
			}
		}

		value = starlarkpkg.SanitizeParamValue(paramDef, value)

		// Handle from_stdin: if parameter is empty and from_stdin=True, read from stdin
		if paramDef != nil && paramDef.FromStdin {
			if isEmptyValue(value) {
				readStdin()
				if stdinContent != "" {
					value = starlarkpkg.SanitizeParamValue(paramDef, starlark.String(stdinContent))
				}
			}
			if paramDef.Required && isEmptyValue(value) {
				_ = cobraCmd.Usage()
				return fmt.Errorf("required parameter '%s' is missing. Use --%s flag or pipe content to stdin", flagName, flagName)
			}
		}

		flagsMembers[flagName] = value
		// Also inject directly as ctx.param_name for new tool system
		paramsMembers[flagName] = value
	}
	flagsStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, flagsMembers)

	// Add args as struct for dot-access by name
	argsMembers := starlark.StringDict{}
	for argName, argDef := range cmd.Args {
		var value starlark.Value
		if argDef.Index < len(args) {
			value = starlark.String(args[argDef.Index])
		} else if argDef.Default != nil {
			value = convertToStarlarkValue(argDef.Default)
		} else {
			value = starlark.None
		}
		argsMembers[argName] = value
		// Also inject directly as ctx.param_name for new tool system
		paramsMembers[argName] = value
	}

	if cmd.Tool != nil {
		if err := starlarkpkg.ValidateToolParams(runtime, registry, cmd.Tool, paramsMembers); err != nil {
			return err
		}
	}

	argsStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, argsMembers)

	// Create context struct
	ctxMembers := starlark.StringDict{
		"flags": flagsStruct,
		"args":  argsStruct,
		// Bazel-style: expose modules on ctx for structured access
		"fs":        runtime.CreateFSModuleForCtx(),
		"git":       runtime.CreateGitModuleForCtx(),
		"llm":       runtime.CreateLLMModuleForCtx(rootSession),
		"shell":     runtime.CreateShellModuleForCtx(),
		"index":     runtime.CreateIndexModuleForCtx(),
		"output":    runtime.CreateOutputModuleForCtx(),             // Buffered output API
		"session":   runtime.CreateSessionModuleForCtx(rootSession), // Root session for this command execution
		"json":      starlarkpkg.NewJSONModule(),
		"yaml":      starlarkpkg.NewYAMLModule(),
		"xml":       starlarkpkg.NewXMLModule(),
		"toml":      starlarkpkg.NewTOMLModule(),
		"csv":       starlarkpkg.NewCSVModule(),
		"env":       starlarkpkg.NewEnvModule(),
		"ui":        starlarkpkg.NewUIModule(),
		"path":      starlarkpkg.NewPathModule(),
		"crypto":    starlarkpkg.NewCryptoModule(),
		"time":      starlarkpkg.NewTimeModule(),
		"regexp":    starlarkpkg.NewRegexpModule(),
		"http":      starlarkpkg.NewHTTPModule(),
		"template":  starlarkpkg.NewTemplateModule(runtime.WorkingDir()),
		"stdin":     runtime.CreateStdinModuleForCtx(),
		"workspace": starlark.String(runtime.WorkingDir()),
		"run":       starlarkpkg.CreateRunFunction(registry, runtime, rootSession, 0), // Enable command chaining
	}

	ctxStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, ctxMembers)

	// Wrap context with parameter injection for new tool system
	ctxWithParams := starlarkpkg.CreateContextWithParams(ctxStruct, paramsMembers)

	// Call handler
	thread := &starlark.Thread{
		Name: "meow-handler",
	}

	// Disable print() - handlers should use ctx.output instead
	thread.Print = func(t *starlark.Thread, msg string) {
		panic("print() is disabled. Use ctx.output.writeline() instead")
	}

	// Respect context cancellation before starting handler
	if cobraCmd.Context() != nil {
		select {
		case <-cobraCmd.Context().Done():
			return cobraCmd.Context().Err()
		default:
			// continue
		}
	}

	// Call handler with context that has injected parameters
	_, err = starlark.Call(thread, cmd.Handler, starlark.Tuple{ctxWithParams}, nil)
	if err != nil {
		return fmt.Errorf("handler failed: %w", err)
	}

	// Flush buffered output at the end
	if err := container.OutputService.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	return nil
}

// convertToStarlarkValue converts a Go value to a Starlark value
func convertToStarlarkValue(v any) starlark.Value {
	if v == nil {
		return starlark.None
	}

	switch x := v.(type) {
	case starlark.Value:
		return x
	case bool:
		return starlark.Bool(x)
	case int:
		return starlark.MakeInt(x)
	case int64:
		return starlark.MakeInt64(x)
	case float64:
		return starlark.Float(x)
	case string:
		return starlark.String(x)
	default:
		// Fallback to string representation
		return starlark.String(fmt.Sprint(v))
	}
}

// Safe coercion helpers to avoid panics on mismatched default types
func coerceBool(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case starlark.Bool:
		return bool(x)
	case string:
		// accept common string forms
		switch strings.ToLower(x) {
		case "1", "true", "t", "yes", "y":
			return true
		default:
			return false
		}
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	default:
		return false
	}
}

func coerceInt(v any) int {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		if i, err := strconv.Atoi(x); err == nil {
			return i
		}
		return 0
	default:
		return 0
	}
}

func coerceFloat(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return f
		}
		return 0
	default:
		return 0
	}
}

func coerceString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case starlark.String:
		return string(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return ""
	}
}
