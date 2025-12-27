// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package summarizefile implements an activity that generates a summary of changes in a single file using an LLM.
package summarizefile

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/generatecontent"
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
	contentGenerationActivityFactory executor.ActivityFactory[*generatecontent.Input, *generatecontent.Output]
	summarizationConfigProvider      SummarizationConfigProvider
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new SummarizeFileChanges activity factory with the provided dependencies.
func NewFactory(
	contentGenerationActivityFactory executor.ActivityFactory[*generatecontent.Input, *generatecontent.Output],
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

		executorCtx.SendRunning(fmt.Sprintf("Summarizing changes in %s", input.Filename))

		if config == nil || config.Skip {
			executorCtx.SendCompleted(fmt.Sprintf("Skipped: %s", input.Filename))
			return buildSkippedOutput(input.Filename), nil
		}

		content := buildSummaryPrompt(input, config)

		invokeOutput, err := f.invokeLLM(ctx, executorCtx, &generatecontent.Input{
			Profile:      config.Profile,
			SystemPrompt: config.SystemPrompt,
			UserPrompt:   content,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate summary for %s: %w", input.Filename, err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("Summarized: %s", input.Filename))

		return &Output{
			Filename: input.Filename,
			Summary:  invokeOutput.Content,
			Skipped:  false,
		}, nil
	}
}

func buildSummaryPrompt(input *Input, config *summarize.ResolvedConfig) string {
	contentParts := []string{fmt.Sprintf("File: %s", input.Filename)}

	if config.IncludeOriginalFile {
		contentParts = append(contentParts, fmt.Sprintf("\nOriginal content:\n%s", input.OriginalFileContent))
	}

	if config.IncludeChangedFile {
		contentParts = append(contentParts, fmt.Sprintf("\nStaged content:\n%s", input.StagedFileContent))
	}

	contentParts = append(contentParts, fmt.Sprintf("\nDiff:\n%s", input.Change))

	return strings.Join(contentParts, "")
}

func buildSkippedOutput(filename string) *Output {
	return &Output{
		Filename: filename,
		Summary:  "",
		Skipped:  true,
	}
}

func (f *Factory) invokeLLM(ctx context.Context, executorCtx *executor.Context, input *generatecontent.Input) (*generatecontent.Output, error) {
	exec, err := requireExecutor(executorCtx)
	if err != nil {
		return nil, err
	}

	contentGenerationActivity := f.contentGenerationActivityFactory.NewActivity()
	output, err := executor.ExecuteActivity[*generatecontent.Input, *generatecontent.Output](
		ctx,
		exec,
		executorCtx,
		"GenerateContent",
		contentGenerationActivity,
		input,
	)
	if err != nil {
		return nil, fmt.Errorf("invoke LLM: %w", err)
	}
	return output, nil
}

func requireExecutor(executorCtx *executor.Context) (executor.Executor, error) {
	exec := executorCtx.GetExecutor()
	if exec == nil {
		return nil, fmt.Errorf("executor not available in context")
	}
	return exec, nil
}
