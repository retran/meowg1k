// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package task defines domain types for predefined tasks with prompts and configurations.
package task

import (
	"github.com/retran/meowg1k/internal/domain/profile"
)

// ResolvedConfig represents a resolved task configuration.
type ResolvedConfig struct {
	Name         string
	Profile      *profile.ResolvedProfile
	SystemPrompt string
	UserPrompt   string
}
