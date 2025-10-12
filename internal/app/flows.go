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

// Package app contains the main application struct and orchestrates cross-cutting adapters.
package app

import (
	"fmt"

	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/composecommit"
	"github.com/retran/meowg1k/internal/activities/composeflatcommit"
	"github.com/retran/meowg1k/internal/activities/composeflatpr"
	"github.com/retran/meowg1k/internal/activities/composepr"
	"github.com/retran/meowg1k/internal/activities/fetchallbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/fetchalldiffs"
	"github.com/retran/meowg1k/internal/activities/fetchbranchfilediff"
	"github.com/retran/meowg1k/internal/activities/fetchfilediff"
	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/activities/listbranchfiles"
	"github.com/retran/meowg1k/internal/activities/liststaged"
	"github.com/retran/meowg1k/internal/activities/summarizeall"
	"github.com/retran/meowg1k/internal/activities/summarizefile"
	"github.com/retran/meowg1k/internal/adapters/gateway"
	"github.com/retran/meowg1k/internal/adapters/git"
	"github.com/retran/meowg1k/internal/adapters/workspace"
	"github.com/retran/meowg1k/internal/core/commit"
	"github.com/retran/meowg1k/internal/core/filter"
	"github.com/retran/meowg1k/internal/core/model"
	"github.com/retran/meowg1k/internal/core/profile"
	"github.com/retran/meowg1k/internal/core/prompt"
	"github.com/retran/meowg1k/internal/core/provider"
	"github.com/retran/meowg1k/internal/core/pullrequest"
	"github.com/retran/meowg1k/internal/core/summarize"
	"github.com/retran/meowg1k/internal/core/task"
	commitFlow "github.com/retran/meowg1k/internal/flows/commit"
	"github.com/retran/meowg1k/internal/flows/generate"
	pr "github.com/retran/meowg1k/internal/flows/pullrequest"
	"github.com/retran/meowg1k/pkg/executor"
)

