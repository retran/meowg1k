// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The one error type this crate returns.

/// Everything that can go wrong in the session layer.
#[derive(Debug, thiserror::Error)]
pub enum SessionError {
    /// A selector or identifier matched nothing.
    #[error("no session matches {needle}")]
    NotFound {
        /// What was asked for.
        needle: String,
    },

    /// A short identifier matched more than one session.
    ///
    /// `[R-SESSION-031]` forbids picking one: the candidates are listed and
    /// the caller decides, because guessing which run someone meant is how a
    /// destructive command hits the wrong one.
    #[error("{needle} matches {} sessions: {}", .candidates.len(), .candidates.join(", "))]
    Ambiguous {
        /// What was asked for.
        needle: String,
        /// Every session it could have meant.
        candidates: Vec<String>,
    },

    /// A name was rejected.
    #[error("invalid session name {name}: {reason}")]
    InvalidName {
        /// The name that was offered.
        name: String,
        /// Why it cannot be used.
        reason: String,
    },

    /// A fork was asked for at a sequence that cannot be forked at.
    ///
    /// `[R-SESSION-053]`: the message names what would work, because "invalid
    /// sequence" leaves somebody guessing at a number they cannot see.
    #[error("cannot fork at {at}: {reason}")]
    ForkPoint {
        /// What was asked for.
        at: u64,
        /// Why not, and what would work instead.
        reason: String,
    },

    /// A compaction would supersede a range that is already superseded.
    ///
    /// `[R-SESSION-013]`.
    #[error("sequences {from}..={to} are already superseded by the compaction at {existing}")]
    AlreadySuperseded {
        /// First sequence of the proposed range.
        from: u64,
        /// Last sequence of the proposed range.
        to: u64,
        /// Where the existing compaction sits.
        existing: u64,
    },

    /// An operation needed a run that is not open, or found one that is.
    #[error("{0}")]
    Lifecycle(String),

    /// The stored body could not be read back.
    #[error("event {seq} could not be decoded: {source}")]
    Decode {
        /// Which event.
        seq: u64,
        /// What serde said.
        #[source]
        source: serde_json::Error,
    },

    /// The store failed.
    #[error(transparent)]
    Store(#[from] meow_store::StoreError),
}

/// The result type every fallible call in this crate returns.
pub type Result<T> = std::result::Result<T, SessionError>;
