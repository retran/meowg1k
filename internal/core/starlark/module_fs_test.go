// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// TestFSWalk tests fs.walk() function
func TestFSWalk(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(string) error
		root          string
		pattern       string
		expectedCount int
		expectError   bool
	}{
		{
			name: "walk directory without pattern",
			setup: func(dir string) error {
				if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("test"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "file2.go"), []byte("test"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "subdir", "file3.txt"), []byte("test"), 0644); err != nil {
					return err
				}
				return nil
			},
			root:          ".",
			pattern:       "",
			expectedCount: 3,
			expectError:   false,
		},
		{
			name: "walk directory with pattern",
			setup: func(dir string) error {
				if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("test"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "file2.go"), []byte("test"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "subdir", "file3.txt"), []byte("test"), 0644); err != nil {
					return err
				}
				return nil
			},
			root:          ".",
			pattern:       "*.txt",
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "walk non-existent directory",
			setup:         func(dir string) error { return nil },
			root:          "nonexistent",
			pattern:       "",
			expectedCount: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			require.NoError(t, tt.setup(tmpDir))

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			// Build args
			args := starlark.Tuple{starlark.String(tt.root)}
			kwargs := []starlark.Tuple{}
			if tt.pattern != "" {
				kwargs = append(kwargs, starlark.Tuple{
					starlark.String("pattern"),
					starlark.String(tt.pattern),
				})
			}

			result, err := runtime.fsWalk(thread, starlark.NewBuiltin("walk", nil), args, kwargs)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			list, ok := result.(*starlark.List)
			require.True(t, ok, "result should be a list")
			assert.Equal(t, tt.expectedCount, list.Len())
		})
	}
}

// TestFSStat tests fs.stat() function
func TestFSStat(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string) (string, error)
		expectError bool
		checkIsDir  bool
	}{
		{
			name: "stat file",
			setup: func(dir string) (string, error) {
				path := filepath.Join(dir, "test.txt")
				err := os.WriteFile(path, []byte("test content"), 0644)
				return "test.txt", err
			},
			expectError: false,
			checkIsDir:  false,
		},
		{
			name: "stat directory",
			setup: func(dir string) (string, error) {
				path := filepath.Join(dir, "testdir")
				err := os.MkdirAll(path, 0755)
				return "testdir", err
			},
			expectError: false,
			checkIsDir:  true,
		},
		{
			name: "stat non-existent file",
			setup: func(dir string) (string, error) {
				return "nonexistent.txt", nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path, err := tt.setup(tmpDir)
			require.NoError(t, err)

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			args := starlark.Tuple{starlark.String(path)}
			result, err := runtime.fsStat(thread, starlark.NewBuiltin("stat", nil), args, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			structVal, ok := result.(*starlarkstruct.Struct)
			require.True(t, ok, "result should be a struct")

			// Check required fields
			size, err := structVal.Attr("size")
			require.NoError(t, err)
			require.NotNil(t, size)

			mtime, err := structVal.Attr("mtime")
			require.NoError(t, err)
			require.NotNil(t, mtime)

			isDir, err := structVal.Attr("is_dir")
			require.NoError(t, err)
			require.NotNil(t, isDir)
			assert.Equal(t, starlark.Bool(tt.checkIsDir), isDir)

			mode, err := structVal.Attr("mode")
			require.NoError(t, err)
			require.NotNil(t, mode)
		})
	}
}

// TestFSListdir tests fs.listdir() function
func TestFSListdir(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(string) error
		path          string
		expectedCount int
		expectError   bool
	}{
		{
			name: "list directory",
			setup: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("test"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "file2.go"), []byte("test"), 0644); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
					return err
				}
				return nil
			},
			path:          ".",
			expectedCount: 3,
			expectError:   false,
		},
		{
			name:          "list non-existent directory",
			setup:         func(dir string) error { return nil },
			path:          "nonexistent",
			expectedCount: 0,
			expectError:   true,
		},
		{
			name: "list empty directory",
			setup: func(dir string) error {
				return os.MkdirAll(filepath.Join(dir, "empty"), 0755)
			},
			path:          "empty",
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			require.NoError(t, tt.setup(tmpDir))

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			args := starlark.Tuple{starlark.String(tt.path)}
			result, err := runtime.fsListdir(thread, starlark.NewBuiltin("listdir", nil), args, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			list, ok := result.(*starlark.List)
			require.True(t, ok, "result should be a list")
			assert.Equal(t, tt.expectedCount, list.Len())
		})
	}
}

