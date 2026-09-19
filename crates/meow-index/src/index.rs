// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Building an index, keeping it current, and answering a query from it.

use std::path::{Path, PathBuf};

use meow_store::Store;

use crate::ann;
use crate::chunk::Chunking;
use crate::embed::{self, Embed, Rejected};
use crate::error::{IndexError, Result};
use crate::walk::Walk;

/// The key under which the index records which model built it.
const MODEL_KEY: &str = "index.model";

/// How many chunks are embedded before the vectors are written.
///
/// `[R-INDEX-022]` wants an interrupted build to keep what it paid for, and a
/// build that wrote nothing until the end would keep nothing.
const COMMIT_EVERY: usize = 64;

/// What a build or an update did.
///
/// `[R-INDEX-032]`: added, changed, removed, and unchanged. Unchanged is the
/// number that tells somebody the incremental path is working.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Built {
    /// Files that were not in the index.
    pub added: usize,
    /// Files whose content or chunking changed.
    pub changed: usize,
    /// Files that went away or became excluded.
    pub removed: usize,
    /// Files that were already current.
    pub unchanged: usize,
    /// Chunks embedded this time.
    pub embedded: usize,
    /// Files the walk skipped, with the reason already reported by it.
    pub skipped: usize,
}

/// One answer to a query.
#[derive(Debug, Clone, PartialEq)]
pub struct Hit {
    /// Which file, relative to the workspace root.
    pub path: String,
    /// Its first line, from one.
    pub first_line: usize,
    /// Its last line, from one.
    pub last_line: usize,
    /// The chunk itself.
    pub text: String,
    /// How alike it is, from -1 to 1.
    pub score: f32,
}

/// What a query asks for.
#[derive(Debug, Clone)]
pub struct Query {
    /// How many results at most.
    pub limit: usize,
    /// The lowest score worth returning.
    pub min_score: f32,
    /// Globs a result's path must match, when there are any.
    ///
    /// `[R-INDEX-044]`: applied before ranking, so a limit of ten returns the
    /// best ten inside the filter rather than whatever survives it.
    pub paths: Vec<String>,
}

impl Default for Query {
    fn default() -> Self {
        Self {
            limit: 10,
            min_score: 0.0,
            paths: Vec::new(),
        }
    }
}

/// The index over one workspace.
#[derive(Debug)]
pub struct Index {
    store: Store,
    root: PathBuf,
    chunking: Chunking,
    walk: Walk,
}

impl Index {
    /// Open the index of a workspace.
    pub fn new(store: Store, root: impl Into<PathBuf>) -> Self {
        Self {
            store,
            root: root.into(),
            chunking: Chunking::default(),
            walk: Walk::default(),
        }
    }

    /// Use different chunking.
    #[must_use]
    pub fn with_chunking(mut self, chunking: Chunking) -> Self {
        self.chunking = chunking;
        self
    }

    /// Use a different walk.
    #[must_use]
    pub fn with_walk(mut self, walk: Walk) -> Self {
        self.walk = walk;
        self
    }

    /// The store underneath.
    pub fn store(&self) -> &Store {
        &self.store
    }

    /// Which model built this index, if anything did.
    pub fn model(&self) -> Result<Option<String>> {
        Ok(self
            .store
            .kv_get(MODEL_KEY)
            .map_err(store_failed)?
            .and_then(|bytes| String::from_utf8(bytes).ok()))
    }

    /// How many chunks there are, and how many are embedded.
    ///
    /// # Errors
    ///
    /// Whatever the store said.
    pub fn counts(&self) -> Result<(usize, usize)> {
        let (total, embedded) = self.store.index_counts().map_err(store_failed)?;
        Ok((total.max(0) as usize, embedded.max(0) as usize))
    }

    /// Forget everything, leaving the rest of the database alone.
    ///
    /// # Errors
    ///
    /// Whatever the store said.
    pub fn clear(&mut self) -> Result<()> {
        self.store.index_clear().map_err(store_failed)?;
        self.store.kv_delete(MODEL_KEY).map_err(store_failed)?;
        self.graph().forget();
        Ok(())
    }

