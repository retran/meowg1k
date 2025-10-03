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

// Package app contains the main application struct and orchestrates cross-cutting services.
package app

import (
	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/composecommit"
	"github.com/retran/meowg1k/internal/activities/fetchalldiffs"
	"github.com/retran/meowg1k/internal/activities/fetchfilediff"
	"github.com/retran/meowg1k/internal/activities/liststaged"
	"github.com/retran/meowg1k/internal/activities/summarizeall"
	"github.com/retran/meowg1k/internal/activities/summarizefile"
	"github.com/retran/meowg1k/internal/flows/commit"
	"github.com/retran/meowg1k/internal/flows/generate"
	"github.com/retran/meowg1k/internal/services/commitconfig"
	"github.com/retran/meowg1k/internal/services/filter"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/internal/services/profile"
	"github.com/retran/meowg1k/internal/services/prompt"
	"github.com/retran/meowg1k/internal/services/provider"
	"github.com/retran/meowg1k/internal/services/summarize"
	"github.com/retran/meowg1k/internal/services/task"
	"github.com/retran/meowg1k/internal/services/workspace"
	"github.com/retran/meowg1k/pkg/executor"
)

// CreateCommitFlow creates a complete commit flow with all dependencies.
func (c *Container) CreateCommitFlow() executor.Flow {
	workspaceService := workspace.NewService()
	gitService := git.NewService(workspaceService)
	filterService := filter.NewService(c.ConfigService)

	providerService := provider.NewService()
	profileService := profile.NewService(c.ConfigService, providerService)

	summarizeService := summarize.NewService(c.ConfigService, profileService)

	commitConfigService := commitconfig.NewService(c.ConfigService, profileService)

	gatewayFactory := gateway.NewFactory()

	listStagedActivityFactory := liststaged.NewFactory(gitService)
	applyFiltersActivityFactory := applyfilters.NewFactory(filterService)
	fetchFileDiffActivityFactory := fetchfilediff.NewFactory(gitService)

	fetchAllDiffsFactory := fetchalldiffs.NewFactory(fetchFileDiffActivityFactory)
	summarizeFileFactory := summarizefile.NewFactory(gatewayFactory, summarizeService)
	summarizeAllFactory := summarizeall.NewFactory(summarizeFileFactory)
	composeCommitFactory := composecommit.NewFactory(gatewayFactory)

	flowFactory := commit.NewFactory(
		listStagedActivityFactory,
		applyFiltersActivityFactory,
		fetchAllDiffsFactory,
		summarizeAllFactory,
		composeCommitFactory,
		commitConfigService,
		c.CommandService,
		c.OutputService,
	)

	return flowFactory.NewFlow()
}

// CreateGenerateFlow creates a complete generate flow with all dependencies.
func (c *Container) CreateGenerateFlow() (executor.Flow, error) {
	providerService := provider.NewService()
	profileService := profile.NewService(c.ConfigService, providerService)

	taskService, err := task.NewService(
		c.CommandService,
		c.ConfigService,
		profileService,
	)
	if err != nil {
		return nil, err
	}

	generatePromptService, err := prompt.NewGeneratePromptService(
		c.CommandService,
		taskService,
	)
	if err != nil {
		return nil, err
	}

	gatewayFactory := gateway.NewFactory()

	flowFactory := generate.NewFlowFactory(
		taskService,
		generatePromptService,
		generatePromptService,
		gatewayFactory,
		c.OutputService,
	)

	return flowFactory.NewFlow(), nil
}
