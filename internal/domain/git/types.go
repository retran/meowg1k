// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package git defines domain types for git operations and file changes.
package git

// FileChange represents a change made to a file in a git repository.
type FileChange struct {
	Filename            string
	Change              string
	OriginalFileContent string
	ChangedFileContent  string
	RenamedFrom         string
}
