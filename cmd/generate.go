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

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/retran/meowg1k/internal/llm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	defaultModel   = "gemini-2.5-flash"
	apiKeyEnvVar   = "MEOW_GEMINI_API_KEY"
	defaultTimeout = 5 * time.Minute
)

type CommandLineArgs struct {
	task         string
	model        string
	userPrompt   string
	systemPrompt string
}

type Task struct {
	name         string
	model        string
	userPrompt   string
	systemPrompt string
}

var (
	commandLineArgs CommandLineArgs

	generateCmd = &cobra.Command{
		Use:   "generate [flags]",
		Short: "Generate, refactor, or explain code with an AI prompt.",
		Long: `The 'generate' command is the core of meowg1k, providing a direct
interface to the AI model for a wide range of programming tasks.

INPUT METHODS:
You can provide input to the model in three primary ways:

  1. Prompt Flag: Use the '-p' or '--userPrompt' flag for direct questions.
     $ meow generate -p "Create a boilerplate for a Go CLI using Cobra."

  2. Piped from stdin: Pipe code or text directly into the command. This is
     ideal for asking a general question about an entire file.
     $ cat main.go | meow generate -p "Explain this code."

  3. Combined: Use a prompt flag and piped stdin together to ask a specific
     question about the provided code. The stdin content is appended to your prompt.
     $ cat main.go | meow generate -p "Refactor this code to be more idiomatic."

CONFIGURABLE TASKS:
For reusable workflows, you can define tasks in your config file
(e.g., ~/.config/meowg1k/config.yaml). A task is a named preset that can
specify its own model, system prompt, and user prompt. This is useful for
creating custom tools like a 'documenter', 'refactorer', or 'tester'.

Example 'config.yaml':
  generate:
    tasks:
      doc:
        systemPrompt: "You are a senior Go developer. Write clear and concise documentation."
        userPrompt: "Write a complete godoc comment for the following code. Do not include the code itself in the output, only the doc comment."
      refactor:
        model: "gemini-2.5-pro"
        systemPrompt: "You are an expert in code refactoring."
        userPrompt: "Refactor the following code to improve performance and readability. Explain your changes briefly."

DEFAULT CONFIGURATION:
You can override the application's built-in defaults by setting 'default'
parameters in your config file. This is useful for setting a preferred model
or a global system prompt for all your interactions.

Example 'config.yaml' with defaults:
  generate:
    defaultModel: "gemini-2.5-pro"
    defaultSystemPrompt: "You are a helpful coding assistant that provides answers in markdown format."
    tasks:
      # ... your tasks here ...

These settings are overridden by more specific ones in the following order of priority:
  1. Command-line flag (e.g., --model)
  2. Task-specific setting
  3. Default setting in config.yaml
  4. Application default`,
		Example: `  # Generate a Python function from a description
  meow generate -p "Write a python function to find prime numbers up to n"

  # Explain a code file by piping it to the command
  cat main.go | meow generate -p "Explain this Go code and point out potential bugs"

  # Refactor a React component using a prompt and piped code
  cat component.js | meow generate -p "Refactor this to use React Hooks and functional components"

  # Write documentation and redirect output to a new file
  cat api.py | meow generate -p "Write a standard PEP 257 docstring for this script" > api_with_docs.py

  # Use the pre-configured 'doc' task (from the example above) on a local file
  cat my_file.go | meow generate -t doc`,
		Run: func(cmd *cobra.Command, args []string) {
			err := run()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Aliases = []string{"gen", "g"}

	generateCmd.Flags().StringVarP(&commandLineArgs.task, "task", "t", "", "The generation task to execute from the config file")
	generateCmd.Flags().StringVarP(&commandLineArgs.model, "model", "m", "", "The model to use for generation (e.g., gemini-2.5-pro)")
	generateCmd.Flags().StringVarP(&commandLineArgs.systemPrompt, "systemPrompt", "s", "", "Set a system-level instruction for the AI (e.g., 'You are a senior Go developer')")
	generateCmd.Flags().StringVarP(&commandLineArgs.userPrompt, "userPrompt", "p", "", "The user prompt for which to generate content")
}

// getSystemLineEnding returns the appropriate line ending for the host operating system.
func getSystemLineEnding() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}
	return "\n"
}

// formatOutput trims leading/trailing whitespace and appends a system-specific line ending.
func formatOutput(content string) string {
	return strings.TrimSpace(content) + getSystemLineEnding()
}

// getApiKey retrieves the Gemini API key from the MEOW_GEMINI_API_KEY environment variable.
func getApiKey() (string, error) {
	apiKey := os.Getenv(apiKeyEnvVar)
	if apiKey == "" {
		return "", fmt.Errorf("the %s environment variable is not set", apiKeyEnvVar)
	}
	return apiKey, nil
}

