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

    /// There is nothing to search.
    ///
    /// `[R-INDEX-041]`: a query against an empty or absent index says so and
    /// builds nothing. A query that quietly built an index would turn a typo
    /// into several minutes and a bill.
    #[error("the index is empty; run `meow index build` first")]
    Empty,

    /// The index was built by a different embedding model.
    ///
    /// `[R-INDEX-051]`: both names, because the fix is to rebuild or to change
    /// the model back and neither is obvious from one of them.
    #[error(
        "the index was built by `{built_by}` and the query used `{asked_by}`; rebuild it or use the model it was built with"
    )]
    WrongModel {
        /// What built the index.
        built_by: String,
        /// What is asking.
        asked_by: String,
    },

    /// The store would not do something.
    #[error("{0}")]
    Store(String),

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