    /// Bring the index up to date with the workspace.
    ///
    /// Satisfies `[R-INDEX-030]` by re-chunking only a file whose hash
    /// changed, where the hash covers the chunking parameters; `[R-INDEX-031]`
    /// by removing a file that is gone or newly excluded; `[R-INDEX-032]` by
    /// counting all four outcomes; and `[R-INDEX-022]` by leaving the
    /// embedding to whatever runs next, so an interrupted build resumes.
    ///
    /// # Errors
    ///
    /// [`IndexError`] when the walk or the store fails.
    pub fn update(&mut self) -> Result<Built> {
        let walked = self.walk.run(&self.root)?;
        let root = self
            .root
            .canonicalize()
            .unwrap_or_else(|_| self.root.clone());

        let mut built = Built {
            skipped: walked.skipped(),
            ..Built::default()
        };

        let mut seen = Vec::with_capacity(walked.files.len());

        for file in &walked.files {
            let relative = file
                .strip_prefix(&root)
                .unwrap_or(file)
                .to_string_lossy()
                .replace('\\', "/");
            seen.push(relative.clone());

            let text = std::fs::read_to_string(file).map_err(|source| IndexError::Io {
                path: file.clone(),
                source,
            })?;
            let hash = self.fingerprint(&text);

            match self.store.index_hash(&relative).map_err(store_failed)? {
                Some(stored) if stored == hash => {
                    built.unchanged += 1;
                    continue;
                }
                Some(_) => built.changed += 1,
                None => built.added += 1,
            }

            let chunks = self.chunking.split(Path::new(&relative), &text)?;
            let rows: Vec<(i64, i64, i64, i64, String)> = chunks
                .into_iter()
                .map(|c| {
                    (
                        c.first_line as i64,
                        c.last_line as i64,
                        c.start as i64,
                        c.end as i64,
                        c.text,
                    )
                })
                .collect();

            self.store
                .index_put_file(&relative, &hash, &rows)
                .map_err(store_failed)?;
        }

        // [R-INDEX-031]: a file that is gone, or that an ignore rule now
        // excludes, leaves nothing behind. The two cases are the same here,
        // which is right: both mean the walk no longer sees it.
        for stored in self.store.index_paths().map_err(store_failed)? {
            if !seen.contains(&stored) {
                self.store
                    .index_remove_file(&stored)
                    .map_err(store_failed)?;
                built.removed += 1;
            }
        }

        Ok(built)
    }

    /// Embed whatever is still waiting.
    ///
    /// Satisfies `[R-INDEX-022]`: only the chunks with no vector, committed in
    /// batches, so an interrupted build keeps what it paid for.
    ///
    /// # Errors
    ///
    /// [`IndexError::ChunkTooLarge`] naming the file and the lines when one
    /// chunk is refused on its own, per `[R-INDEX-021]`.
    pub fn embed(&mut self, embedder: &dyn Embed, batch: usize) -> Result<usize> {
        self.store
            .kv_put(MODEL_KEY, embedder.model().as_bytes())
            .map_err(store_failed)?;

        let mut done = 0;

        loop {
            let pending = self
                .store
                .index_pending(COMMIT_EVERY as i64)
                .map_err(store_failed)?;
            if pending.is_empty() {
                break;
            }

            let texts: Vec<String> = pending.iter().map(|c| c.text.clone()).collect();
            let vectors = embed::in_batches(embedder, &texts, batch).map_err(|(at, why)| {
                let chunk = &pending[at.min(pending.len() - 1)];
                match why {
                    Rejected::TooLarge => IndexError::ChunkTooLarge {
                        path: PathBuf::from(&chunk.path),
                        first: chunk.first_line.max(0) as usize,
                        last: chunk.last_line.max(0) as usize,
                        chars: chunk.text.chars().count(),
                        limit: self.chunking.max_chars,
                    },
                    Rejected::Failed(message) => IndexError::Walk {
                        root: self.root.clone(),
                        message,
                    },
                }
            })?;

            let stored: Vec<(i64, Vec<u8>)> = pending
                .iter()
                .zip(&vectors)
                .map(|(chunk, vector)| (chunk.id, embed::to_bytes(vector)))
                .collect();

            done += stored.len();
            self.store
                .index_put_vectors(&stored)
                .map_err(store_failed)?;
        }

        Ok(done)
    }

