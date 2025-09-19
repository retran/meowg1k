/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package cmd contains the command-line interface for meowg1k.
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/retran/meowg1k/internal/llm/gateway"
	"github.com/retran/meowg1k/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen", "g"},
	Short:   "Generate any content based on input — code, text, or docs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenerate(cmd)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringP(flagTask, "t", "", "Run a predefined task from config (./.meowg1k/config.yaml or ~/.config/meowg1k/config.yaml).")
	generateCmd.Flags().StringP(flagModel, "m", "", "Model name (e.g., gemini-2.5-pro or gemini-2.5-flash). Ignored when --provider=llama.")
	generateCmd.Flags().StringP(flagSystemPrompt, "s", "", "System prompt: high-level instruction for the AI (e.g., 'You are a senior Go engineer').")
	generateCmd.Flags().StringP(flagUserPrompt, "p", "", "User prompt for generation. Can be combined with stdin.")
	generateCmd.Flags().StringP(flagProvider, "P", "", "LLM provider: 'gemini' (cloud) or 'llama' (local llama.cpp).")
	generateCmd.Flags().StringP(flagLlamaBaseURL, "u", "", "Base URL for llama.cpp (required when --provider=llama).")
}

const (
	defaultModel    = "gemini-2.5-flash"
	defaultTimeout  = 5 * time.Minute
	defaultProvider = "gemini"

	keyGenerateTasks               = "generate.tasks"
	keyGenerateDefaultModel        = "generate.defaultModel"
	keyGenerateDefaultSystemPrompt = "generate.defaultSystemPrompt"
	keyGenerateDefaultProvider     = "generate.defaultProvider"
	keyGenerateLlamaBaseURL        = "generate.defaultLlamaBaseURL"
	keyGenerateDefaultTimeout      = "generate.defaultTimeout"

	flagTask         = "task"
	flagModel        = "model"
	flagSystemPrompt = "system-prompt"
	flagUserPrompt   = "user-prompt"
	flagProvider     = "provider"
	flagLlamaBaseURL = "llama-base-url"

	stdinContextWrapper = "\n\n```\n%s\n```"
)

// Task holds configuration for a predefined generation task.
type Task struct {
	name         string
	model        string
	userPrompt   string
	systemPrompt string
	provider     string
	llamaBaseURL string
}

// generationParams holds all the resolved parameters for a generation request.
type generationParams struct {
	Provider     gateway.Provider
	Model        string
	SystemPrompt string
	UserPrompt   string
	LlamaBaseURL string
}

// finalizeOutput formats the generated content by trimming whitespace and ensuring
// it ends with a newline.
func finalizeOutput(content string) string {
	return strings.TrimSpace(content) + "\n"
}

// readUserPromptFromStdin reads from stdin if data is being piped to the command.
func readUserPromptFromStdin() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat stdin: %w", err)
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read from stdin: %w", err)
		}
		return strings.TrimSpace(string(input)), nil
	}
	return "", nil
}

// readTask retrieves a predefined task configuration from Viper.
func readTask(taskName string) (*Task, error) {
	key := fmt.Sprintf("%s.%s", keyGenerateTasks, taskName)

	if !viper.IsSet(key) {
		return nil, fmt.Errorf("task '%s' not found in configuration", taskName)
	}

	return &Task{
		name:         taskName,
		model:        viper.GetString(key + ".model"),
		systemPrompt: viper.GetString(key + ".systemPrompt"),
		userPrompt:   viper.GetString(key + ".userPrompt"),
		provider:     viper.GetString(key + ".provider"),
		llamaBaseURL: viper.GetString(key + ".llamaBaseURL"),
	}, nil
}

// getValueFromFlag returns the flag's value if it was set by the user, otherwise returns an empty string.
func getValueFromFlag(cmd *cobra.Command, flagName string) (string, error) {
	if !cmd.Flags().Changed(flagName) {
		return "", nil
	}
	val, err := cmd.Flags().GetString(flagName)
	if err != nil {
		return "", fmt.Errorf("could not read flag %q: %w", flagName, err)
	}
	return val, nil
}

// resolveString provides a generic way to resolve configuration values.
// It checks sources in order: command-line flag, task-specific config,
// global config (via viperKey), and finally a hardcoded default value.
func resolveString(cmd *cobra.Command, flagName, taskVal, viperKey, defaultVal string) (string, error) {
	flagVal, err := getValueFromFlag(cmd, flagName)
	if err != nil {
		return "", err
	}

	sources := []string{
		flagVal,
		taskVal,
		viper.GetString(viperKey),
		defaultVal,
	}

	for _, v := range sources {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			return trimmed, nil
		}
	}
	return "", nil
}

