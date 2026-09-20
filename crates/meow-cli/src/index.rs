// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The index behind `meow index` and `search.code`.

use std::sync::Arc;

use meow_index::{Chunking, Embed, Index, Query, Rejected, Walk};
use meow_llm::Provider;
use meow_star::port::{Found, Indexed, Search, Stats};
use meow_star::{Registry, Workspace};

/// An embedder over a provider, on a runtime the caller owns.
///
/// `Embed` is synchronous because the index is, and the provider is async
/// because the network is. The handle is where the two meet, and it is the
/// same bridge a Starlark builtin uses: block the calling thread, never hand
/// anybody a future.
pub struct Embedder {
    provider: Arc<dyn Provider>,
    handle: tokio::runtime::Handle,
    model: String,
}

impl std::fmt::Debug for Embedder {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Embedder")
            .field("model", &self.model)
            .finish_non_exhaustive()
    }
}

impl Embedder {
    /// Embed with this provider, calling the model by this name.
    pub fn new(
        provider: Arc<dyn Provider>,
        handle: tokio::runtime::Handle,
        model: impl Into<String>,
    ) -> Self {
        Self {
            provider,
            handle,
            model: model.into(),
        }
    }
}

impl Embed for Embedder {
    fn model(&self) -> &str {
        &self.model
    }

    fn embed(&self, texts: &[String]) -> Result<Vec<Vec<f32>>, Rejected> {
        let cancel = tokio_util::sync::CancellationToken::new();
        self.handle
            .block_on(self.provider.embed(texts, &cancel))
            .map_err(|e| match &e {
                // The batch splitter acts on this one, so a provider saying
                // the request was too big has to arrive as itself rather than
                // as a message the splitter cannot read.
                meow_llm::LlmError::Http { status, .. } if *status == 413 => Rejected::TooLarge,
                _ => Rejected::Failed(e.to_string()),
            })
    }
}

/// How a workspace says it is indexed, turned into what the index takes.
///
/// # Errors
///
/// A message naming what is missing, per `[R-STAR-035]`: a workspace that
/// declares no index gets told to declare one rather than having a model
/// chosen for it.
pub fn configure(registry: &Registry) -> Result<(String, Chunking, Walk), String> {
    let declared = registry.index().ok_or_else(|| {
        "this workspace declares no index; add `meow.index(model = ...)` to .meow/meow.star"
            .to_owned()
    })?;

    let model = registry
        .model(&declared.model)
        .ok_or_else(|| format!("`{}` is not a declared model", declared.model))?;

    let defaults = Chunking::default();
    let chunking = Chunking {
        lines: declared.chunk_lines.map_or(defaults.lines, |n| n as usize),
        overlap: declared.overlap.map_or(defaults.overlap, |n| n as usize),
        max_chars: defaults.max_chars,
    };
    let walk = Walk {
        max_bytes: declared.max_bytes.unwrap_or(Walk::default().max_bytes),
    };

    Ok((model.id.clone(), chunking, walk))
}

/// Open the index of a workspace.
///
/// # Errors
///
/// Whatever opening the database said.
pub fn open(
    workspace: &Workspace,
    chunking: Chunking,
    walk: Walk,
) -> Result<Index, meow_store::StoreError> {
    let store = meow_store::Store::open(workspace.root())?;
    Ok(Index::new(store, workspace.root())
        .with_chunking(chunking)
        .with_walk(walk))
}

/// `@std//search` and `@std//index`, over an index and an embedder.
pub struct Searcher {
    /// The lock is not for contention. `rusqlite::Connection` is not `Sync`,
    /// and the port is called from whichever thread a handler or a tool
    /// happens to be on.
    index: std::sync::Mutex<Index>,
    embedder: Embedder,
    /// `[R-STAR-019]`: `search.text` and `search.files` need no index, but
    /// they must reach the same files it reaches, so they share its walk.
    walk: Walk,
    root: std::path::PathBuf,
}

impl std::fmt::Debug for Searcher {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Searcher").finish_non_exhaustive()
    }
}

impl Searcher {
    /// Search this index with this embedder, walking from this root.
    pub fn new(
        index: Index,
        embedder: Embedder,
        walk: Walk,
        root: impl Into<std::path::PathBuf>,
    ) -> Self {
        Self {
            index: std::sync::Mutex::new(index),
            embedder,
            walk,
            root: root.into(),
        }
    }

