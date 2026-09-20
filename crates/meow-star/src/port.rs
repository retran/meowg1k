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

    /// The same, with the floor below which a hit is not worth returning.
    ///
    /// `[R-STAR-024]`: `search.code` is the call a handler usually wants and
    /// `index.query` is the one with the knobs, so the knobs live here and
    /// `code` is this with a floor of zero.
    ///
    /// # Errors
    ///
    /// As [`Search::code`].
    fn query(
        &self,
        query: &str,
        limit: usize,
        paths: &[String],
        min_score: f32,
    ) -> Result<Vec<Found>, String>;

    /// Walk the workspace and record what changed, without embedding.
    ///
    /// # Errors
    ///
    /// Whatever reading the workspace or writing the store said.
    fn update(&self) -> Result<Indexed, String>;

    /// Walk, record, and embed what has no embedding yet.
    ///
    /// # Errors
    ///
    /// As [`Search::update`], plus whatever reaching the provider said.
    fn build(&self) -> Result<Indexed, String>;

    /// How much is indexed, and by which model.
    ///
    /// # Errors
    ///
    /// Whatever reading the store said.
    fn stats(&self) -> Result<Stats, String>;

    /// Find text in the workspace, with no index involved.
    ///
    /// `[R-STAR-019]`: matching literal text is a walk and a comparison, so
    /// this needs no embedding model and no built graph. It is a port anyway
    /// rather than a capability, because it must obey the same walk the index
    /// obeys and that walk lives beside the index.
    ///
    /// # Errors
    ///
    /// A pattern that will not compile, or whatever reading a file said.
    fn text(
        &self,
        pattern: &str,
        regex: bool,
        limit: usize,
        paths: &[String],
    ) -> Result<Vec<Found>, String>;

    /// Every file the walk reaches whose path matches a glob.
    ///
    /// # Errors
    ///
    /// A glob that will not compile, or whatever walking said.
    fn files(&self, pattern: &str, limit: usize) -> Result<Vec<String>, String>;
}

/// What a walk changed.
///
/// `[R-STAR-024]` asks for counts rather than text, because a handler that
/// reports progress and a handler that decides whether to keep going need a
/// number, and parsing one back out of a sentence is how a report goes stale.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct Indexed {
    /// Files seen on this walk.
    pub files: usize,
    /// Chunks added.
    pub added: usize,
    /// Chunks that changed and were replaced.
    pub changed: usize,
    /// Chunks dropped because their file is gone.
    pub removed: usize,
    /// Chunks embedded on this call, which `update` leaves at zero.
    pub embedded: usize,
}

/// How much of the workspace is indexed.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Stats {
    /// Chunks the index holds.
    pub chunks: usize,
    /// How many of them have an embedding.
    pub embedded: usize,
    /// The model that built it, if it has ever been built.
    pub model: Option<String>,
}

/// What a handler keeps between runs.
///
/// `[R-STAR-026]`. The table is the workspace's, not a session's, so
/// collecting every session leaves it alone - which is the whole reason a
/// handler would use it rather than `ctx.session`.
///
/// Values rather than text, by `[R-STAR-027]`: a store that took strings would
/// put an encode on one side of every handler and a decode on the other, and
/// the two would be written in different places and drift.
pub trait Keep: Send + Sync + std::fmt::Debug {
    /// Read a key, or `None` when it has never been written.
    ///
    /// # Errors
    ///
    /// Whatever reading the store said.
    fn get(&self, key: &str) -> Result<Option<Value>, String>;

    /// Write a key, replacing whatever was there.
    ///
    /// # Errors
    ///
    /// Whatever writing the store said.
    fn put(&self, key: &str, value: &Value) -> Result<(), String>;

    /// Remove a key, and say whether it was there.
    ///
    /// # Errors
    ///
    /// Whatever writing the store said.
    fn delete(&self, key: &str) -> Result<bool, String>;

    /// Every key with this prefix, sorted.
    ///
    /// # Errors
    ///
    /// Whatever reading the store said.
    fn keys(&self, prefix: &str) -> Result<Vec<String>, String>;
}

