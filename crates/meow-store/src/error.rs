// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The one error type this crate returns.

use std::path::PathBuf;

/// Everything that can go wrong in the store.
///
/// Variants are named for what went wrong rather than for where it happened,
/// and each carries enough to act on.
#[derive(Debug, thiserror::Error)]
pub enum StoreError {
    /// The database was written by a newer binary than this one.
    ///
    /// Required by `[R-STORE-006]`: the file is left untouched, and both
    /// versions are named so a user knows which binary to reach for.
    #[error(
        "database at {path} has schema version {found}, but this binary supports at most {supported}"
    )]
    SchemaTooNew {
        /// Where the database lives.
        path: PathBuf,
        /// The version recorded in the file.
        found: u32,
        /// The highest version this binary can apply.
        supported: u32,
    },

    /// A migration failed; the database is still at the version it had before.
    #[error("migration to version {version} failed: {source}")]
    Migration {
        /// The version that was being applied.
        version: u32,
        /// What SQLite said.
        #[source]
        source: rusqlite::Error,
    },

    /// A blob was asked for by a hash that has no row.
    ///
    /// Required by `[R-STORE-013]`: a missing blob is an error naming the
    /// hash, never empty content.
    #[error("no blob with hash {hash}")]
    BlobMissing {
        /// The hash that was asked for.
        hash: String,
    },

    /// A session was asked for by an identifier that has no row.
    #[error("no session with id {id}")]
    SessionMissing {
        /// The identifier that was asked for.
        id: String,
    },

    /// The workspace directory could not be prepared.
    #[error("could not create {path}: {source}")]
    Io {
        /// The path that could not be created or opened.
        path: PathBuf,
        /// What the operating system said.
        #[source]
        source: std::io::Error,
    },

    /// Any other failure SQLite reported.
    ///
    /// Required by `[R-STORE-021]`: a failed write reaches the caller. The
    /// store never logs a failure and carries on.
    #[error("database error: {0}")]
    Sqlite(#[from] rusqlite::Error),
}

/// The result type every fallible call in this crate returns.
pub type Result<T> = std::result::Result<T, StoreError>;
