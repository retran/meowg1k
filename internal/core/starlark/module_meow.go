// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// createMeowModule creates the meow built-in module.
func (r *Runtime) createMeowModule() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"provider": starlark.NewBuiltin("provider", r.meowProvider),
		"model":    starlark.NewBuiltin("model", r.meowModel),
		"preset":   starlark.NewBuiltin("preset", r.meowPreset),
		"presets":  starlark.NewBuiltin("presets", r.meowPresets),
		// Unified tool system (NEW API)
		"param":   CreateParamFunction(),
		"tool":    CreateToolFunction(r.registry),
		"command": CreateCommandFunction(r.registry),
	})
}

// meowProvider registers a provider configuration.
func (r *Runtime) meowProvider(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) { //nolint:gocognit,gocyclo // complexity inherent in parsing and validating provider configuration
	if args.Len() != 1 {
		return nil, fmt.Errorf("provider: expected 1 positional argument, got %d", args.Len())
	}

	name, ok := args.Index(0).(starlark.String)
	if !ok {
		return nil, fmt.Errorf("provider: name must be a string")
	}

	config := ProviderConfig{
		ExtraOpts: make(map[string]interface{}),
	}

	for _, kv := range kwargs {
		keyStr, ok := kv[0].(starlark.String)
		if !ok {
			continue
		}
		key := string(keyStr)
		switch key {
		case "type":
			if s, ok := kv[1].(starlark.String); ok {
				config.Type = string(s)
			}
		case "base_url":
			if s, ok := kv[1].(starlark.String); ok {
				config.BaseURL = string(s)
			}
		case "api_key":
			if s, ok := kv[1].(starlark.String); ok {
				config.APIKey = string(s)
			}
		case "app_id":
			if s, ok := kv[1].(starlark.String); ok {
				config.AppID = string(s)
			}
		case "editor_version":
			if s, ok := kv[1].(starlark.String); ok {
				config.EditorVersion = string(s)
			}
		case "editor_plugin_version":
			if s, ok := kv[1].(starlark.String); ok {
				config.EditorPluginVersion = string(s)
			}
		case "user_agent":
			if s, ok := kv[1].(starlark.String); ok {
				config.UserAgent = string(s)
			}
		case "copilot_integration_id":
			if s, ok := kv[1].(starlark.String); ok {
				config.CopilotIntegrationID = string(s)
			}
		case "openai_organization":
			if s, ok := kv[1].(starlark.String); ok {
				config.OpenAIOrganization = string(s)
			}
		case "tokenizer":
			if s, ok := kv[1].(starlark.String); ok {
				config.Tokenizer = string(s)
			}
		case "retry_count":
			if i, ok := kv[1].(starlark.Int); ok {
				if val, ok := i.Int64(); ok {
					config.RetryCount = int(val)
				}
			}
		default:
			config.ExtraOpts[key] = starlarkToGo(kv[1])
		}
	}

	r.RegisterProvider(string(name), config)
	return starlark.None, nil
}

// meowModel registers a model configuration.
func (r *Runtime) meowModel(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) { //nolint:gocognit,gocyclo // complexity inherent in parsing and validating model configuration
	if args.Len() != 1 {
		return nil, fmt.Errorf("model: expected 1 positional argument, got %d", args.Len())
	}

	name, ok := args.Index(0).(starlark.String)
	if !ok {
		return nil, fmt.Errorf("model: name must be a string")
	}

	config := ModelConfig{
		ExtraOpts: make(map[string]interface{}),
	}

	// Manually parse all kwargs, storing known ones and extras
	for _, kv := range kwargs {
		keyStr, ok := kv[0].(starlark.String)
		if !ok {
			continue
		}
		key := string(keyStr)
		switch key {
		case "provider":
			if s, ok := kv[1].(starlark.String); ok {
				config.Provider = string(s)
			}
		case "model":
			if s, ok := kv[1].(starlark.String); ok {
				config.Model = string(s)
			}
		case "max_input_tokens":
			if i, ok := kv[1].(starlark.Int); ok {
				if val, ok := i.Int64(); ok {
					config.MaxInputTokens = int(val)
				}
			}
		case "max_output_tokens":
			if i, ok := kv[1].(starlark.Int); ok {
				if val, ok := i.Int64(); ok {
					config.MaxOutputTokens = int(val)
				}
			}
		case "rate_limit_rpm":
			if i, ok := kv[1].(starlark.Int); ok {
				if val, ok := i.Int64(); ok {
					config.RateLimitRPM = int(val)
				}
			}
		case "rate_limit_tpm":
			if i, ok := kv[1].(starlark.Int); ok {
				if val, ok := i.Int64(); ok {
					config.RateLimitTPM = int(val)
				}
			}
		case "rate_limit_rpd":
			if i, ok := kv[1].(starlark.Int); ok {
				if val, ok := i.Int64(); ok {
					config.RateLimitRPD = int(val)
				}
			}
		default:
			config.ExtraOpts[key] = starlarkToGo(kv[1])
		}
	}

	r.RegisterModel(string(name), config)
	return starlark.None, nil
}

// meowPreset registers a preset configuration.
func (r *Runtime) meowPreset(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) { //nolint:gocognit,gocyclo // complexity inherent in parsing and validating preset configuration
	if args.Len() != 1 {
		return nil, fmt.Errorf("preset: expected 1 positional argument, got %d", args.Len())
	}

	name, ok := args.Index(0).(starlark.String)
	if !ok {
		return nil, fmt.Errorf("preset: name must be a string")
	}

	config := PresetConfig{
		ExtraOpts: make(map[string]interface{}),
	}

	for _, kv := range kwargs {
		keyStr, ok := kv[0].(starlark.String)
		if !ok {
			continue
		}
		key := string(keyStr)
		switch key {
		case "model":
			if s, ok := kv[1].(starlark.String); ok {
				config.Model = string(s)
			}
		case "extends":
			if s, ok := kv[1].(starlark.String); ok {
				config.Extends = string(s)
			}
		case "temperature":
			if f, ok := kv[1].(starlark.Float); ok {
				config.Temperature = float64(f)
			}
		case "max_tokens":
			if i, ok := kv[1].(starlark.Int); ok {
				if val, ok := i.Int64(); ok {
					config.MaxTokens = int(val)
				}
			}
		case "top_p":
			if f, ok := kv[1].(starlark.Float); ok {
				config.TopP = float64(f)
			}
		case "top_k":
			if i, ok := kv[1].(starlark.Int); ok {
				if val, ok := i.Int64(); ok {
					config.TopK = int(val)
				}
			}
		case "frequency_penalty":
			if f, ok := kv[1].(starlark.Float); ok {
				config.FrequencyPenalty = float64(f)
			}
		case "presence_penalty":
			if f, ok := kv[1].(starlark.Float); ok {
				config.PresencePenalty = float64(f)
			}
		default:
			config.ExtraOpts[key] = starlarkToGo(kv[1])
		}
	}

	r.RegisterPreset(string(name), config)
	return starlark.None, nil
}

// meowPresets returns a list of registered preset names.
func (r *Runtime) meowPresets(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	if args.Len() != 0 {
		return nil, fmt.Errorf("presets: expected 0 arguments, got %d", args.Len())
	}

	names := make([]starlark.Value, 0, len(r.presets))
	for name := range r.presets {
		names = append(names, starlark.String(name))
	}
	return starlark.NewList(names), nil
}
