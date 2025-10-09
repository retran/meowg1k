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
	"fmt"

	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/composecommit"
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
	commitFlow "github.com/retran/meowg1k/internal/flows/commit"
	"github.com/retran/meowg1k/internal/flows/generate"
	"github.com/retran/meowg1k/internal/flows/pr"
	commitService "github.com/retran/meowg1k/internal/services/commit"
	"github.com/retran/meowg1k/internal/services/filter"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/internal/services/model"
	"github.com/retran/meowg1k/internal/services/profile"
	"github.com/retran/meowg1k/internal/services/prompt"
	"github.com/retran/meowg1k/internal/services/provider"
	"github.com/retran/meowg1k/internal/services/pullRequest"
	"github.com/retran/meowg1k/internal/services/summarize"
	"github.com/retran/meowg1k/internal/services/task"
	"github.com/retran/meowg1k/internal/services/workspace"
	"github.com/retran/meowg1k/pkg/executor"
)

// CreateCommitFlow creates a complete commit flow with all dependencies.
func (c *Container) CreateCommitFlow() (executor.Flow, error) {
	workspaceService := workspace.NewService()
	gitService, err := git.NewService(workspaceService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	filterService, err := filter.NewService(c.ConfigService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	providerService := provider.NewService()

	modelService, err := model.NewService(c.ConfigService, providerService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	profileService, err := profile.NewService(c.ConfigService, modelService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	summarizeService, err := summarize.NewService(c.ConfigService, profileService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	commitConfigService, err := commitService.NewService(c.ConfigService, profileService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	gatewayFactory := gateway.NewFactory(c.GetRateLimitRepo)
	invokeLLMFactory, err := invokellm.NewFactory(gatewayFactory)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create invoke llm factory: %w", err)
	}

	// Activities for regular staged commit mode
	listStagedActivityFactory, err := liststaged.NewFactory(gitService)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create list staged activity factory: %w", err)
	}

	fetchFileDiffActivityFactory, err := fetchfilediff.NewFactory(gitService)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create fetch file diff activity factory: %w", err)
	}

	fetchAllDiffsFactory, err := fetchalldiffs.NewFactory(fetchFileDiffActivityFactory)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create fetch all diffs factory: %w", err)
	}

	// Activities for squash/branch diff mode
	listBranchFilesActivityFactory, err := listbranchfiles.NewFactory(gitService)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create list branch files activity factory: %w", err)
	}

	fetchBranchFileDiffActivityFactory, err := fetchbranchfilediff.NewFactory(gitService)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create fetch branch file diff activity factory: %w", err)
	}
	// TODO proper error
	fetchAllBranchDiffsFactory, err := fetchallbranchdiffs.NewFactory(fetchBranchFileDiffActivityFactory)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create fetch all branch diffs factory: %w", err)
	}

	// Common activities
	applyFiltersActivityFactory, err := applyfilters.NewFactory(filterService)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create apply filters activity factory: %w", err)
	}

	summarizeFileFactory, err := summarizefile.NewFactory(invokeLLMFactory, summarizeService)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create summarize file factory: %w", err)
	}

	summarizeAllFactory, err := summarizeall.NewFactory(summarizeFileFactory)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create summarize all factory: %w", err)
	}

	composeCommitFactory, err := composecommit.NewFactory(invokeLLMFactory)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create compose commit factory: %w", err)
	}

	flowFactory, err := commitFlow.NewFactory(
		listStagedActivityFactory,
		listBranchFilesActivityFactory,
		applyFiltersActivityFactory,
		fetchAllDiffsFactory,
		fetchAllBranchDiffsFactory,
		summarizeAllFactory,
		composeCommitFactory,
		commitConfigService,
		c.CommandService,
		c.OutputService,
	)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create commit flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

// CreateGenerateFlow creates a complete generate flow with all dependencies.
func (c *Container) CreateGenerateFlow() (executor.Flow, error) {
	providerService := provider.NewService()

	modelService, err := model.NewService(c.ConfigService, providerService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	profileService, err := profile.NewService(c.ConfigService, modelService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	taskService, err := task.NewService(
		c.CommandService,
		c.ConfigService,
		profileService,
	)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	generatePromptService, err := prompt.NewGeneratePromptService(
		c.CommandService,
		taskService,
	)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	gatewayFactory := gateway.NewFactory(c.GetRateLimitRepo)
	invokeLLMFactory, err := invokellm.NewFactory(gatewayFactory)
	if err != nil {
		// TODO proper error
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
		// TODO proper error
		return nil, fmt.Errorf("failed to create generate flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

// CreatePRFlow creates a complete PR flow with all dependencies.
func (c *Container) CreatePRFlow() (executor.Flow, error) {
	workspaceService := workspace.NewService()
	gitService, err := git.NewService(workspaceService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	filterService, err := filter.NewService(c.ConfigService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	providerService := provider.NewService()

	modelService, err := model.NewService(c.ConfigService, providerService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	profileService, err := profile.NewService(c.ConfigService, modelService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	summarizeService, err := summarize.NewService(c.ConfigService, profileService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	prConfigService, err := pullRequest.NewService(c.ConfigService, profileService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	gatewayFactory := gateway.NewFactory(c.GetRateLimitRepo)
	invokeLLMFactory, err := invokellm.NewFactory(gatewayFactory)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create invoke llm factory: %w", err)
	}

	// Activities for branch diff mode
	listBranchFilesActivityFactory, err := listbranchfiles.NewFactory(gitService)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create list branch files activity factory: %w", err)
	}

	fetchBranchFileDiffActivityFactory, err := fetchbranchfilediff.NewFactory(gitService)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create fetch branch file diff activity factory: %w", err)
	}

	fetchAllBranchDiffsFactory, err := fetchallbranchdiffs.NewFactory(fetchBranchFileDiffActivityFactory)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create fetch all branch diffs factory: %w", err)
	}

	// Common activities
	applyFiltersActivityFactory, err := applyfilters.NewFactory(filterService)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create apply filters activity factory: %w", err)
	}

	summarizeFileFactory, err := summarizefile.NewFactory(invokeLLMFactory, summarizeService)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create summarize file factory: %w", err)
	}

	summarizeAllFactory, err := summarizeall.NewFactory(summarizeFileFactory)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create summarize all factory: %w", err)
	}

	composePRFactory, err := composepr.NewFactory(invokeLLMFactory)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create compose pullRequest factory: %w", err)
	}

	flowFactory, err := pr.NewFactory(
		listBranchFilesActivityFactory,
		applyFiltersActivityFactory,
		fetchAllBranchDiffsFactory,
		summarizeAllFactory,
		composePRFactory,
		prConfigService,
		c.CommandService,
		c.OutputService,
	)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create pullRequest flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}
