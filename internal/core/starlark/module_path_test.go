// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestPathStem(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple file", "/path/to/file.go", "file"},
		{"no extension", "/path/to/README", "README"},
		{"multiple dots", "archive.tar.gz", "archive.tar"},
		{"hidden file", ".gitignore", ""},
		{"path with dot", "/path.dir/file.txt", "file"},
	}

	thread := &starlark.Thread{Name: "test"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := starlark.Tuple{starlark.String(tt.path)}
			result, err := pathStem(thread, starlark.NewBuiltin("path.stem", pathStem), args, nil)
			if err != nil {
				t.Fatalf("pathStem error: %v", err)
			}

			got := string(result.(starlark.String))
			if got != tt.expected {
				t.Errorf("pathStem(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestPathParent(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"absolute path", "/path/to/file.go", "/path/to"},
		{"relative path", "path/to/file.go", "path/to"},
		{"single level", "file.go", "."},
		{"root", "/file.go", "/"},
	}

	thread := &starlark.Thread{Name: "test"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := starlark.Tuple{starlark.String(tt.path)}
			result, err := pathParent(thread, starlark.NewBuiltin("path.parent", pathParent), args, nil)
			if err != nil {
				t.Fatalf("pathParent error: %v", err)
			}

			got := string(result.(starlark.String))
			if got != tt.expected {
				t.Errorf("pathParent(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestPathParts(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{"absolute path", "/path/to/file", []string{"path", "to", "file"}},
		{"relative path", "path/to/file", []string{"path", "to", "file"}},
		{"single level", "file", []string{"file"}},
	}

	thread := &starlark.Thread{Name: "test"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := starlark.Tuple{starlark.String(tt.path)}
			result, err := pathParts(thread, starlark.NewBuiltin("path.parts", pathParts), args, nil)
			if err != nil {
				t.Fatalf("pathParts error: %v", err)
			}

			list := result.(*starlark.List)
			if list.Len() != len(tt.expected) {
				t.Errorf("pathParts(%q) returned %d parts, want %d", tt.path, list.Len(), len(tt.expected))
				return
			}

			for i := 0; i < list.Len(); i++ {
				got := string(list.Index(i).(starlark.String))
				if got != tt.expected[i] {
					t.Errorf("pathParts(%q)[%d] = %q, want %q", tt.path, i, got, tt.expected[i])
				}
			}
		})
	}
}

func TestPathExtension(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple extension", "/path/to/file.go", ".go"},
		{"no extension", "/path/to/README", ""},
		{"multiple dots", "archive.tar.gz", ".gz"},
	}

	thread := &starlark.Thread{Name: "test"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := starlark.Tuple{starlark.String(tt.path)}
			result, err := pathExtension(thread, starlark.NewBuiltin("path.extension", pathExtension), args, nil)
			if err != nil {
				t.Fatalf("pathExtension error: %v", err)
			}

			got := string(result.(starlark.String))
			if got != tt.expected {
				t.Errorf("pathExtension(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

// TestPathJoin tests path.join() function
func TestPathJoin(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{
			name:     "join two parts",
			parts:    []string{"a", "b"},
			expected: filepath.Join("a", "b"),
		},
		{
			name:     "join multiple parts",
			parts:    []string{"a", "b", "c", "d"},
			expected: filepath.Join("a", "b", "c", "d"),
		},
		{
			name:     "join with empty string",
			parts:    []string{"a", "", "b"},
			expected: filepath.Join("a", "", "b"),
		},
		{
			name:     "join no parts",
			parts:    []string{},
			expected: "",
		},
		{
			name:     "join single part",
			parts:    []string{"single"},
			expected: "single",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}
			args := make(starlark.Tuple, len(tt.parts))
			for i, part := range tt.parts {
				args[i] = starlark.String(part)
			}

			result, err := pathJoin(thread, starlark.NewBuiltin("join", pathJoin), args, nil)
			require.NoError(t, err)

			resultStr, ok := result.(starlark.String)
			require.True(t, ok, "result should be a string")
			assert.Equal(t, tt.expected, string(resultStr))
		})
	}
}

// TestPathJoinInvalidType tests path.join() with invalid argument type
func TestPathJoinInvalidType(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	args := starlark.Tuple{starlark.String("a"), starlark.MakeInt(123)}

	_, err := pathJoin(thread, starlark.NewBuiltin("join", pathJoin), args, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be string")
}

// TestPathDirname tests path.dirname() function
func TestPathDirname(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "get directory of file",
			path:     "/path/to/file.txt",
			expected: "/path/to",
		},
		{
			name:     "get directory of path with trailing slash",
			path:     "/path/to/",
			expected: "/path/to",
		},
		{
			name:     "get directory of root",
			path:     "/",
			expected: "/",
		},
		{
			name:     "get directory of relative path",
			path:     "a/b/c",
			expected: "a/b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}
			args := starlark.Tuple{starlark.String(tt.path)}

			result, err := pathDirname(thread, starlark.NewBuiltin("dirname", pathDirname), args, nil)
			require.NoError(t, err)

			resultStr, ok := result.(starlark.String)
			require.True(t, ok, "result should be a string")
			assert.Equal(t, tt.expected, string(resultStr))
		})
	}
}

