// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What a session log holds, and how a run ends.

use std::fmt;
use std::ops::RangeInclusive;

use crate::usage::Usage;

/// Why a run stopped.
///
/// `[R-AGENT-002]` fixes this set at six. `[R-SESSION-040]` reuses it as the
/// terminal state of a session, and `[R-TUI-080]` maps each to its own exit
/// code, so adding a variant is a change to three specifications at once.
#[derive(Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum StopReason {
    /// The model finished.
    Finished,
    /// A budget axis bound the run.
    Budget,
    /// The user or a parent cancelled it.
    Cancelled,
    /// Policy denied a call the run needed.
    Denied,
    /// A tool failed under an abort policy.
    ToolAborted,
    /// The engine or the store failed.
    Failed,
}

impl StopReason {
    /// The wire and command-line spelling.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Finished => "finished",
            Self::Budget => "budget",
            Self::Cancelled => "cancelled",
            Self::Denied => "denied",
            Self::ToolAborted => "tool_aborted",
            Self::Failed => "failed",
        }
    }

    /// Parse the wire spelling.
    pub fn parse(text: &str) -> Option<Self> {
        Some(match text {
            "finished" => Self::Finished,
            "budget" => Self::Budget,
            "cancelled" => Self::Cancelled,
            "denied" => Self::Denied,
            "tool_aborted" => Self::ToolAborted,
            "failed" => Self::Failed,
            _ => return None,
        })
    }
}

impl fmt::Display for StopReason {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// What kind of thing an event records.
///
/// `[R-SESSION-003]` fixes this set at ten. A closed set is the point: adding
/// one is a schema migration and a specification amendment, which is the cost
/// that stops the log growing an eleventh meaning by accident.
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(tag = "type")]
pub enum EventKind {
    /// A run began. A session holds one or more, per `[R-SESSION-005]`.
    Started {
        /// The task the run was given.
        task: String,
        /// Which agent ran.
        agent: String,
    },
    /// A message from the user or the calling script.
    UserMessage {
        /// What was said.
        content: String,
    },
    /// A message from the model, with any tool calls it asked for.
    Assistant {
        /// The text it produced.
        content: String,
        /// The tools it asked to run, by name.
        tool_calls: Vec<String>,
    },
    /// A tool was invoked.
    ToolCall {
        /// The identifier that ties the call to its result.
        id: String,
        /// Which tool.
        name: String,
        /// The arguments, as JSON text.
        args: String,
    },
    /// A tool returned.
    ToolResult {
        /// The identifier of the call it answers.
        id: String,
        /// What it produced.
        output: String,
        /// How long it took.
        duration_ms: u64,
        /// What went wrong, when something did.
        error: Option<String>,
    },
    /// Policy decided about a call, per `[R-SESSION-070]`.
    Policy {
        /// The call it decided about.
        id: String,
        /// `allow`, `ask`, or `deny`.
        decision: String,
        /// The rule that produced it, when one matched.
        rule: Option<String>,
        /// Whether it came from a rule, an answer, or a session grant.
        source: String,
    },
    /// What a model call cost.
    Usage(Usage),
    /// A range of earlier events was summarised, per `[R-SESSION-010]`.
    Compaction {
        /// The inclusive range this supersedes. Those events stay.
        supersedes: RangeInclusive<u64>,
        /// The summary that replaces them for a model call.
        summary: String,
        /// How many tokens the summary saved.
        tokens_saved: u32,
    },
    /// Something worth recording that is not a message.
    Note {
        /// `info`, `warn`, or `error`.
        level: String,
        /// What happened.
        message: String,
    },
    /// A run ended, per `[R-SESSION-005]`.
    Finished {
        /// Why it stopped.
        stop: StopReason,
        /// What bound it, when the reason alone does not say.
        detail: Option<String>,
    },
}

impl EventKind {
    /// The name this kind is stored and serialised under.
    pub fn name(&self) -> &'static str {
        match self {
            Self::Started { .. } => "Started",
            Self::UserMessage { .. } => "UserMessage",
            Self::Assistant { .. } => "Assistant",
            Self::ToolCall { .. } => "ToolCall",
            Self::ToolResult { .. } => "ToolResult",
            Self::Policy { .. } => "Policy",
            Self::Usage(_) => "Usage",
            Self::Compaction { .. } => "Compaction",
            Self::Note { .. } => "Note",
            Self::Finished { .. } => "Finished",
        }
    }

    /// Every kind's name, for a reader validating a log it did not write.
    pub const NAMES: [&'static str; 10] = [
        "Started",
        "UserMessage",
        "Assistant",
        "ToolCall",
        "ToolResult",
        "Policy",
        "Usage",
        "Compaction",
        "Note",
        "Finished",
    ];
}