// TestFSChmod tests fs.chmod() function
func TestFSChmod(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string) (string, error)
		mode        int
		expectError bool
	}{
		{
			name: "chmod file to 0755",
			setup: func(dir string) (string, error) {
				path := filepath.Join(dir, "test.txt")
				err := os.WriteFile(path, []byte("test"), 0644)
				return "test.txt", err
			},
			mode:        0755,
			expectError: false,
		},
		{
			name: "chmod file to 0644",
			setup: func(dir string) (string, error) {
				path := filepath.Join(dir, "test.txt")
				err := os.WriteFile(path, []byte("test"), 0755)
				return "test.txt", err
			},
			mode:        0644,
			expectError: false,
		},
		{
			name: "chmod non-existent file",
			setup: func(dir string) (string, error) {
				return "nonexistent.txt", nil
			},
			mode:        0755,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path, err := tt.setup(tmpDir)
			require.NoError(t, err)

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			args := starlark.Tuple{
				starlark.String(path),
				starlark.MakeInt(tt.mode),
			}
			result, err := runtime.fsChmod(thread, starlark.NewBuiltin("chmod", nil), args, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			success, ok := result.(starlark.Bool)
			require.True(t, ok, "result should be a bool")
			assert.True(t, bool(success))

			// Verify the mode was actually changed
			fullPath := filepath.Join(tmpDir, path)
			info, err := os.Stat(fullPath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(tt.mode), info.Mode().Perm())
		})
	}
}

// TestFSTouch tests fs.touch() function
func TestFSTouch(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string) (string, error)
		mtime       *int64
		expectError bool
		checkExists bool
	}{
		{
			name: "touch new file",
			setup: func(dir string) (string, error) {
				return "newfile.txt", nil
			},
			mtime:       nil,
			expectError: false,
			checkExists: true,
		},
		{
			name: "touch existing file",
			setup: func(dir string) (string, error) {
				path := filepath.Join(dir, "existing.txt")
				err := os.WriteFile(path, []byte("test"), 0644)
				return "existing.txt", err
			},
			mtime:       nil,
			expectError: false,
			checkExists: true,
		},
		{
			name: "touch with specific mtime",
			setup: func(dir string) (string, error) {
				return "newfile.txt", nil
			},
			mtime: func() *int64 {
				t := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
				return &t
			}(),
			expectError: false,
			checkExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path, err := tt.setup(tmpDir)
			require.NoError(t, err)

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			args := starlark.Tuple{starlark.String(path)}
			kwargs := []starlark.Tuple{}
			if tt.mtime != nil {
				kwargs = append(kwargs, starlark.Tuple{
					starlark.String("mtime"),
					starlark.MakeInt64(*tt.mtime),
				})
			}

			result, err := runtime.fsTouch(thread, starlark.NewBuiltin("touch", nil), args, kwargs)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			success, ok := result.(starlark.Bool)
			require.True(t, ok, "result should be a bool")
			assert.True(t, bool(success))

			// Verify the file exists
			if tt.checkExists {
				fullPath := filepath.Join(tmpDir, path)
				info, err := os.Stat(fullPath)
				require.NoError(t, err)
				assert.NotNil(t, info)

				// If mtime was specified, verify it
				if tt.mtime != nil {
					assert.Equal(t, *tt.mtime, info.ModTime().Unix())
				}
			}
		})
	}
}

// TestFSWalkIntegration tests fs.walk() in an integrated scenario
func TestFSWalkIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a realistic directory structure
	structure := map[string]string{
		"README.md":           "# Project",
		"main.go":             "package main",
		"go.mod":              "module test",
		"cmd/app/main.go":     "package main",
		"cmd/app/config.yaml": "version: 1",
		"internal/api.go":     "package internal",
		"internal/db.go":      "package internal",
		"test/test.go":        "package test",
		"test/fixtures.json":  "{}",
	}

	for path, content := range structure {
		fullPath := filepath.Join(tmpDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}

	runtime := NewRuntime(tmpDir)
	thread := &starlark.Thread{Name: "test"}

	// Test: Find all .go files
	args := starlark.Tuple{starlark.String(".")}
	kwargs := []starlark.Tuple{
		{starlark.String("pattern"), starlark.String("*.go")},
	}

	result, err := runtime.fsWalk(thread, starlark.NewBuiltin("walk", nil), args, kwargs)
	require.NoError(t, err)

	list, ok := result.(*starlark.List)
	require.True(t, ok)
	assert.Equal(t, 5, list.Len()) // main.go, cmd/app/main.go, internal/api.go, internal/db.go, test/test.go
}

