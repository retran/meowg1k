// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Agent declarations, in Starlark and in markdown.
#![allow(clippy::unwrap_used)]

use std::path::Path;
use std::time::Duration;

use meow_agent::ToolErrorPolicy;
use meow_star::{AgentDecl, StarError, Workspace, load};
use serde_json::json;
use tempfile::TempDir;

fn workspace(files: &[(&str, &str)]) -> TempDir {
    let dir = tempfile::tempdir().unwrap();
    for (name, body) in files {
        let path = dir.path().join(".meow").join(name);
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, body).unwrap();
    }
    dir
}

fn load_at(dir: &TempDir) -> Result<meow_star::Loaded, StarError> {
    load(&Workspace::at(dir.path()))
}

fn agent(dir: &TempDir, name: &str) -> AgentDecl {
    load_at(dir).unwrap().registry.agent(name).unwrap().clone()
}

const MODELS: &str = r#"
meow.provider(name = "anthropic", kind = "anthropic")
meow.model(name = "fast", provider = "anthropic", id = "x", context = 200000, max_output = 8192)
meow.model(name = "cheap", provider = "anthropic", id = "y", context = 200000, max_output = 8192)
"#;

/// [R-STAR-040] meow.agent requires name, model, and system, and accepts the
/// rest
#[test]
fn an_agent_accepts_every_declared_keyword() {
    let dir = workspace(&[(
        "meow.star",
        &format!(
            r#"{MODELS}
def handler(ctx):
    return ""

meow.tool(name = "grep", about = "search", run = handler)

meow.agent(
    name = "reviewer",
    model = "fast",
    system = "You review code.",
    about = "reviews a diff",
    tools = ["grep"],
    budget = {{"tokens": 50000, "steps": 12}},
    compaction = {{"at": 0.6, "keep_recent": 4, "model": "cheap"}},
    output = meow.schema.object(fields = {{"verdict": meow.schema.string()}}, required = ["verdict"]),
    on_tool_error = "abort",
    policy = [{{"tools": ["fs.*"], "decision": "allow", "paths": ["src/**"]}}],
)
"#
        ),
    )]);

    let reviewer = agent(&dir, "reviewer");

    assert_eq!(reviewer.model, "fast");
    assert_eq!(reviewer.system, "You review code.");
    assert_eq!(reviewer.about, "reviews a diff");
    assert_eq!(reviewer.tools, ["grep"]);
    assert_eq!(reviewer.budget.tokens, Some(50_000));
    assert_eq!(reviewer.budget.steps, Some(12));
    assert_eq!(reviewer.on_tool_error, ToolErrorPolicy::Abort);
    assert_eq!(reviewer.compaction.keep_recent, 4);
    assert_eq!(reviewer.compaction.model.as_deref(), Some("cheap"));
    assert_eq!(reviewer.output.unwrap()["required"], json!(["verdict"]));
    assert!(reviewer.policy.is_some());

    // An axis the declaration left out keeps the engine's default rather than
    // becoming unbounded.
    assert_eq!(reviewer.budget.duration, Some(Duration::from_secs(30 * 60)));
}

/// [R-STAR-040] an unknown value for a declared keyword fails where it is
/// written
#[test]
fn a_bad_on_tool_error_names_what_is_allowed() {
    let dir = workspace(&[(
        "meow.star",
        &format!(
            r#"{MODELS}
meow.agent(name = "a", model = "fast", system = "s", on_tool_error = "explode")
"#
        ),
    )]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("`report` or `abort`"), "{error}");
    assert!(error.contains("explode"), "{error}");
}

