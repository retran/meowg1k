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

// Package query provides flows for RAG query operations.
package query

import (
	"context"
	"encoding/json"
	"fmt"

	queryactivity "github.com/retran/meowg1k/internal/activities/query"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// CommandParametersReader reads command-line parameters and flags.
type CommandParametersReader interface {
	GetQueryTextFlag() (string, error)
	GetSnapshotsFlag() ([]string, error)
	GetTopKFlag() (int, error)
	GetMinScoreFlag() (float32, error)
	GetJsonFlag() (bool, error)
}

// Factory creates instances of the query flow.
type Factory struct {
	queryFactory     executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output]
	parametersReader CommandParametersReader
	outputWriter     ports.OutputWriter
}

// NewFactory creates a new query flow factory.
func NewFactory(
	queryFactory executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output],
	parametersReader CommandParametersReader,
	outputWriter ports.OutputWriter,
) (*Factory, error) {
	if queryFactory == nil {
		return nil, fmt.Errorf("query.NewFactory: queryFactory cannot be nil")
	}
	if parametersReader == nil {
		return nil, fmt.Errorf("query.NewFactory: parametersReader cannot be nil")
	}
	if outputWriter == nil {
		return nil, fmt.Errorf("query.NewFactory: outputWriter cannot be nil")
	}

	return &Factory{
		queryFactory:     queryFactory,
		parametersReader: parametersReader,
		outputWriter:     outputWriter,
	}, nil
}

// NewFlow creates and returns the query flow function.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		flowCtx.SendRunning("Starting query flow")

		// Read command parameters
		queryText, err := f.parametersReader.GetQueryTextFlag()
		if err != nil {
			return fmt.Errorf("failed to get query text: %w", err)
		}

		if queryText == "" {
			return fmt.Errorf("query text is required")
		}

		snapshots, err := f.parametersReader.GetSnapshotsFlag()
		if err != nil {
			return fmt.Errorf("failed to get snapshots: %w", err)
		}

		if len(snapshots) == 0 {
			// Default to searching workdir, stage, and head
			snapshots = []string{"_workdir_", "_stage_", "_head_"}
		}

		topK, err := f.parametersReader.GetTopKFlag()
		if err != nil {
			return fmt.Errorf("failed to get topK: %w", err)
		}

		if topK <= 0 {
			topK = 10 // Default
		}

		minScore, err := f.parametersReader.GetMinScoreFlag()
		if err != nil {
			return fmt.Errorf("failed to get min score: %w", err)
		}

		if minScore < 0 {
			minScore = 0.0 // Default
		}

		useJson, err := f.parametersReader.GetJsonFlag()
		if err != nil {
			return fmt.Errorf("failed to get json flag: %w", err)
		}

		// Execute query activity
		queryActivity := f.queryFactory.NewActivity()
		queryInput := &queryactivity.Input{
			QueryText:        queryText,
			SnapshotPriority: snapshots,
			TopK:             topK,
			MinScore:         minScore,
		}

		exec := flowCtx.GetExecutor()
		if exec == nil {
			return fmt.Errorf("executor not available in context")
		}

		queryFuture := executor.ExecuteActivity(exec, ctx, flowCtx, "Query", queryActivity, queryInput)
		queryOutput, err := queryFuture.Get(ctx)
		if err != nil {
			return fmt.Errorf("query failed: %w", err)
		}

		// Format and output results
		if useJson {
			// JSON output
			jsonData, err := json.MarshalIndent(queryOutput.Results, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal results to JSON: %w", err)
			}
			if err := f.outputWriter.PrintLine(string(jsonData)); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
		} else {
			// Human-readable output
			if len(queryOutput.Results) == 0 {
				if err := f.outputWriter.PrintLine("No results found."); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
			} else {
				if err := f.outputWriter.PrintLine(fmt.Sprintf("Found %d results:\n", len(queryOutput.Results))); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}

				for i, result := range queryOutput.Results {
					// Format each result
					output := fmt.Sprintf("=== Result %d (Score: %.4f) ===\n", i+1, result.Score)
					output += fmt.Sprintf("File: %s (Lines %d-%d)\n", result.FilePath, result.StartLine, result.EndLine)
					output += fmt.Sprintf("\n%s\n", result.TextContent)

					if err := f.outputWriter.PrintLine(output); err != nil {
						return fmt.Errorf("failed to write output: %w", err)
					}
				}
			}
		}

		flowCtx.SendCompleted("Query flow completed")
		return nil
	}
}