// TestFSRead tests fs.read() function
func TestFSRead(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string) (string, string, error)
		expectError bool
	}{
		{
			name: "read file with relative path",
			setup: func(dir string) (string, string, error) {
				content := "Hello, meowg1k!"
				path := filepath.Join(dir, "test.txt")
				err := os.WriteFile(path, []byte(content), 0644)
				return "test.txt", content, err
			},
			expectError: false,
		},
		{
			name: "read file with absolute path",
			setup: func(dir string) (string, string, error) {
				content := "Absolute path content"
				path := filepath.Join(dir, "absolute.txt")
				err := os.WriteFile(path, []byte(content), 0644)
				return path, content, err
			},
			expectError: false,
		},
		{
			name: "read non-existent file",
			setup: func(dir string) (string, string, error) {
				return "nonexistent.txt", "", nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path, expectedContent, err := tt.setup(tmpDir)
			require.NoError(t, err)

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			args := starlark.Tuple{starlark.String(path)}
			result, err := runtime.fsRead(thread, starlark.NewBuiltin("read", nil), args, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			content, ok := result.(starlark.String)
			require.True(t, ok, "result should be a string")
			assert.Equal(t, expectedContent, string(content))
		})
	}
}

// TestFSWrite tests fs.write() function
func TestFSWrite(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		content     string
		expectError bool
	}{
		{
			name:        "write to simple file",
			path:        "output.txt",
			content:     "Test content",
			expectError: false,
		},
		{
			name:        "write to nested directory",
			path:        "subdir/nested/file.txt",
			content:     "Nested content",
			expectError: false,
		},
		{
			name:        "write empty content",
			path:        "empty.txt",
			content:     "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			args := starlark.Tuple{
				starlark.String(tt.path),
				starlark.String(tt.content),
			}
			result, err := runtime.fsWrite(thread, starlark.NewBuiltin("write", nil), args, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			success, ok := result.(starlark.Bool)
			require.True(t, ok, "result should be a bool")
			assert.True(t, bool(success))

			// Verify file was written
			fullPath := filepath.Join(tmpDir, tt.path)
			content, err := os.ReadFile(fullPath)
			require.NoError(t, err)
			assert.Equal(t, tt.content, string(content))
		})
	}
}

// TestFSExists tests fs.exists() function
func TestFSExists(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(string) (string, error)
		expected bool
	}{
		{
			name: "existing file",
			setup: func(dir string) (string, error) {
				path := filepath.Join(dir, "exists.txt")
				err := os.WriteFile(path, []byte("exists"), 0644)
				return "exists.txt", err
			},
			expected: true,
		},
		{
			name: "non-existing file",
			setup: func(dir string) (string, error) {
				return "nonexistent.txt", nil
			},
			expected: false,
		},
		{
			name: "existing directory",
			setup: func(dir string) (string, error) {
				path := filepath.Join(dir, "existingdir")
				err := os.MkdirAll(path, 0755)
				return "existingdir", err
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path, err := tt.setup(tmpDir)
			require.NoError(t, err)

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			args := starlark.Tuple{starlark.String(path)}
			result, err := runtime.fsExists(thread, starlark.NewBuiltin("exists", nil), args, nil)

			require.NoError(t, err)
			exists, ok := result.(starlark.Bool)
			require.True(t, ok, "result should be a bool")
			assert.Equal(t, tt.expected, bool(exists))
		})
	}
}

