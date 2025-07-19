package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	defaultModel   = "gemini-2.5-flash"
	apiKeyEnvVar   = "MEOW_GEMINI_API_KEY"
	defaultTimeout = 5 * time.Minute
)

type CommandLineArgs struct {
	Model      string
	UserPrompt string
	Timeout    time.Duration
}

// TODO streaming
// TODO system prompt
func generateContent(ctx context.Context, client *genai.Client, model, userPrompt string) (string, error) {
	result, err := client.Models.GenerateContent(ctx, model, genai.Text(userPrompt), nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch response from Gemini API: %w", err)
	}

	return result.Text(), nil
}

func parseCommandLineArgs() *CommandLineArgs {
	args := CommandLineArgs{}

	flag.StringVar(&args.Model, "model", defaultModel, "Model to use for content generation.")
	flag.StringVar(&args.UserPrompt, "prompt", "", "The user prompt. Reads from stdin if empty.")
	flag.DurationVar(&args.Timeout, "timeout", defaultTimeout, "Timeout for content generation.")
	flag.Parse()

	return &args
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

func run() error {
	apiKey := os.Getenv(apiKeyEnvVar)
	if apiKey == "" {
		return fmt.Errorf("the MEOW_GEMINI_API_KEY environment variable is not set")
	}

	args := parseCommandLineArgs()

	if args.UserPrompt == "" {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stding: %w", err)
		}
		args.UserPrompt = string(input)
	}

	args.UserPrompt = strings.TrimSpace(args.UserPrompt)

	if args.UserPrompt == "" {
		flag.Usage()
		return fmt.Errorf("no prompt provided via -prompt flag or stdin")
	}

	// TODO read system prompt from config
	ctx, cancel := context.WithTimeout(context.Background(), args.Timeout)
	defer cancel()

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("failed to create Gemini API client: %w", err)
	}

	// TODO spinner
	content, err := generateContent(ctx, client, args.Model, args.UserPrompt)
	if err != nil {
		return err
	}

	_, err = io.WriteString(os.Stdout, formatOutput(content))
	if err != nil {
		return fmt.Errorf("failed to write response to stdout: %w", err)
	}

	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
