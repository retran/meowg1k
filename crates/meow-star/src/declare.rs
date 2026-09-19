// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The `meow` global, and the state the declarations accumulate into.

use serde_json::{Map, Value, json};
use starlark::environment::GlobalsBuilder;
use starlark::eval::Evaluator;
use starlark::starlark_module;
use starlark::values::Value as StarValue;
use starlark::values::none::NoneType;
use starlark::values::tuple::UnpackTuple;
use starlark::values::typing::StarlarkCallable;

use crate::agent;
use crate::args::{self, Args};
use crate::error::StarError;
use crate::registry::{Model, Provider, ToolDecl};
use crate::schema;

/// Which half of a run is executing.
///
/// `[R-STAR-030]` and `[R-STAR-084]` both turn on this one bit: a declaration
/// call is refused outside `Phase::Declaring`, and a runtime module is refused
/// inside it.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Phase {
    /// `.meow/` is being evaluated.
    Declaring,
    /// A handler is running.
    Running,
}

/// Holds [`Declaring`].
///
/// The module exists for its inner attribute. `ProvidesStaticType` is an
/// unsafe trait and its derive writes the `unsafe impl` as a sibling item, so
/// an `#[allow]` on the struct does not cover it. Scoping the exemption to
/// this module keeps the workspace-wide denial intact everywhere else, and
/// there is no unsafe block of ours in here to hide behind it.
mod state {
    #![allow(unsafe_code)]

    use std::cell::{Cell, RefCell};

    use starlark::any::ProvidesStaticType;

    use crate::declare::Phase;
    use crate::registry::{Origin, Registry};

    /// What the declaration files are building.
    ///
    /// Held behind `Evaluator::extra`, which hands out a shared reference, so
    /// the interior mutability is not a shortcut around a borrow but the only
    /// shape the evaluator offers.
    #[derive(Debug, ProvidesStaticType)]
    pub struct Declaring {
        registry: RefCell<Registry>,
        file: RefCell<String>,
        phase: Cell<Phase>,
    }

    impl Default for Declaring {
        fn default() -> Self {
            Self::new()
        }
    }

    impl Declaring {
        /// Start with nothing declared.
        pub fn new() -> Self {
            Self {
                registry: RefCell::new(Registry::new()),
                file: RefCell::new(String::new()),
                phase: Cell::new(Phase::Declaring),
            }
        }

        /// Say which file is being evaluated, and get back the one it replaced.
        ///
        /// The caller restores the previous name, because a `load` evaluates a
        /// second file in the middle of the first and an origin recorded after
        /// that would otherwise name the wrong one.
        pub fn entering(&self, file: &str) -> String {
            self.file.replace(file.to_owned())
        }

        /// Which phase the evaluator is in.
        pub fn phase(&self) -> Phase {
            self.phase.get()
        }

        /// Move to the running phase, after `.meow/` has been evaluated.
        pub fn running(&self) {
            self.phase.set(Phase::Running);
        }

        /// Take the registry out once loading is done.
        pub fn finish(self) -> Registry {
            self.registry.into_inner()
        }

        /// Borrow what has been declared so far.
        pub fn registry(&self) -> std::cell::Ref<'_, Registry> {
            self.registry.borrow()
        }

        /// Borrow it to add to it.
        pub(crate) fn registry_mut(&self) -> std::cell::RefMut<'_, Registry> {
            self.registry.borrow_mut()
        }

        /// Which file a declaration made now belongs to.
        pub(crate) fn origin(&self) -> Origin {
            Origin(self.file.borrow().clone())
        }
    }
}

pub use state::Declaring;

/// Reach the declaration state from inside a builtin.
fn declaring<'a>(eval: &Evaluator<'_, 'a, '_>, what: &str) -> starlark::Result<&'a Declaring> {
    let state = eval
        .extra
        .and_then(|extra| extra.downcast_ref::<Declaring>())
        .ok_or_else(|| {
            starlark::Error::new_other(anyhow::anyhow!("`{what}` is not available here"))
        })?;

    // [R-STAR-030]: a handler that could declare a tool would make the tool
    // set unknowable before a run, and `meow policy explain` depends on it
    // being knowable.
    if state.phase() != Phase::Declaring {
        return Err(starlark::Error::new_other(anyhow::anyhow!(
            "{}",
            StarError::NotDeclaring {
                what: what.to_owned()
            }
        )));
    }
    Ok(state)
}

