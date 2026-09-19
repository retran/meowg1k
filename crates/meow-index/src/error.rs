// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What can go wrong while indexing.

use std::path::PathBuf;

/// Everything the index can fail with.
#[derive(Debug, thiserror::Error)]
pub enum IndexError {
    /// A file could not be read.
    #[error("could not read {path}: {source}")]
    Io {
        /// Which file.
        path: PathBuf,
        /// What the operating system said.
        #[source]
        source: std::io::Error,
    },

    /// The walk itself failed.
    #[error("could not walk {root}: {message}")]
    Walk {
        /// Where it started.
        root: PathBuf,
        /// What went wrong.
        message: String,
    },

    /// One chunk is too large for the embedding model, on its own.
    ///
    /// `[R-INDEX-021]`: the file and the lines, not a generic size error. A
    /// caller who is told "input too large" and nothing else has to bisect
    /// their own repository to find out where.
    #[error("{path}:{first}-{last} is {chars} characters, and the model takes {limit}")]
    ChunkTooLarge {
        /// Which file.
        path: PathBuf,
        /// The first line of the chunk, from one.
        first: usize,
        /// The last line of the chunk, from one.
        last: usize,
        /// How big it is.
        chars: usize,
        /// How big it may be.
        limit: usize,
    },
}

/// The result type every fallible call in this crate returns.
pub type Result<T> = std::result::Result<T, IndexError>;
