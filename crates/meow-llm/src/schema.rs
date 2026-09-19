// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Making an answer satisfy a schema.

use crate::error::{LlmError, Result};

/// How many times a provider asks again before giving up.
pub const SCHEMA_ATTEMPTS: u32 = 3;

/// Check a JSON value against a schema, returning what is wrong with it.
///
/// Enough of JSON Schema for what `meow.schema` can build: types, required
/// fields, enumerations, and the numeric and string bounds. A full validator
/// is a dependency this does not need yet, and the schemas it has to check are
/// the ones this workspace emits.
pub fn validate(
    value: &serde_json::Value,
    schema: &serde_json::Value,
) -> std::result::Result<(), String> {
    let Some(kind) = schema.get("type").and_then(serde_json::Value::as_str) else {
        return Ok(());
    };
    match kind {
        "object" => {
            let Some(obj) = value.as_object() else {
                return Err(format!("expected an object, got {}", kind_of(value)));
            };
            if let Some(required) = schema.get("required").and_then(serde_json::Value::as_array) {
                for field in required.iter().filter_map(serde_json::Value::as_str) {
                    if !obj.contains_key(field) {
                        return Err(format!("missing required field `{field}`"));
                    }
                }
            }
            if let Some(props) = schema
                .get("properties")
                .and_then(serde_json::Value::as_object)
            {
                for (name, sub) in props {
                    if let Some(v) = obj.get(name) {
                        validate(v, sub).map_err(|e| format!("`{name}`: {e}"))?;
                    }
                }
            }
            Ok(())
        }
        "array" => {
            let Some(items) = value.as_array() else {
                return Err(format!("expected an array, got {}", kind_of(value)));
            };
            if let Some(sub) = schema.get("items") {
                for (i, v) in items.iter().enumerate() {
                    validate(v, sub).map_err(|e| format!("[{i}]: {e}"))?;
                }
            }
            Ok(())
        }
        "string" => {
            let Some(s) = value.as_str() else {
                return Err(format!("expected a string, got {}", kind_of(value)));
            };
            if let Some(list) = schema.get("enum").and_then(serde_json::Value::as_array) {
                let allowed: Vec<&str> =
                    list.iter().filter_map(serde_json::Value::as_str).collect();
                if !allowed.contains(&s) {
                    return Err(format!("`{s}` is not one of {}", allowed.join(", ")));
                }
            }
            if let Some(max) = schema.get("maxLength").and_then(serde_json::Value::as_u64)
                && s.chars().count() as u64 > max
            {
                return Err(format!("longer than {max} characters"));
            }
            Ok(())
        }
        "integer" | "number" => {
            let Some(n) = value.as_f64() else {
                return Err(format!("expected a number, got {}", kind_of(value)));
            };
            if kind == "integer" && value.as_i64().is_none() {
                return Err("expected an integer".to_owned());
            }
            if let Some(min) = schema.get("minimum").and_then(serde_json::Value::as_f64)
                && n < min
            {
                return Err(format!("{n} is below the minimum of {min}"));
            }
            if let Some(max) = schema.get("maximum").and_then(serde_json::Value::as_f64)
                && n > max
            {
                return Err(format!("{n} is above the maximum of {max}"));
            }
            Ok(())
        }
        "boolean" => value
            .as_bool()
            .map(|_| ())
            .ok_or_else(|| format!("expected a boolean, got {}", kind_of(value))),
        _ => Ok(()),
    }
}

fn kind_of(v: &serde_json::Value) -> &'static str {
    match v {
        serde_json::Value::Null => "null",
        serde_json::Value::Bool(_) => "a boolean",
        serde_json::Value::Number(_) => "a number",
        serde_json::Value::String(_) => "a string",
        serde_json::Value::Array(_) => "an array",
        serde_json::Value::Object(_) => "an object",
    }
}

/// Parse an answer and check it against a schema.
///
/// # Errors
///
/// [`LlmError::Schema`] describing what is wrong, which `[R-LLM-051]` puts
/// into the follow-up request so the model is told rather than guessing.
pub fn parse_and_validate(
    text: &str,
    schema: &serde_json::Value,
    attempts: u32,
) -> Result<serde_json::Value> {
    let value: serde_json::Value =
        serde_json::from_str(text.trim()).map_err(|e| LlmError::Schema {
            attempts,
            message: format!("not JSON: {e}"),
        })?;
    validate(&value, schema).map_err(|message| LlmError::Schema { attempts, message })?;
    Ok(value)
}