fn fail(e: StarError) -> starlark::Error {
    starlark::Error::new_other(anyhow::anyhow!("{e}"))
}

fn as_json(value: StarValue<'_>) -> starlark::Result<Value> {
    value
        .to_json_value()
        .map_err(|e| starlark::Error::new_other(anyhow::anyhow!("{e}")))
}

fn as_object(value: Option<StarValue<'_>>, what: &str) -> starlark::Result<Map<String, Value>> {
    match value {
        None => Ok(Map::new()),
        Some(value) => match as_json(value)? {
            Value::Object(map) => Ok(map),
            other => Err(starlark::Error::new_other(anyhow::anyhow!(
                "`{what}` must be a dict, and is {other}"
            ))),
        },
    }
}

/// Read a nested declaration through the shape both Starlark and YAML use.
///
/// `[R-STAR-051]`: one structure, filled from either syntax, so a default can
/// only be decided in one place.
fn from_json<T: serde::de::DeserializeOwned>(
    value: Option<StarValue<'_>>,
    what: &str,
) -> starlark::Result<Option<T>> {
    let Some(value) = value else { return Ok(None) };
    let json = as_json(value)?;
    serde_json::from_value(json)
        .map(Some)
        .map_err(|e| starlark::Error::new_other(anyhow::anyhow!("`{what}`: {e}")))
}

fn as_f64(value: StarValue<'_>) -> starlark::Result<f64> {
    match as_json(value)? {
        Value::Number(n) => n.as_f64().ok_or_else(|| {
            starlark::Error::new_other(anyhow::anyhow!("`{n}` is not a number this can hold"))
        }),
        other => Err(starlark::Error::new_other(anyhow::anyhow!(
            "must be a number, and is {other}"
        ))),
    }
}

fn as_strings(value: Option<StarValue<'_>>, what: &str) -> starlark::Result<Vec<String>> {
    match value {
        None => Ok(Vec::new()),
        Some(value) => match as_json(value)? {
            Value::Array(items) => items
                .into_iter()
                .map(|item| match item {
                    Value::String(s) => Ok(s),
                    other => Err(starlark::Error::new_other(anyhow::anyhow!(
                        "`{what}` must be a list of names, and carries {other}"
                    ))),
                })
                .collect(),
            other => Err(starlark::Error::new_other(anyhow::anyhow!(
                "`{what}` must be a list, and is {other}"
            ))),
        },
    }
}

