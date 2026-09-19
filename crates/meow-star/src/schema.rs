// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Building JSON Schema, and checking values against it.
//!
//! One representation serves three readers: the command line, the help text,
//! and the model. `[R-STAR-061]` asks for exactly that, because v0.2.x
//! declared a flag in one place and the model-facing schema in another, and
//! the two drifted.
//!
//! A node is ordinary JSON Schema plus one extension object, `x-meow`, holding
//! what only the command line cares about. Keeping the extras under one key
//! means emitting a schema for the model is a removal rather than a
//! translation.

use serde_json::{Map, Value, json};

use crate::error::{Result, StarError, closest};

/// The key every meow-only annotation lives under.
pub const EXT: &str = "x-meow";

/// Strip the command-line annotations, leaving plain JSON Schema.
///
/// `[R-STAR-061]`: the model sees the same declaration the flag came from,
/// minus the parts that mean nothing to it.
pub fn for_model(node: &Value) -> Value {
    match node {
        Value::Object(fields) => {
            let mut out = Map::new();
            for (key, value) in fields {
                if key == EXT {
                    continue;
                }
                out.insert(key.clone(), for_model(value));
            }
            Value::Object(out)
        }
        Value::Array(items) => Value::Array(items.iter().map(for_model).collect()),
        other => other.clone(),
    }
}

/// Read one `x-meow` annotation off a node.
pub fn ext<'a>(node: &'a Value, key: &str) -> Option<&'a Value> {
    node.get(EXT)?.get(key)
}

/// Build an object schema, checking that `required` names fields it declares.
///
/// Satisfies `[R-STAR-070]` and `[R-STAR-071]`. Checking at build time rather
/// than at validation time is the whole point: a schema that requires a field
/// it never declared rejects every value, and finding that out from a model
/// that cannot satisfy it is an expensive way to learn about a typo.
///
/// # Errors
///
/// [`StarError::Unknown`] naming the field, and the closest declared one when
/// there is a near miss.
pub fn object(fields: Map<String, Value>, required: Vec<String>) -> Result<Value> {
    for name in &required {
        if !fields.contains_key(name) {
            return Err(StarError::Unknown {
                kind: "field",
                name: name.clone(),
                closest: closest(name, fields.keys().map(String::as_str)),
            });
        }
    }
    Ok(json!({
        "type": "object",
        "properties": Value::Object(fields),
        "required": required,
        "additionalProperties": false,
    }))
}

/// What a value failed to satisfy.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Violation {
    /// Which argument, by name.
    pub field: String,
    /// What is wrong with it, in the user's terms.
    pub reason: String,
}

impl std::fmt::Display for Violation {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "`{}`: {}", self.field, self.reason)
    }
}

/// Check one value against one schema node.
///
/// Satisfies `[R-STAR-063]`: the command line and the model reach the same
/// function, so a constraint cannot hold on one path and not the other. This
/// is not a general JSON Schema validator; it covers what `meow.arg` can
/// build, which is the set `[R-STAR-060]` names.
pub fn check(field: &str, node: &Value, value: &Value) -> Vec<Violation> {
    let mut out = Vec::new();
    check_into(field, node, value, &mut out);
    out
}

