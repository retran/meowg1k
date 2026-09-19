// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The typed rows the session layer builds its meaning on.
//!
//! The store keeps these queryable rather than folding them into the opaque
//! body, because two requirements read them on every rebuild: `[R-SESSION-011]`
//! asks which ranges are superseded, and `[R-SESSION-022]` sums usage across a
//! session tree.

use rusqlite::OptionalExtension;

use crate::error::Result;
use crate::{Store, now_secs};

/// One row of the event log, as the store holds it.
#[derive(Debug, Clone)]
pub struct EventRow {
    /// Position in the session, from one.
    pub seq: i64,
    /// When it was recorded, in seconds since the Unix epoch.
    pub at: i64,
    /// Which kind of event, by name.
    pub kind: String,
    /// The event's own fields, as JSON. Opaque here.
    pub body: String,
}

/// A session and when it was created, for retention.
#[derive(Debug, Clone)]
pub struct AgedSession {
    /// Its identifier.
    pub id: String,
    /// The name somebody gave it, if any.
    pub name: Option<String>,
    /// When it was created, in seconds since the Unix epoch.
    pub created_at: i64,
}

/// A summarised range, as the store holds it.
#[derive(Debug, Clone, Copy)]
pub struct CompactionRow {
    /// Where the `Compaction` event itself sits.
    pub seq: i64,
    /// First superseded sequence, inclusive.
    pub from_seq: i64,
    /// Last superseded sequence, inclusive.
    pub to_seq: i64,
}

/// Usage totals, summed.
#[derive(Debug, Clone, Copy, Default)]
pub struct UsageTotals {
    /// Tokens sent.
    pub prompt: i64,
    /// Tokens received.
    pub completion: i64,
    /// Prompt tokens served from a cache, when any provider reported it.
    pub cached: Option<i64>,
    /// Cost in millionths, or `None` when any contributing call was unpriced.
    pub cost_micros: Option<i64>,
}

impl Store {
    /// The sequence number the next event in a session takes.
    pub fn next_seq(&self, session_id: &str) -> Result<i64> {
        Ok(self.conn().query_row(
            "SELECT coalesce(max(seq), 0) + 1 FROM events WHERE session_id = ?1",
            [session_id],
            |r| r.get(0),
        )?)
    }

    /// Append one event with its body.
    ///
    /// One event, one commit, per `[R-STORE-020]`.
    pub fn append(&self, session_id: &str, seq: i64, kind: &str, body: &str) -> Result<()> {
        self.append_with_payload(session_id, seq, kind, body, None)
    }

    /// Append one event whose bulk lives in a blob.
    ///
    /// The body is still written: a reader of the log must be able to say what
    /// happened without fetching anything, and `[R-SESSION-052]` counts the
    /// referents of whatever the body points at.
    ///
    /// # Errors
    ///
    /// Whatever SQLite said.
    pub fn append_with_payload(
        &self,
        session_id: &str,
        seq: i64,
        kind: &str,
        body: &str,
        payload: Option<&crate::BlobHash>,
    ) -> Result<()> {
        self.conn().execute(
            "INSERT INTO events(session_id, seq, at, kind, body, payload)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
            rusqlite::params![
                session_id,
                seq,
                now_secs(),
                kind,
                body,
                payload.map(crate::BlobHash::as_str)
            ],
        )?;
        Ok(())
    }

    /// Every event of a session, in order.
    pub fn events(&self, session_id: &str) -> Result<Vec<EventRow>> {
        let conn = self.conn();
        let mut stmt = conn
            .prepare("SELECT seq, at, kind, body FROM events WHERE session_id = ?1 ORDER BY seq")?;
        let rows = stmt.query_map([session_id], |r| {
            Ok(EventRow {
                seq: r.get(0)?,
                at: r.get(1)?,
                kind: r.get(2)?,
                body: r.get(3)?,
            })
        })?;
        rows.collect::<std::result::Result<_, _>>()
            .map_err(Into::into)
    }

