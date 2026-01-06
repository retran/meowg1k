// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package draftflat implements a generic activity for composing text using an LLM with file context.
package draftflat

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/draftcontent"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ComposeFlat activity.
type Input struct {
	Preset       *preset.ResolvedPreset
	SystemPrompt string
	Intent       string
	Changes      []*git.FileChange
}

// Output defines the output structure for the ComposeFlat activity.
type Output struct {
	Content string // Generic content output (commit message or PR description)
}

// Factory creates instances of the ComposeFlat activity with injected dependencies.
type Factory struct {
	contentGenerationActivityFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]
	activityName                     string // For progress messages
	contentType                      string // For error messages (e.g., "commit message", "PR description")
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ComposeFlat activity factory with the provided content generation activity factory.
// activityName is used in progress messages (e.g., "Composing commit message using flat strategy")
// contentType is used in error messages (e.g., "failed to write commit message").
func NewFactory(
	contentGenerationActivityFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output],
	activityName string,
	contentType string,
) (*Factory, error) {
	if contentGenerationActivityFactory == nil {
		return nil, fmt.Errorf("content generation activity factory cannot be nil")
	}

	if activityName == "" {
		activityName = "Composing content using flat strategy"
	}

	if contentType == "" {
		contentType = "content"
	}

	return &Factory{
		contentGenerationActivityFactory: contentGenerationActivityFactory,
		activityName:                     activityName,
		contentType:                      contentType,
	}, nil
}

// NewActivity creates and returns the ComposeFlat activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			return nil, fmt.Errorf("compose flat factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		executorCtx.SendRunningWithDetails(
			fmt.Sprintf("I'm drafting the %s", f.contentType),
			"strategy=flat",
		)

		if err := validateTokenBudget(input.Preset, input.Changes); err != nil {
			return nil, err
		}

		content := buildFlatPrompt(input.Changes, input.Intent)

		invokeOutput, err := f.invokeLLM(ctx, executorCtx, &draftcontent.Input{
			Preset:       input.Preset,
			SystemPrompt: input.SystemPrompt,
			UserPrompt:   content,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", f.contentType, err)
		}
		if invokeOutput == nil || invokeOutput.Response == nil {
			return nil, fmt.Errorf("InvokeLLM returned nil response")
		}
		text := invokeOutput.Response.Text()

		executorCtx.SendCompletedWithDetails(
			fmt.Sprintf("I've drafted the %s", f.contentType),
			strings.TrimSpace(text),
		)

		return &Output{
			Content: text,
		}, nil
	}
}

func validateTokenBudget(resolvedPreset *preset.ResolvedPreset, changes []*git.FileChange) error {
	estimatedTokens := estimateTokenCount(changes)
	if resolvedPreset.MaxInputTokens > 0 && estimatedTokens > resolvedPreset.MaxInputTokens {
		return fmt.Errorf(
			"these changes are too large for the 'flat' strategy (estimated %d tokens, limit %d). Try the 'summarize' strategy instead",
			estimatedTokens,
			resolvedPreset.MaxInputTokens,
		)
	}
	return nil
}

func estimateTokenCount(changes []*git.FileChange) int {
	var totalChars int
	for _, change := range changes {
		totalChars += len(change.Change)
	}
	return totalChars / 4
}

func buildFlatPrompt(changes []*git.FileChange, intent string) string {
	var contentBuilder strings.Builder
	contentBuilder.WriteString("Git Diff:\n\n")

	for _, change := range changes {
		// Check if this is a rename using the RenamedFrom field
		if change.RenamedFrom != "" {
			contentBuilder.WriteString(fmt.Sprintf("File: %s (renamed from %s)\n", change.Filename, change.RenamedFrom))
		} else {
			contentBuilder.WriteString(fmt.Sprintf("File: %s\n", change.Filename))
		}

		contentBuilder.WriteString("```diff\n")
		contentBuilder.WriteString(change.Change)
		contentBuilder.WriteString("\n```\n\n")
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