// TestFSMkdir tests fs.mkdir() function
func TestFSMkdir(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "create single directory",
			path:        "newdir",
			expectError: false,
		},
		{
			name:        "create nested directories",
			path:        "parent/child/grandchild",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			args := starlark.Tuple{starlark.String(tt.path)}
			result, err := runtime.fsMkdir(thread, starlark.NewBuiltin("mkdir", nil), args, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			success, ok := result.(starlark.Bool)
			require.True(t, ok, "result should be a bool")
			assert.True(t, bool(success))

			// Verify directory exists
			fullPath := filepath.Join(tmpDir, tt.path)
			stat, err := os.Stat(fullPath)
			require.NoError(t, err)
			assert.True(t, stat.IsDir())
		})
	}
}

// TestFSRemove tests fs.remove() function
func TestFSRemove(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string) (string, error)
		expectError bool
	}{
		{
			name: "remove file",
			setup: func(dir string) (string, error) {
				path := filepath.Join(dir, "remove.txt")
				err := os.WriteFile(path, []byte("remove me"), 0644)
				return "remove.txt", err
			},
			expectError: false,
		},
		{
			name: "remove empty directory",
			setup: func(dir string) (string, error) {
				path := filepath.Join(dir, "emptydir")
				err := os.MkdirAll(path, 0755)
				return "emptydir", err
			},
			expectError: false,
		},
		{
			name: "remove directory with contents",
			setup: func(dir string) (string, error) {
				dirPath := filepath.Join(dir, "fulldir")
				if err := os.MkdirAll(filepath.Join(dirPath, "subdir"), 0755); err != nil {
					return "", err
				}
				err := os.WriteFile(filepath.Join(dirPath, "file.txt"), []byte("content"), 0644)
				return "fulldir", err
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path, err := tt.setup(tmpDir)
			require.NoError(t, err)

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			args := starlark.Tuple{starlark.String(path)}
			result, err := runtime.fsRemove(thread, starlark.NewBuiltin("remove", nil), args, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			success, ok := result.(starlark.Bool)
			require.True(t, ok, "result should be a bool")
			assert.True(t, bool(success))

			// Verify path was removed
			fullPath := filepath.Join(tmpDir, path)
			_, err = os.Stat(fullPath)
			assert.True(t, os.IsNotExist(err))
		})
	}
}

// TestFSCopy tests fs.copy() function
func TestFSCopy(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string) (string, string, error)
		expectError bool
	}{
		{
			name: "copy file",
			setup: func(dir string) (string, string, error) {
				srcPath := filepath.Join(dir, "source.txt")
				err := os.WriteFile(srcPath, []byte("copy me"), 0644)
				return "source.txt", "dest.txt", err
			},
			expectError: false,
		},
		{
			name: "copy non-existent file",
			setup: func(dir string) (string, string, error) {
				return "nonexistent.txt", "dest.txt", nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			src, dst, err := tt.setup(tmpDir)
			require.NoError(t, err)

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			args := starlark.Tuple{
				starlark.String(src),
				starlark.String(dst),
			}
			result, err := runtime.fsCopy(thread, starlark.NewBuiltin("copy", nil), args, nil)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			success, ok := result.(starlark.Bool)
			require.True(t, ok, "result should be a bool")
			assert.True(t, bool(success))

			// Verify file was copied
			srcContent, err := os.ReadFile(filepath.Join(tmpDir, src))
			require.NoError(t, err)
			dstContent, err := os.ReadFile(filepath.Join(tmpDir, dst))
			require.NoError(t, err)
			assert.Equal(t, srcContent, dstContent)
		})
	}
}

// TestFSCwd tests fs.cwd() and fs.getcwd() functions
func TestFSCwd(t *testing.T) {
	tmpDir := t.TempDir()

	runtime := NewRuntime(tmpDir)
	thread := &starlark.Thread{Name: "test"}

	// Test cwd()
	result, err := runtime.fsCwd(thread, starlark.NewBuiltin("cwd", nil), starlark.Tuple{}, nil)
	require.NoError(t, err)
	cwd, ok := result.(starlark.String)
	require.True(t, ok, "result should be a string")
	assert.Equal(t, tmpDir, string(cwd))
}

