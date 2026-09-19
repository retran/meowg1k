// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The handler context.
//!
//! `[R-STAR-020]`: six members and `cancelled()`, against v0.2.x's twenty-six.
//! `[R-STAR-021]`: the other twenty were runtime capabilities, and a handler
//! now reaches those with `load`. The difference matters beyond tidiness -
//! a capability on the context is invisible to `meow policy explain`, because
//! nothing in the file says the handler is going to use it.

use std::sync::OnceLock;

use serde_json::Value;
use starlark::environment::{Globals, GlobalsBuilder, Module};
use starlark::eval::Evaluator;
use starlark::starlark_module;
use starlark::values::Value as StarValue;
use starlark::values::none::{NoneOr, NoneType};
use starlark::values::structs::AllocStruct;

use crate::error::{Result, StarError};
use crate::run::running;
use crate::state::Running;

/// The native functions the context is assembled from.
///
/// Built once. They find the run they belong to through `Evaluator::extra`,
/// which is why one set of functions can serve every invocation.
fn parts() -> &'static Globals {
    static PARTS: OnceLock<Globals> = OnceLock::new();
    PARTS.get_or_init(|| {
        GlobalsBuilder::new()
            .with(top_level)
            .with_namespace("out", out_module)
            .with_namespace("ask", ask_module)
            .with_namespace("stdin", stdin_module)
            .with_namespace("session", session_module)
            .build()
    })
}

/// Build the context a handler is called with.
///
/// # Errors
///
/// [`StarError::Starlark`] if a member will not allocate, which would be a
/// defect here rather than in anything a user wrote.
pub fn build<'v>(scope: &Module<'v>, state: &Running) -> Result<StarValue<'v>> {
    let parts = parts();
    scope.frozen_heap().add_reference(parts.heap());

    let member = |name: &str| -> Result<StarValue<'v>> {
        parts
            .iter()
            .find(|(candidate, _)| *candidate == name)
            .map(|(_, value)| value.to_value())
            .ok_or_else(|| StarError::Starlark(format!("the context has no `{name}`")))
    };

    let args = scope.heap().alloc(AllocStruct(
        state
            .args
            .iter()
            .map(|(name, value)| (name.clone(), scope.heap().alloc(value.clone()))),
    ));

    let workspace = scope.heap().alloc(
        state
            .runtime
            .workspace()
            .root()
            .to_string_lossy()
            .into_owned(),
    );

    Ok(scope.heap().alloc(AllocStruct([
        ("args", args),
        ("session", member("session")?),
        ("out", member("out")?),
        ("ask", member("ask")?),
        ("stdin", member("stdin")?),
        ("workspace", workspace),
        ("cancelled", member("cancelled")?),
    ])))
}

fn oops(message: impl std::fmt::Display) -> starlark::Error {
    starlark::Error::new_other(anyhow::anyhow!("{message}"))
}

#[starlark_module]
fn top_level(builder: &mut GlobalsBuilder) {
    /// Whether the user has interrupted.
    ///
    /// A function rather than a field, because a field would be read once at
    /// the top of a handler and never again, which is the opposite of what a
    /// long loop needs.
    fn cancelled(eval: &mut Evaluator) -> starlark::Result<bool> {
        Ok(running(eval, "ctx.cancelled")?.runtime.cancelled())
    }
}

#[starlark_module]
fn out_module(builder: &mut GlobalsBuilder) {
    /// A line of plain text.
    fn write(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator,
    ) -> starlark::Result<NoneType> {
        running(eval, "ctx.out.write")?.runtime.out().write(&text);
        Ok(NoneType)
    }

    /// An aside: counts, timings, what was skipped.
    fn note(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator,
    ) -> starlark::Result<NoneType> {
        running(eval, "ctx.out.note")?.runtime.out().note(&text);
        Ok(NoneType)
    }

    /// Something the reader should act on.
    fn warn(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator,
    ) -> starlark::Result<NoneType> {
        running(eval, "ctx.out.warn")?.runtime.out().warn(&text);
        Ok(NoneType)
    }

    /// A step in a longer piece of work.
    fn step(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator,
    ) -> starlark::Result<NoneType> {
        running(eval, "ctx.out.step")?.runtime.out().step(&text);
        Ok(NoneType)
    }

    /// Text to render as markdown.
    fn markdown(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator,
    ) -> starlark::Result<NoneType> {
        running(eval, "ctx.out.markdown")?
            .runtime
            .out()
            .markdown(&text);
        Ok(NoneType)
    }

    /// One result, with where it is and how much it matters.
    fn finding(
        #[starlark(require = pos)] severity: String,
        #[starlark(require = pos)] location: String,
        #[starlark(require = pos)] summary: String,
        eval: &mut Evaluator,
    ) -> starlark::Result<NoneType> {
        running(eval, "ctx.out.finding")?
            .runtime
            .out()
            .finding(&severity, &location, &summary);
        Ok(NoneType)
    }
}

