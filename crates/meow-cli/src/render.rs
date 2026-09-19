// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Wiring a renderer to the run that feeds it.

use std::io::IsTerminal;
use std::sync::Mutex;

use meow_core::view::ViewEvent;
use meow_star::port::Events;
use meow_ui::theme::{Depth, Theme, unicode};
use meow_ui::{Choice, Conditions, Json, Plain, Renderer, Tty};
use ratatui::backend::CrosstermBackend;

/// The renderer, behind the lock a `Send + Sync` port needs.
///
/// A renderer owns a cursor and a buffer and is not `Sync`; the port is called
/// from whichever thread a handler or a tool happens to be on. One mutex is
/// the whole reconciliation, and it is also what keeps two threads from
/// interleaving half a line each.
pub struct Sink(Mutex<Box<dyn Renderer + Send>>);

impl std::fmt::Debug for Sink {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("Sink")
    }
}

impl Sink {
    /// Wrap a renderer.
    pub fn new(renderer: Box<dyn Renderer + Send>) -> Self {
        Self(Mutex::new(renderer))
    }

    /// Put a question up in the live region.
    pub fn prompt_open(&self, lines: &[String]) {
        if let Ok(mut renderer) = self.0.lock() {
            let _ = renderer.prompt_open(lines);
        }
    }

    /// Take it down again.
    pub fn prompt_close(&self) {
        if let Ok(mut renderer) = self.0.lock() {
            let _ = renderer.prompt_close();
        }
    }

    /// Flush and put the terminal back as it was found.
    pub fn finish(&self) {
        if let Ok(mut renderer) = self.0.lock() {
            let _ = renderer.finish();
        }
    }
}

impl Events for Sink {
    fn event(&self, event: ViewEvent) {
        if let Ok(mut renderer) = self.0.lock() {
            // A renderer that cannot write has nowhere to report it: the
            // place it would report to is the thing that failed. The run
            // carries on, which is what a closed pipe should do.
            let _ = renderer.event(&event);
        }
    }
}

/// What the process can see about where its output is going.
#[derive(Debug, Clone, Copy)]
pub struct Environment {
    /// Whether `--format json` was given.
    pub json: bool,
    /// Whether `--color=never` was given.
    pub color_never: bool,
}

/// Build the renderer this run should use.
///
/// `[R-TUI-001]`: the choice is made here, from what the runtime can see, and
/// there is no way for a script to reach it.
pub fn build(environment: Environment) -> Sink {
    let no_color = std::env::var_os("NO_COLOR").is_some();
    let choice = meow_ui::choose(Conditions {
        json: environment.json,
        stdout_is_terminal: std::io::stdout().is_terminal(),
        no_color,
        color_never: environment.color_never,
    });

    let renderer: Box<dyn Renderer + Send> = match choice {
        Choice::Json => Box::new(Json::new(std::io::stdout())),
        Choice::Plain => Box::new(Plain::new(std::io::stdout())),
        Choice::Terminal => {
            let theme = Theme::new(
                Depth::detect(
                    no_color,
                    std::env::var("COLORTERM").ok().as_deref(),
                    std::env::var("TERM").ok().as_deref(),
                ),
                unicode(
                    std::env::var("LANG").ok().as_deref(),
                    std::env::var("TERM").ok().as_deref(),
                ),
            );
            match Tty::new(CrosstermBackend::new(std::io::stdout()), theme) {
                Ok(tty) => Box::new(tty),
                // A terminal that will not take an inline viewport still has a
                // stdout, and a run that refuses to start because the display
                // is unusual is worse than one that prints plainly.
                Err(_) => Box::new(Plain::new(std::io::stdout())),
            }
        }
    };

    Sink::new(renderer)
}