/// The declaration half of the `meow` global.
#[starlark_module]
fn declarations(builder: &mut GlobalsBuilder) {
    /// Declare a place to reach models.
    fn provider<'v>(
        #[starlark(require = named)] name: String,
        #[starlark(require = named)] kind: String,
        #[starlark(require = named)] api_key: Option<String>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let state = declaring(eval, "meow.provider")?;
        let origin = state.origin();
        state
            .registry_mut()
            .add_provider(Provider {
                name,
                kind,
                api_key,
                origin,
            })
            .map_err(fail)?;
        Ok(NoneType)
    }

    /// Declare a model an agent can name.
    fn model<'v>(
        #[starlark(require = named)] name: String,
        #[starlark(require = named)] provider: String,
        #[starlark(require = named)] id: String,
        #[starlark(require = named)] context: u32,
        #[starlark(require = named)] max_output: u32,
        #[starlark(require = named)] temperature: Option<StarValue<'v>>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let state = declaring(eval, "meow.model")?;
        let origin = state.origin();
        state
            .registry_mut()
            .add_model(Model {
                name,
                provider,
                id,
                context,
                max_output,
                #[allow(clippy::cast_possible_truncation)]
                temperature: temperature.map(as_f64).transpose()?.map(|t| t as f32),
                origin,
            })
            .map_err(fail)?;
        Ok(NoneType)
    }

    /// Declare a tool.
    ///
    /// The handler is checked for being callable and is otherwise untouched
    /// here: binding it belongs to the run, where the frozen module it lives
    /// in is still alive.
    fn tool<'v>(
        #[starlark(require = named)] name: String,
        #[starlark(require = named)] about: String,
        #[starlark(require = named)] run: StarlarkCallable<'v>,
        #[starlark(require = named)] args: Option<StarValue<'v>>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let fields = as_object(args, "args")?;
        let _ = run;
        let args = Args::new(fields).map_err(fail)?;
        let state = declaring(eval, "meow.tool")?;
        let origin = state.origin();
        state
            .registry_mut()
            .add_tool(ToolDecl {
                name,
                about,
                args,
                origin,
            })
            .map_err(fail)?;
        Ok(NoneType)
    }

    /// Declare an agent.
    ///
    /// The keyword arguments are `[R-STAR-040]`'s set, and they are collected
    /// into [`agent::Fields`] rather than used directly, because a markdown
    /// agent fills the same structure and `[R-STAR-051]` asks the two to come
    /// out identical. Naming them explicitly here rather than taking `**kwargs`
    /// keeps Starlark's own error for a misspelled argument, which points at
    /// the call.
    // `meow.agent` takes the ten keyword arguments `[R-STAR-040]` names.
    // Grouping them into a struct would satisfy the lint and change the
    // Starlark surface, and the Starlark surface is the product.
    #[allow(clippy::too_many_arguments)]
    fn agent<'v>(
        #[starlark(require = named)] name: String,
        #[starlark(require = named)] model: String,
        #[starlark(require = named)] system: String,
        #[starlark(require = named)] about: Option<String>,
        #[starlark(require = named)] tools: Option<StarValue<'v>>,
        #[starlark(require = named)] budget: Option<StarValue<'v>>,
        #[starlark(require = named)] compaction: Option<StarValue<'v>>,
        #[starlark(require = named)] output: Option<StarValue<'v>>,
        #[starlark(require = named)] on_tool_error: Option<String>,
        #[starlark(require = named)] policy: Option<StarValue<'v>>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let fields = agent::Fields {
            model: Some(model),
            system: Some(system),
            about,
            tools: Some(as_strings(tools, "tools")?),
            budget: from_json(budget, "budget")?,
            compaction: from_json(compaction, "compaction")?,
            output: output.map(as_json).transpose()?,
            on_tool_error,
            policy: from_json(policy, "policy")?,
            include: None,
        };

        let state = declaring(eval, "meow.agent")?;
        let origin = state.origin();
        let declared = fields
            .build(&name, agent::Source::Starlark, &origin, |_| unreachable!())
            .map_err(fail)?;
        state.registry_mut().add_agent(declared).map_err(fail)?;
        Ok(NoneType)
    }

    /// Put a declared tool or agent on the command line.
    fn command<'v>(
        #[starlark(require = named)] name: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let state = declaring(eval, "meow.command")?;
        let origin = state.origin();
        state
            .registry_mut()
            .add_command(&name, origin)
            .map_err(fail)?;
        Ok(NoneType)
    }

    /// Declare what every agent in this workspace may do.
    ///
    /// `[R-STAR-030]` covers this call for the same reason it covers the
    /// others: a policy written inside a handler would be applied after the
    /// calls it was meant to govern, which reads as a permission bug rather
    /// than as a mistake in the file.
    fn policy<'v>(
        #[starlark(require = named)] rules: StarValue<'v>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let rules: Vec<agent::RuleFields> = from_json(Some(rules), "rules")?.unwrap_or_default();
        let state = declaring(eval, "meow.policy")?;
        let origin = state.origin();
        let policy = agent::build_policy(&rules, &origin).map_err(fail)?;
        state
            .registry_mut()
            .set_policy(policy, origin)
            .map_err(fail)?;
        Ok(NoneType)
    }
}

