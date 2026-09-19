// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The one SQLite database a meowg1k workspace keeps.
//!
//! The store persists the session event log, the content-addressed blobs those
//! events point at, the workspace key-value store, and the response cache. It
//! does not interpret what it stores: deciding that a compaction supersedes a
//! range is the session layer's job, and the store only guarantees the event is
//! written, ordered, and still there later.
//!
//! `docs/spec/store.md` is normative. Every requirement it states is cited by
//! the code that satisfies it and by at least one test.

mod blob;
mod cache;
mod error;
mod kv;
mod migrations;
mod rows;
mod session;

use std::path::{Path, PathBuf};

use rusqlite::Connection;

pub use crate::blob::BlobHash;
pub use crate::cache::CacheKind;
pub use crate::error::{Result, StoreError};
pub use crate::rows::{AgedSession, CompactionRow, EventRow, UsageTotals};
pub use crate::session::SessionRow;

/// How long a blocked write waits before giving up.
///
/// `[R-STORE-023]` requires at least five seconds. Ten leaves room for a slow
/// filesystem without making a genuine deadlock look like a hang forever.
const BUSY_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(10);

/// The size below which `[R-STORE-010]` permits inlining a payload instead of
/// putting it in the blob table. Kept as the documented boundary; see
/// [`Store::put_blob`] for why this store does not use the permission.
pub const INLINE_LIMIT: usize = 512;

/// The number of rows a bulk write commits at a time.
///
/// `[R-STORE-024]` forbids holding the single write lock for the length of an
/// index build, so a bulk write commits in batches this size and lets another
/// writer in between them.
pub const BULK_BATCH: usize = 256;

/// The database for one workspace.
#[derive(Debug)]
pub struct Store {
    conn: Connection,
    path: PathBuf,
}

impl Store {
    /// Open, or create, the database for a workspace root.
    ///
    /// Satisfies `[R-STORE-001]`: one file, at `.meow/.data/meow.db` beneath
    /// the workspace root.
    pub fn open(workspace_root: &Path) -> Result<Self> {
        let dir = workspace_root.join(".meow").join(".data");
        std::fs::create_dir_all(&dir).map_err(|source| StoreError::Io {
            path: dir.clone(),
            source,
        })?;
        Self::open_at(&dir.join("meow.db"))
    }

    /// Open, or create, a database at an exact path.
    pub fn open_at(path: &Path) -> Result<Self> {
        let mut conn = Connection::open(path)?;
        conn.busy_timeout(BUSY_TIMEOUT)?;

        // [R-STORE-002]. Write-ahead logging gives one writer alongside many
        // readers; NORMAL loses at most the last transaction on a power
        // failure, which costs a rerun of one turn and keeps the write path
        // off the agent's critical path.
        conn.pragma_update(None, "journal_mode", "WAL")?;
        conn.pragma_update(None, "synchronous", "NORMAL")?;
        conn.pragma_update(None, "foreign_keys", true)?;

        let version = migrations::migrate(&mut conn, path)?;
        let store = Self {
            conn,
            path: path.to_path_buf(),
        };
        store.restrict_permissions()?;
        debug_assert_eq!(version, migrations::latest_version());
        Ok(store)
    }

    /// Where this database lives.
    pub fn path(&self) -> &Path {
        &self.path
    }

    /// The schema version the file is now at.
    pub fn schema_version(&self) -> Result<u32> {
        Ok(self
            .conn
            .query_row(
                "SELECT value FROM meta WHERE key = 'schema_version'",
                [],
                |r| r.get::<_, String>(0),
            )?
            .parse()
            .unwrap_or(0))
    }

    /// Whether the database is encrypted.
    ///
    /// Always false, and said out loud rather than left to assume, which is
    /// what `[R-STORE-007]` asks of `meow doctor`.
    pub fn is_encrypted(&self) -> bool {
        false
    }

    /// The total size of the database and its side files, in bytes.
    ///
    /// Satisfies `[R-STORE-042]`, so retention can act on size. The
    /// write-ahead log counts, because it is disk a user paid for.
    pub fn size_bytes(&self) -> Result<u64> {
        let mut total = 0;
        for suffix in ["", "-wal", "-shm"] {
            let mut p = self.path.clone().into_os_string();
            p.push(suffix);
            if let Ok(meta) = std::fs::metadata(PathBuf::from(p)) {
                total += meta.len();
            }
        }
        Ok(total)
    }

    /// Borrow the connection. Crate-internal: every public operation goes
    /// through a named method so that no caller can start a transaction that
    /// outlives a single write.
    pub(crate) fn conn(&self) -> &Connection {
        &self.conn
    }

    /// Borrow the connection mutably, for the few operations that need a
    /// transaction of their own.
    pub(crate) fn conn_mut(&mut self) -> &mut Connection {
        &mut self.conn
    }

    /// Make the database readable and writable by its owner only.
    ///
    /// Satisfies `[R-STORE-008]`. This is not encryption; it is the cheap part
    /// that stops every other account on the machine from reading a
    /// transcript.
    #[cfg(unix)]
    fn restrict_permissions(&self) -> Result<()> {
        use std::os::unix::fs::PermissionsExt;
        for suffix in ["", "-wal", "-shm"] {
            let mut p = self.path.clone().into_os_string();
            p.push(suffix);
            let p = PathBuf::from(p);
            if p.exists() {
                std::fs::set_permissions(&p, std::fs::Permissions::from_mode(0o600))
                    .map_err(|source| StoreError::Io { path: p, source })?;
            }
        }
        Ok(())
    }

    /// Windows inherits the user profile's access control list, which already
    /// restricts the file to its owner, so there is nothing to set.
    #[cfg(not(unix))]
    fn restrict_permissions(&self) -> Result<()> {
        Ok(())
    }
}

/// Seconds since the Unix epoch, for the timestamp columns.
pub(crate) fn now_secs() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_or(0, |d| d.as_secs() as i64)
}
