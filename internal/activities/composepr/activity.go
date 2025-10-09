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

// Package composepr provides the activity for composing Pull Request descriptions using summarized changes.
package composepr

import (
	"context"
	"fmt"
	"strings"

	"github.com/retran/meowg1k/internal/activities/invokellm"
	"github.com/retran/meowg1k/internal/activities/summarizefile"
	"github.com/retran/meowg1k/internal/core/profile"
	"github.com/retran/meowg1k/pkg/executor"
)

// Input defines the input structure for the ComposePR activity.
type Input struct {
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	Summaries    []*summarizefile.Output
	Intent       string // Optional developer intent
}

// Output defines the output structure for the ComposePR activity.
type Output struct {
	PRDescription string
}

// Factory creates instances of the ComposePR activity with injected dependencies.
type Factory struct {
	contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new ComposePR activity factory with the provided content generation activity factory.
func NewFactory(contentGenerationActivityFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]) (*Factory, error) {
	if contentGenerationActivityFactory == nil {
		return nil, fmt.Errorf("content generation activity factory cannot be nil")
	}

	return &Factory{
		contentGenerationActivityFactory: contentGenerationActivityFactory,
	}, nil
}

// NewActivity creates and returns the ComposePR activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			return nil, fmt.Errorf("compose PR factory is nil")
		}

		if input == nil {
			return nil, fmt.Errorf("input cannot be nil")
		}

		executorCtx.SendRunning("Composing Pull Request description")

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

		invokeInput := &invokellm.Input{
			Profile:      input.Profile,
			SystemPrompt: input.SystemPrompt,
			UserPrompt:   content,
		}

		invokeFuture := executor.RunActivity[*invokellm.Input, *invokellm.Output](
			executorCtx.GetExecutor(),
			ctx,
			executorCtx,
			"GenerateContent",
			contentGenerationActivity,
			invokeInput,
		)

		invokeOutput, err := invokeFuture.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to generate PR description: %w", err)
		}

		executorCtx.SendCompleted("")

		return &Output{
			PRDescription: invokeOutput.Content,
		}, nil
	}
}