/// Ports that do nothing, for a run with no terminal.
pub mod quiet {
    use super::{Ask, AskError, Events, Keep, Session, Stdin};

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
            Err(NONE.to_owned())
        }

        fn query(
            &self,
            _query: &str,
            _limit: usize,
            _paths: &[String],
            _min_score: f32,
        ) -> Result<Vec<super::Found>, String> {
            Err(NONE.to_owned())
        }

        fn update(&self) -> Result<super::Indexed, String> {
            Err(NONE.to_owned())
        }

        fn build(&self) -> Result<super::Indexed, String> {
            Err(NONE.to_owned())
        }

        fn stats(&self) -> Result<super::Stats, String> {
            Err(NONE.to_owned())
        }

        // These two need no index, so returning an empty list would be a
        // plausible answer - and that is exactly what made the defect this
        // replaces invisible: the binary wired this port for a workspace with
        // no index and every search said the workspace was empty. A port that
        // cannot search says it cannot search.
        fn text(
            &self,
            _pattern: &str,
            _regex: bool,
            _limit: usize,
            _paths: &[String],
        ) -> Result<Vec<super::Found>, String> {
            Err(NONE.to_owned())
        }

        fn files(&self, _pattern: &str, _limit: usize) -> Result<Vec<String>, String> {
            Err(NONE.to_owned())
        }
    }

    /// Keeps what a handler stored, and forgets it when the run ends.
    ///
    /// Durable is the point of the real one, so this is for tests and for a
    /// run with no workspace database - not a fallback the binary should ever
    /// choose quietly.
    #[derive(Debug, Default)]
    pub struct Ephemeral(std::sync::Mutex<std::collections::BTreeMap<String, serde_json::Value>>);

    impl Keep for Ephemeral {
        fn get(&self, key: &str) -> Result<Option<serde_json::Value>, String> {
            Ok(self.0.lock().map_err(poisoned)?.get(key).cloned())
        }

        fn put(&self, key: &str, value: &serde_json::Value) -> Result<(), String> {
            self.0
                .lock()
                .map_err(poisoned)?
                .insert(key.to_owned(), value.clone());
            Ok(())
        }

        fn delete(&self, key: &str) -> Result<bool, String> {
            Ok(self.0.lock().map_err(poisoned)?.remove(key).is_some())
        }

        fn keys(&self, prefix: &str) -> Result<Vec<String>, String> {
            Ok(self
                .0
                .lock()
                .map_err(poisoned)?
                .keys()
                .filter(|key| key.starts_with(prefix))
                .cloned()
                .collect())
        }
    }

    fn poisoned<T>(_: T) -> String {
        "the store is not usable in this run".to_owned()
    }

    /// A store that could not be opened, and says why at every call.
    ///
    /// The alternative was to fall back to [`Ephemeral`] when the database
    /// will not open, and that is the shape of defect this codebase exists to
    /// avoid: a handler would put a value in, get it back within the run, and
    /// find it gone next time, with nothing anywhere saying why.
    #[derive(Debug)]
    pub struct Unopened(pub String);

    impl Keep for Unopened {
        fn get(&self, _key: &str) -> Result<Option<serde_json::Value>, String> {
            Err(self.0.clone())
        }
        fn put(&self, _key: &str, _value: &serde_json::Value) -> Result<(), String> {
            Err(self.0.clone())
        }
        fn delete(&self, _key: &str) -> Result<bool, String> {
            Err(self.0.clone())
        }
        fn keys(&self, _prefix: &str) -> Result<Vec<String>, String> {
            Err(self.0.clone())
        }
    }

    /// `[R-STAR-025]`: no index and no results are different answers, so every
    /// call says which one this is rather than returning nothing.
    ///
    /// This is the double for a run wired with no search at all. A workspace
    /// that merely has no index built is a different thing, and the binary
    /// gives it a port that still walks - see `[R-STAR-019]`.
    const NONE: &str = "this run has no search; run `meow index build` first";

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
