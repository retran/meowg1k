// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package composeflatcommit implements an activity that generates commit messages from a flat list of file changes.
package composeflatcommit

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/composeflat"
	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ComposeFlatCommit activity.
type Input struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	Changes      []*git.FileChange
	Intent       string // Optional developer intent
}

// Output defines the output structure for the ComposeFlatCommit activity.
type Output struct {
	CommitMessage string
}

// Factory creates instances of the ComposeFlatCommit activity with injected dependencies.
type Factory struct {
	genericFactory *composeflat.Factory
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ComposeFlatCommit activity factory with the provided content generation activity factory.
func NewFactory(contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]) (*Factory, error) {
	genericFactory, err := composeflat.NewFactory(
		contentGenerationActivityFactory,
		"Composing commit message",
		"commit message",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create generic factory: %w", err)
	}

	return &Factory{
		genericFactory: genericFactory,
	}, nil
}

// NewActivity creates and returns the ComposeFlatCommit activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	genericActivity := f.genericFactory.NewActivity()

	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		// Convert input to generic format
		genericInput := &composeflat.Input{
			Profile:      input.Profile,
			SystemPrompt: input.SystemPrompt,
			Changes:      input.Changes,
			Intent:       input.Intent,
		}

		// Execute generic activity
		genericOutput, err := genericActivity(ctx, executorCtx, genericInput)
		if err != nil {
			return nil, err
		}

		// Convert output back to commit-specific format
		return &Output{
			CommitMessage: genericOutput.Content,
		}, nil
	}
}
