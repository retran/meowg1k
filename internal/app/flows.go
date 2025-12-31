// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"

	"github.com/retran/meowg1k/internal/activities/agentloop"
	"github.com/retran/meowg1k/internal/activities/buildbatches"
	"github.com/retran/meowg1k/internal/activities/buildindexes"
	"github.com/retran/meowg1k/internal/activities/draftcommit"
	"github.com/retran/meowg1k/internal/activities/draftcommitflat"
	"github.com/retran/meowg1k/internal/activities/draftcontent"
	"github.com/retran/meowg1k/internal/activities/draftpr"
	"github.com/retran/meowg1k/internal/activities/draftprflat"
	"github.com/retran/meowg1k/internal/activities/embedall"
	"github.com/retran/meowg1k/internal/activities/embedbatch"
	"github.com/retran/meowg1k/internal/activities/fetchbranchdiff"
	"github.com/retran/meowg1k/internal/activities/fetchbranchdiffs"
	"github.com/retran/meowg1k/internal/activities/fetchcontext"
	"github.com/retran/meowg1k/internal/activities/fetchstageddiff"
	"github.com/retran/meowg1k/internal/activities/fetchstageddiffs"
	"github.com/retran/meowg1k/internal/activities/filterfiles"
	"github.com/retran/meowg1k/internal/activities/finalizeindex"
	"github.com/retran/meowg1k/internal/activities/listbranchchanges"
	"github.com/retran/meowg1k/internal/activities/liststagedfiles"
	"github.com/retran/meowg1k/internal/activities/prepareindex"
	"github.com/retran/meowg1k/internal/activities/pruneindex"
	"github.com/retran/meowg1k/internal/activities/savechunks"
	"github.com/retran/meowg1k/internal/activities/savefileversion"
	"github.com/retran/meowg1k/internal/activities/scanworktree"
	queryactivity "github.com/retran/meowg1k/internal/activities/searchindex"
	"github.com/retran/meowg1k/internal/activities/splitfile"
	"github.com/retran/meowg1k/internal/activities/splitfiles"
	"github.com/retran/meowg1k/internal/activities/summarizechanges"
	"github.com/retran/meowg1k/internal/activities/summarizefilechanges"
	"github.com/retran/meowg1k/internal/adapters/gateway"
	"github.com/retran/meowg1k/internal/adapters/git"
	indexRepo "github.com/retran/meowg1k/internal/adapters/sqlite/index"
	"github.com/retran/meowg1k/internal/adapters/sqlite/meta"
	"github.com/retran/meowg1k/internal/adapters/workspace"
	agentcore "github.com/retran/meowg1k/internal/core/agent"
	"github.com/retran/meowg1k/internal/core/chunker"
	"github.com/retran/meowg1k/internal/core/commit"
	"github.com/retran/meowg1k/internal/core/filter"
	"github.com/retran/meowg1k/internal/core/index"
	"github.com/retran/meowg1k/internal/core/model"
	"github.com/retran/meowg1k/internal/core/profile"
	"github.com/retran/meowg1k/internal/core/project"
	"github.com/retran/meowg1k/internal/core/prompt"
	"github.com/retran/meowg1k/internal/core/provider"
	pullrequest "github.com/retran/meowg1k/internal/core/pullrequest"
	"github.com/retran/meowg1k/internal/core/retrieval"
	"github.com/retran/meowg1k/internal/core/summarize"
	"github.com/retran/meowg1k/internal/core/task"
	"github.com/retran/meowg1k/internal/core/vector"
	"github.com/retran/meowg1k/internal/domain/config"
	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	askFlow "github.com/retran/meowg1k/internal/flows/ask"
	commitFlow "github.com/retran/meowg1k/internal/flows/commitmsg"
	indexFlow "github.com/retran/meowg1k/internal/flows/index"
	prflow "github.com/retran/meowg1k/internal/flows/pr"
	agentFlow "github.com/retran/meowg1k/internal/flows/run"
	searchFlow "github.com/retran/meowg1k/internal/flows/search"
	"github.com/retran/meowg1k/internal/flows/write"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// CreateCommitMsgFlow creates a complete commit message flow with all dependencies.
