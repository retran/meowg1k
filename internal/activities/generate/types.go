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

package generate

import (
	"github.com/retran/meowg1k/internal/models/config"
	"github.com/spf13/cobra"
)

// GenerateInput represents the input for the generate activity
type GenerateInput struct {
	// Command context from cobra
	Command *cobra.Command

	// Configuration (already loaded and validated)
	Config *config.Config

	// Input content from stdin (if any)
	StdinContent string

	// Command line flags
	TaskName   string // from -t/--task flag
	UserPrompt string // from -p/--user-prompt flag
	Silent     bool   // from --silent flag
}

// GenerateOutput represents the output from the generate activity
type GenerateOutput struct {
	// Generated content ready for output
	Content string

	// Metadata about the generation process
	Metadata map[string]interface{}
}
