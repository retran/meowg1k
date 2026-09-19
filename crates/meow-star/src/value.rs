// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Agents and tools as Starlark values.
//!
//! `meow.agent` and `meow.tool` return something you can hold, rather than
//! registering a name you later quote. That is what `[R-STAR-041]` needs: an
//! agent goes into another agent's `tools` list the same way a tool does, and
//! presents the same schema there. It is also what the migration table in
//! `0.3.0-starlark-api.md` means by replacing `ctx.run("name", k = v)` with
//! `tool.run(k = v)` - the tool value, not its name.
//!
//! Each value holds only its name. Everything else is looked up through
//! `Evaluator::extra`, which keeps these types small enough to freeze into a
//! module without dragging the runtime along behind them.
//!
//! The file carries `#![allow(unsafe_code)]` because `ProvidesStaticType` is
//! an unsafe trait and its derive writes the `unsafe impl` as a sibling item,
//! which an `#[allow]` on the struct does not cover. There is no unsafe block
//! of ours in here.
#![allow(unsafe_code)]

use allocative::Allocative;
use serde_json::{Map, Value};
use starlark::any::ProvidesStaticType;
use starlark::environment::{Methods, MethodsBuilder, MethodsStatic};
use starlark::eval::Evaluator;
use starlark::starlark_module;
use starlark::values::structs::AllocStruct;
use starlark::values::{NoSerialize, StarlarkValue, Value as StarValue, starlark_value};
use starlark::{starlark_simple_value, values::ValueLike};

use crate::run::running;

/// A declared agent.
#[derive(Debug, Allocative, ProvidesStaticType, NoSerialize)]
pub struct Agent {
    /// What it was declared as.
    pub name: String,
}

impl std::fmt::Display for Agent {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "agent({})", self.name)
    }
}

starlark_simple_value!(Agent);

#[starlark_value(type = "agent")]
impl<'v> StarlarkValue<'v> for Agent {
    type Canonical = Self;

    fn get_methods() -> Option<&'static Methods> {
        static METHODS: MethodsStatic = MethodsStatic::new("agent", agent_methods);
        Some(METHODS.methods())
    }
}

/// A declared tool.
#[derive(Debug, Allocative, ProvidesStaticType, NoSerialize)]
pub struct Tool {
    /// What it was declared as.
    pub name: String,
}

impl std::fmt::Display for Tool {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "tool({})", self.name)
    }
}

starlark_simple_value!(Tool);

#[starlark_value(type = "tool")]
impl<'v> StarlarkValue<'v> for Tool {
    type Canonical = Self;

    fn get_methods() -> Option<&'static Methods> {
        static METHODS: MethodsStatic = MethodsStatic::new("tool", tool_methods);
        Some(METHODS.methods())
    }
}

/// One agent and one task, built but not run.
///
/// `[R-STAR-043]`. A Starlark closure cannot cross a thread boundary, so
/// `meow.parallel` is given declarations of work rather than things to call.
#[derive(Debug, Allocative, ProvidesStaticType, NoSerialize)]
pub struct Invocation {
    /// Which agent.
    pub agent: String,
    /// What to give it.
    pub task: String,
}

impl std::fmt::Display for Invocation {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "invocation({}, {:?})", self.agent, self.task)
    }
}

starlark_simple_value!(Invocation);

#[starlark_value(type = "invocation")]
impl<'v> StarlarkValue<'v> for Invocation {
    type Canonical = Self;
}

/// The name a `tools` entry refers to.
///
/// A value or a string, because a Starlark declaration holds values and a
/// markdown one can only hold names. `[R-STAR-051]` needs both to produce the
/// same agent, and they do: both end up as a name here.
pub fn name_of(value: StarValue<'_>) -> Option<String> {
    if let Some(agent) = value.downcast_ref::<Agent>() {
        return Some(agent.name.clone());
    }
    if let Some(tool) = value.downcast_ref::<Tool>() {
        return Some(tool.name.clone());
    }
    value.unpack_str().map(str::to_owned)
}

fn oops(message: impl std::fmt::Display) -> starlark::Error {
    starlark::Error::new_other(anyhow::anyhow!("{message}"))
}

/// Turn an engine outcome into what a handler reads.
///
/// `[R-STAR-042]`: `stop` and `detail` travel with the text, because a caller
/// that cannot tell a finished answer from a budget stop will treat a partial
/// one as complete. That is the defect v0.2.x had, where `agent_turn` returned
/// a bare string and "the model finished", "we hit max_iterations", and "the
/// model said nothing" were all the same value.
pub fn outcome<'v>(
    heap: starlark::values::Heap<'v>,
    outcome: &meow_agent::Outcome,
    session: &str,
) -> StarValue<'v> {
    let usage = heap.alloc(AllocStruct([
        ("prompt", heap.alloc(outcome.usage.prompt)),
        ("completion", heap.alloc(outcome.usage.completion)),
        (
            "cached",
            match outcome.usage.cached {
                Some(cached) => heap.alloc(cached),
                None => StarValue::new_none(),
            },
        ),
        (
            "cost_micros",
            match outcome.usage.cost_micros {
                Some(cost) => heap.alloc(cost),
                None => StarValue::new_none(),
            },
        ),
    ]));

    let steps = heap.alloc(
        outcome
            .steps
            .iter()
            .map(|step| {
                heap.alloc(AllocStruct([
                    ("index", heap.alloc(step.index)),
                    ("text", heap.alloc(step.text.clone())),
                    ("tools", heap.alloc(step.tool_calls.clone())),
                ]))
            })
            .collect::<Vec<_>>(),
    );

    heap.alloc(AllocStruct([
        ("text", heap.alloc(outcome.text.clone())),
        (
            "value",
            match &outcome.value {
                Some(value) => heap.alloc(value.clone()),
                None => StarValue::new_none(),
            },
        ),
        ("stop", heap.alloc(outcome.stop.as_str())),
        (
            "detail",
            match &outcome.detail {
                Some(detail) => heap.alloc(detail.clone()),
                None => StarValue::new_none(),
            },
        ),
        ("ok", heap.alloc(outcome.stop.as_str() == "finished")),
        ("usage", usage),
        ("steps", steps),
        ("session", heap.alloc(session.to_owned())),
    ]))
}