// TestFSGlob tests fs.glob() function
func TestFSGlob(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(string) error
		pattern       string
		ignore        []string
		expectedCount int
		expectError   bool
	}{
		{
			name: "glob txt files",
			setup: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("1"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("2"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "file3.go"), []byte("3"), 0644)
			},
			pattern:       "*.txt",
			ignore:        nil,
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "glob with ignore pattern",
			setup: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("1"), 0644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("2"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("3"), 0644)
			},
			pattern:       "*.txt",
			ignore:        []string{"ignore.txt"},
			expectedCount: 2,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			require.NoError(t, tt.setup(tmpDir))

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			args := starlark.Tuple{starlark.String(tt.pattern)}
			kwargs := []starlark.Tuple{}
			if tt.ignore != nil {
				ignoreValues := make([]starlark.Value, len(tt.ignore))
				for i, pattern := range tt.ignore {
					ignoreValues[i] = starlark.String(pattern)
				}
				kwargs = append(kwargs, starlark.Tuple{
					starlark.String("ignore"),
					starlark.NewList(ignoreValues),
				})
			}

			result, err := runtime.fsGlob(thread, starlark.NewBuiltin("glob", nil), args, kwargs)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			list, ok := result.(*starlark.List)
			require.True(t, ok, "result should be a list")
			assert.Equal(t, tt.expectedCount, list.Len())
		})
	}
}

// TestFSFilter tests fs.filter() function
func TestFSFilter(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(string) error
		dir           string
		pattern       string
		recursive     bool
		expectedCount int
		expectError   bool
	}{
		{
			name: "filter non-recursive",
			setup: func(dir string) error {
				subdir := filepath.Join(dir, "subdir")
				if err := os.MkdirAll(subdir, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(subdir, "test1.txt"), []byte("1"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(subdir, "test2.go"), []byte("2"), 0644)
			},
			dir:           "subdir",
			pattern:       "*.txt",
			recursive:     false,
			expectedCount: 1,
			expectError:   false,
		},
		{
			name: "filter recursive",
			setup: func(dir string) error {
				if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "a", "test1.txt"), []byte("1"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "a", "b", "test2.txt"), []byte("2"), 0644)
			},
			dir:           "a",
			pattern:       "*.txt",
			recursive:     true,
			expectedCount: 2,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			require.NoError(t, tt.setup(tmpDir))

			runtime := NewRuntime(tmpDir)
			thread := &starlark.Thread{Name: "test"}

			kwargs := []starlark.Tuple{
				{starlark.String("dir"), starlark.String(tt.dir)},
				{starlark.String("pattern"), starlark.String(tt.pattern)},
				{starlark.String("recursive"), starlark.Bool(tt.recursive)},
			}

			result, err := runtime.fsFilter(thread, starlark.NewBuiltin("filter", nil), starlark.Tuple{}, kwargs)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			list, ok := result.(*starlark.List)
			require.True(t, ok, "result should be a list")
			assert.Equal(t, tt.expectedCount, list.Len())
		})
	}
}

// TestFSWriteErrors tests error cases for fs.write()
func TestFSWriteErrors(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := NewRuntime(tmpDir)
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing path argument", func(t *testing.T) {
		_, err := runtime.fsWrite(thread, starlark.NewBuiltin("write", nil), starlark.Tuple{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path")
	})

	t.Run("missing content argument", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("test.txt")}
		_, err := runtime.fsWrite(thread, starlark.NewBuiltin("write", nil), args, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "content")
	})

	t.Run("wrong argument type for path", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123), starlark.String("content")}
		_, err := runtime.fsWrite(thread, starlark.NewBuiltin("write", nil), args, nil)
		assert.Error(t, err)
	})

	t.Run("wrong argument type for content", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("test.txt"), starlark.MakeInt(123)}
		_, err := runtime.fsWrite(thread, starlark.NewBuiltin("write", nil), args, nil)
		assert.Error(t, err)
	})
}

// TestFSMkdirErrors tests error cases for fs.mkdir()
func TestFSMkdirErrors(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := NewRuntime(tmpDir)
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing path argument", func(t *testing.T) {
		_, err := runtime.fsMkdir(thread, starlark.NewBuiltin("mkdir", nil), starlark.Tuple{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path")
	})

	t.Run("wrong argument type", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123)}
		_, err := runtime.fsMkdir(thread, starlark.NewBuiltin("mkdir", nil), args, nil)
		assert.Error(t, err)
	})
}

