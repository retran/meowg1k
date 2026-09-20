// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What the `meow` binary is made of.
//!
//! A library as well as a binary, so a test can name the command surface and
//! the exit codes instead of keeping a copy of them. A test that compares
//! against its own copy of a list checks that the copy is consistent with
//! itself.

pub mod ask;
pub mod auth;
pub mod exit;
pub mod index;
mod keep;
pub mod render;
pub mod session;
pub mod surface;
pub mod trust;
pub mod wire;

use meow_star::{StarError, Workspace};

pub use crate::exit::Ending;

/// Find the workspace, parse the command line, and run what it names.
pub fn run() -> Ending {
    // The workspace is found before the command line is parsed, because what
    // the command line accepts depends on what the workspace declares.
    let workspace = std::env::current_dir()
        .ok()
        .and_then(|here| Workspace::discover(&here).ok());

    let loaded = workspace.as_ref().map(meow_star::load);

    let registry = match &loaded {
        Some(Ok(loaded)) => Some(&loaded.registry),
        _ => None,
    };

    let matches = match surface::build(registry).try_get_matches() {
        Ok(matches) => matches,
        Err(e) => {
            // clap writes help and version to stdout and errors to stderr,
            // and both are a usage exit only when they are actually an error.
            let _ = e.print();
            return if e.use_stderr() {
                Ending::Usage
            } else {
                Ending::Passed
            };
        }
    };

    // A command that needs no workspace runs before the load failure is
    // reported, so `meow init` works in an empty directory and `meow doctor`
    // still runs when `.meow/` is broken.
    if let Some(ending) = wire::without_workspace(&matches) {
        return ending;
    }

    let Some(workspace) = workspace else {
        eprintln!(
            "{}",
            StarError::NoWorkspace {
                searched: std::env::current_dir().into_iter().collect(),
            }
        );
        return Ending::Config;
    };

    let loaded = match loaded {
        Some(Ok(loaded)) => loaded,
        Some(Err(e)) => {
            eprintln!("{e}");
            return Ending::Config;
        }
        // The workspace was found a few lines above, so it was also loaded.
        None => return Ending::Config,
    };

    // `[R-AUTH-030]`: between loading and running, which is the only place it
    // can go. Earlier and there is nothing to describe; later and the scripts
    // have already run. Loading reached nothing, by `[R-STAR-084]`, so asking
    // here costs only the question.
    if let Some(ending) = wire::gate_on_trust(&matches, &workspace, &loaded) {
        return ending;
    }

    wire::with_workspace(&matches, workspace, loaded)
}
