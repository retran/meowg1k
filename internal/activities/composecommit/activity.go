// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package composecommit implements an activity that generates commit messages using an LLM.
package composecommit

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/activities/summarizefile"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ComposeCommit activity.
type Input struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	Intent       string
	Summaries    []*summarizefile.Output
}

// Output defines the output structure for the ComposeCommit activity.
type Output struct {
	CommitMessage string
}

// Factory creates instances of the ComposeCommit activity with injected dependencies.
type Factory struct {
	contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ComposeCommit activity factory with the provided content generation activity factory.
func NewFactory(contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]) (*Factory, error) {
	if contentGenerationActivityFactory == nil {
		return nil, fmt.Errorf("content generation activity factory cannot be nil")
	}

	return &Factory{
		contentGenerationActivityFactory: contentGenerationActivityFactory,
	}, nil
}

// NewActivity creates and returns the ComposeCommit activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			return nil, fmt.Errorf("compose commit factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		executorCtx.SendRunning("Composing commit message")

		var contentBuilder strings.Builder
		contentBuilder.WriteString("File Change Summaries:\n\n")

		for _, summary := range input.Summaries {
			if summary.Skipped {
				contentBuilder.WriteString(fmt.Sprintf("- %s: (skipped)\n", summary.Filename))
			} else {
				contentBuilder.WriteString(fmt.Sprintf("- %s: %s\n", summary.Filename, summary.Summary))
			}
		}

		if input.Intent != "" {
			contentBuilder.WriteString(fmt.Sprintf("\nDeveloper Intent: %s\n", input.Intent))
		}

		content := contentBuilder.String()

		contentGenerationActivity := f.contentGenerationActivityFactory.NewActivity()
		exec := executorCtx.GetExecutor()
		if exec == nil {
			return nil, fmt.Errorf("executor not available in context")
		}

		invokeInput := &invokellm.Input{
			Profile:      input.Profile,
			SystemPrompt: input.SystemPrompt,
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
			return nil, fmt.Errorf("failed to generate commit message: %w", err)
		}

		executorCtx.SendCompleted("")

		return &Output{
			CommitMessage: invokeOutput.Content,
		}, nil
	}
}
