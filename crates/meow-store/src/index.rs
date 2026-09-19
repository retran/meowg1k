// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The rows the index writes.
//!
//! `[R-INDEX-050]`: in the same database as sessions, the cache, and the
//! blobs. v0.2.x kept the index in a second SQLite file, which meant two
//! files to keep in step and no way for a chunk and a tool result quoting the
//! same file to share anything.

use rusqlite::OptionalExtension;

use crate::error::Result;
use crate::{Store, now_secs};

/// One chunk, as the store holds it.
#[derive(Debug, Clone)]
pub struct ChunkRow {
    /// Its row identifier.
    pub id: i64,
    /// Which file it came from, relative to the workspace root.
    pub path: String,
    /// Its first line, from one.
    pub first_line: i64,
    /// Its last line, from one.
    pub last_line: i64,
    /// Where it starts in the file, in bytes.
    pub start_byte: i64,
    /// Where it ends, in bytes, exclusive.
    pub end_byte: i64,
    /// Its text.
    pub text: String,
    /// Its embedding, or `None` when it has not been embedded yet.
    pub vector: Option<Vec<u8>>,
}

/// What an update did to one file.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FileChange {
    /// It was not in the index.
    Added,
    /// It was, and its hash differs.
    Changed,
    /// It was, and its hash is the same.
    Unchanged,
}

impl Store {
    /// The hash recorded for a file, if it is in the index.
    pub fn index_hash(&self, path: &str) -> Result<Option<String>> {
        Ok(self
            .conn()
            .query_row(
                "SELECT hash FROM index_files WHERE path = ?1",
                [path],
                |r| r.get(0),
            )
            .optional()?)
    }

    /// Every path the index holds.
    pub fn index_paths(&self) -> Result<Vec<String>> {
        let conn = self.conn();
        let mut stmt = conn.prepare("SELECT path FROM index_files ORDER BY path")?;
        let rows = stmt.query_map([], |r| r.get::<_, String>(0))?;
        rows.collect::<std::result::Result<_, _>>()
            .map_err(Into::into)
    }

    /// Replace a file's chunks with the ones given.
    ///
    /// One transaction: a file whose old chunks are gone and whose new ones
    /// are not yet there would answer a query with nothing and look current
    /// while doing it.
    ///
    /// # Errors
    ///
    /// Whatever SQLite said.
    pub fn index_put_file(
        &mut self,
        path: &str,
        hash: &str,
        chunks: &[(i64, i64, i64, i64, String)],
    ) -> Result<()> {
        let tx = self.conn_mut().transaction()?;
        tx.execute("DELETE FROM index_chunks WHERE path = ?1", [path])?;
        tx.execute(
            "INSERT INTO index_files(path, hash, chunks, at) VALUES (?1, ?2, ?3, ?4)
             ON CONFLICT(path) DO UPDATE SET
                 hash = excluded.hash, chunks = excluded.chunks, at = excluded.at",
            rusqlite::params![path, hash, chunks.len() as i64, now_secs()],
        )?;
        for (first_line, last_line, start_byte, end_byte, text) in chunks {
            tx.execute(
                "INSERT INTO index_chunks(path, first_line, last_line, start_byte, end_byte, text)
                 VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
                rusqlite::params![path, first_line, last_line, start_byte, end_byte, text],
            )?;
        }
        tx.commit()?;
        Ok(())
    }

    /// Forget a file and its chunks.
    pub fn index_remove_file(&self, path: &str) -> Result<()> {
        self.conn()
            .execute("DELETE FROM index_files WHERE path = ?1", [path])?;
        Ok(())
    }

    /// Chunks that still need embedding.
    ///
    /// `[R-INDEX-022]`: what makes a build resumable is that this returns only
    /// the work that is left, so an interrupted run does not pay twice.
    pub fn index_pending(&self, limit: i64) -> Result<Vec<ChunkRow>> {
        self.rows(
            "SELECT id, path, first_line, last_line, start_byte, end_byte, text, vector
             FROM index_chunks WHERE vector IS NULL ORDER BY id LIMIT ?1",
            rusqlite::params![limit],
        )
    }

