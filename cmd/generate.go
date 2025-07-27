/*
Copyright © 2025 Andrew Vasilyev (me@retran.me)

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
		Use:   "generate",
		Short: "Generate content using Google Gemini AI",
		Long: `Generate content using Google Gemini AI models.

This command accepts input via the --userPrompt flag and/or from stdin.
If both are provided, stdin content is wrapped in markdown code blocks.
The generated content is written to stdout.

Examples:
  # Generate content with a direct prompt
  meow generate --userPrompt "Write a haiku about cats"
  
  # Generate content using a different model
  meow generate --model gemini-pro --userPrompt "Explain quantum computing"
  
  # Generate content from stdin only
  echo "Summarize this text" | meow generate
  
  # Combine prompt and stdin input (stdin will be wrapped in code blocks)
  cat document.txt | meow generate --userPrompt "Please summarize the following text:"
  
  # Another combination example
  echo "console.log('hello')" | meow generate --userPrompt "Explain this JavaScript code:"

Environment Variables:
  MEOW_GEMINI_API_KEY    Required. Your Google Gemini API key.`,
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

	generateCmd.Flags().StringVarP(&commandLineArgs.task, "task", "t", "", "The generation task to execute")
	generateCmd.Flags().StringVarP(&commandLineArgs.model, "model", "m", "", "The model to use for generation")
	generateCmd.Flags().StringVarP(&commandLineArgs.systemPrompt, "systemPrompt", "s", "", "The system prompt to provide context for the generation")
	generateCmd.Flags().StringVarP(&commandLineArgs.userPrompt, "userPrompt", "p", "", "The user prompt to generate content for")

	// TODO config via viper
}

func getSystemLineEnding() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}
	return "\n"
}

func formatOutput(content string) string {
	return strings.TrimSpace(content) + getSystemLineEnding()
}

func getApiKey() (string, error) {
	apiKey := os.Getenv(apiKeyEnvVar)
	if apiKey == "" {
		return "", fmt.Errorf("the %s environment variable is not set", apiKeyEnvVar)
	}
	return apiKey, nil
}

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

func readTask(taskName string) (*Task, error) {
	key := "generate.tasks." + taskName

	task := viper.Get(key)
	if task == nil {
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

func buildModel(task *Task) (string, error) {
	model := defaultModel

	modelFromConfig := strings.TrimSpace(viper.GetString("generate.defaultModel"))
	if modelFromConfig != "" {
		model = modelFromConfig
	}

	if task != nil && task.model != "" {
		model = task.model
	}

	modelFromArgs := strings.TrimSpace(commandLineArgs.model)

	if modelFromArgs != "" {
		model = modelFromArgs
	}

	return model, nil
}

func buildSystemPrompt(task *Task) (string, error) {
	systemPrompt := ""

	systemPromptFromConfig := strings.TrimSpace(viper.GetString("generate.defaultSystemPrompt"))
	if systemPromptFromConfig != "" {
		systemPrompt = systemPromptFromConfig
	}

	if task != nil && task.systemPrompt != "" {
		systemPrompt = task.systemPrompt
	}

	systemPromptFromArgs := strings.TrimSpace(commandLineArgs.systemPrompt)
	if systemPromptFromArgs != "" {
		systemPrompt = systemPromptFromArgs
	}

	return systemPrompt, nil
}

func buildUserPrompt(task *Task) (string, error) {
	mainUserPrompt := ""

	if task != nil && task.userPrompt != "" {
		mainUserPrompt = task.userPrompt
	}

	mainUserPromptFromArgs := strings.TrimSpace(commandLineArgs.userPrompt)
	if mainUserPromptFromArgs != "" {
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
		return "", fmt.Errorf("no user prompt provided via --userPrompt flag, stdin or config file")
	}

	return userPrompt, nil
}

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

	// TODO spinner
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
