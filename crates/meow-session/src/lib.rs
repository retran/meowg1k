// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The durable record of one agent run.
//!
//! A session is an append-only log. Nothing that has been written is ever
//! updated or deleted, which is what makes every other property here possible:
//! compaction that can be undone by reading past it, a state that cannot
//! disagree with its own history, and a fork that shares its origin's content
//! without touching it.
//!
//! `docs/spec/session.md` is normative. This crate sits on `meow-store`, which
//! owns the rows, and knows nothing about the engine that produces the events.

mod error;
mod export;
mod fork;
mod log;
mod resolve;
mod retention;

use meow_core::SessionId;

pub use crate::error::{Result, SessionError};
pub use crate::export::{Export, Redaction};
pub use crate::fork::Origin;
pub use crate::log::{Event, Sessions};
pub use crate::resolve::Selector;
pub use crate::retention::{Retention, Swept};

/// What state a session is in.
///
/// `[R-SESSION-040]` fixes the set: `running`, or one of the six stop reasons.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum State {
    /// The last event is not `Finished`.
    Running,
    /// The last `Finished` event carries this reason.
    Stopped(meow_core::StopReason),
}

impl State {
    /// The wire and command-line spelling.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Running => "running",
            Self::Stopped(r) => r.as_str(),
        }
    }
}

impl std::fmt::Display for State {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

/// A session, as the outside sees it.
#[derive(Debug, Clone)]
pub struct Session {
    /// Its identifier.
    pub id: SessionId,
    /// The name someone gave it, if any.
    pub name: Option<String>,
    /// Which agent it recorded.
    pub agent: String,
    /// Its parent, when a sub-agent started it.
    pub parent: Option<SessionId>,
}