// CreateCommitFlow creates a complete commit flow with all dependencies.
func (c *Container) CreateCommitFlow() (executor.Flow, error) {
	workspaceService := workspace.NewService(c.CommandService)
	gitService, err := git.NewService(workspaceService)
	if err != nil {
		return nil, fmt.Errorf("failed to create git service: %w", err)
	}

	filterService, err := filter.NewService(c.ConfigService)
	if err != nil {
		return nil, fmt.Errorf("failed to create filter service: %w", err)
	}

	providerService := provider.NewService()

	modelService, err := model.NewService(c.ConfigService, providerService)
	if err != nil {
		return nil, fmt.Errorf("failed to create model service: %w", err)
	}

	profileService, err := profile.NewService(c.ConfigService, modelService)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile service: %w", err)
	}

	summarizeService, err := summarize.NewService(c.ConfigService, profileService)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize service: %w", err)
	}

	commitConfigService, err := commit.NewService(c.ConfigService, profileService)
	if err != nil {
		return nil, fmt.Errorf("failed to create commit config service: %w", err)
	}

	gatewayFactory, err := gateway.NewFactory(c.GetRateLimitRepo(), c.GetCacheRepo(), c.CommandService, c.TraceLogger, c.CommandService, c.GetHTTPClientService())
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway factory: %w", err)
	}

	invokeLLMFactory, err := invokellm.NewFactory(gatewayFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoke llm factory: %w", err)
	}

	listStagedActivityFactory, err := liststaged.NewFactory(gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create list staged activity factory: %w", err)
	}

	fetchFileDiffActivityFactory, err := fetchfilediff.NewFactory(gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch file diff activity factory: %w", err)
	}

	fetchAllDiffsFactory, err := fetchalldiffs.NewFactory(fetchFileDiffActivityFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch all diffs factory: %w", err)
	}

	listBranchFilesActivityFactory, err := listbranchfiles.NewFactory(gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create list branch files activity factory: %w", err)
	}

	fetchBranchFileDiffActivityFactory, err := fetchbranchfilediff.NewFactory(gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch branch file diff activity factory: %w", err)
	}
	fetchAllBranchDiffsFactory, err := fetchallbranchdiffs.NewFactory(fetchBranchFileDiffActivityFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch all branch diffs factory: %w", err)
	}

	applyFiltersActivityFactory, err := applyfilters.NewFactory(filterService)
	if err != nil {
		return nil, fmt.Errorf("failed to create apply filters activity factory: %w", err)
	}

	summarizeFileFactory, err := summarizefile.NewFactory(invokeLLMFactory, summarizeService)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize file factory: %w", err)
	}

	summarizeAllFactory, err := summarizeall.NewFactory(summarizeFileFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize all factory: %w", err)
	}

	composeCommitFactory, err := composecommit.NewFactory(invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose commit factory: %w", err)
	}

	composeFlatCommitFactory, err := composeflatcommit.NewFactory(invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose flat commit factory: %w", err)
	}

	flowFactory, err := commitFlow.NewFactory(
		listStagedActivityFactory,
		listBranchFilesActivityFactory,
		applyFiltersActivityFactory,
		fetchAllDiffsFactory,
		fetchAllBranchDiffsFactory,
		summarizeAllFactory,
		composeCommitFactory,
		composeFlatCommitFactory,
		commitConfigService,
		c.CommandService,
		c.OutputService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create commit flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

// CreateGenerateFlow creates a complete generate flow with all dependencies.
func (c *Container) CreateGenerateFlow() (executor.Flow, error) {
	providerService := provider.NewService()

	modelService, err := model.NewService(c.ConfigService, providerService)
	if err != nil {
		return nil, fmt.Errorf("failed to create model service: %w", err)
	}

	profileService, err := profile.NewService(c.ConfigService, modelService)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile service: %w", err)
	}

	taskService, err := task.NewService(
		c.CommandService,
		c.ConfigService,
		profileService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create task service: %w", err)
	}

	generatePromptService, err := prompt.NewGeneratePromptService(
		c.CommandService,
		taskService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create generate prompt service: %w", err)
	}

	gatewayFactory, err := gateway.NewFactory(c.GetRateLimitRepo(), c.GetCacheRepo(), c.CommandService, c.TraceLogger, c.CommandService, c.GetHTTPClientService())
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway factory: %w", err)
	}

	invokeLLMFactory, err := invokellm.NewFactory(gatewayFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoke llm factory: %w", err)
	}

	flowFactory, err := generate.NewFlowFactory(
		taskService,
		generatePromptService,
		generatePromptService,
		invokeLLMFactory,
		c.OutputService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create generate flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

// CreatePullRequestFlow creates a complete pull request flow with all dependencies.
func (c *Container) CreatePullRequestFlow() (executor.Flow, error) {
	workspaceService := workspace.NewService(c.CommandService)
	gitService, err := git.NewService(workspaceService)
	if err != nil {
		return nil, fmt.Errorf("failed to create git service: %w", err)
	}

	filterService, err := filter.NewService(c.ConfigService)
	if err != nil {
		return nil, fmt.Errorf("failed to create filter service: %w", err)
	}

	providerService := provider.NewService()

	modelService, err := model.NewService(c.ConfigService, providerService)
	if err != nil {
		return nil, fmt.Errorf("failed to create model service: %w", err)
	}

	profileService, err := profile.NewService(c.ConfigService, modelService)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile service: %w", err)
	}

	summarizeService, err := summarize.NewService(c.ConfigService, profileService)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize service: %w", err)
	}

	prConfigService, err := pullrequest.NewService(c.ConfigService, profileService)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR config service: %w", err)
	}

	gatewayFactory, err := gateway.NewFactory(c.GetRateLimitRepo(), c.GetCacheRepo(), c.CommandService, c.TraceLogger, c.CommandService, c.GetHTTPClientService())
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway factory: %w", err)
	}

	invokeLLMFactory, err := invokellm.NewFactory(gatewayFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoke llm factory: %w", err)
	}

	// Activities for branch diff mode
	listBranchFilesActivityFactory, err := listbranchfiles.NewFactory(gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create list branch files activity factory: %w", err)
	}

	fetchBranchFileDiffActivityFactory, err := fetchbranchfilediff.NewFactory(gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch branch file diff activity factory: %w", err)
	}

	fetchAllBranchDiffsFactory, err := fetchallbranchdiffs.NewFactory(fetchBranchFileDiffActivityFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch all branch diffs factory: %w", err)
	}

	// Common activities
	applyFiltersActivityFactory, err := applyfilters.NewFactory(filterService)
	if err != nil {
		return nil, fmt.Errorf("failed to create apply filters activity factory: %w", err)
	}

	summarizeFileFactory, err := summarizefile.NewFactory(invokeLLMFactory, summarizeService)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize file factory: %w", err)
	}

	summarizeAllFactory, err := summarizeall.NewFactory(summarizeFileFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize all factory: %w", err)
	}

	composePRFactory, err := composepr.NewFactory(invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose pullrequest factory: %w", err)
	}

	composeFlatPRFactory, err := composeflatpr.NewFactory(invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose flat pullrequest factory: %w", err)
	}

	flowFactory, err := pr.NewFactory(
		listBranchFilesActivityFactory,
		applyFiltersActivityFactory,
		fetchAllBranchDiffsFactory,
		summarizeAllFactory,
		composePRFactory,
		composeFlatPRFactory,
		prConfigService,
		c.CommandService,
		c.OutputService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pullrequest flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}
