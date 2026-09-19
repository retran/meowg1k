// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Session rows, at the level the store owns them.
//!
//! What a session *means* - the append-only log, compaction, forking - belongs
//! to `meow-session` and arrives with `docs/spec/session.md`. The store owns
//! the rows, the ordering, and deletion, because deletion has to decrement the
//! reference count of every blob the session referenced and nothing above the
//! store can see those counts.

use crate::blob::BlobHash;
use crate::error::{Result, StoreError};
use crate::{Store, now_secs};

/// A row in the `sessions` table.
#[derive(Debug, Clone)]
pub struct SessionRow {
    /// The full identifier.
    pub id: String,
    /// The optional unique name.
    pub name: Option<String>,
    /// Which agent the session recorded.
    pub agent: String,
    /// The parent session, for a sub-agent.
    pub parent_id: Option<String>,
}

impl Store {
    /// Create a session row.
    ///
    /// The parent must already exist, which is what makes a cycle in the
    /// parent relation impossible rather than merely unlikely; the foreign key
    /// enforces it.
    pub fn create_session(
        &self,
        id: &str,
        agent: &str,
        parent_id: Option<&str>,
    ) -> Result<SessionRow> {
        self.conn().execute(
            "INSERT INTO sessions(id, agent, parent_id, created_at) VALUES (?1, ?2, ?3, ?4)",
            rusqlite::params![id, agent, parent_id, now_secs()],
        )?;
        Ok(SessionRow {
            id: id.to_owned(),
            name: None,
            agent: agent.to_owned(),
            parent_id: parent_id.map(str::to_owned),
        })
    }

    /// Append one event.
    ///
    /// One event, one commit. `[R-STORE-020]` forbids wrapping a turn in a
    /// transaction held open across tool execution: the log is append-only, so
    /// a half-written turn is a true record of how far the run got, and
    /// holding the single write lock for the length of a tool would block
    /// every other session and lose that tool's result on a crash.
    pub fn append_event(
        &self,
        session_id: &str,
        seq: i64,
        kind: &str,
        payload: Option<&BlobHash>,
    ) -> Result<()> {
        self.conn().execute(
            "INSERT INTO events(session_id, seq, at, kind, payload) VALUES (?1, ?2, ?3, ?4, ?5)",
            rusqlite::params![
                session_id,
                seq,
                now_secs(),
                kind,
                payload.map(BlobHash::as_str)
            ],
        )?;
        Ok(())
    }

    /// How many events a session holds.
    pub fn event_count(&self, session_id: &str) -> Result<i64> {
        Ok(self.conn().query_row(
            "SELECT count(*) FROM events WHERE session_id = ?1",
            [session_id],
            |r| r.get(0),
        )?)
    }

    /// Record that the writer of a session is still alive.
    ///
    /// The heartbeat is a column rather than an event, because it is mutable
    /// and the log is not.
    pub fn heartbeat(&self, session_id: &str) -> Result<()> {
        self.conn().execute(
            "UPDATE sessions SET heartbeat = ?1 WHERE id = ?2",
            rusqlite::params![now_secs(), session_id],
        )?;
        Ok(())
    }

    /// Delete a session and everything that belongs to it.
    ///
    /// Satisfies `[R-STORE-040]` and `[R-STORE-041]`. The whole thing goes or
    /// nothing does, because a half-truncated log is worse than no log, and
    /// every blob the events referenced loses one referent on the way out.
    /// A session with children fails rather than orphaning them; the caller
    /// deletes the tree from the leaves.
    pub fn delete_session(&mut self, session_id: &str) -> Result<()> {
        let exists: bool = self.conn().query_row(
            "SELECT count(*) > 0 FROM sessions WHERE id = ?1",
            [session_id],
            |r| r.get(0),
        )?;
        if !exists {
            return Err(StoreError::SessionMissing {
                id: session_id.to_owned(),
            });
        }

        let hashes: Vec<String> = {
            let conn = self.conn();
            let mut stmt = conn.prepare(
                "SELECT payload FROM events WHERE session_id = ?1 AND payload IS NOT NULL",
            )?;
            let rows = stmt.query_map([session_id], |r| r.get::<_, String>(0))?;
            rows.collect::<std::result::Result<_, _>>()?
        };

        let tx = self.conn_mut().transaction()?;
        for hash in &hashes {
            tx.execute(
                "UPDATE blobs SET refcount = refcount - 1 WHERE hash = ?1 AND refcount > 0",
                [hash],
            )?;
        }
        tx.execute("DELETE FROM events WHERE session_id = ?1", [session_id])?;
        tx.execute("DELETE FROM sessions WHERE id = ?1", [session_id])?;
        tx.execute("DELETE FROM blobs WHERE refcount <= 0", [])?;
        tx.commit()?;
        Ok(())
    }
}
