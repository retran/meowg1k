// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! One argument declaration, three readers.

use serde_json::{Map, Value, json};

use crate::error::{Result, StarError};
use crate::schema::{self, EXT, Violation};

/// One declared argument.
#[derive(Debug, Clone)]
pub struct Arg {
    /// What it is called, in both the flag and the schema.
    pub name: String,
    /// The JSON Schema node, with its `x-meow` annotations still on it.
    pub node: Value,
}

impl Arg {
    /// Whether it has to be given.
    pub fn required(&self) -> bool {
        schema::ext(&self.node, "required")
            .and_then(Value::as_bool)
            .unwrap_or(false)
    }

    /// Its place on the command line, when it has one.
    pub fn positional(&self) -> Option<u64> {
        schema::ext(&self.node, "positional").and_then(Value::as_u64)
    }

    /// What it is for.
    pub fn about(&self) -> &str {
        self.node
            .get("description")
            .and_then(Value::as_str)
            .unwrap_or_default()
    }

    /// What it is when nothing says otherwise.
    pub fn default(&self) -> Option<&Value> {
        self.node.get("default")
    }

    /// The flag a user types, for an argument that is not positional.
    ///
    /// Kebab-case, because that is what every other flag in the binary is and
    /// a declaration should not be able to introduce a second convention.
    pub fn flag(&self) -> String {
        format!("--{}", self.name.replace('_', "-"))
    }

    /// One line of help, as the command's usage lists it.
    ///
    /// `[R-STAR-061]`: built from the same node as the flag and the schema, so
    /// a constraint that is enforced is also a constraint that is documented.
    pub fn help(&self) -> String {
        let mut line = self.about().to_owned();
        if let Some(Value::Array(values)) = self.node.get("enum") {
            let names: Vec<String> = values
                .iter()
                .map(|v| v.as_str().map_or_else(|| v.to_string(), str::to_owned))
                .collect();
            line.push_str(&format!(" (one of: {})", names.join(", ")));
        }
        match self.default() {
            Some(default) if !self.required() => {
                let shown = default
                    .as_str()
                    .map_or_else(|| default.to_string(), str::to_owned);
                line.push_str(&format!(" [default: {shown}]"));
            }
            _ if self.required() => line.push_str(" (required)"),
            _ => {}
        }
        line
    }
}

/// Every argument one tool or command takes.
#[derive(Debug, Clone, Default)]
pub struct Args {
    args: Vec<Arg>,
}

impl Args {
    /// Build an argument set from what a declaration passed.
    ///
    /// Checks `[R-STAR-062]`: positional indices are unique and contiguous
    /// from zero. Contiguous because a gap has no meaning a user could act on,
    /// and leaving it to the parser to discover means the error arrives when
    /// somebody runs the command rather than when it is declared.
    ///
    /// # Errors
    ///
    /// [`StarError::Load`] naming the index that repeats or the one missing.
    pub fn new(fields: Map<String, Value>) -> Result<Self> {
        let args: Vec<Arg> = fields
            .into_iter()
            .map(|(name, node)| Arg { name, node })
            .collect();

        let mut seen: Vec<(u64, &str)> = args
            .iter()
            .filter_map(|a| a.positional().map(|i| (i, a.name.as_str())))
            .collect();
        seen.sort_unstable();

        for (at, window) in seen.windows(2).enumerate() {
            let [(a, first), (b, second)] = window else {
                continue;
            };
            if a == b {
                return Err(StarError::Load {
                    message: format!("`{first}` and `{second}` are both positional {a}"),
                });
            }
            let _ = at;
        }
        for (expected, (index, name)) in seen.iter().enumerate() {
            if *index != expected as u64 {
                return Err(StarError::Load {
                    message: format!(
                        "positional arguments must be numbered from zero without gaps: `{name}` is {index}, and {expected} is missing"
                    ),
                });
            }
        }

        Ok(Self { args })
    }

    /// Every argument, in declaration order.
    pub fn iter(&self) -> impl Iterator<Item = &Arg> {
        self.args.iter()
    }

    /// The positional arguments, in the order they are typed.
    pub fn positional(&self) -> Vec<&Arg> {
        let mut out: Vec<&Arg> = self
            .args
            .iter()
            .filter(|a| a.positional().is_some())
            .collect();
        out.sort_by_key(|a| a.positional().unwrap_or_default());
        out
    }

    /// The arguments given as flags.
    pub fn flags(&self) -> impl Iterator<Item = &Arg> {
        self.args.iter().filter(|a| a.positional().is_none())
    }

    /// The schema the model is sent.
    ///
    /// `[R-STAR-061]`: derived, never written twice. A positional argument is
    /// an ordinary property here, because the model has no command line.
    pub fn json_schema(&self) -> Value {
        let mut properties = Map::new();
        let mut required = Vec::new();
        for arg in &self.args {
            properties.insert(arg.name.clone(), schema::for_model(&arg.node));
            if arg.required() {
                required.push(Value::String(arg.name.clone()));
            }
        }
        json!({
            "type": "object",
            "properties": Value::Object(properties),
            "required": required,
            "additionalProperties": false,
        })
    }

    /// Fill in defaults and check every constraint.
    ///
    /// Satisfies `[R-STAR-063]`: the command line and a model-supplied
    /// argument object both arrive here, so neither can be checked more
    /// loosely than the other.
    ///
    /// # Errors
    ///
    /// Every violation at once. Reporting the first would make fixing a
    /// three-argument mistake take three runs.
    pub fn bind(
        &self,
        given: &Map<String, Value>,
    ) -> std::result::Result<Map<String, Value>, Vec<Violation>> {
        let mut out = Map::new();
        let mut problems = Vec::new();

        for arg in &self.args {
            match given.get(&arg.name).or_else(|| arg.default()) {
                Some(value) => {
                    problems.extend(schema::check(&arg.name, &arg.node, value));
                    out.insert(arg.name.clone(), value.clone());
                }
                None if arg.required() => problems.push(Violation {
                    field: arg.name.clone(),
                    reason: "is required and was not given".to_owned(),
                }),
                None => {}
            }
        }

        for name in given.keys() {
            if !self.args.iter().any(|a| &a.name == name) {
                problems.push(Violation {
                    field: name.clone(),
                    reason: format!(
                        "is not an argument{}",
                        crate::error::closest(name, self.args.iter().map(|a| a.name.as_str()))
                            .map(|c| format!(". Did you mean `{c}`?"))
                            .unwrap_or_default()
                    ),
                });
            }
        }

        if problems.is_empty() {
            Ok(out)
        } else {
            Err(problems)
        }
    }
}

/// Attach the command-line annotations to a schema node.
pub(crate) fn annotate(mut node: Value, required: bool, positional: Option<u32>) -> Value {
    let mut ext = Map::new();
    ext.insert("required".to_owned(), Value::Bool(required));
    if let Some(index) = positional {
        ext.insert("positional".to_owned(), json!(index));
    }
    if let Value::Object(fields) = &mut node {
        fields.insert(EXT.to_owned(), Value::Object(ext));
    }
    node
}
