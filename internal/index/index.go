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
	"os"
)

const (
	workers = 32
	buffer  = 1024
)

func Index(ctx context.Context, root string) error {
	files := make(chan string, buffer)

	for range workers {
		go process(files)
	}

	err := Traverse(ctx, root, files, WithIgnorePatterns(".git/", ".devcontainer/", ".meowg1k/", "dist/"))
	return err
}

func process(in <-chan string) {
	for file := range in {

		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(string(content))
		}
	}
}