func (c *Container) CreateCommitMsgFlow() (executor.Flow, error) {
	services, err := c.buildCommitServices()
	if err != nil {
		return nil, err
	}

	factories, err := buildCommitActivityFactories(services)
	if err != nil {
		return nil, err
	}

	flowFactory, err := commitFlow.NewFactory(
		factories.listStagedActivityFactory,
		factories.listBranchFilesActivityFactory,
		factories.applyFiltersActivityFactory,
		factories.fetchAllDiffsFactory,
		factories.fetchAllBranchDiffsFactory,
		factories.summarizeAllFactory,
		factories.composeCommitFactory,
		factories.composeFlatCommitFactory,
		services.commitConfigService,
		c.CommandService,
		c.OutputService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create commit message flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

type commitServices struct {
	gitService          *git.Service
	filterService       *filter.Service
	summarizeService    *summarize.Service
	profileService      *profile.Service
	commitConfigService *commit.Service
	invokeLLMFactory    executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]
}

type commitActivityFactories struct {
	listStagedActivityFactory      executor.ActivityFactory[*liststagedfiles.Input, *liststagedfiles.Output]
	listBranchFilesActivityFactory executor.ActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]
	fetchAllDiffsFactory           executor.ActivityFactory[*fetchstageddiffs.Input, *fetchstageddiffs.Output]
	fetchAllBranchDiffsFactory     executor.ActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]
	applyFiltersActivityFactory    executor.ActivityFactory[*filterfiles.Input, *filterfiles.Output]
	summarizeAllFactory            executor.ActivityFactory[*summarizechanges.Input, *summarizechanges.Output]
	composeCommitFactory           executor.ActivityFactory[*draftcommit.Input, *draftcommit.Output]
	composeFlatCommitFactory       executor.ActivityFactory[*draftcommitflat.Input, *draftcommitflat.Output]
}

func (c *Container) buildCommitServices() (*commitServices, error) {
	common, err := c.buildCommonFlowServices()
	if err != nil {
		return nil, err
	}

	commitConfigService, err := commit.NewService(c.ConfigService, common.profileService)
	if err != nil {
		return nil, fmt.Errorf("failed to create commit config service: %w", err)
	}

	return &commitServices{
		gitService:          common.gitService,
		filterService:       common.filterService,
		summarizeService:    common.summarizeService,
		profileService:      common.profileService,
		commitConfigService: commitConfigService,
		invokeLLMFactory:    common.invokeLLMFactory,
	}, nil
}

func buildCommitActivityFactories(services *commitServices) (*commitActivityFactories, error) {
	listStagedActivityFactory, err := liststagedfiles.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create list staged activity factory: %w", err)
	}

	fetchFileDiffActivityFactory, err := fetchstageddiff.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch file diff activity factory: %w", err)
	}

	fetchAllDiffsFactory, err := fetchstageddiffs.NewFactory(fetchFileDiffActivityFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch all diffs factory: %w", err)
	}

	listBranchFilesActivityFactory, err := listbranchchanges.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create list branch files activity factory: %w", err)
	}

	fetchBranchFileDiffActivityFactory, err := fetchbranchdiff.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch branch file diff activity factory: %w", err)
	}

	fetchAllBranchDiffsFactory, err := fetchbranchdiffs.NewFactory(fetchBranchFileDiffActivityFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch all branch diffs factory: %w", err)
	}

	applyFiltersActivityFactory, err := filterfiles.NewFactory(services.filterService)
	if err != nil {
		return nil, fmt.Errorf("failed to create apply filters activity factory: %w", err)
	}

	summarizeFileFactory, err := summarizefilechanges.NewFactory(services.invokeLLMFactory, services.summarizeService)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize file factory: %w", err)
	}

	summarizeAllFactory, err := summarizechanges.NewFactory(summarizeFileFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize all factory: %w", err)
	}

	composeCommitFactory, err := draftcommit.NewFactory(services.invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose commit factory: %w", err)
	}

	composeFlatCommitFactory, err := draftcommitflat.NewFactory(services.invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose flat commit factory: %w", err)
	}

	return &commitActivityFactories{
		listStagedActivityFactory:      listStagedActivityFactory,
		listBranchFilesActivityFactory: listBranchFilesActivityFactory,
		fetchAllDiffsFactory:           fetchAllDiffsFactory,
		fetchAllBranchDiffsFactory:     fetchAllBranchDiffsFactory,
		applyFiltersActivityFactory:    applyFiltersActivityFactory,
		summarizeAllFactory:            summarizeAllFactory,
		composeCommitFactory:           composeCommitFactory,
		composeFlatCommitFactory:       composeFlatCommitFactory,
	}, nil
}

