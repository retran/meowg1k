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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/activities/filterfiles"
	"github.com/retran/meowg1k/internal/activities/readstagedchanges"
	"github.com/retran/meowg1k/internal/activities/readstagedfiles"
	"github.com/retran/meowg1k/internal/app"
	"github.com/retran/meowg1k/internal/flows/commit"
	"github.com/retran/meowg1k/internal/services/filter"
	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/internal/services/workspace"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/ui"
)

var commitCmd = &cobra.Command{
	Use:     "commit",
	Aliases: []string{"c"},
	Short:   "Generate commit messaged based on staged changes in repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		appContainer, ok := ctx.Value(app.AppContainerKey).(*app.Container)
		if !ok || appContainer == nil {
			return ErrAppNotInitialized
		}

		workspaceService := workspace.NewService()
		gitService := git.NewService(workspaceService)
		filterService := filter.NewService(appContainer.ConfigService)

		readStagedFilesActivityFactory := readstagedfiles.NewFactory(gitService)
		filterFilesActivityFactory := filterfiles.NewFactory(filterService)
		readStagedChangesActivityFactory := readstagedchanges.NewFactory(gitService)

		flowFactory := commit.NewFactory(
			readStagedFilesActivityFactory,
			filterFilesActivityFactory,
			readStagedChangesActivityFactory,
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

		return exec.RunFlow(appContainer.ShutdownService.Context(), "GenerateCommit", flow, executor.DefaultRetryPolicy())
	},
}

func init() {
	rootCmd.AddCommand(commitCmd)
}
