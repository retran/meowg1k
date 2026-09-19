// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! One event stream, for every renderer and for the export.
//!
//! `[R-TUI-032]` asks the live stream and `meow session export --format json`
//! to share one schema and one version, and every persisted kind to serialise
//! identically in both. The cheapest way to promise that and keep it is to
//! make the live stream a superset of the log rather than a parallel
//! vocabulary: [`ViewEvent::Logged`] carries an [`EventKind`] unchanged, and
//! serde's untagged representation means it emits exactly the bytes the export
//! emits. Nothing has to be kept level by hand.

use serde::{Deserialize, Serialize};

use crate::event::{EventKind, StopReason};
use crate::usage::Usage;

/// The schema version every stream begins with.
///
/// One number for the live stream and the export together, by `[R-TUI-032]`.
/// It goes up when a kind is added, removed, or has a field change meaning -
/// never for a field added at the end, which a reader that ignores unknown
/// keys survives.
pub const SCHEMA_VERSION: u32 = 1;

/// One thing a renderer is told about.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(untagged)]
pub enum ViewEvent {
    /// Something the session log also holds.
    ///
    /// `[R-TUI-034]`: an export emits only these, and the live stream can emit
    /// all of them, so neither can carry a kind the other cannot.
    Logged(EventKind),
    /// Something that exists only while a run is in flight.
    Live(LiveKind),
}

impl ViewEvent {
    /// The name this event serialises under.
    pub fn name(&self) -> &'static str {
        match self {
            Self::Logged(kind) => kind.name(),
            Self::Live(kind) => kind.name(),
        }
    }

    /// Whether a session export may carry it.
    pub fn is_persisted(&self) -> bool {
        matches!(self, Self::Logged(_))
    }
}

/// A kind that only a run in flight produces.
///
/// The names here are deliberately disjoint from [`EventKind::NAMES`]; a test
/// checks it, because two kinds sharing a `type` would make the stream
/// ambiguous to anything reading it with `jq`.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum LiveKind {
    /// The version of everything that follows.
    ///
    /// `[R-TUI-031]`: first, always, so a reader knows what it is parsing
    /// before it has to parse anything.
    Schema {
        /// See [`SCHEMA_VERSION`].
        version: u32,
    },
    /// A run began.
    RunStart {
        /// Which agent.
        agent: String,
        /// Which model.
        model: String,
        /// Which session.
        session: String,
    },
    /// A step began.
    StepStart {
        /// Which step, from one.
        step: u32,
    },
    /// More of the answer.
    TextDelta {
        /// What arrived.
        delta: String,
    },
    /// More of the model's reasoning.
    ThinkingDelta {
        /// What arrived.
        delta: String,
    },
    /// A tool is about to run.
    ToolStart {
        /// Ties this to its end.
        id: String,
        /// Which tool.
        name: String,
        /// What it was asked, as JSON text.
        args: String,
    },
    /// A tool finished.
    ToolEnd {
        /// The call it answers.
        id: String,
        /// Which tool.
        name: String,
        /// How long it took.
        duration_ms: u64,
        /// What went wrong, when something did.
        error: Option<String>,
    },
    /// Something a handler printed.
    Output(Output),
    /// Where the run has got to.
    ///
    /// `[R-TUI-012]`: the live region shows the current tool, the elapsed
    /// time, the step count, and the budget consumed, and this is where all
    /// four come from.
    Progress {
        /// Which step.
        step: u32,
        /// Since the run began.
        elapsed_ms: u64,
        /// Prompt plus completion so far.
        tokens: u32,
        /// What it has cost, when the provider says.
        cost_micros: Option<u64>,
        /// What is running now.
        tool: Option<String>,
    },
    /// A run ended.
    RunEnd {
        /// Why it stopped.
        stop: StopReason,
        /// What bound it, when the reason alone does not say.
        detail: Option<String>,
        /// How many steps it took.
        steps: u32,
        /// What it cost altogether.
        usage: Usage,
        /// How long it took.
        elapsed_ms: u64,
        /// Which session holds it.
        session: String,
    },
}

impl LiveKind {
    /// The name this kind serialises under.
    pub fn name(&self) -> &'static str {
        match self {
            Self::Schema { .. } => "Schema",
            Self::RunStart { .. } => "RunStart",
            Self::StepStart { .. } => "StepStart",
            Self::TextDelta { .. } => "TextDelta",
            Self::ThinkingDelta { .. } => "ThinkingDelta",
            Self::ToolStart { .. } => "ToolStart",
            Self::ToolEnd { .. } => "ToolEnd",
            Self::Output(_) => "Output",
            Self::Progress { .. } => "Progress",
            Self::RunEnd { .. } => "RunEnd",
        }
    }

    /// Every live-only kind's name.
    pub const NAMES: [&'static str; 10] = [
        "Schema",
        "RunStart",
        "StepStart",
        "TextDelta",
        "ThinkingDelta",
        "ToolStart",
        "ToolEnd",
        "Output",
        "Progress",
        "RunEnd",
    ];
}

/// What a handler said.
///
/// `[R-TUI-040]` fixes this set at ten, and `[R-TUI-041]` is why there is
/// nothing here that positions a cursor, draws a frame, or paginates: a script
/// says what a thing is and each renderer decides how it looks. v0.2.x had
/// twenty-two layout builtins, which put presentation in userland where it
/// could not be fixed centrally.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "call", rename_all = "snake_case")]
pub enum Output {
    /// A line of plain output.
    Write {
        /// The line.
        text: String,
    },
    /// Markdown: rendered in a terminal, raw in a pipe, a string in JSON.
    Markdown {
        /// The source.
        text: String,
    },
    /// A de-emphasised aside.
    Note {
        /// What to say.
        text: String,
    },
    /// Something the reader should act on.
    Warn {
        /// What to say.
        text: String,
    },
    /// Something that went wrong.
    Error {
        /// What to say.
        text: String,
    },
    /// A labelled phase in the transcript.
    Step {
        /// What is starting.
        text: String,
    },
    /// Rows under headings.
    Table {
        /// The headings.
        columns: Vec<String>,
        /// The rows, each as long as `columns`.
        rows: Vec<Vec<String>>,
    },
    /// A unified diff.
    Diff {
        /// The patch text.
        patch: String,
    },
    /// One result, with where it is and how much it matters.
    Finding {
        /// `low`, `medium`, or `high`.
        severity: String,
        /// Where it is.
        location: String,
        /// What it is.
        summary: String,
    },
    /// A structured payload, first class under `--format json`.
    Json {
        /// The value.
        value: serde_json::Value,
    },
}

impl Output {
    /// Which of the ten calls this is.
    pub fn call(&self) -> &'static str {
        match self {
            Self::Write { .. } => "write",
            Self::Markdown { .. } => "markdown",
            Self::Note { .. } => "note",
            Self::Warn { .. } => "warn",
            Self::Error { .. } => "error",
            Self::Step { .. } => "step",
            Self::Table { .. } => "table",
            Self::Diff { .. } => "diff",
            Self::Finding { .. } => "finding",
            Self::Json { .. } => "json",
        }
    }

    /// Every call `ctx.out` offers.
    pub const CALLS: [&'static str; 10] = [
        "write", "markdown", "note", "warn", "error", "step", "table", "diff", "finding", "json",
    ];
}
