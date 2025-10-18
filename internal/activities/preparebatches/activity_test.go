// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package preparebatches

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/retran/meowg1k/internal/activities/chunkallfiles"
	"github.com/retran/meowg1k/pkg/executor"
)

func TestNewFactory(t *testing.T) {
	factory, err := NewFactory()
	if err != nil {
		t.Fatalf("expected no error, but got %v", err)
	}
	if factory == nil {
		t.Fatal("factory should not be nil")
	}
}

func TestActivity(t *testing.T) {
	exec := executor.NewExecutor(0)
	ctx := context.Background()
	executorCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	factory, err := NewFactory()
	if err != nil {
		t.Fatalf("expected no error, but got %v", err)
	}

	tests := []struct {
		name                string
		chunkResults        *chunkallfiles.Output
		batchSize           int
		expectedNumBatches  int
		expectedLastBatch   Batch
		expectError         bool
		expectedErrorSubstr string
	}{
		{
			name: "should process chunks and create batches",
			chunkResults: &chunkallfiles.Output{
				AllChunkTexts: []string{"chunk1", "chunk2", "chunk3", "chunk4", "chunk5"},
			},
			batchSize:          3,
			expectedNumBatches: 2,
			expectedLastBatch:  Batch{StartIndex: 3, EndIndex: 5, Texts: []string{"chunk4", "chunk5"}},
		},
		{
			name: "should handle single chunk per batch",
			chunkResults: &chunkallfiles.Output{
				AllChunkTexts: []string{"chunk1", "chunk2", "chunk3"},
			},
			batchSize:          1,
			expectedNumBatches: 3,
			expectedLastBatch:  Batch{StartIndex: 2, EndIndex: 3, Texts: []string{"chunk3"}},
		},
		{
			name: "should handle empty chunks",
			chunkResults: &chunkallfiles.Output{
				AllChunkTexts: []string{},
			},
			batchSize:          3,
			expectedNumBatches: 0,
		},
		{
			name: "should handle nil chunks",
			chunkResults: &chunkallfiles.Output{
				AllChunkTexts: nil,
			},
			batchSize:          3,
			expectedNumBatches: 0,
		},
		{
			name: "should handle large batch size",
			chunkResults: &chunkallfiles.Output{
				AllChunkTexts: []string{"chunk1", "chunk2"},
			},
			batchSize:          10,
			expectedNumBatches: 1,
			expectedLastBatch:  Batch{StartIndex: 0, EndIndex: 2, Texts: []string{"chunk1", "chunk2"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			activity := factory.NewActivity()

			input := &Input{
				ChunkResults: tc.chunkResults,
				BatchSize:    tc.batchSize,
				StateName:    "test_state",
			}

			output, err := activity(ctx, executorCtx, input)

			if tc.expectError {
				if err == nil {
					t.Fatal("expected an error, but got none")
				}
				if tc.expectedErrorSubstr != "" && !strings.Contains(err.Error(), tc.expectedErrorSubstr) {
					t.Errorf("expected error to contain '%s', but got '%s'", tc.expectedErrorSubstr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, but got %v", err)
			}

			if output == nil {
				t.Fatal("output should not be nil")
			}

			if output.StateName != "test_state" {
				t.Errorf("expected state name 'test_state', got '%s'", output.StateName)
			}
			if len(output.Batches) != tc.expectedNumBatches {
				t.Fatalf("expected %d batches, but got %d", tc.expectedNumBatches, len(output.Batches))
			}
			if tc.expectedNumBatches > 0 {
				lastBatch := output.Batches[len(output.Batches)-1]
				if !reflect.DeepEqual(lastBatch, tc.expectedLastBatch) {
					t.Errorf("last batch does not match expectation.\nExpected: %+v\nGot:      %+v", tc.expectedLastBatch, lastBatch)
				}
			}
		})
	}
}