/// `meow.arg`: one declaration that becomes a flag, a help line, and a schema.
#[starlark_module]
fn arg_constructors(builder: &mut GlobalsBuilder) {
    /// A string argument.
    fn string(
        #[starlark(require = named)] about: Option<String>,
        #[starlark(require = named)] default: Option<String>,
        #[starlark(require = named, default = true)] required: bool,
        #[starlark(require = named)] positional: Option<u32>,
        #[starlark(require = named)] max_len: Option<u32>,
        #[starlark(require = named)] pattern: Option<String>,
    ) -> starlark::Result<Value> {
        let mut node = json!({ "type": "string" });
        describe(&mut node, about, default.map(Value::String));
        put(&mut node, "maxLength", max_len.map(|v| json!(v)));
        put(&mut node, "pattern", pattern.map(Value::String));
        Ok(args::annotate(node, required, positional))
    }

    /// A whole-number argument.
    fn int(
        #[starlark(require = named)] about: Option<String>,
        #[starlark(require = named)] default: Option<i64>,
        #[starlark(require = named, default = true)] required: bool,
        #[starlark(require = named)] positional: Option<u32>,
        #[starlark(require = named)] min: Option<i64>,
        #[starlark(require = named)] max: Option<i64>,
    ) -> starlark::Result<Value> {
        let mut node = json!({ "type": "integer" });
        describe(&mut node, about, default.map(|v| json!(v)));
        put(&mut node, "minimum", min.map(|v| json!(v)));
        put(&mut node, "maximum", max.map(|v| json!(v)));
        Ok(args::annotate(node, required, positional))
    }

    /// A number argument.
    fn float<'v>(
        #[starlark(require = named)] about: Option<String>,
        #[starlark(require = named)] default: Option<StarValue<'v>>,
        #[starlark(require = named, default = true)] required: bool,
        #[starlark(require = named)] positional: Option<u32>,
        #[starlark(require = named)] min: Option<StarValue<'v>>,
        #[starlark(require = named)] max: Option<StarValue<'v>>,
    ) -> starlark::Result<Value> {
        let mut node = json!({ "type": "number" });
        describe(
            &mut node,
            about,
            default.map(as_f64).transpose()?.map(|v| json!(v)),
        );
        put(
            &mut node,
            "minimum",
            min.map(as_f64).transpose()?.map(|v| json!(v)),
        );
        put(
            &mut node,
            "maximum",
            max.map(as_f64).transpose()?.map(|v| json!(v)),
        );
        Ok(args::annotate(node, required, positional))
    }

    /// A flag that is either on or off.
    fn bool(
        #[starlark(require = named)] about: Option<String>,
        #[starlark(require = named)] default: Option<bool>,
        #[starlark(require = named, default = false)] required: bool,
    ) -> starlark::Result<Value> {
        let mut node = json!({ "type": "boolean" });
        describe(&mut node, about, default.map(Value::Bool));
        Ok(args::annotate(node, required, None))
    }

    /// One of a fixed set of strings.
    fn r#enum<'v>(
        #[starlark(require = named)] values: StarValue<'v>,
        #[starlark(require = named)] about: Option<String>,
        #[starlark(require = named)] default: Option<String>,
        #[starlark(require = named, default = true)] required: bool,
        #[starlark(require = named)] positional: Option<u32>,
    ) -> starlark::Result<Value> {
        let values = as_strings(Some(values), "values")?;
        if values.is_empty() {
            return Err(starlark::Error::new_other(anyhow::anyhow!(
                "`meow.arg.enum` needs at least one value"
            )));
        }
        if let Some(default) = &default
            && !values.contains(default)
        {
            return Err(starlark::Error::new_other(anyhow::anyhow!(
                "the default `{default}` is not one of the values"
            )));
        }
        let mut node = json!({ "type": "string", "enum": values });
        describe(&mut node, about, default.map(Value::String));
        Ok(args::annotate(node, required, positional))
    }

    /// A list of one element type.
    fn list<'v>(
        #[starlark(require = named)] element: StarValue<'v>,
        #[starlark(require = named)] about: Option<String>,
        #[starlark(require = named, default = true)] required: bool,
        #[starlark(require = named)] positional: Option<u32>,
    ) -> starlark::Result<Value> {
        let element = schema::for_model(&as_json(element)?);
        let mut node = json!({ "type": "array", "items": element });
        describe(&mut node, about, None);
        Ok(args::annotate(node, required, positional))
    }
}

