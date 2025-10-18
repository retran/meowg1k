// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package version

import "testing"

func TestVersionVariables(t *testing.T) {
	// Test that version variables have expected default values
	if Version != "dev" {
		t.Errorf("Expected Version to be 'dev', got '%s'", Version)
	}

	if BuildDate != "unknown" {
		t.Errorf("Expected BuildDate to be 'unknown', got '%s'", BuildDate)
	}

	if GitCommit != "unknown" {
		t.Errorf("Expected GitCommit to be 'unknown', got '%s'", GitCommit)
	}
}

func TestVersionVariablesAreStrings(t *testing.T) {
	// Test that all version variables are strings and not empty (this validates linker variables work)
	if len(Version) == 0 {
		t.Error("Version should not be empty")
	}

	if len(BuildDate) == 0 {
		t.Error("BuildDate should not be empty")
	}

	if len(GitCommit) == 0 {
		t.Error("GitCommit should not be empty")
	}
}
