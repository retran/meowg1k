// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What the engine tells whoever is watching.

use meow_core::{StopReason, Usage};

/// Something the engine did.
///
/// `[R-AGENT-070]` requires every observable transition. The renderer draws
/// from these, a script callback receives these, and a test collects these, so
/// none of the three needs to know about the others.
#[derive(Debug, Clone, PartialEq)]
pub enum AgentEvent {
    /// The run began.
    RunStart {
        /// Which agent.
        agent: String,
    },
    /// A step began.
    StepStart {
        /// Which step, from one.
        step: u32,
    },
    /// More of the answer.
    Text(String),
    /// More of the model's reasoning.
    Thinking(String),
    /// The whole text of a step, for a sink that declined deltas.
    StepText {
        /// Which step.
        step: u32,
        /// Everything the model said in it.
        text: String,
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
        /// Which call.
        id: String,
        /// How long it took.
        duration_ms: u64,
        /// What went wrong, when something did.
        error: Option<String>,
    },
    /// Policy decided about a call.
    Policy {
        /// Which call.
        id: String,
        /// What it decided.
        decision: String,
    },
    /// What a model call cost.
    Usage(Usage),
    /// A step ended.
    StepEnd {
        /// Which step.
        step: u32,
    },
    /// The run ended.
    RunEnd {
        /// Why it stopped.
        stop: StopReason,
        /// What bound it.
        detail: Option<String>,
    },
}

/// Where the engine sends what it does.
pub trait Sink: Send {
    /// Take one event.
    ///
    /// # Errors
    ///
    /// Whatever the consumer decides. The engine does not interpret it: it
    /// stops delivering to this sink, records a note, and carries on, per
    /// `[R-AGENT-071]`.
    fn event(&mut self, event: AgentEvent) -> Result<(), String>;

    /// Whether this sink wants text and thinking a token at a time.
    ///
    /// `[R-AGENT-073]`. A sink backed by a script callback pays one call per
    /// token otherwise, which is a cost nobody asked for. Declining gets
    /// [`AgentEvent::StepText`] once per step instead.
    fn wants_deltas(&self) -> bool {
        true
    }
}

/// A sink that keeps everything.
#[derive(Debug, Default)]
pub struct Collect {
    /// Everything it was given, in order.
    pub events: Vec<AgentEvent>,
    /// Whether it asked for deltas.
    pub deltas: bool,
}

impl Collect {
    /// A sink that wants deltas.
    pub fn with_deltas() -> Self {
        Self {
            events: Vec::new(),
            deltas: true,
        }
    }

    /// A sink that does not.
    pub fn without_deltas() -> Self {
        Self {
            events: Vec::new(),
            deltas: false,
        }
    }
}

impl Sink for Collect {
    fn event(&mut self, event: AgentEvent) -> Result<(), String> {
        self.events.push(event);
        Ok(())
    }

    fn wants_deltas(&self) -> bool {
        self.deltas
    }
}

/// A sink that ignores everything, for a caller that wants no events.
///
/// `[R-AGENT-072]`: the engine works with nothing attached.
#[derive(Debug, Default)]
pub struct Discard;

impl Sink for Discard {
    fn event(&mut self, _event: AgentEvent) -> Result<(), String> {
        Ok(())
    }

    fn wants_deltas(&self) -> bool {
        false
    }
}