// TestPathBasename tests path.basename() function
func TestPathBasename(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "get basename of file",
			path:     "/path/to/file.txt",
			expected: "file.txt",
		},
		{
			name:     "get basename of directory",
			path:     "/path/to/dir",
			expected: "dir",
		},
		{
			name:     "get basename of path with trailing slash",
			path:     "/path/to/dir/",
			expected: "dir",
		},
		{
			name:     "get basename of root",
			path:     "/",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}
			args := starlark.Tuple{starlark.String(tt.path)}

			result, err := pathBasename(thread, starlark.NewBuiltin("basename", pathBasename), args, nil)
			require.NoError(t, err)

			resultStr, ok := result.(starlark.String)
			require.True(t, ok, "result should be a string")
			assert.Equal(t, tt.expected, string(resultStr))
		})
	}
}

// TestPathExt tests path.ext() function
func TestPathExt(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "get extension of .txt file",
			path:     "file.txt",
			expected: ".txt",
		},
		{
			name:     "get extension of .tar.gz file",
			path:     "archive.tar.gz",
			expected: ".gz",
		},
		{
			name:     "get extension of file with no extension",
			path:     "file",
			expected: "",
		},
		{
			name:     "get extension of hidden file",
			path:     ".gitignore",
			expected: ".gitignore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}
			args := starlark.Tuple{starlark.String(tt.path)}

			result, err := pathExt(thread, starlark.NewBuiltin("ext", pathExt), args, nil)
			require.NoError(t, err)

			resultStr, ok := result.(starlark.String)
			require.True(t, ok, "result should be a string")
			assert.Equal(t, tt.expected, string(resultStr))
		})
	}
}

// TestPathAbs tests path.abs() function
func TestPathAbs(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	args := starlark.Tuple{starlark.String(".")}

	result, err := pathAbs(thread, starlark.NewBuiltin("abs", pathAbs), args, nil)
	require.NoError(t, err)

	resultStr, ok := result.(starlark.String)
	require.True(t, ok, "result should be a string")
	// Just verify it returns an absolute path
	assert.True(t, filepath.IsAbs(string(resultStr)))
}

