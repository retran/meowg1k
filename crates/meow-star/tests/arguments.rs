// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! One argument declaration, and the three readers it serves.
#![allow(clippy::unwrap_used)]

use std::path::Path;

use meow_star::{Args, StarError, Workspace, load};
use serde_json::{Map, Value, json};
use tempfile::TempDir;

fn workspace(body: &str) -> TempDir {
    let dir = tempfile::tempdir().unwrap();
    write(dir.path(), body);
    dir
}

fn write(root: &Path, body: &str) {
    let path = root.join(".meow");
    std::fs::create_dir_all(&path).unwrap();
    std::fs::write(path.join("meow.star"), body).unwrap();
}

fn load_at(dir: &TempDir) -> Result<meow_star::Loaded, StarError> {
    load(&Workspace::at(dir.path()))
}

/// A tool declaration wrapped around whatever `args` the test needs.
fn tool(args: &str) -> String {
    format!(
        r#"
def handler(ctx):
    return ""

meow.tool(name = "t", about = "a tool", run = handler, args = {args})
"#
    )
}

fn args_of(body: &str) -> Args {
    let dir = workspace(body);
    load_at(&dir)
        .unwrap()
        .registry
        .tool("t")
        .unwrap()
        .args
        .clone()
}

fn given(pairs: &[(&str, Value)]) -> Map<String, Value> {
    pairs
        .iter()
        .map(|(k, v)| ((*k).to_owned(), v.clone()))
        .collect()
}

/// [R-STAR-060] every type builds, each with the constraints that belong to it
#[test]
fn every_argument_type_builds_with_its_constraints() {
    let args = args_of(&tool(
        r#"{
    "title": meow.arg.string(about = "the title", max_len = 10, pattern = "v*"),
    "count": meow.arg.int(about = "how many", min = 1, max = 5, required = False, default = 2),
    "ratio": meow.arg.float(about = "how much", min = 0.0, max = 1.0, required = False),
    "force": meow.arg.bool(about = "do it anyway", default = False),
    "mode": meow.arg.enum(values = ["fast", "slow"], about = "which", default = "fast", required = False),
    "paths": meow.arg.list(element = meow.schema.string(), about = "where", required = False),
}"#,
    ));

    let schema = args.json_schema();
    let properties = schema["properties"].as_object().unwrap();

    assert_eq!(properties["title"]["type"], "string");
    assert_eq!(properties["title"]["maxLength"], 10);
    assert_eq!(properties["title"]["pattern"], "v*");
    assert_eq!(properties["count"]["type"], "integer");
    assert_eq!(properties["count"]["minimum"], 1);
    assert_eq!(properties["count"]["maximum"], 5);
    assert_eq!(properties["count"]["default"], 2);
    assert_eq!(properties["ratio"]["type"], "number");
    assert_eq!(properties["force"]["type"], "boolean");
    assert_eq!(properties["mode"]["enum"], json!(["fast", "slow"]));
    assert_eq!(properties["paths"]["type"], "array");
    assert_eq!(properties["paths"]["items"]["type"], "string");

    assert_eq!(schema["required"], json!(["title"]));
}

/// [R-STAR-060] a default outside an enum's values is refused where it is
/// written
#[test]
fn an_enum_default_must_be_one_of_its_values() {
    let dir = workspace(&tool(
        r#"{"mode": meow.arg.enum(values = ["fast"], default = "slow")}"#,
    ));

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("`slow` is not one of the values"), "{error}");
}

/// [R-STAR-061] one declaration produces the flag, the help line, and the
/// schema
#[test]
fn one_declaration_produces_the_flag_the_help_and_the_schema() {
    let args = args_of(&tool(
        r#"{
    "max_size": meow.arg.int(about = "how many bytes to read", min = 1, required = False, default = 4096),
    "mode": meow.arg.enum(values = ["fast", "slow"], about = "which way", required = True),
}"#,
    ));

    let max_size = args.iter().find(|a| a.name == "max_size").unwrap();
    assert_eq!(max_size.flag(), "--max-size");
    assert_eq!(max_size.help(), "how many bytes to read [default: 4096]");

    let mode = args.iter().find(|a| a.name == "mode").unwrap();
    assert_eq!(mode.help(), "which way (one of: fast, slow) (required)");

    // The same node reaches the model, without the command-line annotations.
    let schema = args.json_schema();
    assert_eq!(schema["properties"]["max_size"]["minimum"], 1);
    assert!(
        schema["properties"]["max_size"].get("x-meow").is_none(),
        "the command-line annotations leaked into the model's schema"
    );
}

/// [R-STAR-062] positional indices are unique
#[test]
fn two_arguments_may_not_share_a_position() {
    let dir = workspace(&tool(
        r#"{
    "src": meow.arg.string(positional = 0),
    "dst": meow.arg.string(positional = 0),
}"#,
    ));

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("both positional 0"), "{error}");
}

/// [R-STAR-062] positional indices run from zero without gaps
#[test]
fn positional_indices_must_be_contiguous_from_zero() {
    let dir = workspace(&tool(
        r#"{
    "src": meow.arg.string(positional = 0),
    "dst": meow.arg.string(positional = 2),
}"#,
    ));

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("without gaps"), "{error}");
    assert!(error.contains("1 is missing"), "{error}");
}

