// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package main is the entry point for the meow CLI application.
package main

import (
	"os"

	"github.com/retran/meowg1k/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// Cobra already printed the error, just exit with error code
		os.Exit(1)
	}
}
