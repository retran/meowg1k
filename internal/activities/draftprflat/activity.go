// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package draftprflat implements an activity that generates pull request descriptions from a flat list of file changes.
package draftprflat

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/draftcontent"
	"github.com/retran/meowg1k/internal/activities/draftflat"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ComposeFlatPR activity.
type Input struct {
	Preset       *preset.ResolvedPreset
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
	genericFactory *draftflat.Factory
}

// Compile-time check to ensure Factory implements ActivityFactory interface.
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ComposeFlatPR activity factory with the provided content generation activity factory.
func NewFactory(contentGenerationActivityFactory executor.ActivityFactory[*draftcontent.Input, *draftcontent.Output]) (*Factory, error) {
	genericFactory, err := draftflat.NewFactory(
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

		genericInput := &draftflat.Input{
			Preset:       input.Preset,
			SystemPrompt: input.SystemPrompt,
			Changes:      input.Changes,
			Intent:       input.Intent,
		}

		genericOutput, err := genericActivity(ctx, executorCtx, genericInput)
		if err != nil {
			return nil, err
		}

		return &Output{
			Description: genericOutput.Content,
		}, nil
	}
}
