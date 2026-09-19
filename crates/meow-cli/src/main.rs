// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The `meow` binary.

use std::process::ExitCode;

fn main() -> ExitCode {
    ExitCode::from(meow_cli::exit::code(meow_cli::run()))
}
