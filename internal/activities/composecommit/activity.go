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

// Package composecommit provides the activity for composing commit messages using summarized changes.
package composecommit

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/activities/summarizefile"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ComposeCommit activity.
type Input struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	Summaries    []*summarizefile.Output
	Intent       string // Optional developer intent
}

// Output defines the output structure for the ComposeCommit activity.
type Output struct {
	CommitMessage string
}

// Factory creates instances of the ComposeCommit activity with injected dependencies.
type Factory struct {
	invokeLLMFactory *invokellm.Factory
}

// NewFactory creates a new ComposeCommit activity factory with injected services.
func NewFactory(gatewayFactory gateway.Factory) *Factory {
	return &Factory{
		invokeLLMFactory: invokellm.NewFactory(gatewayFactory),
	}
}

// NewActivity creates and returns the ComposeCommit activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		if activityInput == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		executorCtx.SendRunning("Composing commit message")

		input, ok := activityInput.(*Input)
		if !ok {
			return nil, fmt.Errorf("%w: %T", executor.ErrInvalidInputType, activityInput)
		}

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

		invokeLLM := f.invokeLLMFactory.NewActivity()

		invokeInput := &invokellm.Input{
			Profile:      input.Profile,
			SystemPrompt: input.SystemPrompt,
			UserPrompt:   content,
		}

		invokeFuture := executorCtx.GetExecutor().RunActivity(ctx, executorCtx, "InvokeLLM", invokeLLM, invokeInput)
		invokeResult, err := invokeFuture.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to generate commit message: %w", err)
		}

		invokeOutput, ok := invokeResult.(*invokellm.Output)
		if !ok {
			return nil, fmt.Errorf("%w: expected *invokellm.Output, got %T", executor.ErrInvalidOutputType, invokeResult)
		}

		executorCtx.SendCompleted("")

		return &Output{
			CommitMessage: invokeOutput.Content,
		}, nil
	}
}
