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

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/llm/gateway"
	"github.com/retran/meowg1k/internal/ui"
	"github.com/spf13/cobra"
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

	generateCmd.Flags().StringP("task", "t", "", "Run a predefined task from config")
	generateCmd.Flags().StringP("user-prompt", "p", "", "User prompt for generation. Can be combined with stdin")
}

const (
	stdinContextWrapper = "\n\n```\n%s\n```"
)

// GenerationParams holds all the resolved parameters for a generation request.
type GenerationParams struct {
	Profile      *config.ResolvedProfile
	SystemPrompt string
	UserPrompt   string
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

// resolveParams consolidates all configuration resolution into a single struct.
func resolveParams(cmd *cobra.Command, cfg *config.Config) (*GenerationParams, error) {
	var task *config.GenerateTask
	var profileName string
	var systemPrompt string
	var userPrompt string

	// Check if a task is specified
	taskName, _ := cmd.Flags().GetString("task")
	if taskName != "" {
		var err error
		task, err = cfg.GetGenerateTask(taskName)
		if err != nil {
			return nil, err
		}

		profileName = task.Profile
		systemPrompt = task.SystemPrompt
		userPrompt = task.UserPrompt
	}

	// If no profile specified from task, use default
	if profileName == "" {
		profileName = cfg.GetDefaultGenerateProfile()
	}

	// If no system prompt from task, use default
	if systemPrompt == "" {
		systemPrompt = cfg.GetDefaultGenerateSystemPrompt()
	}

	// Resolve user prompt from flag or task
	cmdUserPrompt, _ := cmd.Flags().GetString("user-prompt")
	if cmdUserPrompt != "" {
		userPrompt = cmdUserPrompt
	}

	// Add stdin content if available
	stdinContent, err := readUserPromptFromStdin()
	if err != nil {
		return nil, err
	}

	if stdinContent != "" {
		if userPrompt != "" {
			userPrompt = userPrompt + fmt.Sprintf(stdinContextWrapper, stdinContent)
		} else {
			userPrompt = stdinContent
		}
	}

	if userPrompt == "" {
		return nil, fmt.Errorf("no user prompt provided via the --user-prompt flag, stdin, or task configuration")
	}

	// Resolve the profile
	profile, err := cfg.ResolveProfile(profileName)
	if err != nil {
		return nil, err
	}

	return &GenerationParams{
		Profile:      profile,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}, nil
}

// runGenerate executes the main logic of the generate command.
func runGenerate(cmd *cobra.Command) error {
	params, err := resolveParams(cmd, appConfig)
	if err != nil {
		return err
	}

	// Create gateway options based on resolved profile
	opts := []gateway.Option{
		gateway.WithProvider(params.Profile.Provider),
	}

	// Add baseURL for providers that need it
	if params.Profile.BaseURL != "" {
		opts = append(opts, gateway.WithBaseURL(params.Profile.BaseURL))
	}

	// Add API key for providers that need it
	if params.Profile.APIKey != "" {
		opts = append(opts, gateway.WithAPIKey(params.Profile.APIKey))
	}

	gw, err := gateway.NewGenerationGateway(cmd.Context(), opts...)
	if err != nil {
		return fmt.Errorf("failed to initialize gateway: %w", err)
	}

	request := gateway.NewGenerateContentRequest(params.Profile.Model, params.SystemPrompt, params.UserPrompt)

	ctx, cancel := context.WithTimeout(cmd.Context(), params.Profile.Timeout)
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
