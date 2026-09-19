// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Splitting a file into pieces a model can embed.

use std::path::{Path, PathBuf};

use crate::error::Result;

/// How many lines a chunk holds before a new one starts.
pub const DEFAULT_LINES: usize = 60;

/// How many lines two neighbouring chunks share.
///
/// `[R-INDEX-012]`: in lines rather than tokens, because a boundary is a line
/// and the two units cannot both be exact. A definition that straddles a
/// boundary is retrievable from either side.
pub const DEFAULT_OVERLAP: usize = 10;

/// How many characters a chunk may hold.
///
/// A conservative stand-in for the embedding model's input limit, which is
/// counted in tokens. Four characters to a token is the same estimate the
/// engine's compaction uses, so the two cannot disagree about how big
/// something is.
pub const DEFAULT_MAX_CHARS: usize = 8000;

/// One piece of a file.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Chunk {
    /// Which file it came from.
    pub path: PathBuf,
    /// Its text.
    pub text: String,
    /// Where it starts in the file, in bytes.
    pub start: usize,
    /// Where it ends, in bytes, exclusive.
    pub end: usize,
    /// Its first line, from one.
    pub first_line: usize,
    /// Its last line, from one, inclusive.
    pub last_line: usize,
    /// Whether a single line had to be cut to fit.
    ///
    /// `[R-INDEX-014]`: reported, so a minified file does not make the
    /// line-boundary rule and the size rule quietly unsatisfiable.
    pub split_line: bool,
}

impl Chunk {
    /// How many lines it covers.
    pub fn lines(&self) -> usize {
        self.last_line.saturating_sub(self.first_line) + 1
    }
}

/// How to split.
#[derive(Debug, Clone, Copy)]
pub struct Chunking {
    /// How many lines per chunk.
    pub lines: usize,
    /// How many lines two neighbours share.
    pub overlap: usize,
    /// How many characters a chunk may hold.
    pub max_chars: usize,
}

impl Default for Chunking {
    fn default() -> Self {
        Self {
            lines: DEFAULT_LINES,
            overlap: DEFAULT_OVERLAP,
            max_chars: DEFAULT_MAX_CHARS,
        }
    }
}

impl Chunking {
    /// The parameters as text, for the hash that decides whether a file is
    /// stale.
    ///
    /// `[R-INDEX-030]`: the hash covers the parameters as well as the
    /// content, so changing the chunk size does not leave chunks that look
    /// current and were built by different rules.
    pub fn fingerprint(&self) -> String {
        format!(
            "lines={} overlap={} max={}",
            self.lines, self.overlap, self.max_chars
        )
    }

    /// Split a file's text.
    ///
    /// Satisfies `[R-INDEX-010]` by depending on nothing but the text and
    /// these parameters; `[R-INDEX-011]` by carrying the path, the byte
    /// range, and the line range; `[R-INDEX-012]` through the overlap;
    /// `[R-INDEX-013]` by never emitting a chunk over the limit; and
    /// `[R-INDEX-014]` by cutting only on line boundaries, except for a line
    /// that is longer than the whole limit, which is cut and reported.
    ///
    /// # Errors
    ///
    /// Nothing here fails: a long line is split rather than refused, because
    /// only the embedding step knows a provider's real limit and that is
    /// where [`crate::IndexError::ChunkTooLarge`] belongs. The result type is
    /// kept so a future splitter that can fail does not change every caller.
    pub fn split(&self, path: &Path, text: &str) -> Result<Vec<Chunk>> {
        if self.lines == 0 || self.max_chars == 0 {
            return Ok(Vec::new());
        }

        let lines = numbered_lines(text);
        if lines.is_empty() {
            return Ok(Vec::new());
        }

        // The overlap cannot be the whole chunk, or the window never advances.
        let step = self.lines.saturating_sub(self.overlap).max(1);

        let mut out = Vec::new();
        let mut at = 0;

        while at < lines.len() {
            let end = (at + self.lines).min(lines.len());
            let window = &lines[at..end];

            let start_byte = window[0].start;
            let end_byte = window[window.len() - 1].end;
            let piece = &text[start_byte..end_byte];

            if piece.chars().count() <= self.max_chars {
                out.push(Chunk {
                    path: path.to_path_buf(),
                    text: piece.to_owned(),
                    start: start_byte,
                    end: end_byte,
                    first_line: window[0].number,
                    last_line: window[window.len() - 1].number,
                    split_line: false,
                });
            } else {
                out.extend(self.fit(path, text, window));
            }

            if end == lines.len() {
                break;
            }
            at += step;
        }

        Ok(out)
    }

