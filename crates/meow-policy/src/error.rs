// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What can be wrong with a policy.
//!
//! Every variant here is raised when the policy is built, never when a call is
//! evaluated. A policy that only fails on the call it was meant to stop is not
//! a boundary.

/// A policy could not be built.
#[derive(Debug, thiserror::Error)]
pub enum PolicyError {
    /// A tool name pattern has a shape `[R-POLICY-002]` forbids.
    #[error("`{pattern}` is not a valid tool pattern: {reason}")]
    BadPattern {
        /// What was written.
        pattern: String,
        /// Why it cannot be used.
        reason: String,
    },

    /// A glob will not compile.
    #[error("`{pattern}` is not a valid glob: {reason}")]
    BadGlob {
        /// What was written.
        pattern: String,
        /// What the glob library said.
        reason: String,
    },

    /// A selector was put on a rule whose tools do not have it.
    ///
    /// `[R-POLICY-005]`: raised at build time. A `commands` selector on
    /// `fs.*` matches nothing, and finding that out when an agent is denied
    /// mid-run is finding it out too late.
    #[error("no tool matching `{pattern}` supports a `{selector}` selector")]
    UnsupportedSelector {
        /// The rule's tool pattern.
        pattern: String,
        /// The selector that does not apply.
        selector: &'static str,
    },
}

/// The result type every fallible call in this crate returns.
pub type Result<T> = std::result::Result<T, PolicyError>;