// TestPathClean tests path.clean() function
func TestPathClean(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "clean path with double slashes",
			path:     "a//b//c",
			expected: "a/b/c",
		},
		{
			name:     "clean path with dots",
			path:     "a/./b/../c",
			expected: "a/c",
		},
		{
			name:     "clean path with trailing slash",
			path:     "a/b/c/",
			expected: "a/b/c",
		},
		{
			name:     "clean already clean path",
			path:     "a/b/c",
			expected: "a/b/c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}
			args := starlark.Tuple{starlark.String(tt.path)}

			result, err := pathClean(thread, starlark.NewBuiltin("clean", pathClean), args, nil)
			require.NoError(t, err)

			resultStr, ok := result.(starlark.String)
			require.True(t, ok, "result should be a string")
			assert.Equal(t, tt.expected, string(resultStr))
		})
	}
}

// TestPathRel tests path.rel() function
func TestPathRel(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		target   string
		expected string
	}{
		{
			name:     "relative path from parent to child",
			base:     "/a/b",
			target:   "/a/b/c/d",
			expected: "c/d",
		},
		{
			name:     "relative path from child to parent",
			base:     "/a/b/c",
			target:   "/a",
			expected: "../..",
		},
		{
			name:     "relative path between siblings",
			base:     "/a/b",
			target:   "/a/c",
			expected: "../c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := &starlark.Thread{Name: "test"}
			args := starlark.Tuple{
				starlark.String(tt.base),
				starlark.String(tt.target),
			}

			result, err := pathRel(thread, starlark.NewBuiltin("rel", pathRel), args, nil)
			require.NoError(t, err)

			resultStr, ok := result.(starlark.String)
			require.True(t, ok, "result should be a string")
			assert.Equal(t, tt.expected, string(resultStr))
		})
	}
}

// TestPathErrorCases tests error handling in path functions
func TestPathErrorCases(t *testing.T) {
	t.Run("dirname with missing argument", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		_, err := pathDirname(thread, starlark.NewBuiltin("dirname", pathDirname), starlark.Tuple{}, nil)
		assert.Error(t, err)
	})

	t.Run("basename with missing argument", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		_, err := pathBasename(thread, starlark.NewBuiltin("basename", pathBasename), starlark.Tuple{}, nil)
		assert.Error(t, err)
	})

	t.Run("ext with missing argument", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		_, err := pathExt(thread, starlark.NewBuiltin("ext", pathExt), starlark.Tuple{}, nil)
		assert.Error(t, err)
	})

	t.Run("abs with missing argument", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		_, err := pathAbs(thread, starlark.NewBuiltin("abs", pathAbs), starlark.Tuple{}, nil)
		assert.Error(t, err)
	})

	t.Run("clean with missing argument", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		_, err := pathClean(thread, starlark.NewBuiltin("clean", pathClean), starlark.Tuple{}, nil)
		assert.Error(t, err)
	})

	t.Run("rel with missing arguments", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		// Missing both arguments
		_, err := pathRel(thread, starlark.NewBuiltin("rel", pathRel), starlark.Tuple{}, nil)
		assert.Error(t, err)

		// Missing second argument
		args := starlark.Tuple{starlark.String("/a/b")}
		_, err = pathRel(thread, starlark.NewBuiltin("rel", pathRel), args, nil)
		assert.Error(t, err)
	})

	t.Run("stem with missing argument", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		_, err := pathStem(thread, starlark.NewBuiltin("stem", pathStem), starlark.Tuple{}, nil)
		assert.Error(t, err)
	})

	t.Run("parent with missing argument", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		_, err := pathParent(thread, starlark.NewBuiltin("parent", pathParent), starlark.Tuple{}, nil)
		assert.Error(t, err)
	})

	t.Run("parts with missing argument", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		_, err := pathParts(thread, starlark.NewBuiltin("parts", pathParts), starlark.Tuple{}, nil)
		assert.Error(t, err)
	})

	t.Run("join with non-string arguments", func(t *testing.T) {
		thread := &starlark.Thread{Name: "test"}
		args := starlark.Tuple{
			starlark.String("a"),
			starlark.MakeInt(123), // Invalid: not a string
		}
		_, err := pathJoin(thread, starlark.NewBuiltin("join", pathJoin), args, nil)
		assert.Error(t, err)
	})
}
