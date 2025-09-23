/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Vers	// Create registry service
	registryService := providers.NewService()
	commandService := command.NewService(cmd)
	configService := configservice.NewServiceWithConfig(appConfig, configPath, commandService)

	// Create task resolver service
	taskResolverService := tasks.NewService(commandService, configService)

	// Create profile resolver service
	profileResolverService := profiles.NewService(registryService, configService)e "License");
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

	"github.com/retran/meowg1k/internal/activities/generate"
	"github.com/retran/meowg1k/internal/models/config"
	"github.com/retran/meowg1k/internal/services/command"
	configservice "github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/profiles"
	"github.com/retran/meowg1k/internal/services/prompt"
	"github.com/retran/meowg1k/internal/services/providers"
	"github.com/retran/meowg1k/internal/services/tasks"
	utilsio "github.com/retran/meowg1k/internal/utils/io"
	"github.com/retran/meowg1k/pkg/activity"
	"github.com/retran/meowg1k/pkg/ui"
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
	generateCmd.Flags().Bool("silent", false, "Silent mode - only output the result without progress indicators")
}

// runGenerate executes the main logic of the generate command.
func runGenerate(cmd *cobra.Command) error {
	silent, err := cmd.Flags().GetBool("silent")
	if err != nil {
		return fmt.Errorf("failed to get silent flag: %w", err)
	}

	// Check if appConfig is initialized
	if appConfig == nil {
		return fmt.Errorf("configuration not loaded - please run with proper command initialization")
	}

	ctx := cmd.Context()

	// Create progress tracker for the new system
	tracker := ui.NewActivityTracker(silent, "Generating")
	tracker.Start()
	defer tracker.Stop()

	// Use activity-based implementation
	content, err := generateContentWithActivity(ctx, cmd, appConfig, tracker.FeedbackHandler())
	if err != nil {
		return fmt.Errorf("failed to execute generation: %w", err)
	}

	_, err = io.WriteString(os.Stdout, utilsio.FinalizeOutput(content))
	if err != nil {
		return fmt.Errorf("failed to write response to stdout: %w", err)
	}

	return nil
}

// generateContentWithActivity implements the generate logic using the new activity-based approach
func generateContentWithActivity(ctx context.Context, cmd *cobra.Command, appConfig *config.Config, feedbackHandler interface{}) (string, error) {
	// Create all required services
	registryService := providers.NewService()
	commandService := command.NewService(cmd)
	configService := configservice.NewServiceWithConfig(appConfig, configPath, commandService)

	// Create task resolver service
	taskResolverService := tasks.NewService(commandService, configService)

	// Create profile resolver service
	profileResolverService := profiles.NewService(registryService, configService)

	// Create prompt builder service
	promptBuilderService := prompt.NewBuilder(configService)

	// Create gateway factory
	gatewayFactory := gateway.NewGatewayFactory()

	// Create generate activity factory with injected services
	generateFactory := generate.NewGenerateActivityFactory(
		taskResolverService,
		profileResolverService,
		promptBuilderService,
		gatewayFactory,
	)

	// Create the activity function
	generateActivity := generateFactory.CreateActivity()

	// Read stdin content
	stdinContent, err := utilsio.ReadFromStdin()
	if err != nil {
		return "", fmt.Errorf("failed to read stdin: %w", err)
	}

	// Get command line flags
	taskName, err := cmd.Flags().GetString("task")
	if err != nil {
		return "", fmt.Errorf("failed to get task flag: %w", err)
	}

	userPrompt, err := cmd.Flags().GetString("user-prompt")
	if err != nil {
		return "", fmt.Errorf("failed to get user-prompt flag: %w", err)
	}

	silent, err := cmd.Flags().GetBool("silent")
	if err != nil {
		return "", fmt.Errorf("failed to get silent flag: %w", err)
	}

	// Create activity context
	feedbackHandlerFunc, ok := feedbackHandler.(func(activity.Feedback))
	if !ok {
		// Create a default feedback handler if conversion fails
		feedbackHandlerFunc = func(feedback activity.Feedback) {
			// Default implementation - could log or handle feedback
		}
	}

	activityCtx := activity.NewActivityContext("generate", feedbackHandlerFunc)

	// Prepare input
	input := &generate.GenerateInput{
		Command:      cmd,
		Config:       appConfig,
		StdinContent: stdinContent,
		TaskName:     taskName,
		UserPrompt:   userPrompt,
		Silent:       silent,
	}

	// Execute the activity
	output, err := generateActivity(ctx, activityCtx, input)
	if err != nil {
		return "", fmt.Errorf("generate activity failed: %w", err)
	}

	return output.Content, nil
}

// generateContentSimple is a placeholder for the simple generation logic
// TODO: Remove this function after migration is complete
func generateContentSimple(ctx context.Context, cmd *cobra.Command, appConfig interface{}, feedbackHandler interface{}) (string, error) {
	// This is a temporary placeholder
	return "Generated content placeholder", nil
}
