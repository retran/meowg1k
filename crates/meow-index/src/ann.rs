// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The structure a query actually walks.
//!
//! Comparing a query against every stored vector is linear in the corpus and
//! reads the whole index off disk to answer one question. A navigable
//! small-world graph touches a fraction of the vectors instead, and it is
//! built once and kept, so the cost is paid at build time where somebody is
//! already waiting.
//!
//! The graph is a cache, not a record: `index_chunks` in the store is the
//! truth, and the graph is rebuilt from it whenever the two disagree.

use std::path::PathBuf;

use hnsw_rs::prelude::*;

use crate::embed;
use crate::error::{IndexError, Result};

/// How many neighbours each node keeps.
///
/// Sixteen is the usual starting point: recall is already high and the graph
/// is small. The number to move if recall on a real corpus is not good enough.
const CONNECTIONS: usize = 16;

/// How many layers the graph may have.
const LAYERS: usize = 16;

/// How hard the builder looks for neighbours.
const EF_CONSTRUCTION: usize = 200;

/// How hard a search looks, relative to what it was asked for.
///
/// A search that explores exactly as many candidates as it returns misses
/// neighbours that sit behind a worse one, so it explores more and keeps the
/// best. Four times the limit, with a floor, is the usual trade.
const EF_SEARCH_FACTOR: usize = 4;
const EF_SEARCH_FLOOR: usize = 50;

/// What the graph was built from.
///
/// Stored beside it, so a graph built for a different set of chunks is
/// noticed rather than searched.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Signature {
    /// How many embedded chunks there were.
    pub chunks: i64,
    /// The highest chunk identifier.
    pub max_id: i64,
}

impl Signature {
    /// How it is written next to the graph.
    fn encode(self) -> String {
        format!("{} {}", self.chunks, self.max_id)
    }

    /// Read one back, or nothing when the file says something else.
    fn decode(text: &str) -> Option<Self> {
        let (chunks, max_id) = text.trim().split_once(' ')?;
        Some(Self {
            chunks: chunks.parse().ok()?,
            max_id: max_id.parse().ok()?,
        })
    }
}

/// Where the graph lives, under the workspace's data directory.
#[derive(Debug, Clone)]
pub struct Location {
    dir: PathBuf,
}

/// The file the graph's own dump is named after.
const BASENAME: &str = "index";

impl Location {
    /// The graph of a workspace.
    pub fn new(data_dir: impl Into<PathBuf>) -> Self {
        Self {
            dir: data_dir.into(),
        }
    }

    fn stamp(&self) -> PathBuf {
        self.dir.join("index.hnsw.stamp")
    }

    fn ids(&self) -> PathBuf {
        self.dir.join("index.hnsw.ids")
    }

    /// The signature the stored graph was built from, if there is one.
    pub fn stored(&self) -> Option<Signature> {
        Signature::decode(&std::fs::read_to_string(self.stamp()).ok()?)
    }

    /// Whether a graph on disk is still the right one.
    pub fn is_current(&self, signature: Signature) -> bool {
        self.stored() == Some(signature)
    }

    /// Forget the graph, so the next query rebuilds it.
    pub fn forget(&self) {
        // Best effort. A stamp that will not go means the next query rebuilds
        // for nothing, which is slow and not wrong.
        let _ = std::fs::remove_file(self.stamp());
    }
}

/// Build a graph over the vectors given, and write it down.
///
/// # Errors
///
/// [`IndexError::Io`] when the graph cannot be written.
pub fn build(
    vectors: &[(i64, Vec<u8>)],
    signature: Signature,
    location: &Location,
) -> Result<usize> {
    let points: Vec<(i64, Vec<f32>)> = vectors
        .iter()
        .filter_map(|(id, bytes)| Some((*id, embed::from_bytes(bytes)?)))
        .collect();

    let hnsw = Hnsw::<f32, DistCosine>::new(
        CONNECTIONS,
        points.len().max(1),
        LAYERS,
        EF_CONSTRUCTION,
        DistCosine {},
    );

    // The graph numbers its own nodes from zero and a chunk identifier does
    // not, so the two are kept apart rather than hoping they agree.
    let mut ids = Vec::with_capacity(points.len());
    for (node, (id, vector)) in points.iter().enumerate() {
        hnsw.insert((vector.as_slice(), node));
        ids.push(*id);
    }

    std::fs::create_dir_all(&location.dir).map_err(|source| IndexError::Io {
        path: location.dir.clone(),
        source,
    })?;

    hnsw.file_dump(&location.dir, BASENAME)
        .map_err(|e| IndexError::Store(e.to_string()))?;

    let text = ids
        .iter()
        .map(ToString::to_string)
        .collect::<Vec<_>>()
        .join("\n");
    std::fs::write(location.ids(), text).map_err(|source| IndexError::Io {
        path: location.ids(),
        source,
    })?;

    // The stamp is written last, so a dump interrupted half way through is
    // not mistaken for a finished one.
    std::fs::write(location.stamp(), signature.encode()).map_err(|source| IndexError::Io {
        path: location.stamp(),
        source,
    })?;

    Ok(ids.len())
}

/// The nearest chunks to a vector, with their similarity.
///
/// `allowed` is consulted while the graph is walked rather than afterwards, so
/// a filter narrows the search instead of narrowing its results. That is what
/// `[R-INDEX-044]` means by applying the filter before ranking.
///
/// The graph is loaded and dropped inside this call. `hnsw_rs` ties a loaded
/// graph to the reader that produced it, and the data file is mapped rather
/// than read, so what a query actually touches is the pages its walk lands on.
///
/// # Errors
///
/// [`IndexError::Store`] when the graph will not load, which the caller
/// answers by rebuilding it.
pub fn nearest(
    location: &Location,
    vector: &[f32],
    limit: usize,
    allowed: Option<&[usize]>,
) -> Result<Vec<(i64, f32)>> {
    if limit == 0 || allowed.is_some_and(<[usize]>::is_empty) {
        return Ok(Vec::new());
    }

    let ids: Vec<i64> = std::fs::read_to_string(location.ids())
        .map_err(|source| IndexError::Io {
            path: location.ids(),
            source,
        })?
        .lines()
        .filter_map(|line| line.trim().parse().ok())
        .collect();

    // A graph with no identifiers beside it is a damaged cache, not an empty
    // index: the caller checked the row count before getting here. Reporting
    // it lets the caller rebuild instead of answering nothing.
    if ids.is_empty() {
        return Err(IndexError::Store(format!(
            "{} holds no identifiers",
            location.ids().display()
        )));
    }

    let mut io = HnswIo::new_with_options(&location.dir, BASENAME, ReloadOptions::new(true));
    let hnsw = io
        .load_hnsw::<f32, DistCosine>()
        .map_err(|e| IndexError::Store(e.to_string()))?;

    let ef = (limit * EF_SEARCH_FACTOR).max(EF_SEARCH_FLOOR);
    let found = match allowed {
        Some(nodes) => {
            let nodes = nodes.to_vec();
            hnsw.search_filter(vector, limit, ef, Some(&nodes as &dyn FilterT))
        }
        None => hnsw.search(vector, limit, ef),
    };

    Ok(found
        .into_iter()
        .filter_map(|neighbour| {
            let id = *ids.get(neighbour.d_id)?;
            // `DistCosine` is one minus the cosine similarity, and the rest of
            // this crate speaks in similarity.
            Some((id, 1.0 - neighbour.distance))
        })
        .collect())
}
