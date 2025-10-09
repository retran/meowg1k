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

// Package fetchalldiffs contains the parent activity to fetch diffs for multiple files in parallel.
package fetchalldiffs

import (
	"context"
	"errors"
	"fmt"

	"github.com/retran/meowg1k/internal/activities/fetchfilediff"
	"github.com/retran/meowg1k/internal/services/git"
	"github.com/retran/meowg1k/pkg/executor"
	"github.com/retran/meowg1k/pkg/future"
)

// ErrFileDiffActivityFactoryIsNil indicates that the fileDiffActivityFactory is nil.
var ErrFileDiffActivityFactoryIsNil = errors.New("fileDiffActivityFactory is nil")

// Input defines the input structure for the FetchAllDiffs activity.
type Input struct {
	Files []string
}

// Output defines the output structure for the FetchAllDiffs activity.
type Output struct {
	Changes []*git.FileChange
}

// FileDiffActivityFactory creates activities that fetch file diffs.
type FileDiffActivityFactory interface {
	NewActivity() executor.Activity[*fetchfilediff.Input, *git.FileChange]
}

// Factory creates instances of the FetchAllDiffs activity with injected dependencies.
type Factory struct {
	fileDiffActivityFactory FileDiffActivityFactory
}

// Compile-time check to ensure Factory implements ActivityFactory interface
var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

// NewFactory creates a new FetchAllDiffs activity factory with the provided file diff activity factory.
func NewFactory(fileDiffActivityFactory FileDiffActivityFactory) (*Factory, error) {
	if fileDiffActivityFactory == nil {
		return nil, ErrFileDiffActivityFactoryIsNil
	}

	return &Factory{
		fileDiffActivityFactory: fileDiffActivityFactory,
	}, nil
}

// NewActivity creates and returns the FetchAllDiffs activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		if f == nil {
			// TODO proper error
			return nil, errors.New("factory is nil")
		}

		if input == nil {
			return nil, executor.ErrInputCannotBeNil
		}

		executorCtx.SendRunning(fmt.Sprintf("Fetching diffs for %d files", len(input.Files)))

		readChangesFutures := make([]*future.Future[*git.FileChange], 0, len(input.Files))
		for _, file := range input.Files {
			fetchFileDiff := f.fileDiffActivityFactory.NewActivity()
			fut := executor.RunActivity[*fetchfilediff.Input, *git.FileChange](
				executorCtx.GetExecutor(),
				ctx,
				executorCtx,
				file,
				fetchFileDiff,
				&fetchfilediff.Input{
					Filename: file,
				},
			)
			readChangesFutures = append(readChangesFutures, fut)
		}

		changesResults, errs := future.WaitAll(ctx, readChangesFutures...)
		for _, err := range errs {
			if err != nil {
				// TODO proper error
				return nil, fmt.Errorf("failed to read staged changes: %w", err)
			}
		}

		changes := make([]*git.FileChange, 0, len(changesResults))
		changes = append(changes, changesResults...)

		executorCtx.SendCompleted(fmt.Sprintf("Fetched %d diffs", len(changes)))

		return &Output{
			Changes: changes,
		}, nil
	}
}