    /// The index, or what to tell the script about why it is not usable.
    /// The index-free half of this port.
    fn walker(&self) -> Walker {
        Walker::new(self.walk, self.root.clone())
    }

    fn held(&self) -> Result<std::sync::MutexGuard<'_, Index>, String> {
        self.index
            .lock()
            .map_err(|_| "the index is not usable in this run".to_owned())
    }
}

/// `[R-STAR-024]`: counts, not text.
fn counted(built: &meow_index::Built, files: usize) -> Indexed {
    Indexed {
        files,
        added: built.added,
        changed: built.changed,
        removed: built.removed,
        embedded: built.embedded,
    }
}

impl Search for Searcher {
    fn code(&self, query: &str, limit: usize, paths: &[String]) -> Result<Vec<Found>, String> {
        self.query(query, limit, paths, 0.0)
    }

    fn query(
        &self,
        query: &str,
        limit: usize,
        paths: &[String],
        min_score: f32,
    ) -> Result<Vec<Found>, String> {
        let index = self.held()?;
        index
            .query(
                &self.embedder,
                query,
                &Query {
                    limit,
                    min_score,
                    paths: paths.to_vec(),
                },
            )
            .map(|hits| {
                hits.into_iter()
                    .map(|hit| Found {
                        path: hit.path,
                        first_line: hit.first_line,
                        last_line: hit.last_line,
                        text: hit.text,
                        score: hit.score,
                    })
                    .collect()
            })
            .map_err(|e| e.to_string())
    }

    fn update(&self) -> Result<Indexed, String> {
        let mut index = self.held()?;
        let built = index.update().map_err(|e| e.to_string())?;
        let files = built.added + built.changed + built.unchanged;
        Ok(counted(&built, files))
    }

    fn build(&self) -> Result<Indexed, String> {
        let mut index = self.held()?;
        let built = index.update().map_err(|e| e.to_string())?;
        let files = built.added + built.changed + built.unchanged;
        let embedded = index
            .embed(&self.embedder, meow_index::embed::DEFAULT_BATCH)
            .map_err(|e| e.to_string())?;
        let mut done = counted(&built, files);
        // `update` reports what it embedded, which is nothing; the count that
        // matters to a handler is what this call embedded.
        done.embedded = embedded;
        Ok(done)
    }

    fn stats(&self) -> Result<Stats, String> {
        let index = self.held()?;
        let (chunks, embedded) = index.counts().map_err(|e| e.to_string())?;
        Ok(Stats {
            chunks,
            embedded,
            model: index.model().map_err(|e| e.to_string())?,
        })
    }

    fn text(
        &self,
        pattern: &str,
        regex: bool,
        limit: usize,
        paths: &[String],
    ) -> Result<Vec<Found>, String> {
        self.walker().text(pattern, regex, limit, paths)
    }

    fn files(&self, pattern: &str, limit: usize) -> Result<Vec<String>, String> {
        self.walker().files(pattern, limit)
    }
}

/// `search.text` and `search.files`, with no index behind them.
///
/// `[R-STAR-019]` asks both to work in a workspace where `meow index build`
/// has never run, and to reach exactly the files the index reaches. So this
/// needs the walk and the root and nothing else, and it is what a workspace
/// that declares no index gets - the alternative, which this replaces, was a
/// port that answered every search with an empty list, so a handler searching
/// a workspace without an index was told there was nothing in it.
#[derive(Debug)]
pub struct Walker {
    walk: Walk,
    root: std::path::PathBuf,
}

impl Walker {
    /// Search from this root, with this walk.
    pub fn new(walk: Walk, root: impl Into<std::path::PathBuf>) -> Self {
        Self {
            walk,
            root: root.into(),
        }
    }

    /// The root the walk actually reports paths under.
    ///
    /// `Walk::run` canonicalises before it walks, so every path it returns is
    /// under the canonical root, and `Workspace::at` does not canonicalise at
    /// all. Stripping the given root therefore matches nothing whenever the
    /// two spellings differ - which is every path on Windows, where the
    /// canonical form carries a `\\?\` prefix, and any workspace reached
    /// through a symbolic link anywhere else. Every file was then skipped and
    /// the search came back empty.
    fn walked_root(&self) -> std::path::PathBuf {
        self.root
            .canonicalize()
            .unwrap_or_else(|_| self.root.clone())
    }

