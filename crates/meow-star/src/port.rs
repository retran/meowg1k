// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What the handler context needs from whoever is running it.
//!
//! `ctx.out`, `ctx.ask`, and `ctx.stdin` are the terminal, and `ctx.session`
//! is the event log. Both belong to crates that depend on this one, so they
//! arrive as traits rather than as types. That is also what lets a test drive a
//! handler without a terminal, which is most of what makes the run phase
//! testable at all.

use meow_core::EventKind;
use meow_core::view::{LiveKind, Output, ViewEvent};
use meow_llm::Message;
use serde_json::Value;

/// Where everything a run produces goes.
///
/// One stream and one method. `[R-TUI-042]` asks each `ctx.out` call to
/// produce a typed event that all three renderers handle, and the engine's own
/// events travel the same way, so a renderer sees one vocabulary rather than
/// two that have to be kept level. A trait with a method per call would let a
/// renderer quietly handle nine of the ten.
///
/// The ten script calls are the variants of [`Output`], fixed by
/// `[R-TUI-040]`; `[R-TUI-041]` is why none of them positions a cursor, draws
/// a frame, or paginates.
pub trait Events: Send + Sync + std::fmt::Debug {
    /// Take one event.
    fn event(&self, event: ViewEvent);
}

/// Send one thing a handler said.
pub fn say(events: &dyn Events, output: Output) {
    events.event(ViewEvent::Live(LiveKind::Output(output)));
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

    /// Write one event into the log.
    ///
    /// The engine reports what happened and this decides whether it is worth
    /// keeping. A failure here is not reported: the place it would be reported
    /// to is the transcript, and a log that cannot be written is exactly the
    /// case where saying so loudly loses the run as well as the record.
    fn record(&self, kind: EventKind) {
        let _ = kind;
    }

    /// The conversation so far, for a run that is continuing one.
    ///
    /// `[R-SESSION-051]`: rebuilt the way the original saw it, superseded
    /// ranges and all, because a resumed run that saw more than the original
    /// did would answer a different question.
    fn history(&self) -> Vec<Message> {
        Vec::new()
    }
}

/// One answer from the index.
#[derive(Debug, Clone, PartialEq)]
pub struct Found {
    /// Which file, relative to the workspace root.
    pub path: String,
    /// Its first line, from one.
    pub first_line: usize,
    /// Its last line, from one.
    pub last_line: usize,
    /// The text that matched.
    pub text: String,
    /// How alike it is, from -1 to 1.
    pub score: f32,
}

/// Searching the workspace by meaning.
///
/// A port for the same reason the terminal is one: the index lives in a crate
/// beside this, and a script reaches it through `load("@std//search", ...)`
/// rather than through a member of the handler context, per `[R-STAR-021]`.
pub trait Search: Send + Sync + std::fmt::Debug {
    /// Rank the workspace against a question.
    ///
    /// # Errors
    ///
    /// What to tell the script: no index, the wrong model, or a failure
    /// reaching the provider.
    fn code(&self, query: &str, limit: usize, paths: &[String]) -> Result<Vec<Found>, String>;
}

/// Ports that do nothing, for a run with no terminal.
pub mod quiet {
    use super::{Ask, AskError, Events, Session, Stdin};

    /// Discards everything sent to it.
    #[derive(Debug, Default)]
    pub struct Silent;

    impl Events for Silent {
        fn event(&self, _event: meow_core::view::ViewEvent) {}
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

    /// Answers every search with the same refusal.
    #[derive(Debug, Default)]
    pub struct NoIndex;

    impl super::Search for NoIndex {
        fn code(
            &self,
            _query: &str,
            _limit: usize,
            _paths: &[String],
        ) -> Result<Vec<super::Found>, String> {
            Err("this run has no index; run `meow index build` first".to_owned())
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
