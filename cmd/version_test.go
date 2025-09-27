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

package cmd

import (
	"testing"
)

func TestVersionCommand(t *testing.T) {
	// Since the version command uses fmt.Printf directly to stdout,
	// we need to test it differently. Let's test that it runs without error
	// and check the command structure instead.
	
	// Just run the command to ensure it doesn't panic
	versionCmd.Run(versionCmd, []string{})

	// Test that the command has the expected properties
	if versionCmd.Use != "version" {
		t.Errorf("Expected Use to be 'version', got '%s'", versionCmd.Use)
	}

	if versionCmd.Short != "Show version info" {
		t.Errorf("Expected Short to be 'Show version info', got '%s'", versionCmd.Short)
	}

	if versionCmd.Run == nil {
		t.Error("Expected Run function to be defined")
	}
}

func TestVersionCommandInit(t *testing.T) {
	// Test that version command is properly added to root command
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "version" {
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