    /// The last event of a session, or `None` when it has none.
    pub fn last_event(&self, session_id: &str) -> Result<Option<EventRow>> {
        Ok(self
            .conn()
            .query_row(
                "SELECT seq, at, kind, body FROM events WHERE session_id = ?1
                 ORDER BY seq DESC LIMIT 1",
                [session_id],
                |r| {
                    Ok(EventRow {
                        seq: r.get(0)?,
                        at: r.get(1)?,
                        kind: r.get(2)?,
                        body: r.get(3)?,
                    })
                },
            )
            .optional()?)
    }

    /// Record what a model call cost.
    pub fn put_usage(
        &self,
        session_id: &str,
        seq: i64,
        prompt: i64,
        completion: i64,
        cached: Option<i64>,
        cost_micros: Option<i64>,
    ) -> Result<()> {
        self.conn().execute(
            "INSERT INTO usage(session_id, seq, prompt, completion, cached, cost_micros)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
            rusqlite::params![session_id, seq, prompt, completion, cached, cost_micros],
        )?;
        Ok(())
    }

    /// Sum one session's usage, not counting its children.
    ///
    /// The cost is `None` when any contributing call was unpriced, because a
    /// total that silently omits part of the spend is worse than no total.
    pub fn usage_totals(&self, session_id: &str) -> Result<UsageTotals> {
        let conn = self.conn();
        let (prompt, completion, cached, any_cached, unpriced, cost): (
            i64,
            i64,
            Option<i64>,
            i64,
            i64,
            Option<i64>,
        ) = conn.query_row(
            "SELECT coalesce(sum(prompt), 0),
                    coalesce(sum(completion), 0),
                    sum(cached),
                    coalesce(sum(cached IS NOT NULL), 0),
                    coalesce(sum(cost_micros IS NULL), 0),
                    sum(cost_micros)
             FROM usage WHERE session_id = ?1",
            [session_id],
            |r| {
                Ok((
                    r.get(0)?,
                    r.get(1)?,
                    r.get(2)?,
                    r.get(3)?,
                    r.get(4)?,
                    r.get(5)?,
                ))
            },
        )?;
        Ok(UsageTotals {
            prompt,
            completion,
            cached: if any_cached > 0 { cached } else { None },
            cost_micros: if unpriced > 0 { None } else { cost },
        })
    }

    /// Record a summarised range.
    pub fn put_compaction(
        &self,
        session_id: &str,
        seq: i64,
        from_seq: i64,
        to_seq: i64,
        tokens_saved: i64,
    ) -> Result<()> {
        self.conn().execute(
            "INSERT INTO compactions(session_id, seq, from_seq, to_seq, tokens_saved)
             VALUES (?1, ?2, ?3, ?4, ?5)",
            rusqlite::params![session_id, seq, from_seq, to_seq, tokens_saved],
        )?;
        Ok(())
    }

    /// Every summarised range in a session, in order.
    pub fn compactions(&self, session_id: &str) -> Result<Vec<CompactionRow>> {
        let conn = self.conn();
        let mut stmt = conn.prepare(
            "SELECT seq, from_seq, to_seq FROM compactions WHERE session_id = ?1 ORDER BY seq",
        )?;
        let rows = stmt.query_map([session_id], |r| {
            Ok(CompactionRow {
                seq: r.get(0)?,
                from_seq: r.get(1)?,
                to_seq: r.get(2)?,
            })
        })?;
        rows.collect::<std::result::Result<_, _>>()
            .map_err(Into::into)
    }

    /// Store the rebuildable copy of a session's state.
    ///
    /// `[R-SESSION-041]` permits this so that listing a thousand sessions does
    /// not read a thousand events. The log wins on any disagreement, which is
    /// why nothing reads this without being able to fall back.
    pub fn set_state_cache(&self, session_id: &str, state: &str) -> Result<()> {
        self.conn().execute(
            "UPDATE sessions SET state = ?1 WHERE id = ?2",
            rusqlite::params![state, session_id],
        )?;
        Ok(())
    }

