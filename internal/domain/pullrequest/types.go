// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package pullrequest defines domain types for pull request description generation.
package pullrequest

import (
	"github.com/retran/meowg1k/internal/domain/profile"
)

// ResolvedConfig represents the resolved configuration for generating a PR description.
type ResolvedConfig struct {
	Profile      *profile.ResolvedProfile
	Strategy     string // "summarize" or "flat"
	SystemPrompt string
}
