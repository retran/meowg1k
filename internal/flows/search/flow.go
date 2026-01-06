// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package search implements the workflow for semantic code search using vector similarity.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	queryactivity "github.com/retran/meowg1k/internal/activities/searchindex"
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

// Factory creates instances of the search flow.
type Factory struct {
	searchFactory    executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output]
	parametersReader CommandParametersReader
	outputWriter     ports.OutputWriter
}

// NewFactory creates a new search flow factory.
func NewFactory(
	searchFactory executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output],
	parametersReader CommandParametersReader,
	outputWriter ports.OutputWriter,
) (*Factory, error) {
	if searchFactory == nil {
		return nil, fmt.Errorf("search.NewFactory: searchFactory cannot be nil")
	}
	if parametersReader == nil {
		return nil, fmt.Errorf("search.NewFactory: parametersReader cannot be nil")
	}
	if outputWriter == nil {
		return nil, fmt.Errorf("search.NewFactory: outputWriter cannot be nil")
	}

	return &Factory{
		searchFactory:    searchFactory,
		parametersReader: parametersReader,
		outputWriter:     outputWriter,
	}, nil
}

// NewFlow creates and returns the search flow function.
func (f *Factory) NewFlow() executor.Flow {
	return func(ctx context.Context, flowCtx *executor.Context) error {
		params, err := f.resolveSearchParams()
		if err != nil {
			return err
		}
		flowCtx.SendRunningWithDetails(
			"I'm searching the codebase",
			fmt.Sprintf(
				"query=%q\nsnapshots=%s\ntop_k=%d\nmin_score=%.2f",
				params.queryText,
				strings.Join(params.snapshots, ","),
				params.topK,
				params.minScore,
			),
		)

		searchOutput, err := f.runSearch(ctx, flowCtx, params)
		if err != nil {
			return err
		}

		if err := f.outputResults(searchOutput.Results, params.useJSON); err != nil {
			return err
		}

		flowCtx.SendCompletedWithDetails(
			"I've finished the search",
			fmt.Sprintf("results=%d", len(searchOutput.Results)),
		)
		return nil
	}
}

type searchParams struct {
	queryText string
	snapshots []string
	topK      int
	minScore  float32
	useJSON   bool
}

func (f *Factory) resolveSearchParams() (*searchParams, error) {
	queryText, err := f.parametersReader.GetQueryTextFlag()
	if err != nil {
		return nil, fmt.Errorf("failed to get search text: %w", err)
	}
	if queryText == "" {
		return nil, fmt.Errorf("search text is required")
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

	return &searchParams{
		queryText: queryText,
		snapshots: snapshots,
		topK:      topK,
		minScore:  minScore,
		useJSON:   useJSON,
	}, nil
}

func (f *Factory) runSearch(
	ctx context.Context,
	flowCtx *executor.Context,
	params *searchParams,
) (*queryactivity.Output, error) {
	searchActivity := f.searchFactory.NewActivity()
	searchInput := &queryactivity.Input{
		QueryText:        params.queryText,
		SnapshotPriority: params.snapshots,
		TopK:             params.topK,
		MinScore:         params.minScore,
	}

	exec := flowCtx.GetExecutor()
	if exec == nil {
		return nil, fmt.Errorf("executor not available in context")
	}

	searchOutput, err := executor.ExecuteActivity(ctx, exec, flowCtx, "Search", searchActivity, searchInput)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	return searchOutput, nil
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
