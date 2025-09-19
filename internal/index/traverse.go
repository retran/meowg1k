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

package index

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/sabhiram/go-gitignore"
)

// traverseOptions holds the configuration for the Traverse function.
type traverseOptions struct {
	// ignorer holds the compiled ignore patterns.
	ignorer ignore.IgnoreParser
}

// Option is a function that configures traverseOptions.
type Option func(*traverseOptions)

// WithIgnorePatterns provides gitignore-style patterns to exclude files and directories.
// For example: "*.log", "dist/", "/tmp", "node_modules".
func WithIgnorePatterns(patterns ...string) Option {
	return func(opts *traverseOptions) {
		opts.ignorer = ignore.CompileIgnoreLines(patterns...)
	}
}

// traverse recursively walks the directory tree starting from the given root path.
// It sends the absolute path of each found file to the provided 'out' channel,
// respecting the ignore patterns.
func traverse(ctx context.Context, root string, out chan<- string, options ...Option) error {
	defer close(out)

	opts := &traverseOptions{}
	for _, option := range options {
		option(opts)
	}

	absolutePath, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", root, err)
	}

	err = filepath.WalkDir(absolutePath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(absolutePath, path)
		if err != nil {
			return fmt.Errorf("could not get relative path for %s: %w", path, err)
		}

		if opts.ignorer != nil && opts.ignorer.MatchesPath(relativePath) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if entry.IsDir() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- path:
		}

		return nil
	})

	return err
}