    /// Every embedded chunk's identifier and vector, and nothing else.
    ///
    /// Without the text, because building the search structure needs the
    /// coordinates and not the content, and the content is most of the bytes.
    pub fn index_vectors(&self) -> Result<Vec<(i64, Vec<u8>)>> {
        let conn = self.conn();
        let mut stmt = conn
            .prepare("SELECT id, vector FROM index_chunks WHERE vector IS NOT NULL ORDER BY id")?;
        let rows = stmt.query_map([], |r| Ok((r.get(0)?, r.get(1)?)))?;
        rows.collect::<std::result::Result<_, _>>()
            .map_err(Into::into)
    }

    /// Every embedded chunk's identifier and path.
    ///
    /// For a path filter, which has to be known before a search starts rather
    /// than applied to what it returned.
    pub fn index_id_paths(&self) -> Result<Vec<(i64, String)>> {
        let conn = self.conn();
        let mut stmt =
            conn.prepare("SELECT id, path FROM index_chunks WHERE vector IS NOT NULL ORDER BY id")?;
        let rows = stmt.query_map([], |r| Ok((r.get(0)?, r.get(1)?)))?;
        rows.collect::<std::result::Result<_, _>>()
            .map_err(Into::into)
    }

    /// The chunks with these identifiers.
    ///
    /// The text is read here, for the handful of rows a query actually
    /// returns, rather than for every row it scored.
    pub fn index_rows(&self, ids: &[i64]) -> Result<Vec<ChunkRow>> {
        if ids.is_empty() {
            return Ok(Vec::new());
        }
        let places = std::iter::repeat_n("?", ids.len())
            .collect::<Vec<_>>()
            .join(",");
        let sql = format!(
            "SELECT id, path, first_line, last_line, start_byte, end_byte, text, vector
             FROM index_chunks WHERE id IN ({places})"
        );
        self.rows(&sql, rusqlite::params_from_iter(ids))
    }

    /// What the index looked like when it was last searched.
    ///
    /// The count and the highest identifier. Identifiers are never reused, so
    /// re-chunking a file raises the highest one and deleting a file lowers
    /// the count: between them, any change to what is searchable shows up
    /// here, which is what decides whether the search structure is stale.
    pub fn index_signature(&self) -> Result<(i64, i64)> {
        Ok(self.conn().query_row(
            "SELECT count(*), coalesce(max(id), 0) FROM index_chunks WHERE vector IS NOT NULL",
            [],
            |r| Ok((r.get(0)?, r.get(1)?)),
        )?)
    }

    fn rows(&self, sql: &str, params: impl rusqlite::Params) -> Result<Vec<ChunkRow>> {
        let conn = self.conn();
        let mut stmt = conn.prepare(sql)?;
        let rows = stmt.query_map(params, |r| {
            Ok(ChunkRow {
                id: r.get(0)?,
                path: r.get(1)?,
                first_line: r.get(2)?,
                last_line: r.get(3)?,
                start_byte: r.get(4)?,
                end_byte: r.get(5)?,
                text: r.get(6)?,
                vector: r.get(7)?,
            })
        })?;
        rows.collect::<std::result::Result<_, _>>()
            .map_err(Into::into)
    }

    /// Store the embeddings of several chunks.
    ///
    /// # Errors
    ///
    /// Whatever SQLite said.
    pub fn index_put_vectors(&mut self, vectors: &[(i64, Vec<u8>)]) -> Result<()> {
        let tx = self.conn_mut().transaction()?;
        for (id, vector) in vectors {
            tx.execute(
                "UPDATE index_chunks SET vector = ?1 WHERE id = ?2",
                rusqlite::params![vector, id],
            )?;
        }
        tx.commit()?;
        Ok(())
    }

    /// How many chunks there are, and how many are embedded.
    pub fn index_counts(&self) -> Result<(i64, i64)> {
        Ok(self.conn().query_row(
            "SELECT count(*), coalesce(sum(vector IS NOT NULL), 0) FROM index_chunks",
            [],
            |r| Ok((r.get(0)?, r.get(1)?)),
        )?)
    }

    /// Remove every chunk and every vector.
    ///
    /// `[R-INDEX-052]`: the index and nothing else. Sessions and the
    /// key-value store are in the same database and are not the index's to
    /// delete.
    ///
    /// # Errors
    ///
    /// Whatever SQLite said.
    pub fn index_clear(&mut self) -> Result<()> {
        let tx = self.conn_mut().transaction()?;
        tx.execute("DELETE FROM index_chunks", [])?;
        tx.execute("DELETE FROM index_files", [])?;
        tx.commit()?;
        Ok(())
    }
}
