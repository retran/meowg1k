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
	"errors"
	"fmt"

	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/internal/flows/generate"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/profile"
	"github.com/retran/meowg1k/internal/services/prompt"
	"github.com/retran/meowg1k/internal/services/provider"
	"github.com/retran/meowg1k/internal/services/task"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	// ErrAppNotInitialized indicates the application container is not properly initialized
	ErrAppNotInitialized = errors.New("application not initialized")
)

var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen", "g"},
	Short:   "Generate any content based on input — code, text, or docs",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		appContainer, ok := ctx.Value(app.AppContainerKey).(*app.Container)
		if !ok || appContainer == nil {
			return ErrAppNotInitialized
		}

		providerService := provider.NewService()

		profileService := profile.NewService(
			appContainer.ConfigService,
			providerService,
		)

		taskService, err := task.NewService(
			appContainer.CommandService,
			appContainer.ConfigService,
			profileService,
		)
		if err != nil {
			return fmt.Errorf("failed to create task service: %w", err)
		}

		generatePromptService, err := prompt.NewGeneratePromptService(
			appContainer.CommandService,
			taskService,
		)
		if err != nil {
			return fmt.Errorf("failed to create prompt service: %w", err)
		}

		gatewayFactory := gateway.NewFactory()
		activityFactory := generate.NewActivityFactory(gatewayFactory)
		flowFactory := generate.NewFlowFactory(
			taskService,
			generatePromptService,
			generatePromptService,
			activityFactory,
		)
		flow := flowFactory.NewFlow()

		silent, err := appContainer.CommandService.GetSilentFlag()
		if err != nil {
			return fmt.Errorf("failed to get silent flag: %w", err)
		}

		executionTracker := ui.NewExecutionTracker(silent)
		executionTracker.Start()
		defer executionTracker.Stop()

		exec := executor.NewExecutor().
			WithFeedbackHandler(executionTracker.FeedbackHandler())

		return exec.RunFlow(appContainer.ShutdownService.Context(), "GenerateContent", flow)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("task", "t", "", "Run a predefined task from config")
	generateCmd.Flags().StringP("system-prompt", "s", "", "System prompt for generation")
	generateCmd.Flags().StringP("user-prompt", "u", "", "User prompt for generation. Can be combined with stdin")
}