// readUserPromptFromStdin reads from stdin if data is being piped to the command.
// It returns an empty string if stdin is not being used.
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
	key := "generate.tasks." + taskName

	if !viper.IsSet(key) {
		return nil, fmt.Errorf("task '%s' not found in configuration", taskName)
	}

	model := strings.TrimSpace(viper.GetString(key + ".model"))
	systemPrompt := strings.TrimSpace(viper.GetString(key + ".systemPrompt"))
	userPrompt := strings.TrimSpace(viper.GetString(key + ".userPrompt"))

	return &Task{
		name:         taskName,
		model:        model,
		systemPrompt: systemPrompt,
		userPrompt:   userPrompt,
	}, nil
}

// buildModel determines the LLM to use based on a hierarchy:
// 1. --model command-line flag
// 2. Task-specific model from config
// 3. Global default model from config
// 4. Hardcoded default model
func buildModel(task *Task) (string, error) {
	model := defaultModel

	if modelFromConfig := strings.TrimSpace(viper.GetString("generate.defaultModel")); modelFromConfig != "" {
		model = modelFromConfig
	}

	if task != nil && task.model != "" {
		model = task.model
	}

	if modelFromArgs := strings.TrimSpace(commandLineArgs.model); modelFromArgs != "" {
		model = modelFromArgs
	}

	return model, nil
}

// buildSystemPrompt determines the system prompt to use based on a hierarchy:
// 1. --systemPrompt command-line flag
// 2. Task-specific system prompt from config
// 3. Global default system prompt from config
func buildSystemPrompt(task *Task) (string, error) {
	systemPrompt := ""

	if systemPromptFromConfig := strings.TrimSpace(viper.GetString("generate.defaultSystemPrompt")); systemPromptFromConfig != "" {
		systemPrompt = systemPromptFromConfig
	}

	if task != nil && task.systemPrompt != "" {
		systemPrompt = task.systemPrompt
	}

	if systemPromptFromArgs := strings.TrimSpace(commandLineArgs.systemPrompt); systemPromptFromArgs != "" {
		systemPrompt = systemPromptFromArgs
	}

	return systemPrompt, nil
}

// buildUserPrompt constructs the final user prompt by combining inputs.
// It prioritizes the --userPrompt flag, then a task-specific prompt,
// and appends any content from stdin.
func buildUserPrompt(task *Task) (string, error) {
	mainUserPrompt := ""

	if task != nil && task.userPrompt != "" {
		mainUserPrompt = task.userPrompt
	}

	if mainUserPromptFromArgs := strings.TrimSpace(commandLineArgs.userPrompt); mainUserPromptFromArgs != "" {
		mainUserPrompt = mainUserPromptFromArgs
	}

	userPromptFromStdin, err := readUserPromptFromStdin()
	if err != nil {
		return "", err
	}

	userPrompt := mainUserPrompt
	if userPromptFromStdin != "" {
		if userPrompt == "" {
			userPrompt = userPromptFromStdin
		} else {
			userPrompt += "\n\n```\n" + userPromptFromStdin + "\n```"
		}
	}

	if userPrompt == "" {
		return "", fmt.Errorf("no user prompt provided via the --userPrompt flag, stdin, or a task configuration")
	}

	return userPrompt, nil
}

// buildGenerateContentRequest assembles the complete content generation request
// from command-line arguments, configuration files, and stdin.
func buildGenerateContentRequest() (*llm.GenerateContentRequest, error) {
	var task *Task
	if commandLineArgs.task != "" {
		taskFromConfig, err := readTask(commandLineArgs.task)
		if err != nil {
			return nil, err
		}
		task = taskFromConfig
	}

	model, err := buildModel(task)
	if err != nil {
		return nil, err
	}

	systemPrompt, err := buildSystemPrompt(task)
	if err != nil {
		return nil, err
	}

	userPrompt, err := buildUserPrompt(task)
	if err != nil {
		return nil, err
	}

	return llm.NewGenerateContentRequest(model, systemPrompt, userPrompt), nil
}

// run executes the main logic of the generate command.
func run() error {
	apiKey, err := getApiKey()
	if err != nil {
		return err
	}

	gateway, err := llm.NewGeminiGenerationGateway(context.Background(), apiKey)
	if err != nil {
		return err
	}

	request, err := buildGenerateContentRequest()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	content, err := gateway.GenerateContent(ctx, request)
	if err != nil {
		return err
	}

	_, err = io.WriteString(os.Stdout, formatOutput(content))
	if err != nil {
		return fmt.Errorf("failed to write response to stdout: %w", err)
	}

	return nil
}
