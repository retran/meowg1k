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
const MIGRATIONS: &[Migration] = &[
    Migration {
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
    },
    Migration {
        version: 2,
        sql: r#"
        -- The event's own fields, as JSON. Small and opaque to the store:
        -- meow-session owns what the shape means.
        ALTER TABLE events ADD COLUMN body TEXT NOT NULL DEFAULT '{}';

        -- [R-SESSION-020] wants these summable without parsing a string, and
        -- [R-SESSION-022] sums them across a session tree, so they are columns
        -- rather than part of the JSON body. cost_micros is nullable because
        -- [R-SESSION-021] forbids recording an unpriced model as free.
        CREATE TABLE usage (
            session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
            seq         INTEGER NOT NULL,
            prompt      INTEGER NOT NULL,
            completion  INTEGER NOT NULL,
            cached      INTEGER,
            cost_micros INTEGER,
            PRIMARY KEY (session_id, seq)
        ) STRICT;

        -- [R-SESSION-011] and [R-SESSION-013] both ask which ranges are
        -- superseded, on every rebuild, so the range is queryable.
        CREATE TABLE compactions (
            session_id   TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
            seq          INTEGER NOT NULL,
            from_seq     INTEGER NOT NULL,
            to_seq       INTEGER NOT NULL,
            tokens_saved INTEGER NOT NULL,
            PRIMARY KEY (session_id, seq)
        ) STRICT;

        -- A rebuildable copy of the state, which [R-SESSION-041] permits so
        -- that listing a thousand sessions does not read a thousand events.
        -- The log wins on any disagreement.
        ALTER TABLE sessions ADD COLUMN state TEXT;
    "#,
    },
    Migration {
        version: 3,
        sql: r#"
        -- [R-SESSION-052] asks a fork to record where it came from, so a
        -- reader of a forked session can find the run it branched off and the
        -- point it branched at. Two columns rather than a JSON field, because
        -- `meow session list` shows them and a list should not parse.
        ALTER TABLE sessions ADD COLUMN origin_id TEXT REFERENCES sessions(id) ON DELETE SET NULL;
        ALTER TABLE sessions ADD COLUMN origin_seq INTEGER;
    "#,
    },
    Migration {
        version: 4,
        sql: r#"
        -- [R-INDEX-050] puts the vectors in the same database as everything
        -- else, which is what lets a chunk share the blob table with a tool
        -- result quoting the same file.

        -- One row per indexed file. The hash covers the chunking parameters
        -- as well as the content, per [R-INDEX-030], so changing the chunk
        -- size does not leave chunks that look current.
        CREATE TABLE index_files (
            path    TEXT PRIMARY KEY,
            hash    TEXT NOT NULL,
            chunks  INTEGER NOT NULL,
            at      INTEGER NOT NULL
        ) STRICT;

        -- One row per chunk. `vector` is null until the chunk is embedded,
        -- which is what makes a build resumable under [R-INDEX-022]: an
        -- interrupted run finds its own work half done and does the rest.
        CREATE TABLE index_chunks (
            id         INTEGER PRIMARY KEY AUTOINCREMENT,
            path       TEXT NOT NULL REFERENCES index_files(path) ON DELETE CASCADE,
            first_line INTEGER NOT NULL,
            last_line  INTEGER NOT NULL,
            start_byte INTEGER NOT NULL,
            end_byte   INTEGER NOT NULL,
            text       TEXT NOT NULL,
            vector     BLOB
        ) STRICT;

        CREATE INDEX index_chunks_path ON index_chunks(path);
        CREATE INDEX index_chunks_pending ON index_chunks(vector) WHERE vector IS NULL;
    "#,
    },
];

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
