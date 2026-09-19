// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The durable workspace key-value store.
//!
//! This is where `@std//store` keeps what a user wants to survive a run. It is
//! a separate table from anything a session owns, so that collecting a session
//! cannot take it with them.

use rusqlite::OptionalExtension;

use crate::Store;
use crate::error::Result;

impl Store {
    /// Read a value, or `None` when the key has never been written.
    ///
    /// Satisfies `[R-STORE-030]`.
    pub fn kv_get(&self, key: &str) -> Result<Option<Vec<u8>>> {
        Ok(self
            .conn()
            .query_row("SELECT value FROM kv WHERE key = ?1", [key], |r| r.get(0))
            .optional()?)
    }

    /// Write a value, replacing any previous one.
    ///
    /// Satisfies `[R-STORE-030]` and `[R-STORE-031]`: the `kv` table is not
    /// reachable from a session row, so no session deletion can touch it.
    pub fn kv_put(&self, key: &str, value: &[u8]) -> Result<()> {
        self.conn().execute(
            "INSERT INTO kv(key, value) VALUES (?1, ?2)
             ON CONFLICT(key) DO UPDATE SET value = excluded.value",
            rusqlite::params![key, value],
        )?;
        Ok(())
    }

    /// Remove a key. Removing one that is absent is not an error.
    pub fn kv_delete(&self, key: &str) -> Result<()> {
        self.conn()
            .execute("DELETE FROM kv WHERE key = ?1", [key])?;
        Ok(())
    }

    /// Every key, in sorted order so that a listing is stable.
    pub fn kv_keys(&self) -> Result<Vec<String>> {
        let conn = self.conn();
        let mut stmt = conn.prepare("SELECT key FROM kv ORDER BY key")?;
        let rows = stmt.query_map([], |r| r.get::<_, String>(0))?;
        let mut out = Vec::new();
        for r in rows {
            out.push(r?);
        }
        Ok(out)
    }
}