// CreateWriteFlow creates a complete write flow with all dependencies.
func (c *Container) CreateWriteFlow() (executor.Flow, error) {
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
		return nil, fmt.Errorf("failed to create write prompt service: %w", err)
	}

	gatewayFactory, err := gateway.NewFactory(c.GetRateLimitRepo(), c.GetCacheRepo(), c.CommandService, c.TraceLogger, c.CommandService, c.GetHTTPClientService())
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway factory: %w", err)
	}

	invokeLLMFactory, err := draftcontent.NewFactory(gatewayFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoke llm factory: %w", err)
	}

	flowFactory, err := write.NewFlowFactory(
		taskService,
		generatePromptService,
		generatePromptService,
		invokeLLMFactory,
		c.OutputService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create write flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

// CreatePrFlow creates a complete pull request flow with all dependencies.
func (c *Container) CreatePrFlow() (executor.Flow, error) {
	services, err := c.buildPullRequestServices()
	if err != nil {
		return nil, err
	}

	factories, err := buildPullRequestActivityFactories(services)
	if err != nil {
		return nil, err
	}

	flowFactory, err := prflow.NewFactory(
		factories.listBranchFilesActivityFactory,
		factories.applyFiltersActivityFactory,
		factories.fetchAllBranchDiffsFactory,
		factories.summarizeAllFactory,
		factories.composePRFactory,
		factories.composeFlatPRFactory,
		services.prConfigService,
		c.CommandService,
		c.OutputService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pr flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

type pullRequestServices struct {
	gitService       *git.Service
	filterService    *filter.Service
	summarizeService *summarize.Service
	profileService   *profile.Service
	prConfigService  *pullrequest.Service
	invokeLLMFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]
}

type pullRequestActivityFactories struct {
	listBranchFilesActivityFactory executor.ActivityFactory[*listbranchchanges.Input, *listbranchchanges.Output]
	fetchAllBranchDiffsFactory     executor.ActivityFactory[*fetchbranchdiffs.Input, *fetchbranchdiffs.Output]
	applyFiltersActivityFactory    executor.ActivityFactory[*filterfiles.Input, *filterfiles.Output]
	summarizeAllFactory            executor.ActivityFactory[*summarizechanges.Input, *summarizechanges.Output]
	composePRFactory               executor.ActivityFactory[*draftpr.Input, *draftpr.Output]
	composeFlatPRFactory           executor.ActivityFactory[*draftprflat.Input, *draftprflat.Output]
}

func (c *Container) buildPullRequestServices() (*pullRequestServices, error) {
	common, err := c.buildCommonFlowServices()
	if err != nil {
		return nil, err
	}

	prConfigService, err := pullrequest.NewService(c.ConfigService, common.profileService)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR config service: %w", err)
	}

	return &pullRequestServices{
		gitService:       common.gitService,
		filterService:    common.filterService,
		summarizeService: common.summarizeService,
		profileService:   common.profileService,
		prConfigService:  prConfigService,
		invokeLLMFactory: common.invokeLLMFactory,
	}, nil
}

type commonFlowServices struct {
	gitService       *git.Service
	filterService    *filter.Service
	summarizeService *summarize.Service
	profileService   *profile.Service
	invokeLLMFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]
}

func (c *Container) buildCommonFlowServices() (*commonFlowServices, error) {
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

	gatewayFactory, err := gateway.NewFactory(c.GetRateLimitRepo(), c.GetCacheRepo(), c.CommandService, c.TraceLogger, c.CommandService, c.GetHTTPClientService())
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway factory: %w", err)
	}

	invokeLLMFactory, err := draftcontent.NewFactory(gatewayFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoke llm factory: %w", err)
	}

	return &commonFlowServices{
		gitService:       gitService,
		filterService:    filterService,
		summarizeService: summarizeService,
		profileService:   profileService,
		invokeLLMFactory: invokeLLMFactory,
	}, nil
}