    /// The path a hit reports, relative to the root and with forward slashes.
    fn relative(&self, root: &std::path::Path, file: &std::path::Path) -> Option<String> {
        let relative = file.strip_prefix(root).ok()?;
        Some(relative.to_string_lossy().replace('\\', "/"))
    }

    fn text(
        &self,
        pattern: &str,
        regex: bool,
        limit: usize,
        paths: &[String],
    ) -> Result<Vec<Found>, String> {
        let matcher = if regex {
            Some(
                regex::Regex::new(pattern)
                    .map_err(|e| format!("`{pattern}` is not a regular expression: {e}"))?,
            )
        } else {
            None
        };

        let root = self.walked_root();
        let walked = self.walk.run(&self.root).map_err(|e| e.to_string())?;
        let mut out = Vec::new();

        for file in &walked.files {
            let Some(relative) = self.relative(&root, file) else {
                continue;
            };
            if !paths.is_empty() && !paths.iter().any(|p| relative.starts_with(p.as_str())) {
                continue;
            }
            // The walk already refused what is binary or too large, so a read
            // that fails here is a file that went away between the walk and
            // now. Skipping it is right; failing the whole search is not.
            let Ok(body) = std::fs::read_to_string(file) else {
                continue;
            };
            for (number, line) in body.lines().enumerate() {
                let hit = match &matcher {
                    Some(re) => re.is_match(line),
                    None => line.contains(pattern),
                };
                if !hit {
                    continue;
                }
                out.push(Found {
                    path: relative.clone(),
                    first_line: number + 1,
                    last_line: number + 1,
                    text: line.to_owned(),
                    // A literal either matched or it did not, and saying 1.0
                    // keeps one shape for every search in the table.
                    score: 1.0,
                });
                if out.len() >= limit {
                    return Ok(out);
                }
            }
        }

        Ok(out)
    }

    fn files(&self, pattern: &str, limit: usize) -> Result<Vec<String>, String> {
        let matcher = globset::Glob::new(pattern)
            .map_err(|e| format!("`{pattern}` is not a valid glob: {e}"))?
            .compile_matcher();

        let root = self.walked_root();
        let walked = self.walk.run(&self.root).map_err(|e| e.to_string())?;
        let mut out = Vec::new();
        for file in &walked.files {
            let Some(relative) = self.relative(&root, file) else {
                continue;
            };
            if matcher.is_match(&relative) {
                out.push(relative);
                if out.len() >= limit {
                    break;
                }
            }
        }
        Ok(out)
    }
}

/// A workspace with no index: the two searches that need none still work, and
/// everything that needs one says so.
///
/// `[R-STAR-025]`: no index and no results are different answers.
#[derive(Debug)]
pub struct Unindexed {
    walker: Walker,
    /// Why there is no index, in the words the caller should see.
    why: String,
}

impl Unindexed {
    /// A workspace that will walk but not rank, for this reason.
    pub fn new(root: impl Into<std::path::PathBuf>, why: impl Into<String>) -> Self {
        Self {
            walker: Walker::new(Walk::default(), root),
            why: why.into(),
        }
    }
}

impl Search for Unindexed {
    fn code(&self, _query: &str, _limit: usize, _paths: &[String]) -> Result<Vec<Found>, String> {
        Err(self.why.clone())
    }

    fn query(
        &self,
        _query: &str,
        _limit: usize,
        _paths: &[String],
        _min_score: f32,
    ) -> Result<Vec<Found>, String> {
        Err(self.why.clone())
    }

    fn update(&self) -> Result<Indexed, String> {
        Err(self.why.clone())
    }

    fn build(&self) -> Result<Indexed, String> {
        Err(self.why.clone())
    }

    fn stats(&self) -> Result<Stats, String> {
        Err(self.why.clone())
    }

    fn text(
        &self,
        pattern: &str,
        regex: bool,
        limit: usize,
        paths: &[String],
    ) -> Result<Vec<Found>, String> {
        self.walker.text(pattern, regex, limit, paths)
    }

    fn files(&self, pattern: &str, limit: usize) -> Result<Vec<String>, String> {
        self.walker.files(pattern, limit)
    }
}