    /// Cut a window that is over the limit, on line boundaries where it can.
    fn fit(&self, path: &Path, text: &str, window: &[Numbered]) -> Vec<Chunk> {
        let mut out = Vec::new();
        let mut pending: Option<(usize, usize, usize, usize)> = None;

        for line in window {
            let length = text[line.start..line.end].chars().count();

            // A line longer than the whole limit cannot go in any chunk
            // whole, which is the one case [R-INDEX-014] allows a cut inside
            // a line - and requires it to be reported.
            if length > self.max_chars {
                if let Some((start, end, first, last)) = pending.take() {
                    out.push(chunk(path, text, start, end, first, last, false));
                }
                out.extend(self.cut_line(path, text, line));
                continue;
            }

            match pending {
                Some((start, end, first, last))
                    if text[start..end].chars().count() + length <= self.max_chars =>
                {
                    pending = Some((start, line.end, first, line.number));
                    let _ = (end, last);
                }
                Some((start, end, first, last)) => {
                    out.push(chunk(path, text, start, end, first, last, false));
                    pending = Some((line.start, line.end, line.number, line.number));
                }
                None => pending = Some((line.start, line.end, line.number, line.number)),
            }
        }

        if let Some((start, end, first, last)) = pending {
            out.push(chunk(path, text, start, end, first, last, false));
        }
        out
    }

    /// Cut one over-long line into pieces, on character boundaries.
    fn cut_line(&self, path: &Path, text: &str, line: &Numbered) -> Vec<Chunk> {
        let mut out = Vec::new();
        let mut at = line.start;

        while at < line.end {
            // Advance by characters, not bytes, so the cut never lands inside
            // one and produces text no model can read.
            let mut end = at;
            for (taken, (offset, c)) in text[at..line.end].char_indices().enumerate() {
                if taken == self.max_chars {
                    break;
                }
                end = at + offset + c.len_utf8();
            }
            if end == at {
                break;
            }

            out.push(chunk(path, text, at, end, line.number, line.number, true));
            at = end;
        }

        out
    }
}

fn chunk(
    path: &Path,
    text: &str,
    start: usize,
    end: usize,
    first_line: usize,
    last_line: usize,
    split_line: bool,
) -> Chunk {
    Chunk {
        path: path.to_path_buf(),
        text: text[start..end].to_owned(),
        start,
        end,
        first_line,
        last_line,
        split_line,
    }
}

/// One line, with where it sits and what it is called.
struct Numbered {
    number: usize,
    start: usize,
    end: usize,
}

/// Every line of a file, with its byte range including its newline.
///
/// Including the newline is what makes the byte ranges of consecutive chunks
/// meet: a range that stopped short of it would leave a gap nobody could
/// account for when citing a result.
fn numbered_lines(text: &str) -> Vec<Numbered> {
    let mut out = Vec::new();
    let mut start = 0;
    let mut number = 1;

    for (offset, c) in text.char_indices() {
        if c == '\n' {
            out.push(Numbered {
                number,
                start,
                end: offset + 1,
            });
            start = offset + 1;
            number += 1;
        }
    }

    if start < text.len() {
        out.push(Numbered {
            number,
            start,
            end: text.len(),
        });
    }

    out
}