fn check_into(field: &str, node: &Value, value: &Value, out: &mut Vec<Violation>) {
    let Some(kind) = node.get("type").and_then(Value::as_str) else {
        return;
    };

    // `enum` carries its own type, so the membership test comes first and the
    // type test below still applies to whatever the values are.
    if let Some(Value::Array(allowed)) = node.get("enum")
        && !allowed.contains(value)
    {
        out.push(Violation {
            field: field.to_owned(),
            reason: format!("must be one of {}", list(allowed)),
        });
        return;
    }

    match (kind, value) {
        ("string", Value::String(s)) => {
            if let Some(max) = node.get("maxLength").and_then(Value::as_u64)
                && s.chars().count() as u64 > max
            {
                out.push(Violation {
                    field: field.to_owned(),
                    reason: format!(
                        "must be at most {max} characters, and is {}",
                        s.chars().count()
                    ),
                });
            }
            if let Some(pattern) = node.get("pattern").and_then(Value::as_str)
                && !matches_pattern(pattern, s)
            {
                out.push(Violation {
                    field: field.to_owned(),
                    reason: format!("must match `{pattern}`"),
                });
            }
        }
        ("integer", Value::Number(n)) if n.is_i64() || n.is_u64() => {
            bounds(field, node, n.as_f64().unwrap_or_default(), out);
        }
        ("number", Value::Number(n)) => {
            bounds(field, node, n.as_f64().unwrap_or_default(), out);
        }
        ("boolean", Value::Bool(_)) => {}
        ("array", Value::Array(items)) => {
            if let Some(element) = node.get("items") {
                for (i, item) in items.iter().enumerate() {
                    check_into(&format!("{field}[{i}]"), element, item, out);
                }
            }
        }
        ("object", Value::Object(given)) => {
            let properties = node.get("properties").and_then(Value::as_object);
            if let Some(required) = node.get("required").and_then(Value::as_array) {
                for name in required.iter().filter_map(Value::as_str) {
                    if !given.contains_key(name) {
                        out.push(Violation {
                            field: name.to_owned(),
                            reason: "is required and was not given".to_owned(),
                        });
                    }
                }
            }
            if let Some(properties) = properties {
                for (name, item) in given {
                    match properties.get(name) {
                        Some(sub) => check_into(name, sub, item, out),
                        None if node.get("additionalProperties") == Some(&Value::Bool(false)) => {
                            out.push(Violation {
                                field: name.clone(),
                                reason: format!(
                                    "is not an argument{}",
                                    closest(name, properties.keys().map(String::as_str))
                                        .map(|c| format!(". Did you mean `{c}`?"))
                                        .unwrap_or_default()
                                ),
                            });
                        }
                        None => {}
                    }
                }
            }
        }
        _ => out.push(Violation {
            field: field.to_owned(),
            reason: format!("must be {}, and is {}", article(kind), found(value)),
        }),
    }
}

fn bounds(field: &str, node: &Value, n: f64, out: &mut Vec<Violation>) {
    if let Some(min) = node.get("minimum").and_then(Value::as_f64)
        && n < min
    {
        out.push(Violation {
            field: field.to_owned(),
            reason: format!("must be at least {min}, and is {n}"),
        });
    }
    if let Some(max) = node.get("maximum").and_then(Value::as_f64)
        && n > max
    {
        out.push(Violation {
            field: field.to_owned(),
            reason: format!("must be at most {max}, and is {n}"),
        });
    }
}

/// A deliberately small pattern language.
///
/// A full regular expression engine would be another dependency and another
/// way for a declaration file to run for a long time on a crafted input. What
/// a tool argument needs is an anchored shape, so `*` matches any run of
/// characters and everything else is literal.
fn matches_pattern(pattern: &str, text: &str) -> bool {
    let parts: Vec<&str> = pattern.split('*').collect();
    let Some((first, rest)) = parts.split_first() else {
        return pattern == text;
    };
    let Some(mut cursor) = text.strip_prefix(first) else {
        return false;
    };
    let Some((last, middle)) = rest.split_last() else {
        return cursor.is_empty();
    };
    for part in middle {
        match cursor.find(part) {
            Some(at) => cursor = &cursor[at + part.len()..],
            None => return false,
        }
    }
    cursor.len() >= last.len() && cursor.ends_with(last)
}

fn list(values: &[Value]) -> String {
    values
        .iter()
        .map(|v| match v {
            Value::String(s) => format!("`{s}`"),
            other => format!("`{other}`"),
        })
        .collect::<Vec<_>>()
        .join(", ")
}

fn article(kind: &str) -> &'static str {
    match kind {
        "string" => "a string",
        "integer" => "an integer",
        "number" => "a number",
        "boolean" => "a boolean",
        "array" => "a list",
        "object" => "an object",
        _ => "another type",
    }
}

fn found(value: &Value) -> &'static str {
    match value {
        Value::Null => "nothing",
        Value::Bool(_) => "a boolean",
        Value::Number(n) if n.is_f64() => "a number",
        Value::Number(_) => "an integer",
        Value::String(_) => "a string",
        Value::Array(_) => "a list",
        Value::Object(_) => "an object",
    }
}
