// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// createFSModule creates the fs built-in module.
func (r *Runtime) createFSModule() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"read":    starlark.NewBuiltin("read", r.fsRead),
		"glob":    starlark.NewBuiltin("glob", r.fsGlob),
		"exists":  starlark.NewBuiltin("exists", r.fsExists),
		"write":   starlark.NewBuiltin("write", r.fsWrite),
		"mkdir":   starlark.NewBuiltin("mkdir", r.fsMkdir),
		"copy":    starlark.NewBuiltin("copy", r.fsCopy),
		"remove":  starlark.NewBuiltin("remove", r.fsRemove),
		"getcwd":  starlark.NewBuiltin("getcwd", r.fsCwd), // Deprecated: use cwd()
		"cwd":     starlark.NewBuiltin("cwd", r.fsCwd),
		"filter":  starlark.NewBuiltin("filter", r.fsFilter),
		"walk":    starlark.NewBuiltin("walk", r.fsWalk),
		"stat":    starlark.NewBuiltin("stat", r.fsStat),
		"listdir": starlark.NewBuiltin("listdir", r.fsListdir),
		"chmod":   starlark.NewBuiltin("chmod", r.fsChmod),
		"touch":   starlark.NewBuiltin("touch", r.fsTouch),
		"grep":    starlark.NewBuiltin("grep", r.fsGrep),
	})
}

// fsRead implements fs.read().
func (r *Runtime) fsRead(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path); err != nil {
		return nil, fmt.Errorf("fs.read: %w", err)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(r.workingDir, path)
	}

	content, err := os.ReadFile(path) //nolint:gosec // user-controlled path for reading files
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return starlark.String(string(content)), nil
}

// fsGlob implements fs.glob().
func (r *Runtime) fsGlob(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) { //nolint:gocognit // complexity inherent in glob with multiple filter options
	var pattern string
	var ignoreList *starlark.List

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "pattern", &pattern, "ignore?", &ignoreList); err != nil {
		return nil, fmt.Errorf("fs.glob: %w", err)
	}

	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(r.workingDir, pattern)
	}

	ignore := []string{}
	if ignoreList != nil {
		for i := 0; i < ignoreList.Len(); i++ {
			if str, ok := ignoreList.Index(i).(starlark.String); ok {
				ignore = append(ignore, string(str))
			}
		}
	}

	matches, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob failed: %w", err)
	}

	results := []starlark.Value{}
	for _, match := range matches {
		stat, err := os.Stat(match)
		if err != nil || stat.IsDir() {
			continue
		}

		relPath, err := filepath.Rel(r.workingDir, match)
		if err != nil {
			relPath = match
		}

		if shouldIgnore(relPath, ignore) {
			continue
		}

		results = append(results, starlark.String(relPath))
	}

	return starlark.NewList(results), nil
}

// fsExists implements fs.exists().
func (r *Runtime) fsExists(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path); err != nil {
		return nil, fmt.Errorf("fs.exists: %w", err)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(r.workingDir, path)
	}

	_, err := os.Stat(path)
	return starlark.Bool(err == nil), nil
}

func shouldIgnore(path string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := doublestar.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// fsWrite implements fs.write().
func (r *Runtime) fsWrite(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path, content string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path, "content", &content); err != nil {
		return nil, fmt.Errorf("fs.write: %w", err)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(r.workingDir, path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // user-controlled path with standard permissions
		return nil, fmt.Errorf("failed to create parent directories for '%s': %w", path, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // user-controlled path with standard permissions
		return nil, fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return starlark.Bool(true), nil
}

// fsMkdir implements fs.mkdir().
func (r *Runtime) fsMkdir(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path); err != nil {
		return nil, fmt.Errorf("fs.mkdir: %w", err)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(r.workingDir, path)
	}

	if err := os.MkdirAll(path, 0o755); err != nil { //nolint:gosec // user-controlled permissions
		return nil, fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	return starlark.Bool(true), nil
}

// fsCopy implements fs.copy().
func (r *Runtime) fsCopy(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dst string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "src", &src, "dst", &dst); err != nil {
		return nil, fmt.Errorf("fs.copy: %w", err)
	}

	if !filepath.IsAbs(src) {
		src = filepath.Join(r.workingDir, src)
	}
	if !filepath.IsAbs(dst) {
		dst = filepath.Join(r.workingDir, dst)
	}

	source, err := os.Open(src) //nolint:gosec // user-controlled path
	if err != nil {
		return nil, fmt.Errorf("failed to open source file '%s': %w", src, err)
	}
	defer source.Close() //nolint:errcheck // deferred close errors are not critical

	destination, err := os.Create(dst) //nolint:gosec // user-controlled path
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file '%s': %w", dst, err)
	}
	defer destination.Close() //nolint:errcheck // deferred close errors are not critical

	if _, err := io.Copy(destination, source); err != nil {
		return nil, fmt.Errorf("failed to copy from '%s' to '%s': %w", src, dst, err)
	}

	return starlark.Bool(true), nil
}

// fsRemove implements fs.remove().
func (r *Runtime) fsRemove(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path); err != nil {
		return nil, fmt.Errorf("fs.remove: %w", err)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(r.workingDir, path)
	}

	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("failed to remove %s: %w", path, err)
	}

	return starlark.Bool(true), nil
}

