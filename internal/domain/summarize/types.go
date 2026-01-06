// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package summarize defines domain types for file change summarization.
package summarize

import (
	"github.com/retran/meowg1k/internal/domain/config"
	"github.com/retran/meowg1k/internal/domain/preset"
)

// ResolvedConfig holds the resolved summarization configuration for a specific file.
type ResolvedConfig struct {
	Preset              *preset.ResolvedPreset
	Strategy            *config.StrategyConfig
	SystemPrompt        string
	Skip                bool
	IncludeOriginalFile bool
	IncludeChangedFile  bool
}
