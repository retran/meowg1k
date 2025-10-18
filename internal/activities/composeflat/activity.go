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

// Package composeflat implements a generic activity for composing text using an LLM with file context.
package composeflat

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/domain/git"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ComposeFlat activity.
type Input struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	Changes      []*git.FileChange
	Intent       string // Optional developer intent
}

// Output defines the output structure for the ComposeFlat activity.
type Output struct {
	Content string // Generic content output (commit message or PR description)
}

// Factory creates instances of the ComposeFlat activity with injected dependencies.
type Factory struct {
	contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
	activityName                     string // For progress messages
	contentType                      string // For error messages (e.g., "commit message", "PR description")
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ComposeFlat activity factory with the provided content generation activity factory.
// activityName is used in progress messages (e.g., "Composing commit message using flat strategy")
// contentType is used in error messages (e.g., "failed to generate commit message")
func NewFactory(
	contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output],
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

		executorCtx.SendRunning(f.activityName)

		// Estimate token count (rough approximation: 1 token ≈ 4 characters)
		var totalChars int
		for _, change := range input.Changes {
			totalChars += len(change.Change)
		}
		estimatedTokens := totalChars / 4

		// Check if the estimated tokens exceed the profile's max input tokens
		if input.Profile.MaxInputTokens > 0 && estimatedTokens > input.Profile.MaxInputTokens {
			return nil, fmt.Errorf(
				"diff is too large for 'flat' strategy: estimated %d tokens exceeds profile limit of %d tokens. Consider using 'summarize' strategy instead",
				estimatedTokens,
				input.Profile.MaxInputTokens,
			)
		}

		// Build the content with full diffs
		var contentBuilder strings.Builder
		contentBuilder.WriteString("Git Diff:\n\n")

		for _, change := range input.Changes {
			contentBuilder.WriteString(fmt.Sprintf("File: %s\n", change.Filename))
			contentBuilder.WriteString("```diff\n")
			contentBuilder.WriteString(change.Change)
			contentBuilder.WriteString("\n```\n\n")
		}

		if input.Intent != "" {
			contentBuilder.WriteString(fmt.Sprintf("\nDeveloper Intent: %s\n", input.Intent))
		}

		content := contentBuilder.String()

		contentGenerationActivity := f.contentGenerationActivityFactory.NewActivity()

		invokeInput := &invokellm.Input{
			Profile:      input.Profile,
			SystemPrompt: input.SystemPrompt,
			UserPrompt:   content,
		}

		invokeFuture := executor.ExecuteActivity[*invokellm.Input, *invokellm.Output](
			executorCtx.GetExecutor(),
			ctx,
			executorCtx,
			"GenerateContent",
			contentGenerationActivity,
			invokeInput,
		)
		invokeOutput, err := invokeFuture.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to generate %s: %w", f.contentType, err)
		}

		executorCtx.SendCompleted("")

		return &Output{
			Content: invokeOutput.Content,
		}, nil
	}
}
