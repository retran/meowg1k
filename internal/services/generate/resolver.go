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
	"fmt"

	"github.com/retran/meowg1k/internal/config"
	"github.com/spf13/cobra"
)

const (
	stdinContextWrapper = "\n\n```\n%s\n```"
)

// Resolver handles parameter resolution for generation requests.
type Resolver interface {
	ResolveParams(cmd *cobra.Command) (*Params, error)
}

// resolverImpl handles parameter resolution for generation requests.
type resolverImpl struct {
	config          *config.Config
	profileResolver *config.ProfileResolver
	promptResolver  *config.PromptResolver
}

// NewResolver creates a new parameter resolver.
func NewResolver(cfg *config.Config) Resolver {
	return &resolverImpl{
		config:          cfg,
		profileResolver: config.NewProfileResolver(cfg),
		promptResolver:  config.NewPromptResolver(),
	}
}

// ResolveParams consolidates all configuration resolution into a single struct.
// It determines the final parameters for the generation request by checking flags,
// task configurations, and defaults from the loaded configuration.
func (r *resolverImpl) ResolveParams(cmd *cobra.Command) (*Params, error) {
	var task *config.GenerateTask
	var profileName string
	var systemPrompt string

	// Check if a task is specified
	taskName, _ := cmd.Flags().GetString("task")
	if taskName != "" {
		var err error
		task, err = r.config.GetGenerateTask(taskName)
		if err != nil {
			return nil, err
		}

		profileName = task.Profile
		systemPrompt = task.SystemPrompt
	}

	// If no system prompt from task, use default
	if systemPrompt == "" {
		systemPrompt = r.config.GetDefaultGenerateSystemPrompt()
	}

	// Resolve profile
	profile, err := r.profileResolver.ResolveProfile(profileName)
	if err != nil {
		return nil, err
	}

	// Resolve user prompt
	userPrompt, err := r.promptResolver.ResolvePrompt(cmd, "user-prompt", stdinContextWrapper)
	if err != nil {
		return nil, err
	}

	if userPrompt == "" {
		return nil, fmt.Errorf("no user prompt provided via the --user-prompt flag, stdin, or task configuration")
	}

	return &Params{
		Profile:      profile,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}, nil
}
