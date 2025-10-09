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

// Package summarizefile contains the activity to summarize changes in a single file.
package summarizefile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/services/summarize"
	"github.com/retran/meowg1k/pkg/executor"
)

var (
	// ErrContentGenerationActivityFactoryIsNil indicates that the contentGenerationActivityFactory is nil.
	ErrContentGenerationActivityFactoryIsNil = errors.New("contentGenerationActivityFactory is nil")
	// ErrFileSummarizationConfigProviderIsNil indicates that the fileSummarizationConfigProvider is nil.
	ErrFileSummarizationConfigProviderIsNil = errors.New("fileSummarizationConfigProvider is nil")
)

// Input defines the input structure for the SummarizeFile activity.
type Input struct {
	Filename            string
	Change              string
	OriginalFileContent string
	StagedFileContent   string
}

// Output defines the output structure for the SummarizeFile activity.
type Output struct {
	Filename string
	Summary  string
	Skipped  bool
}

// ContentGenerationActivityFactory creates content generation activities.
type ContentGenerationActivityFactory interface {
	NewActivity() executor.Activity[*invokellm.Input, *invokellm.Output]
}

// FileSummarizationConfigProvider provides summarization configuration for files.
type FileSummarizationConfigProvider interface {
	GetSummarizationConfig(filename string) (*summarize.ResolvedSummarizationConfig, error)
}

// Factory creates instances of the SummarizeFileChanges activity with injected dependencies.
type Factory struct {
	contentGenerationActivityFactory ContentGenerationActivityFactory
	fileSummarizationConfigProvider  FileSummarizationConfigProvider
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new SummarizeFileChanges activity factory with the provided dependencies.
func NewFactory(contentGenerationActivityFactory ContentGenerationActivityFactory, fileSummarizationConfigProvider FileSummarizationConfigProvider) (*Factory, error) {
	if contentGenerationActivityFactory == nil {
		return nil, ErrContentGenerationActivityFactoryIsNil
	}
	if fileSummarizationConfigProvider == nil {
		return nil, ErrFileSummarizationConfigProviderIsNil
	}
	return &Factory{
		contentGenerationActivityFactory: contentGenerationActivityFactory,
		fileSummarizationConfigProvider:  fileSummarizationConfigProvider,
	}, nil
}

// NewActivity creates and returns the SummarizeFileChanges activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			return nil, errors.New("factory is nil")
		}
		if input == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		config, err := f.fileSummarizationConfigProvider.GetSummarizationConfig(input.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to get summarization config for %s: %w", input.Filename, err)
		}

		executorCtx.SendRunning(fmt.Sprintf("Summarizing %s", input.Filename))

		if config == nil || config.Skip {
			executorCtx.SendCompleted("Skipped")
			return &Output{
				Filename: input.Filename,
				Summary:  "", // Empty summary means skip
				Skipped:  true,
			}, nil
		}

		var contentParts []string
		contentParts = append(contentParts, fmt.Sprintf("File: %s", input.Filename))

		if config.IncludeOriginalFile {
			contentParts = append(contentParts, fmt.Sprintf("\nOriginal content:\n%s", input.OriginalFileContent))
		}

		if config.IncludeChangedFile {
			contentParts = append(contentParts, fmt.Sprintf("\nStaged content:\n%s", input.StagedFileContent))
		}

		contentParts = append(contentParts, fmt.Sprintf("\nDiff:\n%s", input.Change))

		content := strings.Join(contentParts, "")

		contentGenerationActivity := f.contentGenerationActivityFactory.NewActivity()

		invokeInput := &invokellm.Input{
			Profile:      config.Profile,
			SystemPrompt: config.SystemPrompt,
			UserPrompt:   content,
		}

		invokeFuture := executor.RunActivity[*invokellm.Input, *invokellm.Output](
			executorCtx.GetExecutor(),
			ctx,
			executorCtx,
			"GenerateContent",
			contentGenerationActivity,
			invokeInput,
		)
		invokeOutput, err := invokeFuture.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to generate summary for %s: %w", input.Filename, err)
		}

		executorCtx.SendCompleted(input.Filename)

		return &Output{
			Filename: input.Filename,
			Summary:  invokeOutput.Content,
			Skipped:  false,
		}, nil
	}
}
