// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"

	"github.com/retran/meowg1k/internal/activities/agentstep"
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
	"github.com/retran/meowg1k/internal/core/pullrequest"
	"github.com/retran/meowg1k/internal/core/retrieval"
	"github.com/retran/meowg1k/internal/core/summarize"
	"github.com/retran/meowg1k/internal/core/task"
	"github.com/retran/meowg1k/internal/core/vector"
	"github.com/retran/meowg1k/internal/domain/config"
	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	agentFlow "github.com/retran/meowg1k/internal/flows/agent"
	askFlow "github.com/retran/meowg1k/internal/flows/ask"
	commitFlow "github.com/retran/meowg1k/internal/flows/commit"
	"github.com/retran/meowg1k/internal/flows/generate"
	indexFlow "github.com/retran/meowg1k/internal/flows/index"
	pr "github.com/retran/meowg1k/internal/flows/pullrequest"
	queryFlow "github.com/retran/meowg1k/internal/flows/query"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// CreateCommitFlow creates a complete commit flow with all dependencies.
func (c *Container) CreateCommitFlow() (executor.Flow, error) {
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
		return nil, fmt.Errorf("failed to create commit flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

type commitServices struct {
	gitService          *git.Service
	filterService       *filter.Service
	summarizeService    *summarize.Service
	profileService      *profile.Service
	commitConfigService *commit.Service
	invokeLLMFactory    executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
}

type commitActivityFactories struct {
	listStagedActivityFactory      executor.ActivityFactory[*liststaged.Input, *liststaged.Output]
	listBranchFilesActivityFactory executor.ActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]
	fetchAllDiffsFactory           executor.ActivityFactory[*fetchalldiffs.Input, *fetchalldiffs.Output]
	fetchAllBranchDiffsFactory     executor.ActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]
	applyFiltersActivityFactory    executor.ActivityFactory[*applyfilters.Input, *applyfilters.Output]
	summarizeAllFactory            executor.ActivityFactory[*summarizeall.Input, *summarizeall.Output]
	composeCommitFactory           executor.ActivityFactory[*composecommit.Input, *composecommit.Output]
	composeFlatCommitFactory       executor.ActivityFactory[*composeflatcommit.Input, *composeflatcommit.Output]
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
	listStagedActivityFactory, err := liststaged.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create list staged activity factory: %w", err)
	}

	fetchFileDiffActivityFactory, err := fetchfilediff.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch file diff activity factory: %w", err)
	}

	fetchAllDiffsFactory, err := fetchalldiffs.NewFactory(fetchFileDiffActivityFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch all diffs factory: %w", err)
	}

	listBranchFilesActivityFactory, err := listbranchfiles.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create list branch files activity factory: %w", err)
	}

	fetchBranchFileDiffActivityFactory, err := fetchbranchfilediff.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch branch file diff activity factory: %w", err)
	}

	fetchAllBranchDiffsFactory, err := fetchallbranchdiffs.NewFactory(fetchBranchFileDiffActivityFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch all branch diffs factory: %w", err)
	}

	applyFiltersActivityFactory, err := applyfilters.NewFactory(services.filterService)
	if err != nil {
		return nil, fmt.Errorf("failed to create apply filters activity factory: %w", err)
	}

	summarizeFileFactory, err := summarizefile.NewFactory(services.invokeLLMFactory, services.summarizeService)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize file factory: %w", err)
	}

	summarizeAllFactory, err := summarizeall.NewFactory(summarizeFileFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize all factory: %w", err)
	}

	composeCommitFactory, err := composecommit.NewFactory(services.invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose commit factory: %w", err)
	}

	composeFlatCommitFactory, err := composeflatcommit.NewFactory(services.invokeLLMFactory)
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
	services, err := c.buildPullRequestServices()
	if err != nil {
		return nil, err
	}

	factories, err := buildPullRequestActivityFactories(services)
	if err != nil {
		return nil, err
	}

	flowFactory, err := pr.NewFactory(
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
		return nil, fmt.Errorf("failed to create pullrequest flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

type pullRequestServices struct {
	gitService       *git.Service
	filterService    *filter.Service
	summarizeService *summarize.Service
	profileService   *profile.Service
	prConfigService  *pullrequest.Service
	invokeLLMFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
}

type pullRequestActivityFactories struct {
	listBranchFilesActivityFactory executor.ActivityFactory[*listbranchfiles.Input, *listbranchfiles.Output]
	fetchAllBranchDiffsFactory     executor.ActivityFactory[*fetchallbranchdiffs.Input, *fetchallbranchdiffs.Output]
	applyFiltersActivityFactory    executor.ActivityFactory[*applyfilters.Input, *applyfilters.Output]
	summarizeAllFactory            executor.ActivityFactory[*summarizeall.Input, *summarizeall.Output]
	composePRFactory               executor.ActivityFactory[*composepr.Input, *composepr.Output]
	composeFlatPRFactory           executor.ActivityFactory[*composeflatpr.Input, *composeflatpr.Output]
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
	invokeLLMFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
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

	invokeLLMFactory, err := invokellm.NewFactory(gatewayFactory)
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
	listBranchFilesActivityFactory, err := listbranchfiles.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create list branch files activity factory: %w", err)
	}

	fetchBranchFileDiffActivityFactory, err := fetchbranchfilediff.NewFactory(services.gitService)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch branch file diff activity factory: %w", err)
	}

	fetchAllBranchDiffsFactory, err := fetchallbranchdiffs.NewFactory(fetchBranchFileDiffActivityFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch all branch diffs factory: %w", err)
	}

	applyFiltersActivityFactory, err := applyfilters.NewFactory(services.filterService)
	if err != nil {
		return nil, fmt.Errorf("failed to create apply filters activity factory: %w", err)
	}

	summarizeFileFactory, err := summarizefile.NewFactory(services.invokeLLMFactory, services.summarizeService)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize file factory: %w", err)
	}

	summarizeAllFactory, err := summarizeall.NewFactory(summarizeFileFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarize all factory: %w", err)
	}

	composePRFactory, err := composepr.NewFactory(services.invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose pullrequest factory: %w", err)
	}

	composeFlatPRFactory, err := composeflatpr.NewFactory(services.invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create compose flat pullrequest factory: %w", err)
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
	scanStateFactory          executor.ActivityFactory[struct{}, *scanworkspacestate.Output]
	deduplicateFactory        executor.ActivityFactory[*deduplicateandprepare.Input, *deduplicateandprepare.Output]
	chunkAllFactory           executor.ActivityFactory[*chunkallfiles.Input, *chunkallfiles.Output]
	prepareBatchesFactory     executor.ActivityFactory[*preparebatches.Input, *preparebatches.Output]
	computeAllFactory         executor.ActivityFactory[*computeallembeddings.Input, *computeallembeddings.Output]
	distributeAndSaveFactory  executor.ActivityFactory[*distributeandsave.Input, *distributeandsave.Output]
	finalizeSnapshotsFactory  executor.ActivityFactory[*finalizesnapshots.Input, struct{}]
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

	computeBatchFactory, err := computeembeddingsbatch.NewFactory(embeddingsGW, embeddingModel)
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

// CreateQueryFlow creates a complete query flow with all dependencies.
func (c *Container) CreateQueryFlow() (executor.Flow, error) {
	queryActivityFactory, err := c.buildQueryActivityFactory()
	if err != nil {
		return nil, err
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

// CreateAgentFlow creates the agent workflow with all dependencies.
func (c *Container) CreateAgentFlow() (executor.Flow, error) {
	common, err := c.buildCommonFlowServices()
	if err != nil {
		return nil, err
	}

	agentConfigService, err := agentcore.NewService(c.ConfigService)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent config service: %w", err)
	}

	stepFactory, err := agentstep.NewFactory(common.invokeLLMFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent step factory: %w", err)
	}

	queryActivityFactory, err := c.buildQueryActivityFactory()
	if err != nil {
		return nil, err
	}

	flowFactory, err := agentFlow.NewFactory(
		agentConfigService,
		stepFactory,
		c.CommandService,
		common.profileService,
		c.OutputService,
		workspace.NewService(c.CommandService),
		common.filterService,
		common.gitService,
		queryActivityFactory,
		common.invokeLLMFactory,
		c.CreateIndexReconcileFlow,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent flow factory: %w", err)
	}

	return flowFactory.NewFlow(), nil
}

func (c *Container) buildQueryActivityFactory() (executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output], error) {
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

	queryActivityFactory, err := queryactivity.NewFactory(retrievalSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create query activity factory: %w", err)
	}

	return queryActivityFactory, nil
}

// CreateAskFlow creates a complete ask flow with all dependencies.
func (c *Container) CreateAskFlow() (executor.Flow, error) {
	// Initialize database and repositories
	if err := c.initDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	metaRepo := meta.NewRepository(c.dbHost)
	indexRepoImpl := indexRepo.NewRepository(c.dbHost)

	cfg, err := c.loadAskConfig()
	if err != nil {
		return nil, err
	}

	if err := c.ensureAskSystemPrompt(cfg); err != nil {
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

	retrieveContextFactory, invokeLLMFactory, err := c.buildAskFactories(metaRepo, indexRepoImpl, indexConfig)
	if err != nil {
		return nil, err
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

func (c *Container) loadAskConfig() (*config.Config, error) {
	cfg, err := c.ConfigService.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	if cfg.Ask == nil {
		return nil, fmt.Errorf("ask configuration is missing")
	}
	return cfg, nil
}

func (c *Container) ensureAskSystemPrompt(cfg *config.Config) error {
	systemPrompt := cfg.Ask.SystemPrompt
	if flagPrompt, err := c.CommandService.GetSystemPromptFlag(); err == nil && flagPrompt != "" {
		systemPrompt = flagPrompt
	}
	if systemPrompt == "" {
		return fmt.Errorf("system prompt is required (set in config ask.systemPrompt or via --system-prompt flag)")
	}
	return nil
}

func (c *Container) buildAskFactories(
	metaRepo *meta.Repository,
	indexRepoImpl *indexRepo.Repository,
	indexConfig *domainindex.ResolvedConfig,
) (
	retrieveFactory executor.ActivityFactory[*retrievecontext.Input, *retrievecontext.Output],
	invokeFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output],
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

	retrieveContextFactory, err := retrievecontext.NewFactory(retrievalSvc)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create retrieve context factory: %w", err)
	}

	invokeLLMFactory, err := invokellm.NewFactory(gatewayFactory)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create invoke LLM factory: %w", err)
	}

	return retrieveContextFactory, invokeLLMFactory, nil
}
