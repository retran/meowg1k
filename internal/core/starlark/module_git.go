// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// createGitModule creates the git built-in module.
func (r *Runtime) createGitModule() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		// Functions
		"glob":            starlark.NewBuiltin("glob", r.gitGlob),
		"read":            starlark.NewBuiltin("read", r.gitRead),
		"diff":            starlark.NewBuiltin("diff", r.gitDiff),
		"diff_file":       starlark.NewBuiltin("diff_file", r.gitDiffFile),
		"log":             starlark.NewBuiltin("log", r.gitLog),
		"status":          starlark.NewBuiltin("status", r.gitStatus),
		"branch":          starlark.NewBuiltin("branch", r.gitBranch),
		"commit":          starlark.NewBuiltin("commit", r.gitCommit),
		"push":            starlark.NewBuiltin("push", r.gitPush),
		"create_branch":   starlark.NewBuiltin("create_branch", r.gitCreateBranch),
		"checkout":        starlark.NewBuiltin("checkout", r.gitCheckout),
		"add":             starlark.NewBuiltin("add", r.gitAdd),
		"staged_files":    starlark.NewBuiltin("staged_files", r.gitStagedFiles),
		"modified_files":  starlark.NewBuiltin("modified_files", r.gitModifiedFiles),
		"untracked_files": starlark.NewBuiltin("untracked_files", r.gitUntrackedFiles),

		// Constants
		"STAGED":   starlark.String("staged"),
		"HEAD":     starlark.String("HEAD"),
		"UNSTAGED": starlark.String("unstaged"),
	})
}

// gitDiff implements git.diff().
func (r *Runtime) gitDiff(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var target string = "staged"

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "target?", &target); err != nil {
		return nil, err
	}

	var diffArgs []string
	switch target {
	case "staged":
		diffArgs = []string{"diff", "--cached"}
	case "HEAD":
		diffArgs = []string{"diff", "HEAD"}
	default:
		diffArgs = []string{"diff", target}
	}

	cmd := exec.Command("git", diffArgs...)
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff failed: %w: %s", err, stderr.String())
	}

	rawDiff := stdout.String()

	// Get list of changed files using --name-only for reliable parsing
	nameOnlyArgs := append(diffArgs, "--name-only")
	nameCmd := exec.Command("git", nameOnlyArgs...)
	nameCmd.Dir = r.workingDir

	var nameStdout, nameStderr bytes.Buffer
	nameCmd.Stdout = &nameStdout
	nameCmd.Stderr = &nameStderr

	if err := nameCmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff --name-only failed: %w: %s", err, nameStderr.String())
	}

	// Parse file names (one per line, already properly escaped by git).
	files := []string{}
	for _, line := range strings.Split(strings.TrimSpace(nameStdout.String()), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}

	additions := 0
	deletions := 0
	for _, line := range strings.Split(rawDiff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}

	filesList := starlark.NewList(make([]starlark.Value, len(files)))
	for i, f := range files {
		filesList.SetIndex(i, starlark.String(f))
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"raw":       starlark.String(rawDiff),
		"files":     filesList,
		"additions": starlark.MakeInt(additions),
		"deletions": starlark.MakeInt(deletions),
	}), nil
}

// gitDiffFile implements git.diff_file().
// Returns a struct with: raw (diff content), additions, deletions, file (path).
func (r *Runtime) gitDiffFile(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var target string = "staged"
	var filePath string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "file", &filePath, "target?", &target); err != nil {
		return nil, err
	}

	var diffArgs []string
	switch target {
	case "staged":
		diffArgs = []string{"diff", "--cached", "--", filePath}
	case "HEAD":
		diffArgs = []string{"diff", "HEAD", "--", filePath}
	default:
		diffArgs = []string{"diff", target, "--", filePath}
	}

	cmd := exec.Command("git", diffArgs...)
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff failed for %s: %w: %s", filePath, err, stderr.String())
	}

	rawDiff := stdout.String()

	additions := 0
	deletions := 0
	for _, line := range strings.Split(rawDiff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"raw":       starlark.String(rawDiff),
		"file":      starlark.String(filePath),
		"additions": starlark.MakeInt(additions),
		"deletions": starlark.MakeInt(deletions),
	}), nil
}

// gitLog implements git.log().
func (r *Runtime) gitLog(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var count int = 10

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "count?", &count); err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "log", "--pretty=format:%H%x00%an%x00%ad%x00%s", fmt.Sprintf("-%d", count))
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log failed: %w: %s", err, stderr.String())
	}

	commits := []starlark.Value{}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\x00")
		if len(parts) >= 4 {
			commit := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"hash":    starlark.String(parts[0]),
				"author":  starlark.String(parts[1]),
				"date":    starlark.String(parts[2]),
				"message": starlark.String(parts[3]),
			})
			commits = append(commits, commit)
		}
	}

	return starlark.NewList(commits), nil
}

