// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"

	"github.com/retran/meowg1k/internal/activities/applyfilters"
	"github.com/retran/meowg1k/internal/activities/buildvectorindices"
	"github.com/retran/meowg1k/internal/activities/chunkallfiles"
	"github.com/retran/meowg1k/internal/activities/chunkfile"
	"github.com/retran/meowg1k/internal/activities/cleanupstaledata"
	"github.com/retran/meowg1k/internal/activities/composecommit"
	"github.com/retran/meowg1k/internal/activities/composeflatcommit"
	"github.com/retran/meowg1k/internal/activities/composeflatpr"
	"github.com/retran/meowg1k/internal/activities/composepr"
	"github.com/retran/meowg1k/internal/activities/computeallembeddings"
	"github.com/retran/meowg1k/internal/activities/computeembeddingsbatch"
	"github.com/retran/meowg1k/internal/activities/deduplicateandprepare"
	"github.com/retran/meowg1k/internal/activities/distributeandsave"
	"github.com/retran/meowg1k/internal/activities/fetchallbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/fetchalldiffs"
	"github.com/retran/meowg1k/internal/activities/fetchbranchfilediff"
	"github.com/retran/meowg1k/internal/activities/fetchfilediff"
	"github.com/retran/meowg1k/internal/activities/finalizesnapshots"
	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/activities/listbranchfiles"
	"github.com/retran/meowg1k/internal/activities/liststaged"
	"github.com/retran/meowg1k/internal/activities/preparebatches"
	queryactivity "github.com/retran/meowg1k/internal/activities/query"
	"github.com/retran/meowg1k/internal/activities/retrievecontext"
	"github.com/retran/meowg1k/internal/activities/savedocumentversion"
	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
	"github.com/retran/meowg1k/internal/activities/summarizeall"
	"github.com/retran/meowg1k/internal/activities/summarizefile"
	"github.com/retran/meowg1k/internal/adapters/gateway"
	"github.com/retran/meowg1k/internal/adapters/git"
	indexRepo "github.com/retran/meowg1k/internal/adapters/sqlite/index"
	"github.com/retran/meowg1k/internal/adapters/sqlite/meta"
	"github.com/retran/meowg1k/internal/adapters/workspace"
	"github.com/retran/meowg1k/internal/core/chunker"
	"github.com/retran/meowg1k/internal/core/commit"
	"github.com/retran/meowg1k/internal/core/filter"
	"github.com/retran/meowg1k/internal/core/index"
	"github.com/retran/meowg1k/internal/core/model"
	"github.com/retran/meowg1k/internal/core/profile"
	"github.com/retran/meowg1k/internal/core/project"
	"github.com/retran/meowg1k/internal/core/prompt"
	"github.com/retran/meowg1k/internal/core/provider"
	"github.com/retran/meowg1k/internal/core/pullrequest"
	"github.com/retran/meowg1k/internal/core/retrieval"
	"github.com/retran/meowg1k/internal/core/summarize"
	"github.com/retran/meowg1k/internal/core/task"
	"github.com/retran/meowg1k/internal/core/vector"
	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
	askFlow "github.com/retran/meowg1k/internal/flows/ask"
	commitFlow "github.com/retran/meowg1k/internal/flows/commit"
	"github.com/retran/meowg1k/internal/flows/generate"
	indexFlow "github.com/retran/meowg1k/internal/flows/index"
	pr "github.com/retran/meowg1k/internal/flows/pullrequest"
	queryFlow "github.com/retran/meowg1k/internal/flows/query"
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

