/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUTHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
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

const (
	defaultModel    = "gemini-1.5-flash"
	apiKeyEnvVar    = "MEOW_GEMINI_API_KEY"
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

var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen", "g"},
	Short:   "Generate, refactor, or explain code with an AI prompt.",
	Long: `The 'generate' command provides a direct interface to AI models.
You can provide input via the '-p' flag, by piping from stdin, or both.

$ meowg1k g -p "Explain this code" < main.go

---
## Providers
- **gemini** (default): Uses Google Gemini. Requires the **MEOW_GEMINI_API_KEY**
  environment variable. Use the **--model** flag to choose a model like
  "gemini-1.5-pro".

- **llama**: Connects to a local llama.cpp server. Requires the
  **--llama-base-url** flag. The **--model** flag is ignored, as the model
  is determined when you start the server. For server setup, see
  https://github.com/ggml-org/llama.cpp.

---
## Configuration
You can create reusable tasks and set defaults in a config file. Settings
from a project-specific config will override global settings.

  - Project Config: **./.meowg1k/config.yaml**
  - User Config:    **~/.config/meowg1k/config.yaml**

Command-line flags always take the highest priority.

Example configuration:
  generate:
    defaultProvider: "gemini"
    defaultModel: "gemini-1.5-flash"
    defaultTimeout: "5m"
    tasks:
      pytest-gemini:
        provider: "gemini"
        model: "gemini-1.5-pro"
        systemPrompt: "You are an expert Python TDD developer."
        userPrompt: "Write a complete pytest test file for the following code."
      refactor-llama:
        provider: "llama"
        llamaBaseURL: "http://127.0.0.1:8080"
        systemPrompt: "You are an expert in writing clean, performant code."
        userPrompt: "Refactor the following code."`,
	Example: `  # Get an explanation from the Gemini provider
  cat main.go | meowg1k g -p "Explain this Go code" --model gemini-1.5-pro

  # Use a local model via the llama.cpp server
  # (In another terminal, run: ./server -m ./models/your-model.gguf)
  cat main.go | meowg1k g -p "Refactor this Go code" -P llama -u http://127.0.0.1:8080

  # Use a pre-configured 'pytest-gemini' task from your config file
  cat my_script.py | meowg1k g -t pytest-gemini

  # Use a pre-configured 'refactor-llama' task from your config file
  cat component.js | meowg1k g -t refactor-llama`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringP(flagTask, "t", "", "The generation task to execute from the config file")
	generateCmd.Flags().StringP(flagModel, "m", "", "The model to use for generation (e.g., gemini-1.5-pro)")
	generateCmd.Flags().StringP(flagSystemPrompt, "s", "", "Set a system-level instruction for the AI (e.g., 'You are a senior Go developer')")
	generateCmd.Flags().StringP(flagUserPrompt, "p", "", "The user prompt for which to generate content")
	generateCmd.Flags().StringP(flagProvider, "P", "", "The LLM provider to use (gemini or llama)")
	generateCmd.Flags().StringP(flagLlamaBaseURL, "u", "", "LLaMA base URL (required when using --provider llama)")
}

// finalizeOutput formats the generated content by trimming whitespace and ensuring
// it ends with a newline.
func finalizeOutput(content string) string {
	return strings.TrimSpace(content) + "\n"
}

// getAPIKey retrieves the Gemini API key from the environment.
func getAPIKey() (string, error) {
	apiKey := os.Getenv(apiKeyEnvVar)
	if apiKey == "" {
		return "", fmt.Errorf("the %s environment variable is not set. Please set it to your Gemini API key", apiKeyEnvVar)
	}
	return apiKey, nil
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

// createGateway creates the appropriate gateway based on provider configuration.
func createGateway(ctx context.Context, provider gateway.Provider, llamaBaseURL string) (gateway.GenerationGateway, error) {
	switch provider {
	case gateway.Gemini:
		apiKey, err := getAPIKey()
		if err != nil {
			return nil, err
		}
		return gateway.NewGeminiGenerationGateway(ctx, apiKey)
	case gateway.Llama:
		return gateway.NewLlamaGenerationGateway(llamaBaseURL)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
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
	case gateway.Gemini, gateway.Llama:
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

// run executes the main logic of the generate command.
func run(cmd *cobra.Command) error {
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

	gw, err := createGateway(cmd.Context(), params.Provider, params.LlamaBaseURL)
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
