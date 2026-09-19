// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Turning chunks into vectors, in batches a provider will accept.

/// Something that turns text into vectors.
///
/// A trait rather than `meow_llm::Provider`, because this crate has no reason
/// to know what a chat model is and the layering says so: `meow-index` sits
/// beside the engine, not under it.
pub trait Embed: Send + Sync {
    /// Which model, so the index can record what built it.
    fn model(&self) -> &str;

    /// Embed some texts, in the order given.
    ///
    /// # Errors
    ///
    /// A message for the caller. [`Rejected::TooLarge`] is the one the batch
    /// splitter acts on; anything else stops the build.
    fn embed(&self, texts: &[String]) -> Result<Vec<Vec<f32>>, Rejected>;
}

/// Why a provider would not embed a batch.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Rejected {
    /// The batch was too big, as a batch.
    ///
    /// `[R-INDEX-020]`: split and retried. Every provider has a limit and
    /// none of them agrees on it, so discovering it by being told is more
    /// reliable than configuring it.
    TooLarge,
    /// Something else went wrong.
    Failed(String),
}

impl std::fmt::Display for Rejected {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::TooLarge => f.write_str("the batch is too large"),
            Self::Failed(message) => f.write_str(message),
        }
    }
}

/// How many chunks go in one request before the provider is asked.
pub const DEFAULT_BATCH: usize = 64;

/// Embed a list of texts, splitting any batch the provider refuses.
///
/// Satisfies `[R-INDEX-020]`: a rejected batch is halved and retried, down to
/// one text. A single text that is still refused is the caller's to report,
/// with the file and the lines, which is `[R-INDEX-021]` and is why this
/// returns the index of the offender rather than a message about sizes.
///
/// # Errors
///
/// The position of the text that could not be embedded on its own, and why.
pub fn in_batches(
    embedder: &dyn Embed,
    texts: &[String],
    batch: usize,
) -> Result<Vec<Vec<f32>>, (usize, Rejected)> {
    let batch = batch.max(1);
    let mut out = Vec::with_capacity(texts.len());

    for (offset, window) in texts.chunks(batch).enumerate() {
        let at = offset * batch;
        out.extend(split_and_retry(embedder, window, at)?);
    }

    Ok(out)
}

/// Embed one window, halving it for as long as the provider objects.
fn split_and_retry(
    embedder: &dyn Embed,
    texts: &[String],
    at: usize,
) -> Result<Vec<Vec<f32>>, (usize, Rejected)> {
    match embedder.embed(texts) {
        Ok(vectors) => Ok(vectors),
        // One text that is refused cannot be split any further, so this is
        // where the recursion stops and the caller has something to name.
        Err(reason) if texts.len() == 1 => Err((at, reason)),
        Err(Rejected::TooLarge) => {
            let middle = texts.len() / 2;
            let mut out = split_and_retry(embedder, &texts[..middle], at)?;
            out.extend(split_and_retry(embedder, &texts[middle..], at + middle)?);
            Ok(out)
        }
        Err(other) => Err((at, other)),
    }
}

/// A vector as bytes, for the store.
pub fn to_bytes(vector: &[f32]) -> Vec<u8> {
    let mut out = Vec::with_capacity(vector.len() * 4);
    for value in vector {
        out.extend_from_slice(&value.to_le_bytes());
    }
    out
}

/// A vector from bytes, or nothing when the length is not a multiple of four.
pub fn from_bytes(bytes: &[u8]) -> Option<Vec<f32>> {
    if !bytes.len().is_multiple_of(4) {
        return None;
    }
    Some(
        bytes
            .as_chunks::<4>()
            .0
            .iter()
            .map(|b| f32::from_le_bytes(*b))
            .collect(),
    )
}

/// How alike two vectors are, from -1 to 1.
///
/// Cosine similarity, which is what every embedding model this talks to is
/// trained for. Vectors of different lengths score zero rather than panicking:
/// that means the index was built by another model, and `[R-INDEX-051]`
/// catches it before a query gets this far.
pub fn similarity(a: &[f32], b: &[f32]) -> f32 {
    if a.len() != b.len() {
        return 0.0;
    }
    let mut dot = 0.0_f32;
    let mut left = 0.0_f32;
    let mut right = 0.0_f32;
    for (x, y) in a.iter().zip(b) {
        dot += x * y;
        left += x * x;
        right += y * y;
    }
    let magnitude = left.sqrt() * right.sqrt();
    if magnitude == 0.0 {
        0.0
    } else {
        dot / magnitude
    }
}