// fsCwd implements fs.cwd() (and deprecated fs.getcwd()).
func (r *Runtime) fsCwd(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, fmt.Errorf("fs.cwd: %w", err)
	}

	return starlark.String(r.workingDir), nil
}

// fsFilter implements fs.filter().
// Filters files by pattern with optional recursion.
func (r *Runtime) fsFilter(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var dir string
	var pattern = "*"
	var recursive = false

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"dir", &dir,
		"pattern?", &pattern,
		"recursive?", &recursive,
	); err != nil {
		return nil, fmt.Errorf("fs.filter: %w", err)
	}

	if !filepath.IsAbs(dir) {
		dir = filepath.Join(r.workingDir, dir)
	}

	var matchPattern string
	if recursive {
		matchPattern = filepath.Join(dir, "**", pattern)
	} else {
		matchPattern = filepath.Join(dir, pattern)
	}

	matches, err := doublestar.FilepathGlob(matchPattern)
	if err != nil {
		return nil, fmt.Errorf("fs.filter: glob failed: %w", err)
	}

	files := make([]starlark.Value, 0, len(matches))
	for _, match := range matches {
		stat, err := os.Stat(match)
		if err != nil || stat.IsDir() {
			continue
		}

		rel, err := filepath.Rel(r.workingDir, match)
		if err != nil {
			rel = match
		}
		files = append(files, starlark.String(rel))
	}

	return starlark.NewList(files), nil
}

// fsWalk implements fs.walk().
// Recursively walks a directory tree and returns a flat list of file paths.
func (r *Runtime) fsWalk(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) { //nolint:gocognit // complexity inherent in directory walk with filtering
	var root string
	var pattern string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"root", &root,
		"pattern?", &pattern,
	); err != nil {
		return nil, fmt.Errorf("fs.walk: %w", err)
	}

	if !filepath.IsAbs(root) {
		root = filepath.Join(r.workingDir, root)
	}

	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("failed to walk directory '%s': %w", root, err)
	}

	files := make([]starlark.Value, 0)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if pattern != "" {
			matched, err := doublestar.Match(pattern, filepath.Base(path))
			if err != nil {
				return fmt.Errorf("invalid pattern '%s': %w", pattern, err)
			}
			if !matched {
				return nil
			}
		}

		rel, err := filepath.Rel(r.workingDir, path)
		if err != nil {
			rel = path
		}
		files = append(files, starlark.String(rel))
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory '%s': %w", root, err)
	}

	return starlark.NewList(files), nil
}

// fsStat implements fs.stat().
// Returns file/directory metadata as a struct.
func (r *Runtime) fsStat(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path); err != nil {
		return nil, fmt.Errorf("fs.stat: %w", err)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(r.workingDir, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat '%s': %w", path, err)
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"size":   starlark.MakeInt64(info.Size()),
		"mtime":  starlark.MakeInt64(info.ModTime().Unix()),
		"is_dir": starlark.Bool(info.IsDir()),
		"mode":   starlark.MakeInt64(int64(info.Mode().Perm())),
	}), nil
}

// fsListdir implements fs.listdir().
// Lists directory contents (non-recursive, returns names only).
func (r *Runtime) fsListdir(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "path", &path); err != nil {
		return nil, fmt.Errorf("fs.listdir: %w", err)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(r.workingDir, path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory '%s': %w", path, err)
	}

	names := make([]starlark.Value, 0, len(entries))
	for _, entry := range entries {
		names = append(names, starlark.String(entry.Name()))
	}

	return starlark.NewList(names), nil
}

