// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Branching a run at a point in its log.

use meow_core::SessionId;

use crate::error::{Result, SessionError};
use crate::log::Sessions;

/// Where a session came from.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Origin {
    /// The session it branched off.
    pub session: SessionId,
    /// The last sequence it copied.
    pub seq: u64,
}

impl Sessions {
    /// Branch a session at a sequence.
    ///
    /// Satisfies `[R-SESSION-052]`: the new session's first `n` events are
    /// copies referencing the same blobs, every such blob gains a referent so
    /// collecting the origin cannot delete content the fork points at, the
    /// origin and sequence are recorded, and the origin itself is untouched.
    ///
    /// The fork is a child of nothing. A fork is a sibling of the run it
    /// branched off, not its descendant: making it a child would put it under
    /// the origin in `[R-SESSION-080]`'s deletion rule, so collecting the
    /// origin would take the fork with it, which is the opposite of what the
    /// reference counting is for.
    ///
    /// The identifier is an argument for the same reason `start` takes one:
    /// minting it needs a clock and this crate performs no input or output.
    ///
    /// # Errors
    ///
    /// [`SessionError::ForkPoint`] when the sequence does not exist or falls
    /// inside a superseded range, naming the range that would work.
    pub fn fork(&mut self, origin: &SessionId, at: u64, id: &SessionId) -> Result<SessionId> {
        // The last sequence, not the whole log: decoding every body to learn
        // one number would fail on an event this crate did not write, and the
        // store knows the number already.
        let last = self
            .store()
            .last_event(origin.as_str())?
            .map_or(0, |row| row.seq as u64);

        if at == 0 || at > last {
            return Err(SessionError::ForkPoint {
                at,
                reason: if last == 0 {
                    "the session has no events".to_owned()
                } else {
                    format!("the sequences are 1 to {last}")
                },
            });
        }

        // [R-SESSION-053]: a fork inside a summarised range would copy events
        // the origin's own rebuild no longer shows a model, so the fork would
        // start from a conversation that never happened.
        for range in self.store().compactions(origin.as_str())? {
            let (from, to) = (range.from_seq as u64, range.to_seq as u64);
            if at >= from && at <= to {
                return Err(SessionError::ForkPoint {
                    at,
                    reason: format!(
                        "sequences {from} to {to} were summarised; fork at {} or later",
                        range.seq
                    ),
                });
            }
        }

        let agent = self.session(origin)?.agent;
        self.store().create_session(id.as_str(), &agent, None)?;
        self.store_mut()
            .copy_events(origin.as_str(), id.as_str(), at as i64)?;
        self.store()
            .set_origin(id.as_str(), origin.as_str(), at as i64)?;

        Ok(id.clone())
    }

    /// Where a session branched from, when it did.
    ///
    /// # Errors
    ///
    /// Whatever the store said.
    pub fn origin(&self, id: &SessionId) -> Result<Option<Origin>> {
        Ok(self.store().origin(id.as_str())?.map(|(id, seq)| Origin {
            session: SessionId::from_text(id),
            seq: seq as u64,
        }))
    }
}
