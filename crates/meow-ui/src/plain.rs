// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The renderer for a pipe, a CI log, and a terminal that says it is dumb.

use std::io::Write;

use meow_core::view::{LiveKind, Output, ViewEvent};
use meow_core::{EventKind, Usage};

use crate::Renderer;
use crate::theme::Role;

/// Line-oriented output with no escape sequences at all.
///
/// Satisfies `[R-TUI-020]` by writing nothing but text and newlines, and
/// `[R-TUI-021]` by carrying the same information the terminal renderer does:
/// every step, every tool call with its policy decision, and the totals. What
/// it drops is the live region, which cannot exist without moving a cursor.
pub struct Plain<W: Write> {
    out: W,
    /// The decision policy made about each call, by call identifier.
    ///
    /// Held until the tool line is written, because the decision arrives
    /// before the call and a reader wants them on one line.
    decisions: std::collections::HashMap<String, String>,
    step: u32,
}

impl<W: Write> std::fmt::Debug for Plain<W> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Plain")
            .field("step", &self.step)
            .finish_non_exhaustive()
    }
}

impl<W: Write> Plain<W> {
    /// Write to the given sink.
    pub fn new(out: W) -> Self {
        Self {
            out,
            decisions: std::collections::HashMap::new(),
            step: 0,
        }
    }

    /// Take the sink back.
    pub fn into_inner(self) -> W {
        self.out
    }
}

impl<W: Write> Renderer for Plain<W> {
    fn event(&mut self, event: &ViewEvent) -> std::io::Result<()> {
        match event {
            ViewEvent::Live(LiveKind::Schema { .. } | LiveKind::Progress { .. }) => Ok(()),

            ViewEvent::Live(LiveKind::RunStart {
                agent,
                model,
                session,
            }) => writeln!(self.out, "{agent}: model={model} session={session}"),

            ViewEvent::Live(LiveKind::StepStart { step }) => {
                self.step = *step;
                Ok(())
            }

            // Deltas are dropped and the whole text is written once, per step.
            // A pipe that received a line per token would be unreadable, and
            // `[R-TUI-021]` asks for the same information, not the same
            // granularity.
            ViewEvent::Live(LiveKind::TextDelta { .. } | LiveKind::ThinkingDelta { .. }) => Ok(()),

            ViewEvent::Live(LiveKind::ToolStart { id, name, args }) => {
                let decision = self
                    .decisions
                    .remove(id)
                    .map(|d| format!(" [{d}]"))
                    .unwrap_or_default();
                writeln!(self.out, "  {} {name} {args}{decision}", self.step)
            }

            ViewEvent::Live(LiveKind::ToolEnd {
                name,
                duration_ms,
                error,
                ..
            }) => match error {
                Some(error) => writeln!(
                    self.out,
                    "  {} {name} failed after {duration_ms}ms: {error}",
                    self.step
                ),
                None => writeln!(self.out, "  {} {name} ok in {duration_ms}ms", self.step),
            },

            ViewEvent::Live(LiveKind::Output(output)) => self.output(output),

            ViewEvent::Live(LiveKind::RunEnd {
                stop,
                detail,
                steps,
                usage,
                elapsed_ms,
                ..
            }) => {
                let detail = detail
                    .as_ref()
                    .map(|d| format!(" ({d})"))
                    .unwrap_or_default();
                writeln!(
                    self.out,
                    "{stop}{detail}: {steps} steps, {} tokens, {}, {}",
                    total(usage),
                    cost(usage),
                    duration(*elapsed_ms)
                )
            }

            ViewEvent::Logged(EventKind::Policy {
                id, decision, rule, ..
            }) => {
                let described = match rule {
                    Some(rule) => format!("{decision}: {rule}"),
                    None => decision.clone(),
                };
                self.decisions.insert(id.clone(), described);
                Ok(())
            }

            ViewEvent::Logged(EventKind::Note { level, message }) => {
                writeln!(self.out, "{level}: {message}")
            }

            ViewEvent::Logged(EventKind::Assistant { content, .. }) if !content.is_empty() => {
                writeln!(self.out, "{content}")
            }

            // Everything else is already covered by a live kind above, or is
            // detail an export carries and a reader of a log does not need.
            ViewEvent::Logged(_) => Ok(()),
        }
    }

    fn finish(&mut self) -> std::io::Result<()> {
        self.out.flush()
    }
}

impl<W: Write> Plain<W> {
    fn output(&mut self, output: &Output) -> std::io::Result<()> {
        match output {
            Output::Write { text } | Output::Markdown { text } => writeln!(self.out, "{text}"),
            Output::Note { text } => writeln!(self.out, "{}{text}", Role::Muted.sigil()),
            Output::Warn { text } => writeln!(self.out, "{}{text}", Role::Warning.sigil()),
            Output::Error { text } => writeln!(self.out, "{}{text}", Role::Failure.sigil()),
            Output::Step { text } => writeln!(self.out, "== {text}"),
            Output::Table { columns, rows } => write_table(&mut self.out, columns, rows),
            Output::Diff { patch } => writeln!(self.out, "{}", patch.trim_end()),
            Output::Finding {
                severity,
                location,
                summary,
            } => writeln!(self.out, "{severity}: {location}: {summary}"),
            Output::Json { value } => writeln!(self.out, "{value}"),
        }
    }
}

/// A table as aligned columns, which is what `awk` and a human both want.
pub(crate) fn write_table<W: Write>(
    out: &mut W,
    columns: &[String],
    rows: &[Vec<String>],
) -> std::io::Result<()> {
    let mut widths: Vec<usize> = columns.iter().map(|c| c.chars().count()).collect();
    for row in rows {
        for (i, cell) in row.iter().enumerate() {
            if i < widths.len() {
                widths[i] = widths[i].max(cell.chars().count());
            }
        }
    }

    let line = |cells: &[String]| -> String {
        cells
            .iter()
            .enumerate()
            .map(|(i, cell)| {
                let width = widths.get(i).copied().unwrap_or(0);
                format!("{cell:<width$}")
            })
            .collect::<Vec<_>>()
            .join("  ")
            .trim_end()
            .to_owned()
    };

    writeln!(out, "{}", line(columns))?;
    for row in rows {
        writeln!(out, "{}", line(row))?;
    }
    Ok(())
}

/// Prompt plus completion.
pub(crate) fn total(usage: &Usage) -> u32 {
    usage.prompt.saturating_add(usage.completion)
}

/// What a run cost, or that nobody said.
pub(crate) fn cost(usage: &Usage) -> String {
    match usage.cost_micros {
        Some(micros) => format!("${:.2}", micros as f64 / 1_000_000.0),
        None => "cost unknown".to_owned(),
    }
}

/// How long it took, in the units a person reads.
pub(crate) fn duration(elapsed_ms: u64) -> String {
    let seconds = elapsed_ms / 1000;
    if seconds < 60 {
        return format!("{}.{:01}s", seconds, (elapsed_ms % 1000) / 100);
    }
    format!("{}m{:02}s", seconds / 60, seconds % 60)
}
