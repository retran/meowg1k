// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The renderer a program reads.

use std::io::Write;

use meow_core::view::{LiveKind, SCHEMA_VERSION, ViewEvent};

use crate::Renderer;

/// One JSON object per line.
///
/// Satisfies `[R-TUI-030]` - every line an object with a `type` - and
/// `[R-TUI-031]` by emitting the schema version before anything else. The
/// objects are `meow_core::view` types serialised directly, which is what
/// makes `[R-TUI-032]` hold: there is no second definition here to drift from
/// the one the export uses.
///
/// `[R-TUI-033]` is the caller's half of the bargain. This writes only the
/// stream; diagnostics go to stderr, which this never touches.
pub struct Json<W: Write> {
    out: W,
    announced: bool,
}

impl<W: Write> std::fmt::Debug for Json<W> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Json")
            .field("announced", &self.announced)
            .finish_non_exhaustive()
    }
}

impl<W: Write> Json<W> {
    /// Write to the given sink.
    pub fn new(out: W) -> Self {
        Self {
            out,
            announced: false,
        }
    }

    /// Take the sink back.
    pub fn into_inner(self) -> W {
        self.out
    }

    fn line(&mut self, event: &ViewEvent) -> std::io::Result<()> {
        let text = serde_json::to_string(event).map_err(std::io::Error::other)?;
        writeln!(self.out, "{text}")
    }
}

impl<W: Write> Renderer for Json<W> {
    fn event(&mut self, event: &ViewEvent) -> std::io::Result<()> {
        if !self.announced {
            self.announced = true;
            // Emitted here rather than in `new` so that constructing a
            // renderer writes nothing: a run that never starts should not
            // leave a lone schema line in a pipe.
            let schema = ViewEvent::Live(LiveKind::Schema {
                version: SCHEMA_VERSION,
            });
            if !matches!(event, ViewEvent::Live(LiveKind::Schema { .. })) {
                self.line(&schema)?;
            }
        }
        self.line(event)
    }

    fn finish(&mut self) -> std::io::Result<()> {
        self.out.flush()
    }
}
