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

	"github.com/spf13/cobra"
	"google.golang.org/genai"
)

const (
	defaultModel   = "gemini-2.5-flash"
	apiKeyEnvVar   = "MEOW_GEMINI_API_KEY"
	defaultTimeout = 5 * time.Minute
)

var (
	model       string
	userPrompt  string
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
			err := run(model, userPrompt, defaultTimeout)
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

	generateCmd.Flags().StringVarP(&model, "model", "m", defaultModel, "The model to use for generation")
	generateCmd.Flags().StringVarP(&userPrompt, "userPrompt", "p", "", "The user prompt to generate content for")

	// TODO config via viper
}

func generateContent(ctx context.Context, client *genai.Client, model, userPrompt string) (string, error) {
	result, err := client.Models.GenerateContent(ctx, model, genai.Text(userPrompt), nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch response from Gemini API: %w", err)
	}

	return result.Text(), nil
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

func run(model, userPrompt string, timeout time.Duration) error {
	apiKey := os.Getenv(apiKeyEnvVar)
	if apiKey == "" {
		return fmt.Errorf("the MEOW_GEMINI_API_KEY environment variable is not set")
	}

	var stdinInput string

	stat, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat stdin: %w", err)
	}

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		stdinInput = strings.TrimSpace(string(input))
	}

	var finalPrompt string
	if userPrompt != "" && stdinInput != "" {
		finalPrompt = strings.TrimSpace(userPrompt) + "\n\n```\n" + stdinInput + "\n```"
	} else if userPrompt != "" {
		finalPrompt = strings.TrimSpace(userPrompt)
	} else if stdinInput != "" {
		finalPrompt = stdinInput
	}

	if finalPrompt == "" {
		return fmt.Errorf("no prompt provided via --userPrompt flag or stdin")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("failed to create Gemini API client: %w", err)
	}

	// TODO spinner
	content, err := generateContent(ctx, client, model, finalPrompt)
	if err != nil {
		return err
	}

	_, err = io.WriteString(os.Stdout, formatOutput(content))
	if err != nil {
		return fmt.Errorf("failed to write response to stdout: %w", err)
	}

	return nil
}
