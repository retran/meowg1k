// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"testing"
)

const versionUse = "version"

func TestVersionCommand(t *testing.T) {
	versionCmd.Run(versionCmd, []string{})

	if versionCmd.Use != versionUse {
		t.Errorf("Expected Use to be '%s', got '%s'", versionUse, versionCmd.Use)
	}

	if versionCmd.Short != "Show version info" {
		t.Errorf("Expected Short to be 'Show version info', got '%s'", versionCmd.Short)
	}

	if versionCmd.Run == nil {
		t.Error("Expected Run function to be defined")
	}
}

func TestVersionCommandInit(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == versionUse {
			found = true
			if cmd.Short != "Show version info" {
				t.Errorf("Expected Short description 'Show version info', got '%s'", cmd.Short)
			}
			break
		}
	}
	if !found {
		t.Error("Version command not found in root command")
	}
}
