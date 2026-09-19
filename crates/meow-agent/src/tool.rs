// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What a tool is, and how its arguments are checked before it runs.

use serde_json::Value;
use tokio_util::sync::CancellationToken;

/// A tool failed.
#[derive(Debug, thiserror::Error)]
#[error("{0}")]
pub struct ToolError(pub String);

/// Something the model can ask to run.
#[async_trait::async_trait]
pub trait Tool: Send + Sync {
    /// What the model calls it.
    fn name(&self) -> &str;

    /// What it does, in the model's terms.
    fn description(&self) -> &str;

    /// Its arguments, as JSON Schema.
    fn schema(&self) -> Value;

    /// Run it.
    ///
    /// # Errors
    ///
    /// [`ToolError`] with whatever the model should be told.
    async fn call(&self, args: &Value, cancel: &CancellationToken) -> Result<String, ToolError>;
}

/// The tools one agent was given.
#[derive(Default)]
pub struct ToolSet(Vec<std::sync::Arc<dyn Tool>>);

impl std::fmt::Debug for ToolSet {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_list()
            .entries(self.0.iter().map(|t| t.name()))
            .finish()
    }
}

impl ToolSet {
    /// An empty set.
    pub fn new() -> Self {
        Self::default()
    }

    /// Add one.
    #[must_use]
    pub fn with(mut self, tool: impl Tool + 'static) -> Self {
        self.0.push(std::sync::Arc::new(tool));
        self
    }

    /// Find one by name.
    ///
    /// Only within this set. `[R-AGENT-023]` forbids reaching into a wider
    /// registry: a tool the agent was not given must stay unreachable, however
    /// convincingly the model asks for it.
    pub fn get(&self, name: &str) -> Option<&dyn Tool> {
        self.0
            .iter()
            .find(|t| t.name() == name)
            .map(std::convert::AsRef::as_ref)
    }

    /// Every tool, as the model should be told about them.
    pub fn definitions(&self) -> Vec<meow_llm::ToolDefinition> {
        self.0
            .iter()
            .map(|t| meow_llm::ToolDefinition {
                name: t.name().to_owned(),
                description: t.description().to_owned(),
                parameters: t.schema(),
            })
            .collect()
    }

    /// How many.
    pub fn len(&self) -> usize {
        self.0.len()
    }

    /// Whether the agent was given none.
    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }
}

/// What checking a call's arguments produced.
#[derive(Debug, PartialEq)]
pub enum Checked {
    /// Run it with these.
    Ready(Value),
    /// Do not run it; tell the model this.
    Correction(String),
}

/// Check the model's arguments against the tool's schema.
///
/// Satisfies `[R-AGENT-021]`: a required argument the model omitted is never
/// replaced with a default or a zero. v0.2.x filled it with the zero value for
/// its type, so a model that forgot a required integer received `0` and
/// returned a confident wrong answer. The correction names the argument and
/// its type, because a model told what is missing fixes it on the next turn
/// and a model told nothing repeats itself.
///
/// Satisfies `[R-AGENT-022]`: an optional argument the model omitted takes its
/// declared default, and one with no default stays absent rather than zeroed,
/// so absent is distinguishable from zero, empty, and false all the way
/// through.
pub fn check(args: &Value, schema: &Value) -> Checked {
    let mut obj = args.as_object().cloned().unwrap_or_default();

    let required: Vec<&str> = schema
        .get("required")
        .and_then(Value::as_array)
        .map(|a| a.iter().filter_map(Value::as_str).collect())
        .unwrap_or_default();

    let props = schema.get("properties").and_then(Value::as_object);

    for name in &required {
        if !obj.contains_key(*name) {
            let kind = props
                .and_then(|p| p.get(*name))
                .and_then(|s| s.get("type"))
                .and_then(Value::as_str)
                .unwrap_or("value");
            return Checked::Correction(format!(
                "missing required argument `{name}` ({kind}). Call the tool again with it."
            ));
        }
    }

    if let Some(props) = props {
        for (name, sub) in props {
            if obj.contains_key(name) {
                if let Err(why) = meow_llm::validate(&obj[name], sub) {
                    return Checked::Correction(format!("argument `{name}` is wrong: {why}"));
                }
                continue;
            }
            // Absent and optional: take the declared default, or stay absent.
            // Never a zero value, which is the whole of R-AGENT-022.
            if let Some(default) = sub.get("default") {
                obj.insert(name.clone(), default.clone());
            }
        }
    }

    Checked::Ready(Value::Object(obj))
}
