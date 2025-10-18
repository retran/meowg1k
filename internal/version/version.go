// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package version provides build-time version information populated by the Go linker.
package version

// These variables are populated by the Go linker during the build process.
var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)
