// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Agents written as markdown.
//!
//! A `.md` file under `.meow/agents/` is an agent whose body is its system
//! prompt and whose frontmatter is everything else. In v0.2.x every agent was
//! a Starlark file, and the shipped `lib/agent.star` existed only to hide the
//! boilerplate that made one work; most agents are a prompt and four settings,
//! and this is what a prompt and four settings should look like.

use std::path::Path;

use crate::agent::{AgentDecl, Fields, Source};
use crate::error::{Result, StarError};
use crate::registry::Origin;

/// The fence a frontmatter block sits between.
const FENCE: &str = "---";

/// A file split into its frontmatter and its body.
#[derive(Debug)]
struct Split<'a> {
    yaml: &'a str,
    /// Which line of the file the frontmatter starts on, counting from one.
    yaml_starts_at: usize,
    body: &'a str,
}

/// Split a markdown file at its frontmatter fence.
///
/// A file with no frontmatter is not an error here; `Fields::build` refuses it
/// for the reason that actually applies, which is that it declares no model.
fn split(text: &str) -> Split<'_> {
    let without_bom = text.strip_prefix('\u{feff}').unwrap_or(text);
    let Some(rest) = without_bom.strip_prefix(FENCE) else {
        return Split {
            yaml: "",
            yaml_starts_at: 1,
            body: text,
        };
    };
    let Some(rest) = rest
        .strip_prefix('\n')
        .or_else(|| rest.strip_prefix("\r\n"))
    else {
        return Split {
            yaml: "",
            yaml_starts_at: 1,
            body: text,
        };
    };

    // The closing fence is a line of its own, so searching for "\n---" and
    // checking what follows avoids stopping at a `---` inside a value.
    let mut offset = 0;
    for line in rest.split_inclusive('\n') {
        if line.trim_end() == FENCE {
            return Split {
                yaml: &rest[..offset],
                yaml_starts_at: 2,
                body: &rest[offset + line.len()..],
            };
        }
        offset += line.len();
    }

    Split {
        yaml: "",
        yaml_starts_at: 1,
        body: text,
    }
}

/// Read a markdown agent.
///
/// Satisfies `[R-STAR-050]` through [`Fields`], which is the same structure a
/// `meow.agent` call fills, and `[R-STAR-051]` because the value comes out of
/// the same builder.
///
/// # Errors
///
/// [`StarError::Load`] carrying the file and the line, per `[R-STAR-052]`.
pub fn agent(
    path: &Path,
    name: &str,
    text: &str,
    prompts: impl Fn(&str) -> Result<String>,
) -> Result<AgentDecl> {
    let parts = split(text);

    let fields: Fields = serde_yaml_ng::from_str(parts.yaml).map_err(|e| {
        // `[R-STAR-052]`: every line number the parser produces is relative to
        // the frontmatter, and a user counts from the top of the file. Shifting
        // only the one in the prefix would leave the message contradicting
        // itself, so the ones inside it are shifted too.
        let offset = parts.yaml_starts_at - 1;
        let at = e.location().map_or_else(
            || "frontmatter".to_owned(),
            |l| format!("{}:{}", l.line() + offset, l.column()),
        );
        StarError::Load {
            message: format!(
                "{}:{at}: {}",
                path.display(),
                shift_lines(&e.to_string(), offset)
            ),
        }
    })?;

    let origin = Origin(path.display().to_string());
    let mut declared = fields.build(name, Source::Markdown, &origin, prompts)?;

    // `build` joined the includes and an empty system prompt; the body goes on
    // the end, separated the same way `[R-STAR-054]` separates the includes.
    let body = parts.body.trim();
    if body.is_empty() {
        return Err(StarError::Load {
            message: format!(
                "{}: the body is the system prompt, and it is empty",
                path.display()
            ),
        });
    }
    declared.system = if declared.system.is_empty() {
        body.to_owned()
    } else {
        format!("{}\n\n{body}", declared.system)
    };

    Ok(declared)
}

/// Move every "at line N" in a parser message down by the frontmatter offset.
///
/// The parser counts from the start of the block it was given, and there is no
/// way to tell it otherwise, so the numbers are corrected on the way out.
fn shift_lines(message: &str, offset: usize) -> String {
    const MARKER: &str = "at line ";
    let mut out = String::with_capacity(message.len());
    let mut rest = message;

    while let Some(at) = rest.find(MARKER) {
        let (before, after) = rest.split_at(at + MARKER.len());
        out.push_str(before);
        let digits: String = after.chars().take_while(char::is_ascii_digit).collect();
        match digits.parse::<usize>() {
            Ok(line) => {
                out.push_str(&(line + offset).to_string());
                rest = &after[digits.len()..];
            }
            Err(_) => rest = after,
        }
    }
    out.push_str(rest);
    out
}

/// The name an agent file declares.
///
/// The filename, so a file and the agent it holds cannot drift apart and
/// nobody has to write the name twice.
pub fn name_of(path: &Path) -> Option<String> {
    Some(path.file_stem()?.to_str()?.to_owned())
}