// resolveUserPrompt constructs the final user prompt from the --user-prompt flag,
// a task configuration, and stdin.
func resolveUserPrompt(cmd *cobra.Command, task *Task) (string, error) {
	var sb strings.Builder

	mainUserPrompt := ""
	if task != nil {
		mainUserPrompt = task.userPrompt
	}
	if cmd.Flags().Changed(flagUserPrompt) {
		cmdUserPrompt, err := cmd.Flags().GetString(flagUserPrompt)
		if err != nil {
			return "", err
		}
		mainUserPrompt = cmdUserPrompt
	}

	if mainUserPrompt != "" {
		sb.WriteString(mainUserPrompt)
	}

	userPromptFromStdin, err := readUserPromptFromStdin()
	if err != nil {
		return "", err
	}

	if userPromptFromStdin != "" {
		if sb.Len() > 0 {
			sb.WriteString(fmt.Sprintf(stdinContextWrapper, userPromptFromStdin))
		} else {
			sb.WriteString(userPromptFromStdin)
		}
	}

	if sb.Len() == 0 {
		return "", fmt.Errorf("no user prompt provided via the --user-prompt flag, stdin, or a task configuration")
	}

	return sb.String(), nil
}

// resolveParams consolidates all configuration resolution into a single struct.
// It determines the final parameters for the generation request by checking flags,
// task configurations, global settings, and defaults.
func resolveParams(cmd *cobra.Command) (*generationParams, error) {
	var task *Task
	if cmd.Flags().Changed(flagTask) {
		taskName, err := cmd.Flags().GetString(flagTask)
		if err != nil {
			return nil, err
		}
		task, err = readTask(taskName)
		if err != nil {
			return nil, err
		}
	}

	var taskProvider, taskModel, taskSystemPrompt, taskLlamaBaseURL string
	if task != nil {
		taskProvider = task.provider
		taskModel = task.model
		taskSystemPrompt = task.systemPrompt
		taskLlamaBaseURL = task.llamaBaseURL
	}

	providerStr, err := resolveString(cmd, flagProvider, taskProvider, keyGenerateDefaultProvider, defaultProvider)
	if err != nil {
		return nil, err
	}

	model, err := resolveString(cmd, flagModel, taskModel, keyGenerateDefaultModel, defaultModel)
	if err != nil {
		return nil, err
	}

	systemPrompt, err := resolveString(cmd, flagSystemPrompt, taskSystemPrompt, keyGenerateDefaultSystemPrompt, "")
	if err != nil {
		return nil, err
	}

	llamaBaseURL, err := resolveString(cmd, flagLlamaBaseURL, taskLlamaBaseURL, keyGenerateLlamaBaseURL, "")
	if err != nil {
		return nil, err
	}

	userPrompt, err := resolveUserPrompt(cmd, task)
	if err != nil {
		return nil, err
	}

	provider := gateway.Provider(providerStr)
	switch provider {
	case gateway.Gemini, gateway.Llama, gateway.Nebius:
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerStr)
	}

	if provider == gateway.Llama && llamaBaseURL == "" {
		return nil, fmt.Errorf("llama provider requires a base URL, set it with the -u flag or the 'generate.defaultLlamaBaseURL' key in your config")
	}

	return &generationParams{
		Provider:     provider,
		Model:        model,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		LlamaBaseURL: llamaBaseURL,
	}, nil
}

// runGenerate executes the main logic of the generate command.
func runGenerate(cmd *cobra.Command) error {
	timeout := defaultTimeout
	if timeoutStr := viper.GetString(keyGenerateDefaultTimeout); timeoutStr != "" {
		parsedTimeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid timeout value '%s' in config: %v. Using default %s\n",
				timeoutStr, err, defaultTimeout)
		} else {
			timeout = parsedTimeout
		}
	}

	params, err := resolveParams(cmd)
	if err != nil {
		return err
	}

	gw, err := gateway.NewGenerationGateway(
		cmd.Context(),
		gateway.WithProvider(params.Provider),
		gateway.WithBaseURL(params.LlamaBaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize gateway: %w", err)
	}

	request := gateway.NewGenerateContentRequest(params.Model, params.SystemPrompt, params.UserPrompt)

	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()

	content, err := ui.RunWithSpinnerWithMessage(func() (string, error) {
		return gw.GenerateContent(ctx, request)
	}, "Generating content...")
	if err != nil {
		return err
	}

	_, err = io.WriteString(os.Stdout, finalizeOutput(content))
	if err != nil {
		return fmt.Errorf("failed to write response to stdout: %w", err)
	}

	return nil
}
