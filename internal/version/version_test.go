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
	if Version == "" {
		t.Error("Version should not be empty")
	}

	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}

	if GitCommit == "" {
		t.Error("GitCommit should not be empty")
	}
}
