package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chdir into a temp dir for isolated filesystem ops
func withTempDir(t *testing.T) func() {
	t.Helper()
	oldwd, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	return func() { _ = os.Chdir(oldwd) }
}

func TestUpdateGitignore_AppendsSentinelOnce(t *testing.T) {
	restore := withTempDir(t)
	defer restore()

	// No .gitignore: should create and add sentinel block
	if err := updateGitignore(); err != nil {
		t.Fatalf("updateGitignore failed: %v", err)
	}
	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("read .gitignore failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# meowg1k") {
		t.Fatalf("sentinel not found in .gitignore")
	}

	// Call again: should not duplicate
	if err := updateGitignore(); err != nil {
		t.Fatalf("second updateGitignore failed: %v", err)
	}
	data2, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("read .gitignore failed: %v", err)
	}
	content2 := string(data2)
	if strings.Count(content2, "# meowg1k") != 1 {
		t.Fatalf("sentinel duplicated in .gitignore")
	}
}

func TestUpdateGitignore_PreservesTrailingNewline(t *testing.T) {
	restore := withTempDir(t)
	defer restore()

	// Pre-existing file without trailing newline
	if err := os.WriteFile(".gitignore", []byte("node_modules"), 0644); err != nil {
		t.Fatalf("write .gitignore failed: %v", err)
	}
	if err := updateGitignore(); err != nil {
		t.Fatalf("updateGitignore failed: %v", err)
	}
	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatalf("read .gitignore failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "node_modules\n") {
		t.Fatalf("existing content not preserved with newline")
	}
	if !strings.Contains(content, "# meowg1k") {
		t.Fatalf("sentinel not appended")
	}
}

func TestInitGlobalConfig(t *testing.T) {
	t.Run("creates global config successfully", func(t *testing.T) {
		// Setup: Use a temp home directory
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)

		cmd := &cobra.Command{}
		cmd.SetOut(bytes.NewBuffer(nil))

		err := initGlobalConfig(cmd, false)
		require.NoError(t, err)

		// Verify file was created
		configPath := filepath.Join(tempHome, ".config", "meowg1k", "init.star")
		_, err = os.Stat(configPath)
		require.NoError(t, err, "init.star should exist")

		// Verify content
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "meow.provider")
	})

	t.Run("fails when config exists without force", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)

		// Create existing config
		configDir := filepath.Join(tempHome, ".config", "meowg1k")
		os.MkdirAll(configDir, 0755)
		initFile := filepath.Join(configDir, "init.star")
		os.WriteFile(initFile, []byte("existing"), 0644)

		cmd := &cobra.Command{}
		cmd.SetOut(bytes.NewBuffer(nil))

		err := initGlobalConfig(cmd, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("overwrites config with force flag", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)

		// Create existing config
		configDir := filepath.Join(tempHome, ".config", "meowg1k")
		os.MkdirAll(configDir, 0755)
		initFile := filepath.Join(configDir, "init.star")
		os.WriteFile(initFile, []byte("old content"), 0644)

		cmd := &cobra.Command{}
		cmd.SetOut(bytes.NewBuffer(nil))

		err := initGlobalConfig(cmd, true)
		require.NoError(t, err)

		// Verify content was overwritten
		content, err := os.ReadFile(initFile)
		require.NoError(t, err)
		assert.NotContains(t, string(content), "old content")
		assert.Contains(t, string(content), "meow.provider")
	})
}

func TestInitProjectConfig(t *testing.T) {
	t.Run("creates project config successfully", func(t *testing.T) {
		restore := withTempDir(t)
		defer restore()

		// Create a .git directory to simulate git repo
		os.Mkdir(".git", 0755)

		cmd := &cobra.Command{}
		cmd.SetOut(bytes.NewBuffer(nil))

		err := initProjectConfig(cmd, false)
		require.NoError(t, err)

		// Verify directory and files were created
		_, err = os.Stat(".meowg1k")
		require.NoError(t, err, ".meowg1k directory should exist")

		_, err = os.Stat(".meowg1k/init.star")
		require.NoError(t, err, "init.star should exist")

		// Verify gitignore was updated
		gitignoreContent, err := os.ReadFile(".gitignore")
		require.NoError(t, err)
		assert.Contains(t, string(gitignoreContent), "# meowg1k")
	})

	t.Run("fails when project config exists without force", func(t *testing.T) {
		restore := withTempDir(t)
		defer restore()

		// Create existing project config
		os.MkdirAll(".meowg1k", 0755)
		os.WriteFile(".meowg1k/init.star", []byte("existing"), 0644)

		cmd := &cobra.Command{}
		cmd.SetOut(bytes.NewBuffer(nil))

		err := initProjectConfig(cmd, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("works in non-git directory", func(t *testing.T) {
		restore := withTempDir(t)
		defer restore()

		cmd := &cobra.Command{}
		buf := bytes.NewBuffer(nil)
		cmd.SetOut(buf)

		err := initProjectConfig(cmd, false)
		require.NoError(t, err)

		// Should show warning
		output := buf.String()
		assert.Contains(t, output, "Warning: Not in a git repository")
	})
}

func TestRunInit(t *testing.T) {
	t.Run("routes to global config with --global flag", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)

		cmd := &cobra.Command{}
		cmd.Flags().Bool("global", true, "")
		cmd.Flags().Bool("force", false, "")
		cmd.SetOut(bytes.NewBuffer(nil))

		err := runInit(cmd, nil)
		require.NoError(t, err)

		// Verify global config was created
		configPath := filepath.Join(tempHome, ".config", "meowg1k", "init.star")
		_, err = os.Stat(configPath)
		require.NoError(t, err)
	})

	t.Run("routes to project config without --global flag", func(t *testing.T) {
		restore := withTempDir(t)
		defer restore()

		cmd := &cobra.Command{}
		cmd.Flags().Bool("global", false, "")
		cmd.Flags().Bool("force", false, "")
		cmd.SetOut(bytes.NewBuffer(nil))

		err := runInit(cmd, nil)
		require.NoError(t, err)

		// Verify project config was created
		_, err = os.Stat(".meowg1k/init.star")
		require.NoError(t, err)
	})
}
