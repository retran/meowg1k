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

// Package cmd contains the command-line interface for meowg1k.
package cmd

import (
	"fmt"

	"github.com/retran/meowg1k/internal/index"
	"github.com/retran/meowg1k/internal/llm/gateway"
	"github.com/spf13/cobra"
)

var indexCmd = &cobra.Command{
	Use:     "index",
	Aliases: []string{"i"},
	Short:   "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIndex(cmd)
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}

// runIndex executes the main logic of the index command.
func runIndex(cmd *cobra.Command) error {
	embeddingGateway, err := gateway.NewEmbeddingGateway(
		cmd.Context(),
		gateway.WithProvider(gateway.Gemini))
	if err != nil {
		return err
	}

	request := gateway.NewComputeEmbeddingRequest(
		"gemini-embedding-001",
		[]string{"Isaac Newton", "Isaac"},
		gateway.SemanticSimilarity,
	)

	embeddings, err := embeddingGateway.ComputeEmbeddings(cmd.Context(), request)
	if err != nil {
		return err
	}

	similarity, err := embeddingGateway.ComputeDistance(embeddings[0], embeddings[1])
	if err != nil {
		return err
	}

	fmt.Println(similarity)

	index.Index(cmd.Context(), ".")

	return nil
}