func buildPullRequestActivityFactories(services *pullRequestServices) (*pullRequestActivityFactories, error) {
	listBranchFilesActivityFactory, err := listbranchchanges.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create list branch files activity factory: %w", err)
	}

	fetchBranchFileDiffActivityFactory, err := fetchbranchdiff.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch branch file diff activity factory: %w", err)
	}

	fetchAllBranchDiffsFactory, err := fetchbranchdiffs.NewFactory(fetchBranchFileDiffActivityFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch all branch diffs factory: %w", err)
	}

	applyFiltersActivityFactory, err := filterfiles.NewFactory(services.filterService)
	if err != nil {
		return nil, fmt.Errorf("failed to create apply filters activity factory: %w", err)
	}

	summarizeFileFactory, err := summarizefilechanges.NewFactory(services.invokeLLMFactory, services.summarizeService)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize file factory: %w", err)
	}

	summarizeAllFactory, err := summarizechanges.NewFactory(summarizeFileFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize all factory: %w", err)
	}

	composePRFactory, err := draftpr.NewFactory(services.invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose pr factory: %w", err)
	}

	composeFlatPRFactory, err := draftprflat.NewFactory(services.invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose flat pr factory: %w", err)
	}

	return &pullRequestActivityFactories{
		listBranchFilesActivityFactory: listBranchFilesActivityFactory,
		fetchAllBranchDiffsFactory:     fetchAllBranchDiffsFactory,
		applyFiltersActivityFactory:    applyFiltersActivityFactory,
		summarizeAllFactory:            summarizeAllFactory,
		composePRFactory:               composePRFactory,
		composeFlatPRFactory:           composeFlatPRFactory,
	}, nil
}

