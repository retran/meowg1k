// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"testing"
)

func TestMainVersion(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })

	os.Args = []string{"meow", "version"}
	main()
}
