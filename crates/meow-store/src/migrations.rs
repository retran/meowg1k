// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Forward-only schema migrations.
//!
//! `[R-STORE-005]` forbids a downgrade path, so this module has no way to
//! express one: a migration is a version and the SQL that reaches it.

use rusqlite::Connection;

use crate::error::{Result, StoreError};

/// One step from the previous schema version to this one.
struct Migration {
    version: u32,
    sql: &'static str,
}

/// Every migration, in ascending order. Append only; never edit one that has
/// shipped, because a database somewhere has already applied it.
const MIGRATIONS: &[Migration] = &[Migration {
    version: 1,
    sql: r#"
        CREATE TABLE meta (
            key   TEXT PRIMARY KEY,
            value TEXT NOT NULL
        ) STRICT;

        -- Content-addressed payloads. `data` holds the bytes whether the
        -- payload was inlined or not, so [R-STORE-010]'s "invisible to a
        -- reader" is structural rather than a rule to remember.
        CREATE TABLE blobs (
            hash     TEXT PRIMARY KEY,
            data     BLOB NOT NULL,
            refcount INTEGER NOT NULL DEFAULT 0,
            bytes    INTEGER NOT NULL
        ) STRICT;

        CREATE TABLE sessions (
            id         TEXT PRIMARY KEY,
            name       TEXT UNIQUE,
            agent      TEXT NOT NULL,
            parent_id  TEXT REFERENCES sessions(id) ON DELETE RESTRICT,
            created_at INTEGER NOT NULL,
            heartbeat  INTEGER
        ) STRICT;

        CREATE TABLE events (
            session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
            seq        INTEGER NOT NULL,
            at         INTEGER NOT NULL,
            kind       TEXT NOT NULL,
            payload    TEXT REFERENCES blobs(hash),
            PRIMARY KEY (session_id, seq)
        ) STRICT;

        -- [R-STORE-031]: the workspace store is its own table, so deleting a
        -- session cannot touch it.
        CREATE TABLE kv (
            key   TEXT PRIMARY KEY,
            value BLOB NOT NULL
        ) STRICT;

        CREATE TABLE cache (
            request_hash TEXT NOT NULL,
            model        TEXT NOT NULL,
            response     BLOB NOT NULL,
            bytes        INTEGER NOT NULL,
            created_at   INTEGER NOT NULL,
            PRIMARY KEY (request_hash, model)
        ) STRICT;

        CREATE INDEX events_by_session ON events(session_id, seq);
        CREATE INDEX cache_by_age ON cache(created_at);
    "#,
}];

/// The highest version this binary knows how to reach.
pub(crate) fn latest_version() -> u32 {
    MIGRATIONS.last().map_or(0, |m| m.version)
}

/// Read the version recorded in the file, or zero for a fresh database.
fn recorded_version(conn: &Connection) -> Result<u32> {
    let has_meta: bool = conn.query_row(
        "SELECT count(*) > 0 FROM sqlite_master WHERE type = 'table' AND name = 'meta'",
        [],
        |r| r.get(0),
    )?;
    if !has_meta {
        return Ok(0);
    }
    let v: Option<String> = conn
        .query_row(
            "SELECT value FROM meta WHERE key = 'schema_version'",
            [],
            |r| r.get(0),
        )
        .ok();
    Ok(v.and_then(|s| s.parse().ok()).unwrap_or(0))
}

/// Bring the database up to [`latest_version`].
///
/// Satisfies `[R-STORE-004]`: migrations run in ascending order, each in its
/// own transaction, so a failure leaves the database at the version it had
/// before that migration rather than part-way through it. Satisfies
/// `[R-STORE-006]`: a database from the future is an error, and the file is
/// not touched.
pub(crate) fn migrate(conn: &mut Connection, path: &std::path::Path) -> Result<u32> {
    let current = recorded_version(conn)?;
    let latest = latest_version();

    if current > latest {
        return Err(StoreError::SchemaTooNew {
            path: path.to_path_buf(),
            found: current,
            supported: latest,
        });
    }

    for m in MIGRATIONS.iter().filter(|m| m.version > current) {
        let tx = conn.transaction()?;
        tx.execute_batch(m.sql)
            .map_err(|source| StoreError::Migration {
                version: m.version,
                source,
            })?;
        tx.execute(
            "INSERT INTO meta(key, value) VALUES ('schema_version', ?1)
             ON CONFLICT(key) DO UPDATE SET value = excluded.value",
            [m.version.to_string()],
        )
        .map_err(|source| StoreError::Migration {
            version: m.version,
            source,
        })?;
        tx.commit().map_err(|source| StoreError::Migration {
            version: m.version,
            source,
        })?;
    }

    Ok(latest)
}