/// [R-STAR-050] a .md file under .meow/agents/ declares an agent, named by its
/// file
#[test]
fn a_markdown_file_declares_an_agent() {
    let dir = workspace(&[
        ("meow.star", MODELS),
        (
            "agents/reviewer.md",
            r#"---
model: fast
about: reviews a diff
tools: [grep]
budget:
  tokens: 50000
  steps: 12
on_tool_error: abort
---
You review code. Be blunt.
"#,
        ),
        ("meow2.star", ""),
    ]);

    // The tool the frontmatter names has to exist for the reference to resolve.
    std::fs::write(
        dir.path().join(".meow").join("meow.star"),
        format!("{MODELS}\ndef handler(ctx):\n    return \"\"\n\nmeow.tool(name = \"grep\", about = \"search\", run = handler)\n"),
    )
    .unwrap();

    let reviewer = agent(&dir, "reviewer");

    assert_eq!(reviewer.name, "reviewer");
    assert_eq!(reviewer.model, "fast");
    assert_eq!(reviewer.about, "reviews a diff");
    assert_eq!(reviewer.tools, ["grep"]);
    assert_eq!(reviewer.budget.tokens, Some(50_000));
    assert_eq!(reviewer.on_tool_error, ToolErrorPolicy::Abort);
    assert_eq!(reviewer.system, "You review code. Be blunt.");
}

/// [R-STAR-050] frontmatter may not carry system, because the body is the
/// prompt
#[test]
fn frontmatter_may_not_carry_the_system_prompt() {
    let dir = workspace(&[
        ("meow.star", MODELS),
        (
            "agents/two_prompts.md",
            "---\nmodel: fast\nsystem: one prompt\n---\nanother prompt\n",
        ),
    ]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("may not carry `system`"), "{error}");
}

/// [R-STAR-051] a markdown agent and a Starlark agent come out identical
#[test]
fn the_two_ways_of_declaring_an_agent_agree() {
    let starlark = workspace(&[(
        "meow.star",
        &format!(
            r#"{MODELS}
meow.agent(
    name = "twin",
    model = "fast",
    system = "Be brief.",
    about = "a twin",
    budget = {{"tokens": 1000, "steps": 3}},
    on_tool_error = "abort",
)
"#
        ),
    )]);

    let markdown = workspace(&[
        ("meow.star", MODELS),
        (
            "agents/twin.md",
            r#"---
model: fast
about: a twin
budget:
  tokens: 1000
  steps: 3
on_tool_error: abort
---
Be brief.
"#,
        ),
    ]);

    let from_starlark = agent(&starlark, "twin");
    let from_markdown = agent(&markdown, "twin");

    assert_eq!(from_starlark.name, from_markdown.name);
    assert_eq!(from_starlark.model, from_markdown.model);
    assert_eq!(from_starlark.system, from_markdown.system);
    assert_eq!(from_starlark.about, from_markdown.about);
    assert_eq!(from_starlark.tools, from_markdown.tools);
    assert_eq!(from_starlark.budget, from_markdown.budget);
    assert_eq!(from_starlark.compaction.at, from_markdown.compaction.at);
    assert_eq!(from_starlark.on_tool_error, from_markdown.on_tool_error);
    assert_eq!(from_starlark.output, from_markdown.output);

    // Only where it was written differs, which is the point of recording it.
    assert_ne!(from_starlark.origin, from_markdown.origin);
}

