// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
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
	"github.com/retran/meowg1k/internal/ports"
)

// Package-level runtime instance shared across command execution.
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

// registerFlag adds a single flag to a cobra command based on its type.
func registerFlag(cobraCmd *cobra.Command, flagName string, flagDef *starlarkpkg.FlagDef) {
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
}

// addCommandFlags registers all flags for a command and marks required ones.
func addCommandFlags(cobraCmd *cobra.Command, cmd *starlarkpkg.Command) error {
	for flagName, flagDef := range cmd.Flags {
		registerFlag(cobraCmd, flagName, flagDef)

		if strings.Contains(flagDef.Description, "(default)") {
			if flag := cobraCmd.Flags().Lookup(flagName); flag != nil {
				flag.DefValue = ""
			}
		}

		// Only enforce required at Cobra level if stdin can't satisfy it.
		// When from_stdin=True, the handler reads stdin at runtime, so Cobra
		// must not reject the command before our handler gets a chance to run.
		if flagDef.Required && !flagDef.FromStdin {
			if err := cobraCmd.MarkFlagRequired(flagName); err != nil {
				return fmt.Errorf("required flag %s not found: %w", flagName, err)
			}
		}
	}
	return nil
}

// buildCobraCommand converts a Starlark command to a Cobra command.
func buildCobraCommand(runtime *starlarkpkg.Runtime, cmd *starlarkpkg.Command) (*cobra.Command, error) {
	cobraCmd := &cobra.Command{
		Use:   cmd.Name,
		Short: cmd.Description,
		Long:  cmd.LongDescription,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return executeStarlarkHandler(runtime, runtime.Registry(), cmd, cobraCmd, args)
		},
	}

	if err := addCommandFlags(cobraCmd, cmd); err != nil {
		return nil, err
	}

	return cobraCmd, nil
}

// setupRuntimeServices wires the runtime with all required services from the container.
func setupRuntimeServices(runtime *starlarkpkg.Runtime, container *app.Container) error {
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

	// Wire the shutdown context so LLM calls are cancelled when the user aborts.
	runtime.SetContext(container.ShutdownService.Context())

	// Wire Ctrl+C in the TUI to cancel the running operation via shutdown.
	container.OutputService.SetCancel(container.ShutdownService.Shutdown)

	// Set up Index services (optional - some commands don't need them)
	if indexServices, err := container.CreateIndexServicesForStarlark(); err == nil {
		runtime.SetIndexServices(indexServices)
	}
	// If index services fail to initialize, that's OK - commands that need them will fail later

	return nil
}

// setupSessionService creates the session service and returns it (may be nil if unavailable).
func setupSessionService(runtime *starlarkpkg.Runtime, container *app.Container) ports.SessionService {
	sessionService, err := container.CreateSessionService()
	if err != nil {
		return nil
	}
	runtime.SetSessionService(sessionService)
	return sessionService
}

// stdinReader manages lazy, once-only stdin reading with pipe replay.
type stdinReader struct {
	original *os.File
	restore  func()
	content  string
	checked  bool
}

func newStdinReader() *stdinReader {
	return &stdinReader{original: os.Stdin}
}

// read reads stdin once and replays it via a pipe so it can be re-read by the handler.
func (s *stdinReader) read() {
	if s.checked {
		return
	}
	s.checked = true

	stat, err := os.Stdin.Stat()
	if err != nil {
		return
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		s.content = strings.TrimSpace(string(data))
		return
	}

	s.content = strings.TrimSpace(string(data))

	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	if _, err := w.Write(data); err != nil {
		if closeErr := w.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close pipe writer: %v\n", closeErr)
		}
		if closeErr := r.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close pipe reader: %v\n", closeErr)
		}
		return
	}
	if closeErr := w.Close(); closeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to close pipe writer: %v\n", closeErr)
	}

	os.Stdin = r
	s.restore = func() {
		if closeErr := r.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close pipe reader: %v\n", closeErr)
		}
		os.Stdin = s.original
	}
}

// isEmptyStarlarkValue returns true for empty string or None values.
func isEmptyStarlarkValue(val starlark.Value) bool {
	switch v := val.(type) {
	case starlark.String:
		return v.GoString() == ""
	case starlark.NoneType:
		return true
	default:
		return false
	}
}

// getFlagValue reads the current value of a flag from cobra flags by type.
func getFlagValue(cobraCmd *cobra.Command, flagName, flagType string) starlark.Value {
	switch flagType {
	case "bool":
		val, err := cobraCmd.Flags().GetBool(flagName)
		if err != nil {
			val = false
		}
		return starlark.Bool(val)
	case "int":
		val, err := cobraCmd.Flags().GetInt(flagName)
		if err != nil {
			val = 0
		}
		return starlark.MakeInt(val)
	case "float":
		val, err := cobraCmd.Flags().GetFloat64(flagName)
		if err != nil {
			val = 0.0
		}
		return starlark.Float(val)
	default: // string
		val, err := cobraCmd.Flags().GetString(flagName)
		if err != nil {
			val = ""
		}
		return starlark.String(val)
	}
}