// fsChmod implements fs.chmod().
// Changes file/directory permissions.
func (r *Runtime) fsChmod(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var mode int

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path", &path,
		"mode", &mode,
	); err != nil {
		return nil, fmt.Errorf("fs.chmod: %w", err)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(r.workingDir, path)
	}

	if err := os.Chmod(path, os.FileMode(mode)); err != nil { // integer conversion is intentional for user-provided mode
		return nil, fmt.Errorf("failed to chmod '%s': %w", path, err)
	}

	return starlark.Bool(true), nil
}

// fsTouch implements fs.touch().
// Creates an empty file or updates its timestamp.
func (r *Runtime) fsTouch(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	var mtimeVal starlark.Value = starlark.None

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"path", &path,
		"mtime?", &mtimeVal,
	); err != nil {
		return nil, fmt.Errorf("fs.touch: %w", err)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(r.workingDir, path)
	}

	var targetTime time.Time
	if mtimeVal != starlark.None {
		if mtimeInt, ok := mtimeVal.(starlark.Int); ok {
			unixTime, _ := mtimeInt.Int64()
			targetTime = time.Unix(unixTime, 0)
		} else {
			return nil, fmt.Errorf("mtime must be an integer (unix timestamp)")
		}
	} else {
		targetTime = time.Now()
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // user-controlled path with standard permissions
			return nil, fmt.Errorf("failed to create parent directories for '%s': %w", path, err)
		}
		file, err := os.Create(path) //nolint:gosec // user-controlled path
		if err != nil {
			return nil, fmt.Errorf("failed to create file '%s': %w", path, err)
		}
		file.Close() //nolint:errcheck // closing a newly-created file; errors are not critical
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat '%s': %w", path, err)
	}

	if err := os.Chtimes(path, targetTime, targetTime); err != nil {
		return nil, fmt.Errorf("failed to update timestamp for '%s': %w", path, err)
	}

	return starlark.Bool(true), nil
}

// fsGrep implements fs.grep().
// Recursively searches files under root for lines matching a regexp pattern.
//
// Signature:
//
//	fs.grep(pattern, root=".", glob="*", ignore_case=False, max_matches=0)
func (r *Runtime) fsGrep(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) { //nolint:gocognit,gocyclo // complexity inherent in grep with multiple search options
	var (
		pattern    string
		root       = "."
		glob       = "*"
		ignoreCase = false
		maxMatches = 0
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"pattern", &pattern,
		"root?", &root,
		"glob?", &glob,
		"ignore_case?", &ignoreCase,
		"max_matches?", &maxMatches,
	); err != nil {
		return nil, fmt.Errorf("fs.grep: %w", err)
	}

	if !filepath.IsAbs(root) {
		root = filepath.Join(r.workingDir, root)
	}

	rePattern := pattern
	if ignoreCase {
		rePattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(rePattern)
	if err != nil {
		return nil, fmt.Errorf("fs.grep: invalid pattern %q: %w", pattern, err)
	}

	results := make([]starlark.Value, 0)
	total := 0

	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			// Skip hidden directories (e.g. .git) for performance
			if strings.HasPrefix(info.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply glob filter against the base filename
		if glob != "" && glob != "*" {
			matched, matchErr := doublestar.Match(glob, filepath.Base(path))
			if matchErr != nil || !matched {
				return nil
			}
		}

		f, openErr := os.Open(path) //nolint:gosec // user-controlled path for file searching
		if openErr != nil {
			return nil // skip unreadable files
		}
		defer f.Close() //nolint:errcheck // deferred close errors are not critical

		rel, relErr := filepath.Rel(r.workingDir, path)
		if relErr != nil {
			rel = path
		}

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			text := scanner.Text()
			if re.MatchString(text) {
				match := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
					"file": starlark.String(rel),
					"line": starlark.MakeInt(lineNum),
					"text": starlark.String(text),
				})
				results = append(results, match)
				total++
				if maxMatches > 0 && total >= maxMatches {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	if walkErr != nil && !errors.Is(walkErr, filepath.SkipAll) {
		return nil, fmt.Errorf("fs.grep: walk failed: %w", walkErr)
	}

	return starlark.NewList(results), nil
}