// TestFSRemoveErrors tests error cases for fs.remove()
func TestFSRemoveErrors(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := NewRuntime(tmpDir)
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing path argument", func(t *testing.T) {
		_, err := runtime.fsRemove(thread, starlark.NewBuiltin("remove", nil), starlark.Tuple{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path")
	})

	t.Run("wrong argument type", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123)}
		_, err := runtime.fsRemove(thread, starlark.NewBuiltin("remove", nil), args, nil)
		assert.Error(t, err)
	})

	t.Run("remove non-existent path", func(t *testing.T) {
		// RemoveAll doesn't error on non-existent paths, but returns success
		args := starlark.Tuple{starlark.String("nonexistent-path-12345")}
		result, err := runtime.fsRemove(thread, starlark.NewBuiltin("remove", nil), args, nil)
		require.NoError(t, err)
		success, ok := result.(starlark.Bool)
		require.True(t, ok)
		assert.True(t, bool(success))
	})
}

// TestFSCwdErrors tests error cases for fs.cwd()
func TestFSCwdErrors(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := NewRuntime(tmpDir)
	thread := &starlark.Thread{Name: "test"}

	t.Run("extra arguments not allowed", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("unexpected")}
		_, err := runtime.fsCwd(thread, starlark.NewBuiltin("cwd", nil), args, nil)
		assert.Error(t, err)
	})
}

// TestFSReadErrors tests error cases for fs.read()
func TestFSReadErrors(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := NewRuntime(tmpDir)
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing path argument", func(t *testing.T) {
		_, err := runtime.fsRead(thread, starlark.NewBuiltin("read", nil), starlark.Tuple{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path")
	})

	t.Run("wrong argument type", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123)}
		_, err := runtime.fsRead(thread, starlark.NewBuiltin("read", nil), args, nil)
		assert.Error(t, err)
	})
}

// TestFSExistsErrors tests error cases for fs.exists()
func TestFSExistsErrors(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := NewRuntime(tmpDir)
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing path argument", func(t *testing.T) {
		_, err := runtime.fsExists(thread, starlark.NewBuiltin("exists", nil), starlark.Tuple{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path")
	})

	t.Run("wrong argument type", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123)}
		_, err := runtime.fsExists(thread, starlark.NewBuiltin("exists", nil), args, nil)
		assert.Error(t, err)
	})
}

// TestFSCopyErrors tests error cases for fs.copy()
func TestFSCopyErrors(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := NewRuntime(tmpDir)
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing source argument", func(t *testing.T) {
		_, err := runtime.fsCopy(thread, starlark.NewBuiltin("copy", nil), starlark.Tuple{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "src")
	})

	t.Run("missing destination argument", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("source.txt")}
		_, err := runtime.fsCopy(thread, starlark.NewBuiltin("copy", nil), args, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dst")
	})

	t.Run("wrong argument type for source", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123), starlark.String("dest.txt")}
		_, err := runtime.fsCopy(thread, starlark.NewBuiltin("copy", nil), args, nil)
		assert.Error(t, err)
	})
}