// resolveParamDef looks up the param definition for a flag from a tool command.
func resolveParamDef(cmd *starlarkpkg.Command, flagName string) *starlarkpkg.Param {
	if cmd.Tool == nil {
		return nil
	}
	if p, ok := cmd.Tool.Params[flagName]; ok {
		return p
	}
	return nil
}

// applyStdinFallback reads stdin and fills value if param is from_stdin and value is empty.
// Returns an error if the param is required and still empty after stdin read.
func applyStdinFallback(cobraCmd *cobra.Command, flagName string, paramDef *starlarkpkg.Param, value starlark.Value, sr *stdinReader) (starlark.Value, error) {
	if isEmptyStarlarkValue(value) {
		sr.read()
		if sr.content != "" {
			value = starlarkpkg.SanitizeParamValue(paramDef, starlark.String(sr.content))
		}
	}
	if paramDef.Required && isEmptyStarlarkValue(value) {
		if usageErr := cobraCmd.Usage(); usageErr != nil {
			return nil, fmt.Errorf("failed to print usage: %w", usageErr)
		}
		return nil, fmt.Errorf("required parameter '%s' is missing. Use --%s flag or pipe content to stdin", flagName, flagName)
	}
	return value, nil
}

// resolveFlagValue determines the starlark value for a flag, applying defaults and stdin fallback.
func resolveFlagValue(
	cobraCmd *cobra.Command,
	flagName string,
	flagDef *starlarkpkg.FlagDef,
	paramDef *starlarkpkg.Param,
	sr *stdinReader,
) (starlark.Value, error) {
	flag := cobraCmd.Flags().Lookup(flagName)

	var value starlark.Value
	if flag != nil && !flag.Changed && flagDef.Default != nil {
		value = convertToStarlarkValue(flagDef.Default)
	} else {
		value = getFlagValue(cobraCmd, flagName, flagDef.Type)
	}

	value = starlarkpkg.SanitizeParamValue(paramDef, value)

	if paramDef != nil && paramDef.FromStdin {
		return applyStdinFallback(cobraCmd, flagName, paramDef, value, sr)
	}

	return value, nil
}

// buildParamMembers collects all flag and arg values into maps for ctx injection.
func buildParamMembers(
	runtime *starlarkpkg.Runtime,
	registry *starlarkpkg.Registry,
	cmd *starlarkpkg.Command,
	cobraCmd *cobra.Command,
	args []string,
	sr *stdinReader,
) (flagsMembers starlark.StringDict, argsMembers starlark.StringDict, paramsMembers map[string]starlark.Value, err error) {
	flagsMembers = starlark.StringDict{}
	argsMembers = starlark.StringDict{}
	paramsMembers = make(map[string]starlark.Value)

	for flagName, flagDef := range cmd.Flags {
		paramDef := resolveParamDef(cmd, flagName)
		value, resolveErr := resolveFlagValue(cobraCmd, flagName, flagDef, paramDef, sr)
		if resolveErr != nil {
			return nil, nil, nil, resolveErr
		}
		flagsMembers[flagName] = value
		paramsMembers[flagName] = value
	}

	for argName, argDef := range cmd.Args {
		var value starlark.Value
		switch {
		case argDef.Index < len(args):
			value = starlark.String(args[argDef.Index])
		case argDef.Default != nil:
			value = convertToStarlarkValue(argDef.Default)
		default:
			value = starlark.None
		}
		argsMembers[argName] = value
		paramsMembers[argName] = value
	}

	if cmd.Tool != nil {
		if validateErr := starlarkpkg.ValidateToolParams(runtime, registry, cmd.Tool, paramsMembers); validateErr != nil {
			return nil, nil, nil, fmt.Errorf("tool parameter validation failed: %w", validateErr)
		}
	}

	return flagsMembers, argsMembers, paramsMembers, nil
}

