// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package draftcommit implements an activity that generates commit messages using an LLM.
package draftcommit

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/draftcontent"
	"github.com/retran/meowg1k/internal/activities/summarizefilechanges"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ComposeCommit activity.
type Input struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	Intent       string
	Summaries    []*summarizefilechanges.Output
}

// Output defines the output structure for the ComposeCommit activity.
type Output struct {
	CommitMessage string
}

// Factory creates instances of the ComposeCommit activity with injected dependencies.
type Factory struct {
	contentGenerationActivityFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ComposeCommit activity factory with the provided content generation activity factory.
func NewFactory(contentGenerationActivityFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]) (*Factory, error) {
	if contentGenerationActivityFactory == nil {
		return nil, fmt.Errorf("contentGenerationActivityFactory cannot be nil")
	}

	return &Factory{
		contentGenerationActivityFactory: contentGenerationActivityFactory,
	}, nil
}

// NewActivity creates and returns the ComposeCommit activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			return nil, fmt.Errorf("factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		executorCtx.SendRunningWithDetails("I'm drafting a commit message", fmt.Sprintf("summaries=%d", len(input.Summaries)))

		content := buildCommitPrompt(input.Summaries, input.Intent)

		invokeOutput, err := f.invokeLLM(ctx, executorCtx, &draftcontent.Input{
			Profile:      input.Profile,
			SystemPrompt: input.SystemPrompt,
			UserPrompt:   content,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to write commit message: %w", err)
		}
		if invokeOutput == nil || invokeOutput.Response == nil {
			return nil, fmt.Errorf("InvokeLLM returned nil response")
		}
		text := invokeOutput.Response.Text()

		executorCtx.SendCompletedWithDetails("I've drafted the commit message", strings.TrimSpace(text))

		return &Output{
			CommitMessage: text,
		}, nil
	}
}

func buildCommitPrompt(summaries []*summarizefilechanges.Output, intent string) string {
	var contentBuilder strings.Builder
	contentBuilder.WriteString("File Change Summaries:\n\n")

	for _, summary := range summaries {
		if summary.Skipped {
			contentBuilder.WriteString(fmt.Sprintf("- %s: (skipped)\n", summary.Filename))
		} else {
			contentBuilder.WriteString(fmt.Sprintf("- %s: %s\n", summary.Filename, summary.Summary))
		}
	}

	if intent != "" {
		contentBuilder.WriteString(fmt.Sprintf("\nDeveloper Intent: %s\n", intent))
	}

	return contentBuilder.String()
}

func (f *Factory) invokeLLM(ctx context.Context, executorCtx *executor.Context, input *draftcontent.Input) (*draftcontent.Output, error) {
	exec, err := requireExecutor(executorCtx)
	if err != nil {
		return nil, err
	}

	contentGenerationActivity := f.contentGenerationActivityFactory.NewActivity()
	output, err := executor.ExecuteActivity[*draftcontent.Input, *draftcontent.Output](
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
