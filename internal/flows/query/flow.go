// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package query implements the workflow for semantic code search using vector similarity.
package query

import (
	"context"
	"encoding/json"
	"fmt"

	queryactivity "github.com/retran/meowg1k/internal/activities/query"
	"github.com/retran/meowg1k/internal/core/retrieval"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// CommandParametersReader reads command-line parameters and flags.
type CommandParametersReader interface {
	GetQueryTextFlag() (string, error)
	GetSnapshotsFlag() ([]string, error)
	GetTopKFlag() (int, error)
	GetMinScoreFlag() (float32, error)
	GetJSONFlag() (bool, error)
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
		flowCtx.SendRunning("Query Flow")

		params, err := f.resolveQueryParams()
		if err != nil {
			return err
		}

		queryOutput, err := f.runQuery(ctx, flowCtx, params)
		if err != nil {
			return err
		}

		return f.outputResults(queryOutput.Results, params.useJSON)
	}
}

type queryParams struct {
	queryText string
	snapshots []string
	topK      int
	minScore  float32
	useJSON   bool
}

func (f *Factory) resolveQueryParams() (*queryParams, error) {
	queryText, err := f.parametersReader.GetQueryTextFlag()
	if err != nil {
		return nil, fmt.Errorf("failed to get query text: %w", err)
	}
	if queryText == "" {
		return nil, fmt.Errorf("query text is required")
	}

	snapshots, err := f.parametersReader.GetSnapshotsFlag()
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots: %w", err)
	}
	if len(snapshots) == 0 {
		snapshots = []string{"_workdir_", "_stage_", "_head_"}
	}

	topK, err := f.parametersReader.GetTopKFlag()
	if err != nil {
		return nil, fmt.Errorf("failed to get topK: %w", err)
	}
	if topK <= 0 {
		topK = 10
	}

	minScore, err := f.parametersReader.GetMinScoreFlag()
	if err != nil {
		return nil, fmt.Errorf("failed to get min score: %w", err)
	}
	if minScore < 0 {
		minScore = 0.0
	}

	useJSON, err := f.parametersReader.GetJSONFlag()
	if err != nil {
		return nil, fmt.Errorf("failed to get json flag: %w", err)
	}

	return &queryParams{
		queryText: queryText,
		snapshots: snapshots,
		topK:      topK,
		minScore:  minScore,
		useJSON:   useJSON,
	}, nil
}

func (f *Factory) runQuery(
	ctx context.Context,
	flowCtx *executor.Context,
	params *queryParams,
) (*queryactivity.Output, error) {
	queryActivity := f.queryFactory.NewActivity()
	queryInput := &queryactivity.Input{
		QueryText:        params.queryText,
		SnapshotPriority: params.snapshots,
		TopK:             params.topK,
		MinScore:         params.minScore,
	}

	exec := flowCtx.GetExecutor()
	if exec == nil {
		return nil, fmt.Errorf("executor not available in context")
	}

	queryOutput, err := executor.ExecuteActivity(ctx, exec, flowCtx, "Query", queryActivity, queryInput)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return queryOutput, nil
}

func (f *Factory) outputResults(results []retrieval.SearchResult, useJSON bool) error {
	if useJSON {
		jsonData, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal results to JSON: %w", err)
		}
		if err := f.outputWriter.PrintLine(string(jsonData)); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		return nil
	}

	if len(results) == 0 {
		if err := f.outputWriter.PrintLine("No results found."); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		return nil
	}

	if err := f.outputWriter.PrintLine(fmt.Sprintf("Found %d results:\n", len(results))); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	for i, result := range results {
		output := fmt.Sprintf("=== Result %d (Score: %.4f) ===\n", i+1, result.Score)
		output += fmt.Sprintf("File: %s (Lines %d-%d)\n", result.FilePath, result.StartLine, result.EndLine)
		output += fmt.Sprintf("\n%s\n", result.TextContent)

		if err := f.outputWriter.PrintLine(output); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	return nil
}