/// `meow.schema`: JSON Schema for what a model must return.
#[starlark_module]
fn schema_constructors(builder: &mut GlobalsBuilder) {
    /// An object with named fields.
    fn object<'v>(
        #[starlark(require = named)] fields: StarValue<'v>,
        #[starlark(require = named)] required: Option<StarValue<'v>>,
        #[starlark(require = named)] about: Option<String>,
    ) -> starlark::Result<Value> {
        let fields = as_object(Some(fields), "fields")?;
        let fields = fields
            .into_iter()
            .map(|(k, v)| (k, schema::for_model(&v)))
            .collect();
        let required = as_strings(required, "required")?;
        let mut node = schema::object(fields, required).map_err(fail)?;
        describe(&mut node, about, None);
        Ok(node)
    }

    /// A list of one element type.
    fn list<'v>(
        #[starlark(require = named)] element: StarValue<'v>,
        #[starlark(require = named)] about: Option<String>,
    ) -> starlark::Result<Value> {
        let element = schema::for_model(&as_json(element)?);
        let mut node = json!({ "type": "array", "items": element });
        describe(&mut node, about, None);
        Ok(node)
    }

    /// Text.
    fn string(#[starlark(require = named)] about: Option<String>) -> starlark::Result<Value> {
        let mut node = json!({ "type": "string" });
        describe(&mut node, about, None);
        Ok(node)
    }

    /// A whole number.
    fn int(#[starlark(require = named)] about: Option<String>) -> starlark::Result<Value> {
        let mut node = json!({ "type": "integer" });
        describe(&mut node, about, None);
        Ok(node)
    }

    /// A number.
    fn float(#[starlark(require = named)] about: Option<String>) -> starlark::Result<Value> {
        let mut node = json!({ "type": "number" });
        describe(&mut node, about, None);
        Ok(node)
    }

    /// True or false.
    fn bool(#[starlark(require = named)] about: Option<String>) -> starlark::Result<Value> {
        let mut node = json!({ "type": "boolean" });
        describe(&mut node, about, None);
        Ok(node)
    }

    /// One of a fixed set of strings.
    fn r#enum<'v>(
        #[starlark(require = named)] values: StarValue<'v>,
        #[starlark(require = named)] about: Option<String>,
    ) -> starlark::Result<Value> {
        let values = as_strings(Some(values), "values")?;
        let mut node = json!({ "type": "string", "enum": values });
        describe(&mut node, about, None);
        Ok(node)
    }
}

/// `print`, defined only to refuse.
///
/// `[R-STAR-082]`: leaving it undefined would also fail, with "variable
/// `print` not found", which tells a user that they made a typo rather than
/// that printing is the wrong thing to do here. Defining it lets the error
/// name what to use instead.
#[starlark_module]
fn refusals(builder: &mut GlobalsBuilder) {
    /// Refused: writing through the terminal frame corrupts it.
    fn print<'v>(#[starlark(args)] args: UnpackTuple<StarValue<'v>>) -> starlark::Result<NoneType> {
        let _ = args;
        Err(starlark::Error::new_other(anyhow::anyhow!(
            "`print` writes through the terminal frame and corrupts it. Use `ctx.out` instead."
        )))
    }
}

fn describe(node: &mut Value, about: Option<String>, default: Option<Value>) {
    put(node, "description", about.map(Value::String));
    put(node, "default", default);
}

fn put(node: &mut Value, key: &str, value: Option<Value>) {
    if let (Value::Object(fields), Some(value)) = (node, value) {
        fields.insert(key.to_owned(), value);
    }
}

/// Everything a declaration file can see.
///
/// `[R-STAR-010]`: this function is the only place the surface is assembled.
/// A second builder is how v0.2.x ended up with two context shapes.
pub fn globals() -> starlark::environment::Globals {
    starlark::environment::GlobalsBuilder::standard()
        .with(refusals)
        .with_namespace("meow", |builder| {
            declarations(builder);
            builder.namespace("arg", arg_constructors);
            builder.namespace("schema", schema_constructors);
        })
        .build()
}
