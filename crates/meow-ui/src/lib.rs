// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! One event stream, three renderers.
//!
//! v0.2.x ran three rendering stacks at once - a `uilive` progress logger, an
//! 807-line Bubble Tea program, and a set of lipgloss widgets - each of which
//! owned the cursor, and which one won was timing-dependent. Here there is one
//! stream of [`ViewEvent`] and three things that consume it, and a run picks
//! exactly one.
//!
//! This crate depends on `meow-core` for the event types and on nothing else
//! in the workspace. It does not know the engine exists, which is what lets a
//! renderer be driven from a recorded log with no terminal attached.

pub mod json;
pub mod plain;
pub mod theme;
pub mod tty;

use meow_core::view::ViewEvent;

pub use crate::json::Json;
pub use crate::plain::Plain;
pub use crate::theme::{Depth, Role, Theme};
pub use crate::tty::Tty;

/// Something that turns events into output.
///
/// `[R-TUI-005]`: all three take the same stream, and none of them needs a
/// terminal to be driven.
pub trait Renderer {
    /// Take one event.
    ///
    /// # Errors
    ///
    /// Whatever writing to the sink failed with.
    fn event(&mut self, event: &ViewEvent) -> std::io::Result<()>;

    /// Show a prompt and leave it up until it is answered.
    ///
    /// `[R-TUI-060]`: it occupies the live region and never overwrites the
    /// transcript above it. A renderer with no live region writes to stderr,
    /// which is right for the same reason: stdout may be a pipe carrying a
    /// result, and a question does not belong in it.
    ///
    /// # Errors
    ///
    /// Whatever writing to the sink failed with.
    fn prompt_open(&mut self, lines: &[String]) -> std::io::Result<()> {
        use std::io::Write;
        let mut err = std::io::stderr();
        for line in lines {
            writeln!(err, "{line}")?;
        }
        err.flush()
    }

    /// Take the prompt down again.
    ///
    /// # Errors
    ///
    /// Whatever writing to the sink failed with.
    fn prompt_close(&mut self) -> std::io::Result<()> {
        Ok(())
    }

    /// Finish: flush, and put the terminal back as it was found.
    ///
    /// # Errors
    ///
    /// Whatever writing to the sink failed with.
    fn finish(&mut self) -> std::io::Result<()>;
}

/// Which renderer a run should use.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Choice {
    /// The inline terminal renderer.
    Terminal,
    /// Line-oriented text with no escape sequences.
    Plain,
    /// One JSON object per line.
    Json,
}

/// What the runtime knows about where output is going.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Conditions {
    /// Whether `--format json` was given.
    pub json: bool,
    /// Whether stdout is a terminal.
    pub stdout_is_terminal: bool,
    /// Whether `NO_COLOR` is set.
    pub no_color: bool,
    /// Whether `--color=never` was given.
    pub color_never: bool,
}

/// Choose a renderer.
///
/// Satisfies `[R-TUI-001]` by being a function of the runtime's conditions and
/// nothing a script can reach; `[R-TUI-002]` by letting `--format json` win
/// whatever the terminal is, because a program asking for JSON gets JSON even
/// from an interactive shell; `[R-TUI-003]` by treating a pipe, `NO_COLOR`,
/// and `--color=never` alike; and `[R-TUI-004]` by falling through to the
/// inline renderer.
pub fn choose(conditions: Conditions) -> Choice {
    if conditions.json {
        return Choice::Json;
    }
    if !conditions.stdout_is_terminal || conditions.no_color || conditions.color_never {
        return Choice::Plain;
    }
    Choice::Terminal
}
