// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package pr defines domain types for pull request description generation.
package pr

import (
	"github.com/retran/meowg1k/internal/domain/preset"
)

// ResolvedConfig represents the resolved configuration for generating a PR description.
type ResolvedConfig struct {
	Preset       *preset.ResolvedPreset
	Strategy     string // "summarize" or "flat"
	SystemPrompt string
}