// gitStatus implements git.status().
func (r *Runtime) gitStatus(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git status failed: %w: %s", err, stderr.String())
	}

	files := []starlark.Value{}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")

	for _, line := range lines {
		if line != "" {
			files = append(files, starlark.String(line))
		}
	}

	return starlark.NewList(files), nil
}

// gitBranch implements git.branch().
func (r *Runtime) gitBranch(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git branch failed: %w: %s", err, stderr.String())
	}

	branch := strings.TrimSpace(stdout.String())
	return starlark.String(branch), nil
}

// gitCommit implements git.commit().
func (r *Runtime) gitCommit(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var message string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "message", &message); err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git commit failed: %w: %s", err, stderr.String())
	}

	hashCmd := exec.Command("git", "rev-parse", "HEAD")
	hashCmd.Dir = r.workingDir
	var hashOut bytes.Buffer
	hashCmd.Stdout = &hashOut

	var commitHash string
	if err := hashCmd.Run(); err == nil {
		commitHash = strings.TrimSpace(hashOut.String())
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"success": starlark.Bool(true),
		"message": starlark.String(message),
		"hash":    starlark.String(commitHash),
		"output":  starlark.String(stdout.String()),
	}), nil
}

// gitPush implements git.push().
func (r *Runtime) gitPush(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var remote, branch string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "remote?", &remote, "branch?", &branch); err != nil {
		return nil, err
	}

	cmdArgs := []string{"push"}
	if remote != "" {
		cmdArgs = append(cmdArgs, remote)
		if branch != "" {
			cmdArgs = append(cmdArgs, branch)
		}
	}

	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git push failed: %w: %s", err, stderr.String())
	}

	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		branchCmd := exec.Command("git", "branch", "--show-current")
		branchCmd.Dir = r.workingDir
		var branchOut bytes.Buffer
		branchCmd.Stdout = &branchOut
		if err := branchCmd.Run(); err == nil {
			branch = strings.TrimSpace(branchOut.String())
		}
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"success": starlark.Bool(true),
		"remote":  starlark.String(remote),
		"branch":  starlark.String(branch),
		"output":  starlark.String(stderr.String()), // git push outputs to stderr
	}), nil
}

// gitCreateBranch implements git.create_branch().
func (r *Runtime) gitCreateBranch(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var shouldCheckout bool = false

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &name, "should_checkout?", &shouldCheckout); err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "branch", name)
	cmd.Dir = r.workingDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git branch failed: %w: %s", err, stderr.String())
	}

	checkedOut := false
	if shouldCheckout {
		checkoutCmd := exec.Command("git", "checkout", name)
		checkoutCmd.Dir = r.workingDir
		checkoutCmd.Stderr = &stderr

		if err := checkoutCmd.Run(); err != nil {
			return nil, fmt.Errorf("git checkout failed: %w: %s", err, stderr.String())
		}
		checkedOut = true
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"success":     starlark.Bool(true),
		"name":        starlark.String(name),
		"checked_out": starlark.Bool(checkedOut),
	}), nil
}

// gitCheckout implements git.checkout().
func (r *Runtime) gitCheckout(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var target string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "target", &target); err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "checkout", target)
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git checkout failed: %w: %s", err, stderr.String())
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"success": starlark.Bool(true),
		"target":  starlark.String(target),
		"output":  starlark.String(stderr.String()),
	}), nil
}

// gitAdd implements git.add().
func (r *Runtime) gitAdd(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var paths *starlark.List

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "paths", &paths); err != nil {
		return nil, err
	}

	cmdArgs := []string{"add"}
	filesAdded := starlark.NewList([]starlark.Value{})

	for i := 0; i < paths.Len(); i++ {
		if str, ok := paths.Index(i).(starlark.String); ok {
			pathStr := string(str)
			cmdArgs = append(cmdArgs, pathStr)
			filesAdded.Append(starlark.String(pathStr))
		}
	}

	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git add failed: %w: %s", err, stderr.String())
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"success":     starlark.Bool(true),
		"files_added": filesAdded,
		"count":       starlark.MakeInt(filesAdded.Len()),
	}), nil
}