// CreateIndexReconcileFlow creates a complete index reconciliation flow with all dependencies.
func (c *Container) CreateIndexReconcileFlow() (executor.Flow, error) {
	workspaceService := workspace.NewService(c.CommandService)
	gitService, err := git.NewService(workspaceService)
	if err != nil {
		return nil, fmt.Errorf("failed to create git service: %w", err)
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

	// Get resolved index configuration
	indexConfigService, err := index.NewConfigService(c.ConfigService, profileService)
	if err != nil {
		return nil, fmt.Errorf("failed to create index config service: %w", err)
	}

	indexConfig, err := indexConfigService.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get index config: %w", err)
	}

	// Initialize database and repositories
	if err := c.initDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	filterService, err := filter.NewService(c.ConfigService)
	if err != nil {
		return nil, fmt.Errorf("failed to create filter service: %w", err)
	}

	metaRepo := meta.NewRepository(c.dbHost)
	indexRepoImpl := indexRepo.NewRepository(c.dbHost)

	// Initialize services
	projectStateSvc := project.NewStateService(gitService, filterService, workspaceService)
	chunkerSvc := chunker.NewService(indexConfig.ChunkerMaxRunes, indexConfig.ChunkerOverlapRunes)
	indexSvc, err := index.NewService(indexRepoImpl, indexRepoImpl)
	if err != nil {
		return nil, fmt.Errorf("failed to create index service: %w", err)
	}
	vectorIndexSvc := vector.NewService(indexRepoImpl, indexRepoImpl, metaRepo)

	// Create gateway factory for embeddings
	gatewayFactory, err := gateway.NewFactory(
		c.GetRateLimitRepo(),
		c.GetCacheRepo(),
		c.CommandService,
		c.TraceLogger,
		c.CommandService,
		c.GetHTTPClientService(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway factory: %w", err)
	}

	// Create embeddings gateway
	embeddingsGW, err := gatewayFactory.NewEmbeddingsGateway(c.ShutdownService.Context(), indexConfig.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings gateway: %w", err)
	}

	// Initialize activity factories
	cleanupFactory, err := cleanupstaledata.NewFactory(indexRepoImpl, metaRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to create cleanup factory: %w", err)
	}

	scanStateFactory, err := scanworkspacestate.NewFactory(projectStateSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create scan state factory: %w", err)
	}

	deduplicateFactory, err := deduplicateandprepare.NewFactory(indexSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create deduplicate factory: %w", err)
	}

	chunkFileFactory, err := chunkfile.NewFactory(chunkerSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunk file factory: %w", err)
	}

	chunkAllFactory, err := chunkallfiles.NewFactory(chunkFileFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunk all factory: %w", err)
	}

	prepareBatchesFactory, err := preparebatches.NewFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create prepare batches factory: %w", err)
	}

	computeBatchFactory, err := computeembeddingsbatch.NewFactory(embeddingsGW, indexConfig.Profile.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute batch factory: %w", err)
	}

	computeAllFactory, err := computeallembeddings.NewFactory(computeBatchFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute all factory: %w", err)
	}

	saveDocVersionFactory, err := savedocumentversion.NewFactory(indexSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create save document version factory: %w", err)
	}

	distributeAndSaveFactory, err := distributeandsave.NewFactory(saveDocVersionFactory, indexRepoImpl)
	if err != nil {
		return nil, fmt.Errorf("failed to create distribute and save factory: %w", err)
	}

	finalizeSnapshotsFactory, err := finalizesnapshots.NewFactory(indexSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create finalize snapshots factory: %w", err)
	}

	buildVectorIndicesFactory, err := buildvectorindices.NewFactory(vectorIndexSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create build vector indices factory: %w", err)
	}

	// Create index flow factory
	flowFactory, err := indexFlow.NewFactory(
		cleanupFactory,
		scanStateFactory,
		deduplicateFactory,
		chunkAllFactory,
		prepareBatchesFactory,
		computeAllFactory,
		distributeAndSaveFactory,
		finalizeSnapshotsFactory,
		buildVectorIndicesFactory,
		indexConfig.BatchSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create index flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

// CreateQueryFlow creates a complete query flow with all dependencies.
func (c *Container) CreateQueryFlow() (executor.Flow, error) {
	// Initialize database and repositories
	if err := c.initDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	metaRepo := meta.NewRepository(c.dbHost)
	indexRepoImpl := indexRepo.NewRepository(c.dbHost)

	providerService := provider.NewService()
	modelService, err := model.NewService(c.ConfigService, providerService)
	if err != nil {
		return nil, fmt.Errorf("failed to create model service: %w", err)
	}

	profileService, err := profile.NewService(c.ConfigService, modelService)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile service: %w", err)
	}

	// Get resolved index configuration
	indexConfigService, err := index.NewConfigService(c.ConfigService, profileService)
	if err != nil {
		return nil, fmt.Errorf("failed to create index config service: %w", err)
	}

	indexConfig, err := indexConfigService.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get index config: %w", err)
	}

	// Initialize services
	vectorSearchSvc, err := vector.NewSearchService(metaRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector search service: %w", err)
	}

	// Create gateway factory for embeddings
	gatewayFactory, err := gateway.NewFactory(
		c.GetRateLimitRepo(),
		c.GetCacheRepo(),
		c.CommandService,
		c.TraceLogger,
		c.CommandService,
		c.GetHTTPClientService(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway factory: %w", err)
	}

	// Create embeddings gateway
	embeddingsGW, err := gatewayFactory.NewEmbeddingsGateway(c.ShutdownService.Context(), indexConfig.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings gateway: %w", err)
	}

	// Create retrieval service with RetrievalDocument for indexing, RetrievalQuery for search
	retrievalSvc, err := retrieval.NewService(embeddingsGW, vectorSearchSvc, indexRepoImpl, indexConfig.Profile.Model, domainGateway.RetrievalQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to create retrieval service: %w", err)
	}

	// Create query activity factory
	queryActivityFactory, err := queryactivity.NewFactory(retrievalSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create query activity factory: %w", err)
	}

	// Create query flow factory
	queryFlowFactory, err := queryFlow.NewFactory(
		queryActivityFactory,
		c.CommandService,
		c.OutputService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create query flow factory: %w", err)
	}

	return queryFlowFactory.NewFlow(), nil
}

// CreateAskFlow creates a complete ask flow with all dependencies.
func (c *Container) CreateAskFlow() (executor.Flow, error) {
	// Initialize database and repositories
	if err := c.initDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	metaRepo := meta.NewRepository(c.dbHost)
	indexRepoImpl := indexRepo.NewRepository(c.dbHost)

	// Get configuration
	config, err := c.ConfigService.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	// Validate ask configuration exists
	if config.Ask == nil {
		return nil, fmt.Errorf("ask configuration is missing")
	}

	// Initialize services
	providerService := provider.NewService()
	modelService, err := model.NewService(c.ConfigService, providerService)
	if err != nil {
		return nil, fmt.Errorf("failed to create model service: %w", err)
	}

	profileService, err := profile.NewService(c.ConfigService, modelService)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile service: %w", err)
	}

	// Get resolved index configuration
	indexConfigService, err := index.NewConfigService(c.ConfigService, profileService)
	if err != nil {
		return nil, fmt.Errorf("failed to create index config service: %w", err)
	}

	indexConfig, err := indexConfigService.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get index config: %w", err)
	}

	vectorSearchSvc, err := vector.NewSearchService(metaRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector search service: %w", err)
	}

	// Create gateway factory
	gatewayFactory, err := gateway.NewFactory(
		c.GetRateLimitRepo(),
		c.GetCacheRepo(),
		c.CommandService,
		c.TraceLogger,
		c.CommandService,
		c.GetHTTPClientService(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway factory: %w", err)
	}

	// Create embeddings gateway
	embeddingsGW, err := gatewayFactory.NewEmbeddingsGateway(c.ShutdownService.Context(), indexConfig.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings gateway: %w", err)
	}

	// Create retrieval service with RetrievalQuery for ask flow
	retrievalSvc, err := retrieval.NewService(embeddingsGW, vectorSearchSvc, indexRepoImpl, indexConfig.Profile.Model, domainGateway.RetrievalQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to create retrieval service: %w", err)
	}

	// Create retrieve context activity factory
	retrieveContextFactory, err := retrievecontext.NewFactory(retrievalSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create retrieve context factory: %w", err)
	}

	// Create invoke LLM factory
	invokeLLMFactory, err := invokellm.NewFactory(gatewayFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoke LLM factory: %w", err)
	}

	// Determine system prompt with mandatory check
	// Priority: flag > config
	systemPrompt := config.Ask.SystemPrompt
	if flagPrompt, err := c.CommandService.GetSystemPromptFlag(); err == nil && flagPrompt != "" {
		systemPrompt = flagPrompt
	}

	// System prompt is mandatory
	if systemPrompt == "" {
		return nil, fmt.Errorf("system prompt is required (set in config ask.systemPrompt or via --system-prompt flag)")
	}

	// Create ask flow factory with resolved parameters
	askFlowFactory, err := askFlow.NewFactory(
		retrieveContextFactory,
		invokeLLMFactory,
		c.CommandService,
		profileService,
		c.OutputService,
		c.ConfigService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ask flow factory: %w", err)
	}

	return askFlowFactory.NewFlow(), nil
}