// CreateIndexReconcileFlow creates a complete index reconciliation flow with all dependencies.
func (c *Container) CreateIndexReconcileFlow() (executor.Flow, error) {
	workspaceService := workspace.NewService(c.CommandService)
	gitService, err := git.NewService(workspaceService)
	if err != nil {
		return nil, fmt.Errorf("failed to create git service: %w", err)
	}

	indexConfig, err := c.resolveIndexConfig()
	if err != nil {
		return nil, err
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

	factories, err := buildIndexReconcileFactories(
		projectStateSvc,
		chunkerSvc,
		indexSvc,
		indexRepoImpl,
		metaRepo,
		embeddingsGW,
		indexConfig.Profile.Model,
		vectorIndexSvc,
	)
	if err != nil {
		return nil, err
	}

	// Create index flow factory
	flowFactory, err := indexFlow.NewFactory(
		factories.cleanupFactory,
		factories.scanStateFactory,
		factories.deduplicateFactory,
		factories.chunkAllFactory,
		factories.prepareBatchesFactory,
		factories.computeAllFactory,
		factories.distributeAndSaveFactory,
		factories.finalizeSnapshotsFactory,
		factories.buildVectorIndicesFactory,
		indexConfig.BatchSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create index flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

type indexReconcileFactories struct {
	cleanupFactory            executor.ActivityFactory[struct{}, struct{}]
	scanStateFactory          executor.ActivityFactory[struct{}, *scanworktree.Output]
	deduplicateFactory        executor.ActivityFactory[*prepareindex.Input, *prepareindex.Output]
	chunkAllFactory           executor.ActivityFactory[*splitfiles.Input, *splitfiles.Output]
	prepareBatchesFactory     executor.ActivityFactory[*buildbatches.Input, *buildbatches.Output]
	computeAllFactory         executor.ActivityFactory[*embedall.Input, *embedall.Output]
	distributeAndSaveFactory  executor.ActivityFactory[*savechunks.Input, *savechunks.Output]
	finalizeSnapshotsFactory  executor.ActivityFactory[*finalizeindex.Input, struct{}]
	buildVectorIndicesFactory executor.ActivityFactory[struct{}, struct{}]
}

func (c *Container) resolveIndexConfig() (*domainindex.ResolvedConfig, error) {
	profileService, err := c.buildProfileService()
	if err != nil {
		return nil, err
	}

	indexConfigService, err := index.NewConfigService(c.ConfigService, profileService)
	if err != nil {
		return nil, fmt.Errorf("failed to create index config service: %w", err)
	}

	indexConfig, err := indexConfigService.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get index config: %w", err)
	}

	return indexConfig, nil
}

func (c *Container) buildProfileService() (*profile.Service, error) {
	providerService := provider.NewService()

	modelService, err := model.NewService(c.ConfigService, providerService)
	if err != nil {
		return nil, fmt.Errorf("failed to create model service: %w", err)
	}

	profileService, err := profile.NewService(c.ConfigService, modelService)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile service: %w", err)
	}

	return profileService, nil
}

func buildIndexReconcileFactories(
	projectStateSvc *project.StateService,
	chunkerSvc *chunker.Service,
	indexSvc *index.Service,
	indexRepoImpl *indexRepo.Repository,
	metaRepo *meta.Repository,
	embeddingsGW ports.EmbeddingsGateway,
	embeddingModel string,
	vectorIndexSvc *vector.Service,
) (*indexReconcileFactories, error) {
	cleanupFactory, err := pruneindex.NewFactory(indexRepoImpl, metaRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to create cleanup factory: %w", err)
	}

	scanStateFactory, err := scanworktree.NewFactory(projectStateSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create scan state factory: %w", err)
	}

	deduplicateFactory, err := prepareindex.NewFactory(indexSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create deduplicate factory: %w", err)
	}

	chunkFileFactory, err := splitfile.NewFactory(chunkerSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunk file factory: %w", err)
	}

	chunkAllFactory, err := splitfiles.NewFactory(chunkFileFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunk all factory: %w", err)
	}

	prepareBatchesFactory, err := buildbatches.NewFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create prepare batches factory: %w", err)
	}

	computeBatchFactory, err := embedbatch.NewFactory(embeddingsGW, embeddingModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute batch factory: %w", err)
	}

	computeAllFactory, err := embedall.NewFactory(computeBatchFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute all factory: %w", err)
	}

	saveDocVersionFactory, err := savefileversion.NewFactory(indexSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create save document version factory: %w", err)
	}

	distributeAndSaveFactory, err := savechunks.NewFactory(saveDocVersionFactory, indexRepoImpl)
	if err != nil {
		return nil, fmt.Errorf("failed to create distribute and save factory: %w", err)
	}

	finalizeSnapshotsFactory, err := finalizeindex.NewFactory(indexSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create finalize snapshots factory: %w", err)
	}

	buildVectorIndicesFactory, err := buildindexes.NewFactory(vectorIndexSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create build vector indices factory: %w", err)
	}

	return &indexReconcileFactories{
		cleanupFactory:            cleanupFactory,
		scanStateFactory:          scanStateFactory,
		deduplicateFactory:        deduplicateFactory,
		chunkAllFactory:           chunkAllFactory,
		prepareBatchesFactory:     prepareBatchesFactory,
		computeAllFactory:         computeAllFactory,
		distributeAndSaveFactory:  distributeAndSaveFactory,
		finalizeSnapshotsFactory:  finalizeSnapshotsFactory,
		buildVectorIndicesFactory: buildVectorIndicesFactory,
	}, nil
}

// CreateSearchFlow creates a complete search flow with all dependencies.
func (c *Container) CreateSearchFlow() (executor.Flow, error) {
	searchActivityFactory, err := c.buildSearchActivityFactory()
	if err != nil {
		return nil, err
	}

	searchFlowFactory, err := searchFlow.NewFactory(
		searchActivityFactory,
		c.CommandService,
		c.OutputService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create search flow factory: %w", err)
	}

	return searchFlowFactory.NewFlow(), nil
}

