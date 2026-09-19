// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The response cache.
//!
//! Embedding responses are cached by default: the same text embedded twice
//! gives the same vector, and paying for it twice is waste. Generation
//! responses are not, unless the caller asks. An agent that retries wants a
//! fresh attempt, and a cache would hand it the answer that already failed.

use rusqlite::OptionalExtension;

use crate::error::Result;
use crate::{Store, now_secs};

/// What a cached response is a response to.
///
/// The distinction exists to make `[R-STORE-045]` structural: there is no way
/// to cache a generation without naming [`CacheKind::Generation`], and the
/// default path does not reach for it.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CacheKind {
    /// An embedding. Cached by default.
    Embedding,
    /// A generation. Cached only when the caller opts in.
    Generation,
}

impl Store {
    /// Look a response up.
    ///
    /// Satisfies `[R-STORE-046]`: the model is part of the key, so changing a
    /// model cannot return another model's answer. A lookup for a generation
    /// misses unless the caller opted in when writing.
    pub fn cache_get(&self, request_hash: &str, model: &str) -> Result<Option<Vec<u8>>> {
        Ok(self
            .conn()
            .query_row(
                "SELECT response FROM cache WHERE request_hash = ?1 AND model = ?2",
                rusqlite::params![request_hash, model],
                |r| r.get(0),
            )
            .optional()?)
    }

    /// Store a response.
    ///
    /// Satisfies `[R-STORE-045]`. A [`CacheKind::Generation`] entry is written
    /// only when `opted_in` is true; passing false for one is a no-op rather
    /// than an error, so a caller that forwards a flag does not have to branch.
    pub fn cache_put(
        &self,
        request_hash: &str,
        model: &str,
        kind: CacheKind,
        opted_in: bool,
        response: &[u8],
    ) -> Result<bool> {
        if kind == CacheKind::Generation && !opted_in {
            return Ok(false);
        }
        self.conn().execute(
            "INSERT INTO cache(request_hash, model, response, bytes, created_at)
             VALUES (?1, ?2, ?3, ?4, ?5)
             ON CONFLICT(request_hash, model) DO UPDATE SET
                 response = excluded.response,
                 bytes = excluded.bytes,
                 created_at = excluded.created_at",
            rusqlite::params![
                request_hash,
                model,
                response,
                response.len() as i64,
                now_secs()
            ],
        )?;
        Ok(true)
    }

    /// Evict cache entries older than `max_age_secs`, then the oldest
    /// remaining until the cache is under `max_bytes`.
    ///
    /// Satisfies `[R-STORE-047]`. Nothing here touches a session: a session
    /// that quoted a response holds its own copy in the blob table, so
    /// evicting the cache entry cannot change what a transcript says.
    pub fn cache_evict(&self, max_age_secs: i64, max_bytes: i64) -> Result<usize> {
        let conn = self.conn();
        // The bound is inclusive. Timestamps have second resolution, so with a
        // strict comparison an entry written this second survives
        // `max_age_secs = 0`, and "evict everything" becomes inexpressible.
        let cutoff = now_secs() - max_age_secs;
        let mut removed = conn.execute("DELETE FROM cache WHERE created_at <= ?1", [cutoff])?;

        let mut total: i64 =
            conn.query_row("SELECT coalesce(sum(bytes), 0) FROM cache", [], |r| {
                r.get(0)
            })?;
        while total > max_bytes {
            let oldest: Option<(String, String, i64)> = conn
                .query_row(
                    "SELECT request_hash, model, bytes FROM cache ORDER BY created_at LIMIT 1",
                    [],
                    |r| Ok((r.get(0)?, r.get(1)?, r.get(2)?)),
                )
                .optional()?;
            let Some((hash, model, bytes)) = oldest else {
                break;
            };
            conn.execute(
                "DELETE FROM cache WHERE request_hash = ?1 AND model = ?2",
                rusqlite::params![hash, model],
            )?;
            total -= bytes;
            removed += 1;
        }
        Ok(removed)
    }

    /// The total size of the cached responses, in bytes.
    pub fn cache_bytes(&self) -> Result<i64> {
        Ok(self
            .conn()
            .query_row("SELECT coalesce(sum(bytes), 0) FROM cache", [], |r| {
                r.get(0)
            })?)
    }
}
