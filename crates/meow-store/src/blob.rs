// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Content-addressed payloads.
//!
//! Every payload is addressed by the BLAKE3 hash of its content, so an agent
//! that reads the same file at three steps stores it once.

use rusqlite::OptionalExtension;

use crate::Store;
use crate::error::{Result, StoreError};

/// The address of a payload: the BLAKE3 hash of its bytes, in lower-case hex.
#[derive(Debug, Clone, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct BlobHash(String);

impl BlobHash {
    /// Hash a payload.
    pub fn of(bytes: &[u8]) -> Self {
        Self(blake3::hash(bytes).to_hex().to_string())
    }

    /// The hash as hex.
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl std::fmt::Display for BlobHash {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}

impl Store {
    /// Store a payload and return its address.
    ///
    /// Satisfies `[R-STORE-010]`, `[R-STORE-011]`, and `[R-STORE-012]`.
    /// Writing a payload that is already present stores no second copy and
    /// does not fail; it raises the reference count, because a second referent
    /// now exists.
    ///
    /// `[R-STORE-010]` permits inlining a payload under
    /// [`crate::INLINE_LIMIT`] rather than putting it in the blob table, and
    /// this store does not take that permission: every payload goes in the
    /// same table. A `BLOB` column already stores small values in the row
    /// SQLite writes anyway, so the second path would buy nothing and cost a
    /// branch on every read. The requirement is satisfied either way, because
    /// it asks that the choice be invisible.
    pub fn put_blob(&self, bytes: &[u8]) -> Result<BlobHash> {
        let hash = BlobHash::of(bytes);
        self.conn().execute(
            "INSERT INTO blobs(hash, data, refcount, bytes) VALUES (?1, ?2, 1, ?3)
             ON CONFLICT(hash) DO UPDATE SET refcount = refcount + 1",
            rusqlite::params![hash.as_str(), bytes, bytes.len() as i64],
        )?;
        Ok(hash)
    }

    /// Read a payload back.
    ///
    /// Satisfies `[R-STORE-013]`: a hash with no row is an error naming the
    /// hash. Returning empty content would let a lost payload read as an empty
    /// one, which is the failure a content-addressed store exists to prevent.
    pub fn get_blob(&self, hash: &BlobHash) -> Result<Vec<u8>> {
        self.conn()
            .query_row(
                "SELECT data FROM blobs WHERE hash = ?1",
                [hash.as_str()],
                |r| r.get(0),
            )
            .optional()?
            .ok_or_else(|| StoreError::BlobMissing {
                hash: hash.to_string(),
            })
    }

    /// How many distinct payloads the store holds.
    ///
    /// Retention and `meow doctor` both want it, which is why it is API
    /// rather than a test helper.
    pub fn blob_count(&self) -> Result<i64> {
        Ok(self
            .conn()
            .query_row("SELECT count(*) FROM blobs", [], |r| r.get(0))?)
    }

    /// How many referents a payload has.
    pub fn blob_refcount(&self, hash: &BlobHash) -> Result<i64> {
        Ok(self
            .conn()
            .query_row(
                "SELECT refcount FROM blobs WHERE hash = ?1",
                [hash.as_str()],
                |r| r.get(0),
            )
            .optional()?
            .unwrap_or(0))
    }

    /// Drop one referent, deleting the payload when the last one goes.
    ///
    /// Satisfies `[R-STORE-012]`.
    pub fn release_blob(&self, hash: &BlobHash) -> Result<()> {
        let conn = self.conn();
        conn.execute(
            "UPDATE blobs SET refcount = refcount - 1 WHERE hash = ?1 AND refcount > 0",
            [hash.as_str()],
        )?;
        conn.execute(
            "DELETE FROM blobs WHERE hash = ?1 AND refcount <= 0",
            [hash.as_str()],
        )?;
        Ok(())
    }

    /// Store many payloads, committing in batches.
    ///
    /// Satisfies `[R-STORE-024]`: an index build writes tens of thousands of
    /// chunks, and holding the one write lock for the whole of it would block
    /// every agent run in the workspace. Committing every [`crate::BULK_BATCH`]
    /// rows lets another writer in between batches.
    pub fn put_blobs_bulk(&mut self, payloads: &[Vec<u8>]) -> Result<Vec<BlobHash>> {
        let mut out = Vec::with_capacity(payloads.len());
        for batch in payloads.chunks(crate::BULK_BATCH) {
            let tx = self.conn_mut().transaction()?;
            for bytes in batch {
                let hash = BlobHash::of(bytes);
                tx.execute(
                    "INSERT INTO blobs(hash, data, refcount, bytes) VALUES (?1, ?2, 1, ?3)
                     ON CONFLICT(hash) DO UPDATE SET refcount = refcount + 1",
                    rusqlite::params![hash.as_str(), bytes, bytes.len() as i64],
                )?;
                out.push(hash);
            }
            tx.commit()?;
        }
        Ok(out)
    }
}