// TestFSGlobErrors tests error cases for fs.glob()
func TestFSGlobErrors(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := NewRuntime(tmpDir)
	thread := &starlark.Thread{Name: "test"}

	t.Run("missing pattern argument", func(t *testing.T) {
		_, err := runtime.fsGlob(thread, starlark.NewBuiltin("glob", nil), starlark.Tuple{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pattern")
	})

	t.Run("wrong argument type for pattern", func(t *testing.T) {
		args := starlark.Tuple{starlark.MakeInt(123)}
		_, err := runtime.fsGlob(thread, starlark.NewBuiltin("glob", nil), args, nil)
		assert.Error(t, err)
	})

	t.Run("invalid glob pattern", func(t *testing.T) {
		args := starlark.Tuple{starlark.String("[")} // Invalid glob pattern
		_, err := runtime.fsGlob(thread, starlark.NewBuiltin("glob", nil), args, nil)
		assert.Error(t, err)
	})
}

// TestFSGrep tests fs.grep() function
func TestFSGrep(t *testing.T) {
	setup := func(dir string) error {
		if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n\nfunc Hello() {}\nfunc World() {}\n"), 0644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello world\ngoodbye\n"), 0644); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "sub", "c.go"), []byte("package sub\n\nfunc Hello() string { return \"hello\" }\n"), 0644); err != nil {
			return err
		}
		return nil
	}

	t.Run("basic pattern match", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, setup(tmpDir))
		rt := NewRuntime(tmpDir)
		thread := &starlark.Thread{Name: "test"}

		args := starlark.Tuple{starlark.String("Hello")}
		result, err := rt.fsGrep(thread, starlark.NewBuiltin("grep", nil), args, nil)
		require.NoError(t, err)

		list, ok := result.(*starlark.List)
		require.True(t, ok)
		// a.go line 3, b.txt line 1 (hello, case-sensitive no match), sub/c.go line 3
		assert.Equal(t, 2, list.Len(), "should match Hello in a.go and sub/c.go")

		first, ok := list.Index(0).(*starlarkstruct.Struct)
		require.True(t, ok)
		fileVal, err := first.Attr("file")
		require.NoError(t, err)
		assert.Equal(t, "a.go", string(fileVal.(starlark.String)))

		lineVal, err := first.Attr("line")
		require.NoError(t, err)
		lineInt, ok := lineVal.(starlark.Int)
		require.True(t, ok)
		n, _ := lineInt.Int64()
		assert.Equal(t, int64(3), n)
	})

	t.Run("ignore_case", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, setup(tmpDir))
		rt := NewRuntime(tmpDir)
		thread := &starlark.Thread{Name: "test"}

		args := starlark.Tuple{starlark.String("hello")}
		kwargs := []starlark.Tuple{{starlark.String("ignore_case"), starlark.Bool(true)}}
		result, err := rt.fsGrep(thread, starlark.NewBuiltin("grep", nil), args, kwargs)
		require.NoError(t, err)

		list, ok := result.(*starlark.List)
		require.True(t, ok)
		// a.go: "func Hello()" x2, b.txt: "hello world", sub/c.go: Hello + "hello"
		assert.GreaterOrEqual(t, list.Len(), 3)
	})

	t.Run("glob filter", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, setup(tmpDir))
		rt := NewRuntime(tmpDir)
		thread := &starlark.Thread{Name: "test"}

		args := starlark.Tuple{starlark.String("hello")}
		kwargs := []starlark.Tuple{
			{starlark.String("ignore_case"), starlark.Bool(true)},
			{starlark.String("glob"), starlark.String("*.go")},
		}
		result, err := rt.fsGrep(thread, starlark.NewBuiltin("grep", nil), args, kwargs)
		require.NoError(t, err)

		list, ok := result.(*starlark.List)
		require.True(t, ok)
		// Only .go files should be searched — no b.txt results
		for i := 0; i < list.Len(); i++ {
			s, ok := list.Index(i).(*starlarkstruct.Struct)
			require.True(t, ok)
			fv, err := s.Attr("file")
			require.NoError(t, err)
			assert.True(t, strings.HasSuffix(string(fv.(starlark.String)), ".go"))
		}
	})

	t.Run("max_matches limit", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, setup(tmpDir))
		rt := NewRuntime(tmpDir)
		thread := &starlark.Thread{Name: "test"}

		args := starlark.Tuple{starlark.String("func")}
		kwargs := []starlark.Tuple{{starlark.String("max_matches"), starlark.MakeInt(2)}}
		result, err := rt.fsGrep(thread, starlark.NewBuiltin("grep", nil), args, kwargs)
		require.NoError(t, err)

		list, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 2, list.Len())
	})

	t.Run("no matches returns empty list", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, setup(tmpDir))
		rt := NewRuntime(tmpDir)
		thread := &starlark.Thread{Name: "test"}

		args := starlark.Tuple{starlark.String("ZZZNOMATCH")}
		result, err := rt.fsGrep(thread, starlark.NewBuiltin("grep", nil), args, nil)
		require.NoError(t, err)

		list, ok := result.(*starlark.List)
		require.True(t, ok)
		assert.Equal(t, 0, list.Len())
	})

	t.Run("invalid regexp returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		rt := NewRuntime(tmpDir)
		thread := &starlark.Thread{Name: "test"}

		args := starlark.Tuple{starlark.String("[")} // invalid regexp
		_, err := rt.fsGrep(thread, starlark.NewBuiltin("grep", nil), args, nil)
		assert.Error(t, err)
	})
}