// CreateRunFlow creates the run workflow with all dependencies.
func (c *Container) CreateRunFlow() (executor.Flow, error) {
	common, err := c.buildCommonFlowServices()
	if err != nil {
		return nil, err
	}

	agentConfigService, err := agentcore.NewService(c.ConfigService)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent config service: %w", err)
	}

	stepFactory, err := agentloop.NewFactory(common.invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent step factory: %w", err)
	}

	flowFactory, err := agentFlow.NewFactory(
		agentConfigService,
		stepFactory,
		c.CommandService,
		common.profileService,
		c.OutputService,
		workspace.NewService(c.CommandService),
		common.invokeLLMFactory,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create run flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

func (c *Container) buildSearchActivityFactory() (executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output], error) {
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

	embeddingsGW, err := gatewayFactory.NewEmbeddingsGateway(c.ShutdownService.Context(), indexConfig.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings gateway: %w", err)
	}

	retrievalSvc, err := retrieval.NewService(embeddingsGW, vectorSearchSvc, indexRepoImpl, indexConfig.Profile.Model, domainGateway.RetrievalQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to create retrieval service: %w", err)
	}

	searchActivityFactory, err := queryactivity.NewFactory(retrievalSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create search activity factory: %w", err)
	}

	return searchActivityFactory, nil
}

// CreateAskFlow creates a complete ask flow with all dependencies.
func (c *Container) CreateAskFlow() (executor.Flow, error) {
	// Initialize database and repositories
	if err := c.initDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	metaRepo := meta.NewRepository(c.dbHost)
	indexRepoImpl := indexRepo.NewRepository(c.dbHost)

	cfg, err := c.loadAnswerConfig()
	if err != nil {
		return nil, err
	}

	if err := c.ensureAnswerSystemPrompt(cfg); err != nil {
		return nil, err
	}

	indexConfig, err := c.resolveIndexConfig()
	if err != nil {
		return nil, err
	}

	profileService, err := c.buildProfileService()
	if err != nil {
		return nil, err
	}

	retrieveContextFactory, invokeLLMFactory, err := c.buildAnswerFactories(metaRepo, indexRepoImpl, indexConfig)
	if err != nil {
		return nil, err
	}

	// Create answer flow factory with resolved parameters
	askFlowFactory, err := askFlow.NewFactory(
		retrieveContextFactory,
		invokeLLMFactory,
		c.CommandService,
		profileService,
		c.OutputService,
		c.ConfigService,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create answer flow factory: %w", err)
	}

	return askFlowFactory.NewFlow(), nil
}

func (c *Container) loadAnswerConfig() (*config.Config, error) {
	cfg, err := c.ConfigService.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	if cfg.Answer == nil {
		return nil, fmt.Errorf("answer configuration is missing")
	}
	return cfg, nil
}

func (c *Container) ensureAnswerSystemPrompt(cfg *config.Config) error {
	systemPrompt := cfg.Answer.SystemPrompt
	if flagPrompt, err := c.CommandService.GetSystemPromptFlag(); err == nil && flagPrompt != "" {
		systemPrompt = flagPrompt
	}
	if systemPrompt == "" {
		return fmt.Errorf("system prompt is required (set in config answer.systemPrompt or via --system-prompt flag)")
	}
	return nil
}

func (c *Container) buildAnswerFactories(
	metaRepo *meta.Repository,
	indexRepoImpl *indexRepo.Repository,
	indexConfig *domainindex.ResolvedConfig,
) (
	retrieveFactory executor.ActivityFactory[*fetchcontext.Input, *fetchcontext.Output],
	invokeFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output],
	err error,
) {
	vectorSearchSvc, err := vector.NewSearchService(metaRepo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create vector search service: %w", err)
	}

	gatewayFactory, err := gateway.NewFactory(
		c.GetRateLimitRepo(),
		c.GetCacheRepo(),
		c.CommandService,
		c.TraceLogger,
		c.CommandService,
		c.GetHTTPClientService(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gateway factory: %w", err)
	}

	embeddingsGW, err := gatewayFactory.NewEmbeddingsGateway(c.ShutdownService.Context(), indexConfig.Profile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create embeddings gateway: %w", err)
	}

	retrievalSvc, err := retrieval.NewService(embeddingsGW, vectorSearchSvc, indexRepoImpl, indexConfig.Profile.Model, domainGateway.RetrievalQuery)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create retrieval service: %w", err)
	}

	retrieveContextFactory, err := fetchcontext.NewFactory(retrievalSvc)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create retrieve context factory: %w", err)
	}

	invokeLLMFactory, err := draftcontent.NewFactory(gatewayFactory)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create invoke LLM factory: %w", err)
	}

	return retrieveContextFactory, invokeLLMFactory, nil
}