#[starlark_module]
fn ask_module(builder: &mut GlobalsBuilder) {
    /// Ask for a line of text.
    fn text(
        #[starlark(require = pos)] prompt: String,
        #[starlark(require = named)] default: Option<String>,
        eval: &mut Evaluator,
    ) -> starlark::Result<String> {
        running(eval, "ctx.ask.text")?
            .runtime
            .ask()
            .text(&prompt, default.as_deref())
            .map_err(oops)
    }

    /// Ask for a yes or a no.
    fn confirm(
        #[starlark(require = pos)] prompt: String,
        #[starlark(require = named, default = false)] default: bool,
        eval: &mut Evaluator,
    ) -> starlark::Result<bool> {
        running(eval, "ctx.ask.confirm")?
            .runtime
            .ask()
            .confirm(&prompt, default)
            .map_err(oops)
    }

    /// Ask the person to pick one.
    fn select<'v>(
        #[starlark(require = pos)] prompt: String,
        #[starlark(require = pos)] choices: StarValue<'v>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        let state = running(eval, "ctx.ask.select")?;
        let choices = match choices.to_json_value().map_err(oops)? {
            Value::Array(items) => items
                .into_iter()
                .map(|item| match item {
                    Value::String(s) => Ok(s),
                    other => Err(oops(format!(
                        "a choice must be a string, and one is {other}"
                    ))),
                })
                .collect::<starlark::Result<Vec<String>>>()?,
            other => return Err(oops(format!("`choices` must be a list, and is {other}"))),
        };
        state.runtime.ask().select(&prompt, &choices).map_err(oops)
    }
}

#[starlark_module]
fn stdin_module(builder: &mut GlobalsBuilder) {
    /// Whether anything was piped in.
    fn is_piped(eval: &mut Evaluator) -> starlark::Result<bool> {
        Ok(running(eval, "ctx.stdin.is_piped")?
            .runtime
            .stdin()
            .is_piped())
    }

    /// Read all of it.
    fn read(eval: &mut Evaluator) -> starlark::Result<String> {
        running(eval, "ctx.stdin.read")?
            .runtime
            .stdin()
            .read()
            .map_err(oops)
    }
}

#[starlark_module]
fn session_module(builder: &mut GlobalsBuilder) {
    /// Which session this invocation belongs to.
    fn id(eval: &mut Evaluator) -> starlark::Result<String> {
        Ok(running(eval, "ctx.session.id")?.runtime.session().id())
    }

    /// Read something this run stored.
    fn get<'v>(
        #[starlark(require = pos)] key: String,
        #[starlark(require = named)] default: Option<StarValue<'v>>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneOr<StarValue<'v>>> {
        let stored = running(eval, "ctx.session.get")?
            .runtime
            .session()
            .get(&key);
        Ok(match (stored, default) {
            (Some(value), _) => NoneOr::Other(eval.heap().alloc(value)),
            (None, Some(default)) => NoneOr::Other(default),
            (None, None) => NoneOr::None,
        })
    }

    /// Store something for the rest of this run.
    fn set<'v>(
        #[starlark(require = pos)] key: String,
        #[starlark(require = pos)] value: StarValue<'v>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let value = value.to_json_value().map_err(oops)?;
        running(eval, "ctx.session.set")?
            .runtime
            .session()
            .set(&key, value);
        Ok(NoneType)
    }
}
