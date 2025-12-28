// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCommand(t *testing.T) {
	if initCmd.Use != "init" {
		t.Errorf("Expected Use to be 'init', got '%s'", initCmd.Use)
	}

	if initCmd.Short != "Initialize a new meowg1k project configuration" {
		t.Errorf("Expected Short to be 'Initialize a new meowg1k project configuration', got '%s'", initCmd.Short)
	}

	if initCmd.RunE == nil {
		t.Error("Expected RunE function to be defined")
	}
}

func TestInitCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "init" {
			found = true
			if cmd.Short != "Initialize a new meowg1k project configuration" {
				t.Errorf("Expected Short description 'Initialize a new meowg1k project configuration', got '%s'", cmd.Short)
			}
			break
		}
	}
	if !found {
		t.Error("Init command not found in root command")
	}
}

func TestInitCommandCreatesConfig(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "meowg1k-init-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Run init command
	buf := new(bytes.Buffer)
	initCmd.SetOut(buf)
	initCmd.SetErr(buf)

	err = initCmd.RunE(initCmd, []string{})
	if err != nil {
		t.Fatalf("Failed to run init command: %v", err)
	}

	// Check if config file was created
	configPath := filepath.Join(tmpDir, ".meowg1k.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Configuration file was not created at %s", configPath)
	}

	// Check if config file contains expected content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)
	expectedStrings := []string{
		"models:",
		"gemini-flash:",
		"gemini-pro:",
		"profiles:",
		"fast:",
		"smart:",
		"write:",
		"filter:",
		"summarize:",
		"commit:",
		"pr:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("Config file does not contain expected string: %s", expected)
		}
	}

	// Check output message
	output := buf.String()
	if !strings.Contains(output, "Configuration file created") {
		t.Errorf("Expected success message in output, got: %s", output)
	}
}

func TestInitCommandFailsIfConfigExists(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "meowg1k-init-test-exists-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create existing config file
	configPath := filepath.Join(tmpDir, ".meowg1k.yaml")
	if err := os.WriteFile(configPath, []byte("existing config"), 0o644); err != nil {
		t.Fatalf("Failed to create existing config: %v", err)
	}

	// Run init command without force flag
	buf := new(bytes.Buffer)
	initCmd.SetOut(buf)
	initCmd.SetErr(buf)

	err = initCmd.RunE(initCmd, []string{})
	if err == nil {
		t.Error("Expected init command to fail when config exists, but it succeeded")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected error message about existing config, got: %v", err)
	}

	// Check that original config was not modified
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if string(content) != "existing config" {
		t.Errorf("Existing config was modified without --force flag")
	}
}

func TestInitCommandForceOverwrite(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "meowg1k-init-test-force-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create existing config file
	configPath := filepath.Join(tmpDir, ".meowg1k.yaml")
	if err := os.WriteFile(configPath, []byte("existing config"), 0o644); err != nil {
		t.Fatalf("Failed to create existing config: %v", err)
	}

	// Run init command with force flag
	buf := new(bytes.Buffer)
	initCmd.SetOut(buf)
	initCmd.SetErr(buf)
	initCmd.Flags().Set("force", "true")
	defer initCmd.Flags().Set("force", "false") // Reset for other tests

	err = initCmd.RunE(initCmd, []string{})
	if err != nil {
		t.Fatalf("Failed to run init command with force flag: %v", err)
	}

	// Check that config was overwritten
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if string(content) == "existing config" {
		t.Error("Config was not overwritten with --force flag")
	}

	// Check output message
	output := buf.String()
	if !strings.Contains(output, "Overwriting") {
		t.Errorf("Expected overwrite message in output, got: %s", output)
	}
}

func TestInitCommandFlagForce(t *testing.T) {
	flag := initCmd.Flags().Lookup("force")
	if flag == nil {
		t.Error("Expected --force flag to be defined")
		return
	}

	if flag.Shorthand != "f" {
		t.Errorf("Expected shorthand 'f', got '%s'", flag.Shorthand)
	}

	if flag.DefValue != "false" {
		t.Errorf("Expected default value 'false', got '%s'", flag.DefValue)
	}
}

