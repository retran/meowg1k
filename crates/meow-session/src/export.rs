// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Turning a session into something to read or to attach to a pull request.

use meow_core::view::ViewEvent;
use meow_core::{EventKind, SessionId};

use crate::error::Result;
use crate::log::Sessions;

/// What must not appear in an export.
///
/// `[R-SESSION-092]`: every value the policy marked sensitive, in both
/// formats. The caller supplies the arguments to hide, because which ones they
/// are is a property of the policy and the tool set, and this crate knows
/// neither.
#[derive(Debug, Clone, Default)]
pub struct Redaction {
    /// Argument names whose values are replaced, whatever tool they belong to.
    pub arguments: Vec<String>,
    /// Whether the model's reasoning is included.
    ///
    /// `[R-SESSION-093]`: off unless asked for, because it is the part of a
    /// transcript least likely to be meant for an audience.
    pub include_thinking: bool,
}

/// What an export produced.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Export {
    /// The text to write out.
    pub text: String,
}

impl Sessions {
    /// Export a session as one JSON object per line.
    ///
    /// Satisfies `[R-SESSION-090]`: the objects are `meow_core::view` types,
    /// the same ones the live `--format json` renderer writes, so a persisted
    /// kind serialises identically in both by construction rather than by
    /// agreement. `[R-TUI-034]` holds here too: only persisted kinds appear,
    /// because only persisted kinds exist in a log.
    ///
    /// # Errors
    ///
    /// Whatever reading the log said.
    pub fn export_json(&self, id: &SessionId, redaction: &Redaction) -> Result<Export> {
        let mut text = String::new();
        text.push_str(&schema_line());

        for event in self.events(id)? {
            let Some(kind) = prepare(event.kind, redaction) else {
                continue;
            };
            let line = serde_json::to_string(&ViewEvent::Logged(kind)).map_err(|source| {
                crate::error::SessionError::Decode {
                    seq: event.seq,
                    source,
                }
            })?;
            text.push_str(&line);
            text.push('\n');
        }

        Ok(Export { text })
    }

    /// Export a session as markdown.
    ///
    /// Satisfies `[R-SESSION-091]`: the transcript, the tool calls with their
    /// policy decisions, and the usage totals.
    ///
    /// # Errors
    ///
    /// Whatever reading the log said.
    pub fn export_markdown(&self, id: &SessionId, redaction: &Redaction) -> Result<Export> {
        let session = self.session(id)?;
        let usage = self.usage(id)?;
        let state = self.state(id)?;

        let mut text = String::new();
        text.push_str(&format!("# {} · {}\n\n", session.agent, id.short()));
        text.push_str(&format!("- session `{id}`\n"));
        if let Some(name) = &session.name {
            text.push_str(&format!("- name `{name}`\n"));
        }
        if let Some(origin) = self.origin(id)? {
            text.push_str(&format!(
                "- forked from `{}` at {}\n",
                origin.session, origin.seq
            ));
        }
        text.push_str(&format!("- state {state}\n"));
        text.push_str(&format!(
            "- {} prompt + {} completion tokens{}\n\n",
            usage.prompt,
            usage.completion,
            match usage.cost_micros {
                Some(micros) => format!(", ${:.2}", micros as f64 / 1_000_000.0),
                None => ", cost unknown".to_owned(),
            }
        ));

        // The decision arrives before the call it decided about, and a reader
        // wants them together.
        let mut decisions: std::collections::HashMap<String, String> =
            std::collections::HashMap::new();
        let mut runs = 0_u32;

        for event in self.events(id)? {
            let Some(kind) = prepare(event.kind, redaction) else {
                continue;
            };
            match kind {
                // A session holds one or more runs, so `Started` appears once
                // per run. The first is the task; a later one is somebody
                // coming back to it. A tool takes arguments rather than a
                // task, so an empty one is normal and says nothing.
                EventKind::Started { task, .. } => {
                    runs += 1;
                    if task.trim().is_empty() {
                        continue;
                    }
                    if runs == 1 {
                        text.push_str(&format!("## Task\n\n{task}\n\n"));
                    } else {
                        text.push_str(&format!("## Continued\n\n{task}\n\n"));
                    }
                }
                EventKind::UserMessage { content } => {
                    text.push_str(&format!("## Said\n\n{content}\n\n"));
                }
                EventKind::Assistant { content, .. } if !content.trim().is_empty() => {
                    text.push_str(&format!("{content}\n\n"));
                }
                EventKind::Policy {
                    id, decision, rule, ..
                } => {
                    let described = match rule {
                        Some(rule) => format!("{decision} ({rule})"),
                        None => decision,
                    };
                    decisions.insert(id, described);
                }
                EventKind::ToolCall { id, name, args } => {
                    let decision = decisions
                        .remove(&id)
                        .map_or_else(String::new, |d| format!(" — {d}"));
                    text.push_str(&format!("- `{name}` `{args}`{decision}\n"));
                }
                EventKind::ToolResult {
                    error: Some(error), ..
                } => {
                    text.push_str(&format!("  - failed: {error}\n"));
                }
                EventKind::Compaction {
                    supersedes,
                    tokens_saved,
                    ..
                } => {
                    text.push_str(&format!(
                        "\n> events {} to {} were summarised, saving {tokens_saved} tokens\n\n",
                        supersedes.start(),
                        supersedes.end()
                    ));
                }
                EventKind::Note { level, message } => {
                    text.push_str(&format!("> {level}: {message}\n\n"));
                }
                EventKind::Finished { stop, detail } => {
                    let detail = detail.map_or_else(String::new, |d| format!(" ({d})"));
                    text.push_str(&format!("\n**{stop}{detail}**\n"));
                }
                EventKind::Assistant { .. }
                | EventKind::ToolResult { .. }
                | EventKind::Usage(_) => {}
            }
        }

        Ok(Export { text })
    }
}

/// The schema line every JSON stream begins with.
fn schema_line() -> String {
    format!(
        "{{\"type\":\"Schema\",\"version\":{}}}\n",
        meow_core::view::SCHEMA_VERSION
    )
}

/// Apply the redaction to one event, or drop it.
fn prepare(kind: EventKind, redaction: &Redaction) -> Option<EventKind> {
    match kind {
        // `[R-SESSION-092]`: the arguments are the only place a marked value
        // reaches the log, because the policy layer redacts before the prompt
        // and the engine records what it was told.
        EventKind::ToolCall { id, name, args } => Some(EventKind::ToolCall {
            id,
            name,
            args: redact(&args, &redaction.arguments),
        }),
        other => Some(other),
    }
    .filter(|kind| redaction.include_thinking || !is_thinking(kind))
}

/// Whether an event carries the model's reasoning.
///
/// Reasoning is not a kind of its own in a log: it arrives as a note the
/// engine writes when a provider returns it, which is why this looks at the
/// level rather than at the variant.
fn is_thinking(kind: &EventKind) -> bool {
    matches!(kind, EventKind::Note { level, .. } if level == "thinking")
}

/// Replace the values of named arguments in a JSON object.
fn redact(args: &str, arguments: &[String]) -> String {
    if arguments.is_empty() {
        return args.to_owned();
    }
    let Ok(serde_json::Value::Object(mut map)) = serde_json::from_str(args) else {
        return args.to_owned();
    };
    for name in arguments {
        if map.contains_key(name) {
            map.insert(
                name.clone(),
                serde_json::Value::String(meow_policy::REDACTED.to_owned()),
            );
        }
    }
    serde_json::Value::Object(map).to_string()
}
