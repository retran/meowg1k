// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What a run returns.

use meow_core::{StopReason, Usage};
use serde_json::Value;

/// One pass of the loop.
#[derive(Debug, Clone, PartialEq)]
pub struct Step {
    /// Which step, from one.
    pub index: u32,
    /// What the model said in it.
    pub text: String,
    /// The tools it asked for, by name.
    pub tool_calls: Vec<String>,
}

/// What a run produced.
///
/// `[R-AGENT-001]`: every run that starts returns one of these. v0.2.x
/// returned a bare string, so a caller could not tell "the model finished"
/// from "we exhausted max_iterations" from "the model returned empty text" -
/// and the last two arrived as the same error, with the transcript discarded.
#[derive(Debug, Clone, PartialEq)]
pub struct Outcome {
    /// Why it stopped.
    pub stop: StopReason,
    /// What bound it: which budget axis, which tool, which rule.
    ///
    /// `[R-AGENT-004]`. Without it, `budget` says a run stopped and not what
    /// to raise to let it finish.
    pub detail: Option<String>,
    /// The model's last text, whatever the stop reason.
    pub text: String,
    /// The parsed answer, when a schema was asked for and the run finished.
    pub value: Option<Value>,
    /// What it cost.
    pub usage: Usage,
    /// The transcript.
    pub steps: Vec<Step>,
}

impl Outcome {
    /// Whether the model finished rather than being stopped.
    pub fn ok(&self) -> bool {
        self.stop == StopReason::Finished
    }
}
