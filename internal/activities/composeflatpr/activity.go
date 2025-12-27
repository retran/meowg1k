// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package composeflatpr implements an activity that generates pull request descriptions from a flat list of file changes.
package composeflatpr

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/composeflat"
	"github.com/retran/meowg1k/internal/activities/generatecontent"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ComposeFlatPR activity.
type Input struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	Intent       string
	Changes      []*git.FileChange
}

// Output defines the output structure for the ComposeFlatPR activity.
type Output struct {
	Description string
}

// Factory creates instances of the ComposeFlatPR activity with injected dependencies.
type Factory struct {
	genericFactory *composeflat.Factory
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ComposeFlatPR activity factory with the provided content generation activity factory.
func NewFactory(contentGenerationActivityFactory executor.ActivityFactory[*generatecontent.Input, *generatecontent.Output]) (*Factory, error) {
	genericFactory, err := composeflat.NewFactory(
		contentGenerationActivityFactory,
		"Composing PR description",
		"pull request description",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create generic factory: %w", err)
	}

	return &Factory{
		genericFactory: genericFactory,
	}, nil
}

// NewActivity creates and returns the ComposeFlatPR activity function with added progress reporting.
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

		// Convert output back to PR-specific format
		return &Output{
			Description: genericOutput.Content,
		}, nil
	}
}
