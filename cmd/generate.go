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
	"io"
	"os"

	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/generate"
	utilsio "github.com/retran/meowg1k/internal/utils/io"
	"github.com/retran/meowg1k/internal/utils/ui"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen", "g"},
	Short:   "Generate any content based on input — code, text, or docs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenerate(cmd)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringP("task", "t", "", "Run a predefined task from config")
	generateCmd.Flags().StringP("user-prompt", "p", "", "User prompt for generation. Can be combined with stdin")
}

// runGenerate executes the main logic of the generate command.
func runGenerate(cmd *cobra.Command) error {
	factory := gateway.NewGatewayFactory()
	resolver := generate.NewResolver(appConfig)

	params, err := resolver.ResolveParams(cmd)
	if err != nil {
		return err
	}

	gw, err := factory.CreateGenerationGateway(cmd.Context(), params.Profile.Provider, params.Profile.BaseURL, params.Profile.APIKey)
	if err != nil {
		return err
	}

	service := generate.NewService(gw)

	content, err := ui.RunWithSpinnerWithMessage(func() (string, error) {
		return service.Generate(cmd.Context(), params)
	}, "Generating content...")
	if err != nil {
		return err
	}

	_, err = io.WriteString(os.Stdout, utilsio.FinalizeOutput(content))
	if err != nil {
		return fmt.Errorf("failed to write response to stdout: %w", err)
	}

	return nil
}