// buildContextMembers creates the full ctx struct members map.
func buildContextMembers(
	runtime *starlarkpkg.Runtime,
	registry *starlarkpkg.Registry,
	container *app.Container,
	rootSession *session.Session,
	flagsMembers starlark.StringDict,
	argsMembers starlark.StringDict,
) starlark.StringDict {
	return starlark.StringDict{
		"flags": starlarkstruct.FromStringDict(starlarkstruct.Default, flagsMembers),
		"args":  starlarkstruct.FromStringDict(starlarkstruct.Default, argsMembers),
		// Bazel-style: expose modules on ctx for structured access
		"fs":        runtime.CreateFSModuleForCtx(),
		"git":       runtime.CreateGitModuleForCtx(),
		"llm":       runtime.CreateLLMModuleForCtx(rootSession),
		"shell":     runtime.CreateShellModuleForCtx(),
		"index":     runtime.CreateIndexModuleForCtx(),
		"output":    runtime.CreateOutputModuleForCtx(),
		"session":   runtime.CreateSessionModuleForCtx(rootSession),
		"json":      starlarkpkg.NewJSONModule(),
		"yaml":      starlarkpkg.NewYAMLModule(),
		"xml":       starlarkpkg.NewXMLModule(),
		"toml":      starlarkpkg.NewTOMLModule(),
		"csv":       starlarkpkg.NewCSVModule(),
		"env":       starlarkpkg.NewEnvModule(),
		"ui":        starlarkpkg.NewUIModuleWithUIWriter(0, container.OutputService),
		"path":      starlarkpkg.NewPathModule(),
		"crypto":    starlarkpkg.NewCryptoModule(),
		"time":      starlarkpkg.NewTimeModule(),
		"regexp":    starlarkpkg.NewRegexpModule(),
		"http":      starlarkpkg.NewHTTPModule(),
		"template":  starlarkpkg.NewTemplateModule(runtime.WorkingDir()),
		"stdin":     runtime.CreateStdinModuleForCtx(),
		"workspace": starlark.String(runtime.WorkingDir()),
		"run":       starlarkpkg.CreateRunFunction(registry, runtime, rootSession, 0),
	}
}

// logSessionWarning writes a session lifecycle warning to stderr, only when err is non-nil.
func logSessionWarning(msg string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: %v\n", msg, err)
	}
}

// checkContextCancelled returns an error if the context is done.
func checkContextCancelled(cobraCmd *cobra.Command) error {
	ctx := cobraCmd.Context()
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
		return nil
	}
}

// createSessionCleanup creates a root session and returns a cleanup function that finalizes it.
// If sessionService is nil, returns nil session and a no-op cleanup.
func createSessionCleanup(ctx context.Context, sessionService ports.SessionService, cmdName string) (*session.Session, func(error), error) {
	if sessionService == nil {
		return nil, func(error) {}, nil
	}
	rootSession, err := sessionService.CreateSession(ctx, nil, cmdName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create root session: %w", err)
	}
	svc, sid := sessionService, rootSession.ID
	cleanup := func(handlerErr error) {
		if handlerErr != nil {
			logSessionWarning("failed to mark session as failed", svc.FailSession(ctx, sid))
		} else {
			logSessionWarning("failed to mark session as complete", svc.CompleteSession(ctx, sid))
		}
	}
	return rootSession, cleanup, nil
}

// executeStarlarkHandler executes a Starlark command handler.
func executeStarlarkHandler(runtime *starlarkpkg.Runtime, registry *starlarkpkg.Registry, cmd *starlarkpkg.Command, cobraCmd *cobra.Command, args []string) (err error) {
	ctx := cobraCmd.Context()
	if ctx == nil {
		return fmt.Errorf("command context is nil")
	}

	container, ok := ctx.Value(app.AppContainerKey).(*app.Container)
	if !ok || container == nil {
		return fmt.Errorf("container not found in context")
	}

	if setupErr := setupRuntimeServices(runtime, container); setupErr != nil {
		return setupErr
	}

	sessionService := setupSessionService(runtime, container)
	rootSession, cleanup, sessionErr := createSessionCleanup(ctx, sessionService, cmd.Name)
	if sessionErr != nil {
		return sessionErr
	}
	defer func() { cleanup(err) }()

	sr := newStdinReader()
	defer func() {
		if sr.restore != nil {
			sr.restore()
		}
	}()

	flagsMembers, argsMembers, paramsMembers, buildErr := buildParamMembers(runtime, registry, cmd, cobraCmd, args, sr)
	if buildErr != nil {
		return buildErr
	}

	ctxMembers := buildContextMembers(runtime, registry, container, rootSession, flagsMembers, argsMembers)
	ctxWithParams := starlarkpkg.CreateContextWithParams(
		starlarkstruct.FromStringDict(starlarkstruct.Default, ctxMembers),
		paramsMembers,
	)

	if cancelErr := checkContextCancelled(cobraCmd); cancelErr != nil {
		return cancelErr
	}

	thread := &starlark.Thread{Name: "meow-handler"}
	thread.Print = func(_ *starlark.Thread, _ string) {
		panic("print() is disabled. Use ctx.output.writeline() instead")
	}

	_, err = starlark.Call(thread, cmd.Handler, starlark.Tuple{ctxWithParams}, nil)
	if err != nil {
		return fmt.Errorf("handler failed: %w", err)
	}

	if flushErr := container.OutputService.Flush(); flushErr != nil {
		return fmt.Errorf("failed to flush output: %w", flushErr)
	}

	return nil
}

// convertToStarlarkValue converts a Go value to a Starlark value.
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

// Safe coercion helpers to avoid panics on mismatched default types.
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
