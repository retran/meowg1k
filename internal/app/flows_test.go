// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/adapters/sqlite/cache"
	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
	"github.com/retran/meowg1k/internal/adapters/sqlite/ratelimit"
	"github.com/retran/meowg1k/internal/ports"
)

// mockDBHost is a test mock implementation of db.Host using in-memory SQLite.
type mockDBHost struct {
	mainDB    *sql.DB
	projectDB *sql.DB
}

func (h *mockDBHost) GetMainDB() (*sql.DB, error) {
	return h.mainDB, nil
}

func (h *mockDBHost) GetProjectDB() (*sql.DB, error) {
	return h.projectDB, nil
}

func (h *mockDBHost) Close() error {
	if err := h.mainDB.Close(); err != nil {
		return err
	}
	return h.projectDB.Close()
}

// newMockDBHost creates a new mock db.Host with in-memory databases for testing.
func newMockDBHost() (ports.Host, error) {
	mainDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	// Run migrations
	allMigrations := []migrations.Migration{}
	allMigrations = append(allMigrations, ratelimit.Migrations...)
	allMigrations = append(allMigrations, cache.Migrations...)
	if err := migrations.RunMigrations(mainDB, allMigrations); err != nil {
		mainDB.Close()
		return nil, err
	}

	projectDB := mainDB // Use same DB for both

	return &mockDBHost{
		mainDB:    mainDB,
		projectDB: projectDB,
	}, nil
}

func TestCreateCommitMsgFlow(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `models:
  gpt-35-turbo:
    provider: "openai"
    model: "gpt-3.5-turbo"
    maxInputTokens: 1000
    maxOutputTokens: 500
profiles:
  test:
    model: "gpt-35-turbo"
write:
  default:
    profile: "test"
    systemPrompt: "You are a helpful assistant"
filter:
  ignore:
    - "*.tmp"
    - ".git/**"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	cmd := &cobra.Command{
		Use: "test",
	}

	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("workspace", "", "workspace root path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")

	cmd.Flags().Set("config", configPath)

	// Create mock DB host for testing (uses in-memory database)
	dbHost, err := newMockDBHost()
	if err != nil {
		t.Fatalf("Failed to create mock DB host: %v", err)
	}
	defer dbHost.Close()

	container, err := NewTestAppContainer(cmd, dbHost)
	if err != nil {
		t.Fatalf("NewTestAppContainer returned error: %v", err)
	}

	flow, err := container.CreateCommitMsgFlow()
	if err != nil {
		t.Fatalf("CreateCommitMsgFlow returned error: %v", err)
	}
	if flow == nil {
		t.Error("CreateCommitMsgFlow returned nil")
	}

	// Note: We don't execute the flow here as it requires a full git environment
	// and proper executor context. The fact that it was created without panic is sufficient.
}

func TestCreateWriteFlow(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `models:
  gpt-35-turbo:
    provider: "openai"
    model: "gpt-3.5-turbo"
    maxInputTokens: 1000
    maxOutputTokens: 500
profiles:
  test:
    model: "gpt-35-turbo"
write:
  default:
    profile: "test"
    systemPrompt: "You are a helpful assistant"
  tasks:
    test:
      profile: "test"
      systemPrompt: "Test system prompt"
      userPrompt: "Test user prompt"
filter:
  ignore:
    - "*.tmp"
    - ".git/**"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	cmd := &cobra.Command{
		Use: "test",
	}

	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("workspace", "", "workspace root path")
	cmd.Flags().String("task", "test", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")

	cmd.Flags().Set("config", configPath)

	// Create mock DB host for testing (uses in-memory database)
	dbHost, err := newMockDBHost()
	if err != nil {
		t.Fatalf("Failed to create mock DB host: %v", err)
	}
	defer dbHost.Close()

	container, err := NewTestAppContainer(cmd, dbHost)
	if err != nil {
		t.Fatalf("NewTestAppContainer returned error: %v", err)
	}

	flow, err := container.CreateWriteFlow()
	if err != nil {
		t.Errorf("CreateWriteFlow returned error: %v", err)
	}
	if flow == nil {
		t.Error("CreateWriteFlow returned nil")
	}
}

func TestCreateWriteFlowWithUserPrompt(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `models:
  gpt-35-turbo:
    provider: "openai"
    model: "gpt-3.5-turbo"
    maxInputTokens: 1000
    maxOutputTokens: 500
profiles:
  test:
    model: "gpt-35-turbo"
write:
  default:
    profile: "test"
    systemPrompt: "You are a helpful assistant"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	cmd := &cobra.Command{
		Use: "test",
	}

	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("workspace", "", "workspace root path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "Custom prompt", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")

	cmd.Flags().Set("config", configPath)
	cmd.Flags().Set("user-prompt", "Custom prompt")

	// Create mock DB host for testing (uses in-memory database)
	dbHost, err := newMockDBHost()
	if err != nil {
		t.Fatalf("Failed to create mock DB host: %v", err)
	}
	defer dbHost.Close()

	container, err := NewTestAppContainer(cmd, dbHost)
	if err != nil {
		t.Fatalf("NewTestAppContainer returned error: %v", err)
	}

	flow, err := container.CreateWriteFlow()
	if err != nil {
		t.Errorf("CreateWriteFlow returned error: %v", err)
	}
	if flow == nil {
		t.Error("CreateWriteFlow returned nil")
	}
}

func TestCreateWriteFlowWithMissingProfile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	// Create config with missing profile reference
	configContent := `models:
  gpt-35-turbo:
    provider: "openai"
    model: "gpt-3.5-turbo"
profiles:
  test:
    model: "gpt-35-turbo"
write:
  default:
    profile: "nonexistent"
    systemPrompt: "You are a helpful assistant"
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	cmd := &cobra.Command{
		Use: "test",
	}

	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().String("workspace", "", "workspace root path")
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Bool("silent", false, "silent mode")

	cmd.Flags().Set("config", configPath)

	// Create mock DB host for testing (uses in-memory database)
	dbHost, err := newMockDBHost()
	if err != nil {
		t.Fatalf("Failed to create mock DB host: %v", err)
	}
	defer dbHost.Close()

	container, err := NewTestAppContainer(cmd, dbHost)
	if err != nil {
		t.Fatalf("NewTestAppContainer returned error: %v", err)
	}

	flow, err := container.CreateWriteFlow()
	// This should return an error because the profile doesn't exist
	if err == nil {
		t.Error("CreateWriteFlow should return error for missing profile")
	}
	if flow != nil {
		t.Error("CreateWriteFlow should return nil flow on error")
	}
}