// gitGlob implements git.glob().
// Lists files in a git ref with pattern matching and ignore filters.
func (r *Runtime) gitGlob(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var ref string = "HEAD"
	var pattern string = "**/*"
	var ignoreList *starlark.List

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"ref?", &ref,
		"pattern?", &pattern,
		"ignore?", &ignoreList,
	); err != nil {
		return nil, err
	}

	ignore := []string{}
	if ignoreList != nil {
		for i := 0; i < ignoreList.Len(); i++ {
			if str, ok := ignoreList.Index(i).(starlark.String); ok {
				ignore = append(ignore, string(str))
			}
		}
	}

	var files []string
	var err error

	switch ref {
	case "stage", "staged":
		files, err = r.listStagedFiles()
	case "workdir", "working":
		// For workdir, we'll use fs.glob instead (not git)
		return nil, fmt.Errorf("git.glob() does not support ref='%s' - use fs.glob() for working directory files", ref)
	default:
		// HEAD or commit hash
		files, err = r.listFilesAtRef(ref)
	}

	if err != nil {
		return nil, err
	}

	results := []starlark.Value{}
	for _, file := range files {
		if shouldIgnoreFile(file, ignore) {
			continue
		}

		matched, err := doublestar.Match(pattern, file)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}

		if matched {
			results = append(results, starlark.String(file))
		}
	}

	return starlark.NewList(results), nil
}

// gitRead implements git.read().
// Reads file content from git ref.
// Supports both git.read(ref="HEAD", path="file.go") and git.read("HEAD:file.go")
func (r *Runtime) gitRead(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Try to parse as single argument with git notation "ref:path"
	if args.Len() == 1 && len(kwargs) == 0 {
		if refPath, ok := args.Index(0).(starlark.String); ok {
			parts := strings.SplitN(string(refPath), ":", 2)
			if len(parts) == 2 {
				content, err := r.readFileFromGit(parts[0], parts[1])
				if err != nil {
					return nil, err
				}
				return starlark.String(content), nil
			}
		}
	}

	var ref string = "HEAD"
	var path string

	if err := starlark.UnpackArgs(
		b.Name(), args, kwargs,
		"ref?", &ref,
		"path", &path,
	); err != nil {
		return nil, err
	}

	content, err := r.readFileFromGit(ref, path)
	if err != nil {
		return nil, err
	}

	return starlark.String(content), nil
}

// listFilesAtRef lists all files at a specific git ref (commit/branch/HEAD)
func (r *Runtime) listFilesAtRef(ref string) ([]string, error) {
	cmd := exec.Command("git", "ls-tree", "-r", "--name-only", ref)
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git ls-tree failed for ref %s: %w: %s", ref, err, stderr.String())
	}

	files := []string{}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// listStagedFiles lists all files in the staging area (excluding deletions)
func (r *Runtime) listStagedFiles() ([]string, error) {
	// Use --diff-filter to exclude deleted files (D)
	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=d")
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff --cached failed: %w: %s", err, stderr.String())
	}

	files := []string{}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// readFileFromGit reads file content from git at specified ref
func (r *Runtime) readFileFromGit(ref, path string) (string, error) {
	var cmd *exec.Cmd

	switch ref {
	case "stage", "staged":
		// Read from staging area (git index)
		cmd = exec.Command("git", "show", ":"+path)
	default:
		// Read from commit/ref
		cmd = exec.Command("git", "show", ref+":"+path)
	}

	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git show failed for %s:%s: %w: %s", ref, path, err, stderr.String())
	}

	return stdout.String(), nil
}

// shouldIgnoreFile checks if a file path matches any ignore pattern
func shouldIgnoreFile(path string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := doublestar.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// gitStagedFiles implements git.staged_files().
// Returns a list of all staged files.
func (r *Runtime) gitStagedFiles(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff --cached --name-only failed: %w: %s", err, stderr.String())
	}

	files := []starlark.Value{}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line != "" {
			files = append(files, starlark.String(line))
		}
	}

	return starlark.NewList(files), nil
}

// gitModifiedFiles implements git.modified_files().
// Returns a list of modified (but not staged) files.
func (r *Runtime) gitModifiedFiles(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	cmd := exec.Command("git", "diff", "--name-only")
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff --name-only failed: %w: %s", err, stderr.String())
	}

	files := []starlark.Value{}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line != "" {
			files = append(files, starlark.String(line))
		}
	}

	return starlark.NewList(files), nil
}

// gitUntrackedFiles implements git.untracked_files().
// Returns a list of untracked files.
func (r *Runtime) gitUntrackedFiles(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git ls-files failed: %w: %s", err, stderr.String())
	}

	files := []starlark.Value{}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line != "" {
			files = append(files, starlark.String(line))
		}
	}

	return starlark.NewList(files), nil
}