func TestInitCommandWorkspaceFlag(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "meowg1k-init-test-workspace-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Don't change to temp directory - use --workspace flag instead
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Run init command with --workspace flag
	buf := new(bytes.Buffer)
	initCmd.SetOut(buf)
	initCmd.SetErr(buf)
	rootCmd.PersistentFlags().Set("workspace", tmpDir)
	defer rootCmd.PersistentFlags().Set("workspace", "") // Reset for other tests

	err = initCmd.RunE(initCmd, []string{})
	if err != nil {
		t.Fatalf("Failed to run init command with workspace flag: %v", err)
	}

	// Verify we're still in original directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if cwd != originalWd {
		t.Errorf("Working directory changed unexpectedly from %s to %s", originalWd, cwd)
	}

	// Check if config file was created in the specified workspace
	configPath := filepath.Join(tmpDir, ".meowg1k.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Configuration file was not created at %s", configPath)
	}

	// Check output message contains the correct path
	output := buf.String()
	if !strings.Contains(output, tmpDir) {
		t.Errorf("Expected output to contain workspace path %s, got: %s", tmpDir, output)
	}
}

func TestInitCommandSilentFlag(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "meowg1k-init-test-silent-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Run init command with --silent flag
	buf := new(bytes.Buffer)
	initCmd.SetOut(buf)
	initCmd.SetErr(buf)
	rootCmd.PersistentFlags().Set("silent", "true")
	defer rootCmd.PersistentFlags().Set("silent", "false") // Reset for other tests

	err = initCmd.RunE(initCmd, []string{})
	if err != nil {
		t.Fatalf("Failed to run init command with silent flag: %v", err)
	}

	// Check output is minimal (just the path)
	output := strings.TrimSpace(buf.String())
	configPath := filepath.Join(tmpDir, ".meowg1k.yaml")

	// In silent mode, output should only be the path
	// Use filepath.EvalSymlinks to handle /tmp -> /private/var/folders on macOS
	expectedPath, _ := filepath.EvalSymlinks(configPath)
	outputPath, _ := filepath.EvalSymlinks(output)
	if outputPath != expectedPath {
		t.Errorf("Expected silent output to be just the path %s, got: %s", expectedPath, outputPath)
	}

	// Should not contain verbose messages
	if strings.Contains(output, "Next steps") {
		t.Error("Silent mode should not contain 'Next steps' message")
	}
	if strings.Contains(output, "✓") {
		t.Error("Silent mode should not contain success icon")
	}
}

func TestInitCommandSilentFlagWithForce(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "meowg1k-init-test-silent-force-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create existing config file
	configPath := filepath.Join(tmpDir, ".meowg1k.yaml")
	if err := os.WriteFile(configPath, []byte("existing config"), 0o644); err != nil {
		t.Fatalf("Failed to create existing config: %v", err)
	}

	// Run init command with --silent and --force flags
	buf := new(bytes.Buffer)
	initCmd.SetOut(buf)
	initCmd.SetErr(buf)
	rootCmd.PersistentFlags().Set("silent", "true")
	initCmd.Flags().Set("force", "true")
	defer rootCmd.PersistentFlags().Set("silent", "false") // Reset for other tests
	defer initCmd.Flags().Set("force", "false")            // Reset for other tests

	err = initCmd.RunE(initCmd, []string{})
	if err != nil {
		t.Fatalf("Failed to run init command with silent and force flags: %v", err)
	}

	// In silent mode, should not see "Overwriting" message
	output := buf.String()
	if strings.Contains(output, "Overwriting") {
		t.Error("Silent mode should not contain 'Overwriting' message")
	}

	// Should only output the path
	outputTrimmed := strings.TrimSpace(output)
	// Use filepath.EvalSymlinks to handle /tmp -> /private/var/folders on macOS
	expectedPath, _ := filepath.EvalSymlinks(configPath)
	outputPath, _ := filepath.EvalSymlinks(outputTrimmed)
	if outputPath != expectedPath {
		t.Errorf("Expected silent output to be just the path %s, got: %s", expectedPath, outputPath)
	}
}