    /// Answer a query.
    ///
    /// Satisfies `[R-INDEX-040]` by ranking on descending similarity with the
    /// path, the lines, the text, and the score; `[R-INDEX-041]` by saying the
    /// index is empty rather than building one; `[R-INDEX-042]` by applying
    /// both the limit and the minimum; `[R-INDEX-044]` by filtering before
    /// ranking; and `[R-INDEX-051]` by refusing when the model that built the
    /// index is not the model asking.
    ///
    /// # Errors
    ///
    /// [`IndexError::Empty`] when there is nothing to search, and
    /// [`IndexError::WrongModel`] when the index was built by another model.
    pub fn query(&self, embedder: &dyn Embed, text: &str, query: &Query) -> Result<Vec<Hit>> {
        let built_by = self.model()?.ok_or(IndexError::Empty)?;
        if built_by != embedder.model() {
            // [R-INDEX-051]: two models put different meanings in the same
            // coordinates, so comparing across them produces numbers that look
            // like scores and are not.
            return Err(IndexError::WrongModel {
                built_by,
                asked_by: embedder.model().to_owned(),
            });
        }

        let (chunks, max_id) = self.store.index_signature().map_err(store_failed)?;
        if chunks == 0 {
            return Err(IndexError::Empty);
        }
        let signature = ann::Signature { chunks, max_id };

        // [R-INDEX-044]: the filter decides which nodes the walk may visit,
        // so it narrows the search rather than narrowing its results. A
        // filter applied afterwards would return the best ten overall and
        // then throw most of them away.
        let allowed = self.allowed(&query.paths)?;

        // The query's own embedding is cached, so asking the same thing twice
        // costs one request. `[R-INDEX-043]`.
        let vector = self.query_vector(embedder, text)?;

        let found = self.nearest(&signature, &vector, query.limit, allowed.as_deref())?;

        // The text is read for the handful of rows that won, not for every
        // row that was scored.
        let ids: Vec<i64> = found.iter().map(|(id, _)| *id).collect();
        let rows = self.store.index_rows(&ids).map_err(store_failed)?;

        let mut hits: Vec<Hit> = found
            .into_iter()
            .filter(|(_, score)| *score >= query.min_score)
            .filter_map(|(id, score)| {
                let row = rows.iter().find(|row| row.id == id)?;
                Some(Hit {
                    path: row.path.clone(),
                    first_line: row.first_line.max(0) as usize,
                    last_line: row.last_line.max(0) as usize,
                    text: row.text.clone(),
                    score,
                })
            })
            .collect();

        hits.sort_by(|a, b| {
            b.score
                .partial_cmp(&a.score)
                // Two equal scores keep a stable order rather than whichever
                // the sort happens to pick, so the same query twice reads the
                // same way.
                .unwrap_or(std::cmp::Ordering::Equal)
                .then_with(|| a.path.cmp(&b.path))
                .then_with(|| a.first_line.cmp(&b.first_line))
        });
        hits.truncate(query.limit);

        Ok(hits)
    }