    /// Give a session a unique name.
    pub fn set_session_name(&self, session_id: &str, name: &str) -> Result<()> {
        self.conn().execute(
            "UPDATE sessions SET name = ?1 WHERE id = ?2",
            rusqlite::params![name, session_id],
        )?;
        Ok(())
    }

    /// Every session, newest first, with an optional agent filter.
    pub fn list_sessions(&self, agent: Option<&str>, limit: i64) -> Result<Vec<crate::SessionRow>> {
        let conn = self.conn();
        let mut stmt = conn.prepare(
            "SELECT id, name, agent, parent_id FROM sessions
             WHERE (?1 IS NULL OR agent = ?1)
             ORDER BY id DESC LIMIT ?2",
        )?;
        let rows = stmt.query_map(rusqlite::params![agent, limit], |r| {
            Ok(crate::SessionRow {
                id: r.get(0)?,
                name: r.get(1)?,
                agent: r.get(2)?,
                parent_id: r.get(3)?,
            })
        })?;
        rows.collect::<std::result::Result<_, _>>()
            .map_err(Into::into)
    }

    /// Every session with the time it was created, oldest first.
    ///
    /// Retention deletes from the old end, so it needs the age and the whole
    /// set rather than a page of the newest.
    pub fn sessions_by_age(&self) -> Result<Vec<AgedSession>> {
        let conn = self.conn();
        let mut stmt =
            conn.prepare("SELECT id, name, created_at FROM sessions ORDER BY created_at, id")?;
        let rows = stmt.query_map([], |r| {
            Ok(AgedSession {
                id: r.get(0)?,
                name: r.get(1)?,
                created_at: r.get(2)?,
            })
        })?;
        rows.collect::<std::result::Result<_, _>>()
            .map_err(Into::into)
    }

    /// Every session whose identifier ends with `suffix`, or whose name is it.
    ///
    /// The suffix rather than the prefix, because `[R-SESSION-034]` makes the
    /// short form the random tail.
    pub fn resolve_session(&self, needle: &str) -> Result<Vec<crate::SessionRow>> {
        let conn = self.conn();
        let mut stmt = conn.prepare(
            "SELECT id, name, agent, parent_id FROM sessions
             WHERE id = ?1 OR name = ?1 OR id LIKE '%' || ?1
             ORDER BY id DESC",
        )?;
        let rows = stmt.query_map([needle], |r| {
            Ok(crate::SessionRow {
                id: r.get(0)?,
                name: r.get(1)?,
                agent: r.get(2)?,
                parent_id: r.get(3)?,
            })
        })?;
        rows.collect::<std::result::Result<_, _>>()
            .map_err(Into::into)
    }

    /// The children of a session, oldest first.
    pub fn children(&self, session_id: &str) -> Result<Vec<crate::SessionRow>> {
        let conn = self.conn();
        let mut stmt = conn.prepare(
            "SELECT id, name, agent, parent_id FROM sessions
             WHERE parent_id = ?1 ORDER BY id",
        )?;
        let rows = stmt.query_map([session_id], |r| {
            Ok(crate::SessionRow {
                id: r.get(0)?,
                name: r.get(1)?,
                agent: r.get(2)?,
                parent_id: r.get(3)?,
            })
        })?;
        rows.collect::<std::result::Result<_, _>>()
            .map_err(Into::into)
    }

    /// When the writer of a session last said it was alive.
    pub fn heartbeat_at(&self, session_id: &str) -> Result<Option<i64>> {
        Ok(self
            .conn()
            .query_row(
                "SELECT heartbeat FROM sessions WHERE id = ?1",
                [session_id],
                |r| r.get(0),
            )
            .optional()?
            .flatten())
    }
}
