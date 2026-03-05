// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/retran/meowg1k/internal/domain/session"
)

// CreateRunFunction creates a ctx.run() function that allows calling other commands.
// parentSession is the session of the current context (can be nil if no session).
func CreateRunFunction(registry *Registry, runtime *Runtime, parentSession *session.Session, depth int) *starlark.Builtin {
	return starlark.NewBuiltin("run", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("run() requires command name or tool object as first argument")
		}

		var commandName string
		var cmd *Command
		var exists bool

		if toolValue, ok := args[0].(*ToolValue); ok {
			commandName = toolValue.Tool.Name
			cmd, exists = registry.Get(commandName)
			if !exists {
				return nil, fmt.Errorf("tool '%s' not registered as command", commandName)
			}
		} else if name, ok := starlark.AsString(args[0]); ok {
			commandName = name
			cmd, exists = registry.Get(commandName)
			if !exists {
				return nil, fmt.Errorf("command '%s' not found", commandName)
			}
		} else {
			return nil, fmt.Errorf("run() first argument must be a command name (string) or tool object")
		}

		// Create child session for this tool invocation (if parent session exists)
		var childSession *session.Session
		var handlerErr error
		sessionService := runtime.sessionService
		if sessionService != nil && parentSession != nil {
			var err error
			childSession, err = sessionService.CreateSession(context.Background(), &parentSession.ID, commandName)
			if err != nil {
				return nil, fmt.Errorf("failed to create child session: %w", err)
			}
			// Mark session as completed or failed after execution
			defer func() {
				if handlerErr != nil {
					_ = sessionService.FailSession(context.Background(), childSession.ID)
				} else {
					_ = sessionService.CompleteSession(context.Background(), childSession.ID)
				}
			}()
		}

		flagsMembers := starlark.StringDict{}
		argsMembers := starlark.StringDict{}
		paramsMembers := make(map[string]starlark.Value)

		for flagName, flagDef := range cmd.Flags {
			var value starlark.Value
			if flagDef.Default != nil {
				value = convertGoValueToStarlark(flagDef.Default)
			} else {
				switch flagDef.Type {
				case "bool":
					value = starlark.False
				case "int":
					value = starlark.MakeInt(0)
				case "float":
					value = starlark.Float(0.0)
				default:
					value = starlark.String("")
				}
			}

			if cmd.Tool != nil {
				if param, ok := cmd.Tool.Params[flagName]; ok {
					value = SanitizeParamValue(param, value)
				}
			}

			flagsMembers[flagName] = value
			paramsMembers[flagName] = value
		}

		for _, kv := range kwargs {
			if len(kv) != 2 {
				continue
			}
			key, ok := starlark.AsString(kv[0])
			if !ok {
				continue
			}

			value := kv[1]
			if cmd.Tool != nil {
				if param, ok := cmd.Tool.Params[key]; ok {
					value = SanitizeParamValue(param, value)
				}
			}

			flagsMembers[key] = value
			paramsMembers[key] = value
		}

		for i := 1; i < len(args); i++ {
			for argName, argDef := range cmd.Args {
				if argDef.Index == i-1 {
					value := args[i]
					if cmd.Tool != nil {
						if param, ok := cmd.Tool.Params[argName]; ok {
							value = SanitizeParamValue(param, value)
						}
					}
					argsMembers[argName] = value
					paramsMembers[argName] = value
				}
			}
		}

		for argName, argDef := range cmd.Args {
			if _, exists := argsMembers[argName]; !exists && argDef.Default != nil {
				value := convertGoValueToStarlark(argDef.Default)
				if cmd.Tool != nil {
					if param, ok := cmd.Tool.Params[argName]; ok {
						value = SanitizeParamValue(param, value)
					}
				}
				argsMembers[argName] = value
				paramsMembers[argName] = value
			}
		}

		if cmd.Tool != nil {
			if err := ValidateToolParams(runtime, registry, cmd.Tool, paramsMembers); err != nil {
				return nil, fmt.Errorf("run(): %w", err)
			}
		}

		flagsStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, flagsMembers)
		argsStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, argsMembers)

		childCtxMembers := starlark.StringDict{
			"flags":     flagsStruct,
			"args":      argsStruct,
			"fs":        runtime.CreateFSModuleForCtx(),
			"git":       runtime.CreateGitModuleForCtx(),
			"llm":       runtime.CreateLLMModuleForCtx(childSession),
			"shell":     runtime.CreateShellModuleForCtx(),
			"index":     runtime.CreateIndexModuleForCtx(),
			"output":    runtime.CreateOutputModuleForCtx(),              // Shared output buffer
			"session":   runtime.CreateSessionModuleForCtx(childSession), // Pass child session
			"json":      NewJSONModule(),
			"yaml":      NewYAMLModule(),
			"xml":       NewXMLModule(),
			"toml":      NewTOMLModule(),
			"csv":       NewCSVModule(),
			"env":       NewEnvModule(),
			"ui":        runtime.CreateUIModuleForCtx(depth + 1), // Indent child output
			"path":      NewPathModule(),
			"crypto":    NewCryptoModule(),
			"time":      NewTimeModule(),
			"regexp":    NewRegexpModule(),
			"http":      NewHTTPModule(),
			"template":  NewTemplateModule(runtime.WorkingDir()),
			"stdin":     runtime.CreateStdinModuleForCtx(),
			"workspace": starlark.String(runtime.WorkingDir()),
			"run":       CreateRunFunction(registry, runtime, childSession, depth+1), // Recursive with child as parent
		}

		childCtxStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, childCtxMembers)
		childCtx := CreateContextWithParams(childCtxStruct, paramsMembers)

		result, handlerErr := starlark.Call(thread, cmd.Handler, starlark.Tuple{childCtx}, nil)
		if handlerErr != nil {
			return nil, fmt.Errorf("command '%s' failed: %w", commandName, handlerErr)
		}

		return result, nil
	})
}

// convertGoValueToStarlark converts Go value to Starlark value
func convertGoValueToStarlark(v any) starlark.Value {
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
		return starlark.String(fmt.Sprint(v))
	}
}
