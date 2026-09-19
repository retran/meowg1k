// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The index behind `meow index` and `search.code`.

use std::sync::Arc;

use meow_index::{Chunking, Embed, Index, Query, Rejected, Walk};
use meow_llm::Provider;
use meow_star::port::{Found, Search};
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

/// `search.code`, over an index and an embedder.
pub struct Searcher {
    /// The lock is not for contention. `rusqlite::Connection` is not `Sync`,
    /// and the port is called from whichever thread a handler or a tool
    /// happens to be on.
    index: std::sync::Mutex<Index>,
    embedder: Embedder,
}

impl std::fmt::Debug for Searcher {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Searcher").finish_non_exhaustive()
    }
}

impl Searcher {
    /// Search this index with this embedder.
    pub fn new(index: Index, embedder: Embedder) -> Self {
        Self {
            index: std::sync::Mutex::new(index),
            embedder,
        }
    }
}

impl Search for Searcher {
    fn code(&self, query: &str, limit: usize, paths: &[String]) -> Result<Vec<Found>, String> {
        let index = self
            .index
            .lock()
            .map_err(|_| "the index is not usable in this run".to_owned())?;

        index
            .query(
                &self.embedder,
                query,
                &Query {
                    limit,
                    min_score: 0.0,
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
}