/// [R-STAR-051] a markdown agent is a duplicate of a Starlark agent with the
/// same name, and says so
#[test]
fn a_markdown_agent_collides_with_a_starlark_one() {
    let dir = workspace(&[
        (
            "meow.star",
            &format!(r#"{MODELS}meow.agent(name = "twin", model = "fast", system = "s")"#),
        ),
        ("agents/twin.md", "---\nmodel: fast\n---\nBe brief.\n"),
    ]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("agent `twin` is declared twice"), "{error}");
    assert!(error.contains("twin.md"), "{error}");
    assert!(error.contains("meow.star"), "{error}");
}

/// [R-STAR-052] frontmatter that is not valid YAML fails with the file and the
/// line
#[test]
fn broken_yaml_names_the_file_and_the_line() {
    let dir = workspace(&[
        ("meow.star", MODELS),
        (
            "agents/broken.md",
            "---\nmodel: fast\ntools: [unclosed\n---\nA prompt.\n",
        ),
    ]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("broken.md"), "{error}");
    // The bad line is line 3 of the file, and the flow sequence it belongs to
    // opens on line 3 as well. Every number in the message counts from the top
    // of the file, not from the top of the frontmatter.
    assert!(
        error.contains("broken.md:4:1"),
        "the line is missing: {error}"
    );
    assert!(
        error.contains("at line 3 column 8"),
        "a line number inside the message still counts from the frontmatter: {error}"
    );
}

/// [R-STAR-052] a key outside the allowed set fails, naming the key
#[test]
fn an_unknown_frontmatter_key_is_refused() {
    let dir = workspace(&[
        ("meow.star", MODELS),
        (
            "agents/odd.md",
            "---\nmodel: fast\ntemperature: 0.7\n---\nA prompt.\n",
        ),
    ]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("odd.md"), "{error}");
    assert!(error.contains("temperature"), "{error}");
    assert!(error.contains("unknown field"), "{error}");
}

/// [R-STAR-053] a .md file under .meow/lib/ loads as a string
#[test]
fn a_prompt_file_loads_as_a_string() {
    let dir = workspace(&[
        (
            "meow.star",
            &format!(
                r#"load("//lib/style.md", "text")
{MODELS}
meow.agent(name = "writer", model = "fast", system = text + "\n\nWrite well.")
"#
            ),
        ),
        ("lib/style.md", "Use short sentences.\n"),
    ]);

    let writer = agent(&dir, "writer");
    assert_eq!(writer.system, "Use short sentences.\n\n\nWrite well.");
}

/// [R-STAR-054] include prepends each file in the order given, separated by a
/// blank line
#[test]
fn include_prepends_each_prompt_in_order() {
    let dir = workspace(&[
        ("meow.star", MODELS),
        ("lib/style.md", "Use short sentences.\n"),
        ("lib/tone.md", "Be direct.\n"),
        (
            "agents/writer.md",
            "---\nmodel: fast\ninclude:\n  - //lib/style.md\n  - //lib/tone.md\n---\nWrite the release notes.\n",
        ),
    ]);

    let writer = agent(&dir, "writer");
    assert_eq!(
        writer.system,
        "Use short sentences.\n\nBe direct.\n\nWrite the release notes."
    );
}

/// [R-STAR-054] an include that leaves .meow/ is refused, like every other load
#[test]
fn an_include_may_not_leave_the_config_directory() {
    let dir = workspace(&[
        ("meow.star", MODELS),
        (
            "agents/writer.md",
            "---\nmodel: fast\ninclude: [//../secrets.md]\n---\nWrite.\n",
        ),
    ]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("leaves .meow/"), "{error}");
}

/// [R-STAR-054] include is the only composition: a Starlark agent has no such
/// key
#[test]
fn include_belongs_to_a_markdown_agent() {
    let dir = workspace(&[
        ("meow.star", MODELS),
        ("lib/style.md", "Be brief.\n"),
        (
            "agents/writer.md",
            "---\nmodel: fast\ninclude: [//lib/style.txt]\n---\nWrite.\n",
        ),
    ]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("names a markdown file"), "{error}");
}

/// [R-STAR-050] a markdown agent with no body has no system prompt, and says so
#[test]
fn a_markdown_agent_needs_a_body() {
    let dir = workspace(&[
        ("meow.star", MODELS),
        ("agents/empty.md", "---\nmodel: fast\n---\n\n"),
    ]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("the body is the system prompt"), "{error}");
}

/// A workspace with no agents directory loads, because most do not have one.
#[test]
fn a_workspace_without_markdown_agents_loads() {
    let dir = workspace(&[("meow.star", MODELS)]);
    assert!(load_at(&dir).is_ok());
    assert!(!Path::new(&dir.path().join(".meow").join("agents")).exists());
}