/// [R-STAR-062] positional arguments come back in the order they are typed
#[test]
fn positional_arguments_keep_their_order() {
    let args = args_of(&tool(
        r#"{
    "dst": meow.arg.string(positional = 1),
    "src": meow.arg.string(positional = 0),
    "force": meow.arg.bool(required = False, default = False),
}"#,
    ));

    let names: Vec<&str> = args.positional().iter().map(|a| a.name.as_str()).collect();
    assert_eq!(names, ["src", "dst"]);

    let flags: Vec<&str> = args.flags().map(|a| a.name.as_str()).collect();
    assert_eq!(flags, ["force"]);
}

/// [R-STAR-063] a constraint holds for a command-line value and a
/// model-supplied one alike
#[test]
fn a_constraint_holds_on_both_paths() {
    let args = args_of(&tool(
        r#"{
    "count": meow.arg.int(about = "how many", min = 1, max = 5),
    "mode": meow.arg.enum(values = ["fast", "slow"], required = False, default = "fast"),
}"#,
    ));

    // What a command line produces, after parsing the text of a flag.
    let from_cli = args.bind(&given(&[("count", json!(9))])).unwrap_err();
    assert_eq!(from_cli.len(), 1);
    assert!(
        from_cli[0].to_string().contains("must be at most 5"),
        "{:?}",
        from_cli
    );

    // What a model produces, as a JSON object.
    let from_model = args.bind(&given(&[("count", json!(0)), ("mode", json!("sideways"))]));
    let problems = from_model.unwrap_err();
    assert_eq!(problems.len(), 2, "{problems:?}");
    assert!(
        problems
            .iter()
            .any(|p| p.to_string().contains("at least 1"))
    );
    assert!(
        problems
            .iter()
            .any(|p| p.to_string().contains("must be one of"))
    );

    // A value that satisfies both gets the declared default filled in.
    let bound = args.bind(&given(&[("count", json!(3))])).unwrap();
    assert_eq!(bound["count"], json!(3));
    assert_eq!(bound["mode"], json!("fast"));
}

/// [R-STAR-063] a missing required argument and an unknown one are both
/// reported, and reported together
#[test]
fn every_problem_is_reported_at_once() {
    let args = args_of(&tool(
        r#"{
    "count": meow.arg.int(min = 1),
    "title": meow.arg.string(),
}"#,
    ));

    let problems = args.bind(&given(&[("titel", json!("x"))])).unwrap_err();
    let text: Vec<String> = problems.iter().map(ToString::to_string).collect();

    assert_eq!(problems.len(), 3, "{text:?}");
    assert!(
        text.iter().any(|p| p.contains("`count`: is required")),
        "{text:?}"
    );
    assert!(
        text.iter().any(|p| p.contains("`title`: is required")),
        "{text:?}"
    );
    assert!(
        text.iter().any(|p| p.contains("Did you mean `title`?")),
        "{text:?}"
    );
}

/// [R-STAR-063] a wrong type is reported in the user's terms
#[test]
fn a_wrong_type_says_what_was_wanted_and_what_arrived() {
    let args = args_of(&tool(r#"{"count": meow.arg.int()}"#));

    let problems = args.bind(&given(&[("count", json!("three"))])).unwrap_err();
    assert!(
        problems[0]
            .to_string()
            .contains("must be an integer, and is a string"),
        "{problems:?}"
    );
}

/// [R-STAR-070] meow.schema builds each kind, and emits JSON Schema
#[test]
fn a_schema_emits_json_schema() {
    let dir = workspace(
        r#"
meow.provider(name = "anthropic", kind = "anthropic")
meow.model(name = "fast", provider = "anthropic", id = "x", context = 1, max_output = 1)
meow.agent(
    name = "reviewer",
    model = "fast",
    system = "review",
    output = meow.schema.object(
        fields = {
            "verdict": meow.schema.enum(values = ["ship", "hold"], about = "what to do"),
            "score": meow.schema.int(about = "out of ten"),
            "weight": meow.schema.float(),
            "blocking": meow.schema.bool(),
            "notes": meow.schema.list(element = meow.schema.string()),
        },
        required = ["verdict", "score"],
    ),
)
"#,
    );

    let loaded = load_at(&dir).unwrap();
    let output = loaded
        .registry
        .agent("reviewer")
        .unwrap()
        .output
        .clone()
        .unwrap();

    assert_eq!(output["type"], "object");
    assert_eq!(output["required"], json!(["verdict", "score"]));
    assert_eq!(output["additionalProperties"], json!(false));

    let fields = output["properties"].as_object().unwrap();
    assert_eq!(fields["verdict"]["enum"], json!(["ship", "hold"]));
    assert_eq!(fields["verdict"]["description"], "what to do");
    assert_eq!(fields["score"]["type"], "integer");
    assert_eq!(fields["weight"]["type"], "number");
    assert_eq!(fields["blocking"]["type"], "boolean");
    assert_eq!(fields["notes"]["items"]["type"], "string");
}

/// [R-STAR-071] a required field the schema does not declare fails when the
/// schema is built
#[test]
fn a_schema_may_not_require_a_field_it_does_not_declare() {
    let dir = workspace(
        r#"
meow.schema.object(
    fields = {"verdict": meow.schema.string()},
    required = ["verdic"],
)
"#,
    );

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("field `verdic` is not declared"), "{error}");
    assert!(error.contains("Did you mean `verdict`?"), "{error}");
}
