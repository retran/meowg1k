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

package ports

import (
	"context"
	"database/sql"

	"github.com/retran/meowg1k/internal/core/commit"
	"github.com/retran/meowg1k/internal/core/config"
	"github.com/retran/meowg1k/internal/core/gateway"
	"github.com/retran/meowg1k/internal/core/model"
	"github.com/retran/meowg1k/internal/core/profile"
	"github.com/retran/meowg1k/internal/core/provider"
	"github.com/retran/meowg1k/internal/core/pullRequest"
	"github.com/retran/meowg1k/internal/core/summarize"
	"github.com/retran/meowg1k/internal/core/task"
	"github.com/retran/meowg1k/pkg/executor"
)

// Writer writes output to the user (used in activities).
type Writer interface {
	Print(content string) error
	PrintLine(content string) error
	Printf(format string, args ...any) error
	Flush() error
}

// OutputWriter writes output to the user (used in flows).
type OutputWriter interface {
	PrintLine(line string) error
}

// ConfigResolver reads the application configuration.
type ConfigResolver interface {
	Get() (*config.Config, error)
}

// FilePathResolver resolves the configuration file path.
type FilePathResolver interface {
	GetConfigPath() (string, error)
}

// CommitConfigProvider provides commit message configuration.
type CommitConfigProvider interface {
	GetCommitConfig() (*commit.ResolvedConfig, error)
}

// PRConfigProvider provides pull request configuration.
type PRConfigProvider interface {
	GetPRConfig() (*pullRequest.ResolvedConfig, error)
}

// TaskConfigProvider provides resolved task configuration.
type TaskConfigProvider interface {
	Get() (*task.ResolvedConfig, error)
}

// FileSummarizationConfigProvider provides summarization configuration for files.
type FileSummarizationConfigProvider interface {
	GetSummarizationConfig(filename string) (*summarize.ResolvedConfig, error)
}

// ProfileResolver resolves profile configurations.
type ProfileResolver interface {
	Get(profile profile.Profile) (*profile.ResolvedProfile, error)
}

// ModelResolver resolves model configurations.
type ModelResolver interface {
	Get(model model.Model) (*model.ResolvedModel, error)
}

// ProviderDefinitionRegistry retrieves provider definitions.
type ProviderDefinitionRegistry interface {
	Get(providerType provider.Provider) (provider.ProviderDefinition, error)
}

// CommandParametersReader reads command-line parameters and flags.
type CommandParametersReader interface {
	GetTargetBranchFlag() (string, error)
	GetBaseBranchFlag() (string, error)
	GetIntentFlag() (string, error)
	GetStdIn() (string, error)
}

// TaskParametersReader reads task parameters from command line.
type TaskParametersReader interface {
	GetTaskName() (string, error)
	GetUserPrompt() (string, error)
}

// StandardInputReader reads content from standard input.
type StandardInputReader interface {
	GetStdIn() (string, error)
}

// WorkspaceDirProvider provides the workspace directory path.
type WorkspaceDirProvider interface {
	Get() (string, error)
}

// StagedChangesReader reads staged file changes from git.
type StagedChangesReader interface {
	ReadStagedChanges(filename string) (string, error)
	ReadOriginalFileContent(filename string) (string, error)
	ReadStagedFileContent(filename string) (string, error)
}

// BranchDiffReader reads file diffs between branches.
type BranchDiffReader interface {
	GetBranchDiff(filename, targetBranch string) (string, error)
	ReadOriginalFileContent(filename string) (string, error)
	ReadStagedFileContent(filename string) (string, error)
}

// StagedFileListReader reads list of staged files from git.
type StagedFileListReader interface {
	ReadStagedFiles() ([]string, error)
}

// BranchFileListReader reads list of changed files in a branch.
type BranchFileListReader interface {
	GetChangedFilesInBranch(targetBranch string) ([]string, error)
}

// FileIgnoreChecker checks if a file should be ignored based on filter rules.
type FileIgnoreChecker interface {
	IsIgnoredFile(file string) bool
}

// GenerationGateway defines the contract for a client that generates content using an LLM.
type GenerationGateway interface {
	GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (string, error)
}

// EmbeddingsGateway defines the contract for a client that computes text embeddings
// and measures the distance between them.
type EmbeddingsGateway interface {
	ComputeEmbeddings(ctx context.Context, request *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error)
	ComputeDistance(first, second gateway.Embedding) (float64, error)
}

// Gateway defines the contract for a client that supports both content generation and embeddings.
type Gateway interface {
	GenerationGateway
	EmbeddingsGateway
}

// GenerationGatewayFactory creates generation gateways for LLM providers.
type GenerationGatewayFactory interface {
	NewGenerationGateway(ctx context.Context, profile *profile.ResolvedProfile) (GenerationGateway, error)
}

// UserPromptProvider provides the user prompt for content generation.
type UserPromptProvider interface {
	GetUserPrompt() (string, error)
}

// SystemPromptProvider provides the system prompt for content generation.
type SystemPromptProvider interface {
	GetSystemPrompt() (string, error)
}

// TaskConfigurationProvider provides task configuration.
type TaskConfigurationProvider interface {
	Get() (*task.ResolvedConfig, error)
}

// Host provides access to database connections.
type Host interface {
	GetDB() (*sql.DB, error)
	GetProjectDB() (*sql.DB, error)
	Close() error
}

// DBPathService defines the interface for determining database paths.
type DBPathService interface {
	GetMainDBPath() (string, error)
}

// Executor defines the interface for executing flows and activities.
type Executor interface {
	RunFlow(ctx context.Context, name string, flow executor.Flow) error
}