#[starlark_module]
fn agent_methods(builder: &mut MethodsBuilder) {
    /// Run this agent on a task and wait for it.
    ///
    /// `[R-STAR-081]`: the call blocks the script thread. There is no future
    /// and no callback, because either one would mean a Starlark value
    /// crossing a thread boundary.
    fn run<'v>(
        this: &Agent,
        #[starlark(require = pos)] task: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        let state = running(eval, "agent.run")?;
        let result = state
            .runtime
            .run_agent(&this.name, &task, Some(&state.ledger), state.depth)
            .map_err(oops)?;
        let session = state.runtime.session().id();
        Ok(outcome(eval.heap(), &result, &session))
    }

    /// Build an invocation without running it.
    fn call<'v>(
        this: &Agent,
        #[starlark(require = pos)] task: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<Invocation> {
        running(eval, "agent.call")?;
        Ok(Invocation {
            agent: this.name.clone(),
            task,
        })
    }

    /// What it was declared as.
    #[starlark(attribute)]
    fn name(this: &Agent) -> starlark::Result<String> {
        Ok(this.name.clone())
    }
}

#[starlark_module]
fn tool_methods(builder: &mut MethodsBuilder) {
    /// Run this tool with the given arguments.
    fn run<'v>(
        this: &Tool,
        #[starlark(kwargs)] kwargs: StarValue<'v>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        let state = running(eval, "tool.run")?;
        let given = match kwargs.to_json_value().map_err(oops)? {
            Value::Object(map) => map,
            _ => Map::new(),
        };

        let declared = state
            .runtime
            .registry()
            .tool(&this.name)
            .ok_or_else(|| oops(format!("tool `{}` is not declared", this.name)))?;

        let bound = declared.args.bind(&given).map_err(|problems| {
            oops(
                problems
                    .iter()
                    .map(ToString::to_string)
                    .collect::<Vec<_>>()
                    .join("; "),
            )
        })?;
        let handler = declared.handler.clone();

        state
            .runtime
            .call_handler(&handler, &bound, &state.ledger, state.depth)
            .map_err(oops)
    }

    /// What it was declared as.
    #[starlark(attribute)]
    fn name(this: &Tool) -> starlark::Result<String> {
        Ok(this.name.clone())
    }
}

/// Run several invocations at once.
///
/// Satisfies `[R-STAR-043]` by taking invocations rather than callables, and
/// `[R-STAR-044]` by saying why when it is given something else. A Starlark
/// function looks like the natural argument here and cannot be one: it lives
/// on its evaluator's heap, and the runs happen on other threads.
pub fn parallel<'v>(
    items: StarValue<'v>,
    eval: &mut Evaluator<'v, '_, '_>,
) -> starlark::Result<StarValue<'v>> {
    let state = running(eval, "meow.parallel")?;

    let mut invocations = Vec::new();
    for item in items.iterate(eval.heap())? {
        let Some(invocation) = item.downcast_ref::<Invocation>() else {
            return Err(oops(format!(
                "`meow.parallel` takes invocations built with `agent.call(...)`, and was given {}. \
                 A Starlark function cannot be one: it belongs to the evaluator that made it, and \
                 the runs happen on other threads.",
                item.get_type()
            )));
        };
        invocations.push((invocation.agent.clone(), invocation.task.clone()));
    }

    // Sequentially for now: each run already blocks this thread, and running
    // them at once needs the engine's own fan-out, which takes specs rather
    // than names. The ordering guarantee is the part a caller depends on, and
    // it holds either way.
    let session = state.runtime.session().id();
    let mut results = Vec::with_capacity(invocations.len());
    for (agent, task) in invocations {
        let result = state
            .runtime
            .run_agent(&agent, &task, Some(&state.ledger), state.depth)
            .map_err(oops)?;
        results.push(outcome(eval.heap(), &result, &session));
    }

    Ok(eval.heap().alloc(results))
}

/// An agent value for a declared name.
pub fn agent(name: &str) -> Agent {
    Agent {
        name: name.to_owned(),
    }
}

/// A tool value for a declared name.
pub fn tool(name: &str) -> Tool {
    Tool {
        name: name.to_owned(),
    }
}
