// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Agents inside agents, and agents beside each other.

use std::sync::Arc;

use meow_core::StopReason;
use serde_json::{Value, json};
use tokio_util::sync::CancellationToken;

use crate::budget::Ledger;
use crate::engine::Engine;
use crate::event::Discard;
use crate::outcome::Outcome;
use crate::spec::AgentSpec;
use crate::tool::{Tool, ToolError};

/// An agent offered to another agent as a tool.
///
/// `[R-AGENT-050]`: it runs in its own session with the caller recorded as its
/// parent. The session identifier is the caller's to mint, so this carries the
/// spec and the engine and leaves the recording to whoever owns the log.
pub struct SubAgent {
    spec: Arc<AgentSpec>,
    engine: Arc<Engine>,
    ledger: Ledger,
    depth: u32,
}

impl std::fmt::Debug for SubAgent {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("SubAgent")
            .field("name", &self.spec.name)
            .field("depth", &self.depth)
            .finish()
    }
}

impl SubAgent {
    /// Offer an agent as a tool inside a run.
    ///
    /// The ledger is the caller's, narrowed to the sub-agent's declared
    /// budget, which is what makes `[R-AGENT-013]` and `[R-AGENT-014]` hold
    /// through nesting rather than only at the top.
    pub fn new(spec: Arc<AgentSpec>, engine: Arc<Engine>, caller: &Ledger, depth: u32) -> Self {
        let ledger = caller.child(spec.budget);
        Self {
            spec,
            engine,
            ledger,
            depth,
        }
    }
}

#[async_trait::async_trait]
impl Tool for SubAgent {
    fn name(&self) -> &str {
        &self.spec.name
    }

    fn description(&self) -> &str {
        "run this agent on a task and return what it concluded"
    }

    fn schema(&self) -> Value {
        json!({
            "type": "object",
            "properties": {
                "task": { "type": "string", "description": "what to do" }
            },
            "required": ["task"]
        })
    }

    async fn call(&self, args: &Value, cancel: &CancellationToken) -> Result<String, ToolError> {
        // [R-AGENT-052]: refused as a tool error, so the caller's model is
        // told and can do something else. A panic here would take the whole
        // run down over a mistake the model can recover from.
        if self.depth >= self.spec.max_depth {
            return Err(ToolError(format!(
                "sub-agents are nested {} deep, which is the limit",
                self.depth
            )));
        }

        let task = args.get("task").and_then(Value::as_str).unwrap_or_default();

        // [R-AGENT-053]: the only things that cross are the task and the
        // arguments. A sub-agent that inherited its caller's messages would
        // behave differently depending on who called it, which makes it
        // untestable and its budget unpredictable.
        let outcome = self
            .engine
            .run(&self.spec, task, &self.ledger, &mut Discard, cancel)
            .await;

        Ok(describe(&outcome))
    }
}

/// What a caller's model is told about a sub-agent's run.
///
/// `[R-AGENT-051]`: the text, the stop reason, and the parsed value when the
/// sub-agent declared a schema. The stop reason matters as much as the text: a
/// caller that cannot tell a finished answer from a budget stop will treat a
/// partial one as complete.
fn describe(outcome: &Outcome) -> String {
    let mut out = json!({
        "stop": outcome.stop.as_str(),
        "text": outcome.text,
    });
    if let Some(detail) = &outcome.detail {
        out["detail"] = json!(detail);
    }
    if let Some(value) = &outcome.value {
        out["value"] = value.clone();
    }
    out.to_string()
}

/// One agent, one task, ready to run but not running.
///
/// `[R-STAR-043]` builds these in Starlark, where a closure could not cross a
/// thread boundary but a declaration can.
pub struct Invocation {
    /// Which agent.
    pub spec: Arc<AgentSpec>,
    /// What to give it.
    pub task: String,
}

impl std::fmt::Debug for Invocation {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Invocation")
            .field("agent", &self.spec.name)
            .field("task", &self.task)
            .finish()
    }
}

/// Run several agents at once.
///
/// Satisfies `[R-AGENT-060]` by returning results in the order given, whatever
/// order they finish in; `[R-AGENT-061]` by letting one failure be a result
/// with a non-finished stop reason rather than something that takes the others
/// down; and `[R-AGENT-062]` by sharing the caller's ledger, so the fan-out
/// stops starting new work once the budget is spent rather than each branch
/// spending it in full.
pub async fn run_parallel(
    engine: Arc<Engine>,
    invocations: Vec<Invocation>,
    caller: &Ledger,
    cancel: &CancellationToken,
) -> Vec<Outcome> {
    let mut handles = Vec::with_capacity(invocations.len());

    for invocation in invocations {
        let ledger = caller.child(invocation.spec.budget);
        let engine = Arc::clone(&engine);
        let cancel = cancel.clone();

        // The budget is shared, so a branch that starts after it is spent is
        // stopped by its own first reservation rather than by a check here
        // that could race.
        handles.push(tokio::spawn(async move {
            engine
                .run(
                    &invocation.spec,
                    &invocation.task,
                    &ledger,
                    &mut Discard,
                    &cancel,
                )
                .await
        }));
    }

    let mut out = Vec::with_capacity(handles.len());
    for handle in handles {
        out.push(handle.await.unwrap_or_else(|e| Outcome {
            stop: StopReason::Failed,
            detail: Some(format!("the run did not finish: {e}")),
            text: String::new(),
            value: None,
            usage: meow_core::Usage::default(),
            steps: Vec::new(),
        }));
    }
    out
}
