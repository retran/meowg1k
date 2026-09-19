// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What the handler context needs from whoever is running it.
//!
//! `ctx.out`, `ctx.ask`, and `ctx.stdin` are the terminal, and `ctx.session`
//! is the event log. Both belong to crates that depend on this one, so they
//! arrive as traits rather than as types. That is also what lets a test drive a
//! handler without a terminal, which is most of what makes the run phase
//! testable at all.

use serde_json::Value;

/// Where a handler's output goes.
///
/// The methods are what `0.3.0-starlark-api.md` section 6 lists, and they are
/// deliberately not one `write` with a severity argument: a renderer decides
/// differently for each, and a severity string is one typo away from being
/// silently ignored.
pub trait Out: Send + Sync + std::fmt::Debug {
    /// A line of plain text.
    fn write(&self, text: &str);
    /// An aside: counts, timings, what was skipped.
    fn note(&self, text: &str);
    /// Something the reader should act on.
    fn warn(&self, text: &str);
    /// A step in a longer piece of work.
    fn step(&self, text: &str);
    /// Text to render as markdown.
    fn markdown(&self, text: &str);
    /// One result, with where it is and how much it matters.
    fn finding(&self, severity: &str, location: &str, summary: &str);
}

/// A question could not be asked.
#[derive(Debug, thiserror::Error)]
pub enum AskError {
    /// There is nobody to answer.
    ///
    /// A handler that asks in a pipeline should fail rather than block
    /// forever or silently take the default, because either one turns an
    /// unattended run into a wrong answer.
    #[error("`{what}` needs a terminal, and this run has none")]
    NotATerminal {
        /// Which call.
        what: &'static str,
    },
    /// The person declined to answer.
    #[error("the question was not answered")]
    Declined,
}

/// How a handler asks a person something.
pub trait Ask: Send + Sync + std::fmt::Debug {
    /// Ask for a line of text.
    ///
    /// # Errors
    ///
    /// [`AskError`] when there is no terminal or the person declined.
    fn text(&self, prompt: &str, default: Option<&str>) -> Result<String, AskError>;

    /// Ask for a yes or a no.
    ///
    /// # Errors
    ///
    /// [`AskError`] when there is no terminal or the person declined.
    fn confirm(&self, prompt: &str, default: bool) -> Result<bool, AskError>;

    /// Ask the person to pick one.
    ///
    /// # Errors
    ///
    /// [`AskError`] when there is no terminal or the person declined.
    fn select(&self, prompt: &str, choices: &[String]) -> Result<String, AskError>;
}

/// What was piped in.
pub trait Stdin: Send + Sync + std::fmt::Debug {
    /// Whether anything was.
    fn is_piped(&self) -> bool;

    /// Read all of it.
    ///
    /// # Errors
    ///
    /// Whatever the operating system said.
    fn read(&self) -> std::io::Result<String>;
}

/// This invocation's session.
///
/// `ctx.session.set` writes into the event log rather than into a side table,
/// so a value a handler stored is replayable with the run that stored it.
pub trait Session: Send + Sync + std::fmt::Debug {
    /// Which session.
    fn id(&self) -> String;

    /// Read something this run stored.
    fn get(&self, key: &str) -> Option<Value>;

    /// Store something for the rest of this run.
    fn set(&self, key: &str, value: Value);
}

/// Ports that do nothing, for a run with no terminal.
pub mod quiet {
    use super::{Ask, AskError, Out, Session, Stdin};

    /// Discards everything written to it.
    #[derive(Debug, Default)]
    pub struct Silent;

    impl Out for Silent {
        fn write(&self, _text: &str) {}
        fn note(&self, _text: &str) {}
        fn warn(&self, _text: &str) {}
        fn step(&self, _text: &str) {}
        fn markdown(&self, _text: &str) {}
        fn finding(&self, _severity: &str, _location: &str, _summary: &str) {}
    }

    /// Refuses every question.
    #[derive(Debug, Default)]
    pub struct NoTerminal;

    impl Ask for NoTerminal {
        fn text(&self, _prompt: &str, _default: Option<&str>) -> Result<String, AskError> {
            Err(AskError::NotATerminal {
                what: "ctx.ask.text",
            })
        }
        fn confirm(&self, _prompt: &str, _default: bool) -> Result<bool, AskError> {
            Err(AskError::NotATerminal {
                what: "ctx.ask.confirm",
            })
        }
        fn select(&self, _prompt: &str, _choices: &[String]) -> Result<String, AskError> {
            Err(AskError::NotATerminal {
                what: "ctx.ask.select",
            })
        }
    }

    /// Nothing was piped in.
    #[derive(Debug, Default)]
    pub struct Closed;

    impl Stdin for Closed {
        fn is_piped(&self) -> bool {
            false
        }
        fn read(&self) -> std::io::Result<String> {
            Ok(String::new())
        }
    }

    /// Keeps what a run stored, and forgets it afterwards.
    #[derive(Debug, Default)]
    pub struct Memory {
        id: String,
        values: std::sync::Mutex<std::collections::BTreeMap<String, serde_json::Value>>,
    }

    impl Memory {
        /// A session with the given identifier.
        pub fn new(id: impl Into<String>) -> Self {
            Self {
                id: id.into(),
                values: std::sync::Mutex::new(std::collections::BTreeMap::new()),
            }
        }
    }

    impl Session for Memory {
        fn id(&self) -> String {
            self.id.clone()
        }
        fn get(&self, key: &str) -> Option<serde_json::Value> {
            self.values.lock().ok()?.get(key).cloned()
        }
        fn set(&self, key: &str, value: serde_json::Value) {
            if let Ok(mut values) = self.values.lock() {
                values.insert(key.to_owned(), value);
            }
        }
    }
}
