// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The agent loop: send a prompt and the tool schemas to a model, run the
//! tools it asks for, feed the results back, and stop when the model is done
//! or a bound is reached.
//!
//! The engine owns budgets, cancellation, and the outcome it returns. It does
//! not know Starlark exists, does not render anything, and does not talk to a
//! vendor API.
//!
//! `docs/spec/agent.md` is normative.

mod budget;
mod compaction;
mod engine;
mod event;
mod nested;
mod outcome;
mod spec;
mod tool;

pub use crate::budget::{Axis, Budget, Ledger};
pub use crate::compaction::{Compaction, estimate_tokens, range_to_compact};
pub use crate::engine::Engine;
pub use crate::event::{AgentEvent, Collect, Discard, Sink};
pub use crate::nested::{Invocation, SubAgent, run_parallel};
pub use crate::outcome::{Outcome, Step};
pub use crate::spec::{AgentSpec, DescribeCall, ToolErrorPolicy};
pub use crate::tool::{Checked, Tool, ToolError, ToolSet, check};