    /// Search the graph, building it first when it is missing or stale.
    ///
    /// `[R-INDEX-022]`'s neighbour in spirit: the graph is a cache over the
    /// rows, so losing it costs a rebuild and never an answer. A stale one is
    /// noticed by its stamp rather than trusted.
    fn nearest(
        &self,
        signature: &ann::Signature,
        vector: &[f32],
        limit: usize,
        allowed: Option<&[usize]>,
    ) -> Result<Vec<(i64, f32)>> {
        let location = self.graph();

        if !location.is_current(*signature) {
            let vectors = self.store.index_vectors().map_err(store_failed)?;
            ann::build(&vectors, *signature, &location)?;
        }

        match ann::nearest(&location, vector, limit, allowed) {
            Ok(found) => Ok(found),
            // A graph that will not load is a cache that went bad, and the
            // rows it was built from are still here. One rebuild, then the
            // failure is real.
            Err(_) => {
                let vectors = self.store.index_vectors().map_err(store_failed)?;
                ann::build(&vectors, *signature, &location)?;
                ann::nearest(&location, vector, limit, allowed)
            }
        }
    }

    /// Where the graph lives.
    fn graph(&self) -> ann::Location {
        ann::Location::new(self.root.join(".meow").join(".data"))
    }

    /// Which nodes a path filter allows, as graph node positions.
    ///
    /// `None` when there is no filter, which lets the search skip the check
    /// entirely rather than pass one that always says yes.
    fn allowed(&self, globs: &[String]) -> Result<Option<Vec<usize>>> {
        let Some(matcher) = self.matcher(globs)? else {
            return Ok(None);
        };

        // Node positions follow the order `index_vectors` returns, which is by
        // identifier, and `index_id_paths` uses the same order. The two lists
        // line up because both are `ORDER BY id` over the same rows.
        let allowed = self
            .store
            .index_id_paths()
            .map_err(store_failed)?
            .into_iter()
            .enumerate()
            .filter(|(_, (_, path))| matcher.is_match(path))
            .map(|(node, _)| node)
            .collect();

        Ok(Some(allowed))
    }

    /// The embedding of a query, from the cache when it is there.
    fn query_vector(&self, embedder: &dyn Embed, text: &str) -> Result<Vec<f32>> {
        let key = meow_store::BlobHash::of(text.as_bytes());

        if let Some(bytes) = self
            .store
            .cache_get(key.as_str(), embedder.model())
            .map_err(store_failed)?
            && let Some(vector) = embed::from_bytes(&bytes)
        {
            return Ok(vector);
        }

        let vectors = embedder
            .embed(std::slice::from_ref(&text.to_owned()))
            .map_err(|why| IndexError::Walk {
                root: self.root.clone(),
                message: why.to_string(),
            })?;
        let vector = vectors.into_iter().next().unwrap_or_default();

        self.store
            .cache_put(
                key.as_str(),
                embedder.model(),
                meow_store::CacheKind::Embedding,
                true,
                &embed::to_bytes(&vector),
            )
            .map_err(store_failed)?;

        Ok(vector)
    }

    /// Compile the path filter, when there is one.
    fn matcher(&self, globs: &[String]) -> Result<Option<globset::GlobSet>> {
        if globs.is_empty() {
            return Ok(None);
        }
        let mut builder = globset::GlobSetBuilder::new();
        for pattern in globs {
            let glob = globset::Glob::new(pattern).map_err(|e| IndexError::Walk {
                root: self.root.clone(),
                message: format!("`{pattern}` is not a valid glob: {e}"),
            })?;
            builder.add(glob);
        }
        builder.build().map(Some).map_err(|e| IndexError::Walk {
            root: self.root.clone(),
            message: e.to_string(),
        })
    }

    /// What decides whether a file is stale.
    ///
    /// `[R-INDEX-030]`: the content and the chunking parameters together.
    /// Hashing the content alone would leave chunks that look current after
    /// somebody changed the chunk size, and those chunks answer queries.
    fn fingerprint(&self, text: &str) -> String {
        let mut material = self.chunking.fingerprint();
        material.push('\n');
        material.push_str(text);
        meow_store::BlobHash::of(material.as_bytes())
            .as_str()
            .to_owned()
    }
}

fn store_failed(e: meow_store::StoreError) -> IndexError {
    IndexError::Store(e.to_string())
}
