// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package commit defines domain types for commit message generation.
package commit

import (
	"github.com/retran/meowg1k/internal/domain/preset"
)

// ResolvedConfig represents the resolved configuration for generating a commit message.
type ResolvedConfig struct {
	Preset       *preset.ResolvedPreset
	Strategy     string // "summarize" or "flat"
	SystemPrompt string
}
