// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package summarizefile implements an activity that generates a summary of changes in a single file using an LLM.
package summarizefile

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/domain/summarize"
	"github.com/retran/meowg1k/pkg/executor"
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

// SummarizationConfigProvider provides summarization configuration for files.
type SummarizationConfigProvider interface {
	Get(filename string) (*summarize.ResolvedConfig, error)
}

// Factory creates instances of the SummarizeFileChanges activity with injected dependencies.
type Factory struct {
	contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
	summarizationConfigProvider      SummarizationConfigProvider
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new SummarizeFileChanges activity factory with the provided dependencies.
func NewFactory(
	contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output],
	summarizationConfigProvider SummarizationConfigProvider,
) (*Factory, error) {
	if contentGenerationActivityFactory == nil {
		return nil, fmt.Errorf("content generation activity factory is nil")
	}

	if summarizationConfigProvider == nil {
		return nil, fmt.Errorf("file summarization config provider is nil")
	}

	return &Factory{
		contentGenerationActivityFactory: contentGenerationActivityFactory,
		summarizationConfigProvider:      summarizationConfigProvider,
	}, nil
}

// NewActivity creates and returns the SummarizeFileChanges activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			return nil, fmt.Errorf("factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		config, err := f.summarizationConfigProvider.Get(input.Filename)
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
		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		invokeInput := &invokellm.Input{
			Profile:      config.Profile,
			SystemPrompt: config.SystemPrompt,
			UserPrompt:   content,
		}

		invokeFuture := executor.ExecuteActivity[*invokellm.Input, *invokellm.Output](
			ctx,
			exec,
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
