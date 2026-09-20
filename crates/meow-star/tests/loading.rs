// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Loading a workspace: discovery, the load scheme, and the declaration phase.
//!
//! `clippy.toml` exempts `#[test]` functions from the `unwrap` denial but not
//! the helpers beside them, and a helper that returns `Result` only to have
//! every caller unwrap it hides which line failed.
#![allow(clippy::unwrap_used)]

use std::path::{Path, PathBuf};

use meow_star::{StarError, Workspace, load};
use tempfile::TempDir;

/// A workspace whose `.meow/` holds the given files.
fn workspace(files: &[(&str, &str)]) -> TempDir {
    let dir = tempfile::tempdir().unwrap();
    write_files(dir.path(), files);
    dir
}

fn write_files(root: &Path, files: &[(&str, &str)]) {
    for (name, body) in files {
        let path = root.join(".meow").join(name);
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, body).unwrap();
    }
}

fn load_at(dir: &TempDir) -> Result<meow_star::Loaded, StarError> {
    load(&Workspace::at(dir.path()))
}

/// Enough declarations for a reference to resolve.
const PRELUDE: &str = r#"
meow.provider(name = "anthropic", kind = "anthropic")
meow.model(
    name = "fast",
    provider = "anthropic",
    id = "claude-haiku-4-5",
    context = 200000,
    max_output = 8192,
)
"#;

/// [R-STAR-001] the workspace is the nearest ancestor with .meow/meow.star,
/// and nothing outside it is merged in
#[test]
fn discovery_takes_the_nearest_ancestor_and_merges_nothing() {
    let outer = tempfile::tempdir().unwrap();
    write_files(outer.path(), &[("meow.star", "")]);

    let inner = outer.path().join("project");
    write_files(&inner, &[("meow.star", "")]);

    let deep = inner.join("src").join("nested");
    std::fs::create_dir_all(&deep).unwrap();

    let found = Workspace::discover(&deep).unwrap();
    assert_eq!(found.root(), inner.as_path());
    assert_ne!(found.root(), outer.path());
}

/// [R-STAR-002] running outside a workspace names what was searched and says
/// what to do about it
#[test]
fn no_workspace_names_the_directories_and_suggests_init() {
    let dir = tempfile::tempdir().unwrap();
    let deep = dir.path().join("a").join("b");
    std::fs::create_dir_all(&deep).unwrap();

    let error = Workspace::discover(&deep).unwrap_err();
    let message = error.to_string();

    assert!(message.contains("meow init"), "{message}");
    assert!(message.contains(&deep.display().to_string()), "{message}");
    assert!(
        message.contains(&dir.path().display().to_string()),
        "the ancestors searched are not listed: {message}"
    );
}

/// [R-STAR-003] @std// resolves a runtime module, and an unknown name lists
/// what there is
#[test]
fn an_unknown_std_module_lists_the_ones_that_exist() {
    let dir = workspace(&[("meow.star", r#"load("@std//nope", "get")"#)]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("there is no module `@std//nope`"), "{error}");
    assert!(
        error.contains("env"),
        "the available modules are not listed: {error}"
    );
}

/// [R-STAR-003] a module that exists resolves, and its members are usable
#[test]
fn std_env_resolves_and_its_members_are_usable() {
    // `PATH` rather than a variable this test sets: `set_var` is unsafe in
    // edition 2024 and the workspace denies unsafe, and reading a variable
    // that is always present proves the same thing.
    let dir = workspace(&[(
        "meow.star",
        &format!(
            r#"
load("@std//env", "get")
{PRELUDE}
meow.provider(name = "present", kind = "openai", api_key = get("PATH"))
meow.provider(name = "absent", kind = "openai", api_key = get("MEOW_NOT_SET_ANYWHERE", "fallback"))
"#
        ),
    )]);

    let loaded = load_at(&dir).unwrap();
    assert!(
        loaded
            .registry
            .provider("present")
            .unwrap()
            .api_key
            .is_some()
    );
    assert_eq!(
        loaded
            .registry
            .provider("absent")
            .unwrap()
            .api_key
            .as_deref(),
        Some("fallback")
    );
}

/// [R-STAR-004] // resolves inside .meow/, and a path that escapes it fails
#[test]
fn a_local_load_stays_inside_the_config_directory() {
    let dir = workspace(&[
        (
            "meow.star",
            "load(\"//lib/models.star\", \"setup\")\nsetup()",
        ),
        (
            "lib/models.star",
            &format!("def setup():\n    pass\n{PRELUDE}"),
        ),
    ]);
    assert!(load_at(&dir).is_ok());

    let escaping = workspace(&[("meow.star", r#"load("//../outside.star", "x")"#)]);
    let error = load_at(&escaping).unwrap_err().to_string();
    assert!(error.contains("leaves .meow/"), "{error}");
}

/// [R-STAR-005] [R-PKG-001] a package path is never read as a local path
///
/// The workspace here has a file at exactly the path the load names. Reading
/// `@acme//lib/models.star` as `//lib/models.star` would work on this machine
/// and find nothing on any other, which is the failure this pins.
#[test]
fn a_package_load_is_never_read_as_a_local_path() {
    let dir = workspace(&[
        ("meow.star", r#"load("@acme//lib/models.star", "setup")"#),
        ("lib/models.star", "def setup():\n    pass"),
    ]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(
        error.contains("does not declare") && error.contains("acme"),
        "an undeclared package must be refused by name: {error}"
    );
    // The proof that it was not read as a local path is that it failed at
    // all: `//lib/models.star` is there and would have loaded.
    assert!(
        load_at(&workspace(&[
            ("meow.star", r#"load("//lib/models.star", "setup")"#),
            (
                "lib/models.star",
                &format!("def setup():\n    pass\n{PRELUDE}")
            ),
        ]))
        .is_ok(),
        "the same file loads through `//`, so the failure above is about the scheme"
    );
}

/// [R-STAR-006] an import cycle is reported with the ring in order
#[test]
fn an_import_cycle_is_listed_in_order() {
    let dir = workspace(&[
        ("meow.star", "load(\"//a.star\", \"a\")"),
        ("a.star", "load(\"//b.star\", \"b\")\na = 1"),
        ("b.star", "load(\"//a.star\", \"a\")\nb = 1"),
    ]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("import cycle"), "{error}");
    assert!(error.contains("a.star -> b.star -> a.star"), "{error}");
}

/// [R-STAR-007] a file named by several load statements is evaluated once
#[test]
fn a_file_loaded_twice_is_evaluated_once() {
    let dir = workspace(&[
        (
            "meow.star",
            "load(\"//one.star\", \"one\")\nload(\"//two.star\", \"two\")",
        ),
        ("one.star", "load(\"//shared.star\", \"shared\")\none = 1"),
        ("two.star", "load(\"//shared.star\", \"shared\")\ntwo = 2"),
        ("shared.star", &format!("shared = 1\n{PRELUDE}")),
    ]);

    let loaded = load_at(&dir).unwrap();

    let shared: Vec<&PathBuf> = loaded
        .files
        .iter()
        .filter(|p| p.ends_with("shared.star"))
        .collect();
    assert_eq!(
        shared.len(),
        1,
        "evaluated more than once: {:?}",
        loaded.files
    );

    // Declaring twice would have failed anyway, which is the observable half
    // of the same guarantee.
    assert!(loaded.registry.model("fast").is_some());
}

/// [R-STAR-010] one module table decides what exists, and a module that
/// exists resolves in both phases
#[test]
fn one_table_separates_an_unknown_module_from_an_unavailable_call() {
    let unknown = workspace(&[("meow.star", r#"load("@std//nope", "x")"#)]);
    let unknown = load_at(&unknown).unwrap_err().to_string();
    assert!(unknown.contains("there is no module"), "{unknown}");

    // A module that exists loads in the declaration phase. What it refuses is
    // being called there, which is the distinction [R-STAR-084] draws.
    let loads = workspace(&[("meow.star", "load(\"@std//json\", \"parse\")")]);
    assert!(
        load_at(&loads).is_ok(),
        "a module that exists would not load"
    );

    let called = workspace(&[(
        "meow.star",
        "load(\"@std//json\", \"parse\")\nparse(\"{}\")",
    )]);
    let called = load_at(&called).unwrap_err().to_string();
    assert!(
        called.contains("not available while .meow/ is being loaded"),
        "a module call during declaration was allowed: {called}"
    );
}

/// [R-STAR-031] declaring the same name twice names both sites
#[test]
fn a_duplicate_declaration_names_both_files() {
    let dir = workspace(&[
        (
            "meow.star",
            "load(\"//extra.star\", \"extra\")\nmeow.provider(name = \"anthropic\", kind = \"anthropic\")",
        ),
        (
            "extra.star",
            "extra = 1\nmeow.provider(name = \"anthropic\", kind = \"openai\")",
        ),
    ]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("declared twice"), "{error}");
    assert!(error.contains("extra.star"), "{error}");
    assert!(error.contains("meow.star"), "{error}");
}

/// [R-STAR-032] a reference is resolved after every file is evaluated, so
/// declaration order does not matter
#[test]
fn a_reference_resolves_whatever_the_declaration_order() {
    let forward = workspace(&[(
        "meow.star",
        r#"
meow.agent(name = "helper", model = "fast", system = "help")
meow.provider(name = "anthropic", kind = "anthropic")
meow.model(name = "fast", provider = "anthropic", id = "x", context = 1, max_output = 1)
"#,
    )]);
    assert!(
        load_at(&forward).is_ok(),
        "an agent could not name a model declared below it"
    );

    let missing = workspace(&[(
        "meow.star",
        &format!(r#"{PRELUDE}meow.agent(name = "helper", model = "fastt", system = "help")"#),
    )]);
    let error = load_at(&missing).unwrap_err().to_string();
    assert!(error.contains("model `fastt` is not declared"), "{error}");
}

/// [R-STAR-033] a command that collides with a built-in is refused
#[test]
fn a_command_may_not_take_a_builtin_name() {
    let dir = workspace(&[(
        "meow.star",
        &format!(
            r#"{PRELUDE}
meow.command(meow.agent(name = "session", model = "fast", system = "help"))
"#
        ),
    )]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("built-in command"), "{error}");
}

/// [R-STAR-082] print says what to use instead
#[test]
fn print_fails_and_points_at_ctx_out() {
    let dir = workspace(&[("meow.star", r#"print("hello")"#)]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("ctx.out"), "{error}");
}

/// [R-STAR-083] a declaration file has no way to reach the world, because
/// every module but `env` refuses to be called during declaration
#[test]
fn a_declaration_file_cannot_reach_the_world() {
    let calls = [
        ("json", "parse(\"{}\")", "parse"),
        ("path", "join(\"a\", \"b\")", "join"),
        ("text", "dedent(\"x\")", "dedent"),
    ];

    for (module, call, symbol) in calls {
        let dir = workspace(&[(
            "meow.star",
            &format!("load(\"@std//{module}\", \"{symbol}\")\n{call}"),
        )]);
        let error = load_at(&dir).unwrap_err().to_string();
        assert!(
            error.contains("not available while .meow/ is being loaded"),
            "@std//{module} was usable during declaration: {error}"
        );
    }
}

/// [R-STAR-084] only load and @std//env are callable during declaration
#[test]
fn only_env_is_callable_during_declaration() {
    let allowed = workspace(&[("meow.star", "load(\"@std//env\", \"get\")\nget(\"PATH\")")]);
    assert!(load_at(&allowed).is_ok());

    let refused = workspace(&[(
        "meow.star",
        "load(\"@std//text\", \"tokens\")\ntokens(\"hello\")",
    )]);
    let error = load_at(&refused).unwrap_err().to_string();
    assert!(error.contains("text.tokens"), "{error}");
    assert!(
        error.contains("not available while .meow/ is being loaded"),
        "{error}"
    );
}

/// [R-STAR-080] each load owns its evaluators, and nothing survives into the
/// next one
#[test]
fn two_loads_of_one_workspace_are_independent() {
    let dir = workspace(&[("meow.star", PRELUDE)]);

    let first = load_at(&dir).unwrap();
    let second = load_at(&dir).unwrap();

    assert!(first.registry.model("fast").is_some());
    assert!(second.registry.model("fast").is_some());
    assert_eq!(first.files.len(), second.files.len());
}

/// [R-STAR-090] a Starlark error carries the file and the line
#[test]
fn an_error_points_at_the_line_that_caused_it() {
    let dir = workspace(&[("meow.star", "x = 1\ny = undefined_name\n")]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("meow.star"), "{error}");
    assert!(error.contains(":2"), "the line is missing: {error}");
}

/// [R-STAR-091] a near-miss name is suggested
#[test]
fn a_near_miss_suggests_the_declared_name() {
    let dir = workspace(&[(
        "meow.star",
        &format!(r#"{PRELUDE}meow.agent(name = "helper", model = "fasr", system = "help")"#),
    )]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("Did you mean `fast`?"), "{error}");
}

/// A workspace with a chat model and an embedding model.
const TWO_KINDS: &str = r#"
meow.provider(name = "p", kind = "anthropic")
meow.model(name = "fast", provider = "p", id = "m", context = 1, max_output = 1)
meow.model(
    name = "embed",
    provider = "p",
    id = "e",
    context = 1,
    max_output = 0,
    kind = "embedding",
)
"#;

/// [R-STAR-034] a model says what it is for, and chat is the default
#[test]
fn a_model_declares_what_it_is_for() {
    let dir = workspace(&[("meow.star", TWO_KINDS)]);
    let loaded = load_at(&dir).unwrap();

    assert_eq!(
        loaded.registry.model("fast").unwrap().kind,
        meow_star::registry::ModelKind::Chat,
        "a model that says nothing should answer"
    );
    assert_eq!(
        loaded.registry.model("embed").unwrap().kind,
        meow_star::registry::ModelKind::Embedding
    );

    let wrong = workspace(&[(
        "meow.star",
        &format!(
            r#"{TWO_KINDS}meow.model(name = "odd", provider = "p", id = "x", context = 1, max_output = 1, kind = "oracle")"#
        ),
    )]);
    let error = load_at(&wrong).unwrap_err().to_string();
    assert!(error.contains("`chat` or `embedding`"), "{error}");
}

/// [R-STAR-034] an agent given an embedding model is refused, naming both
#[test]
fn an_agent_may_not_use_an_embedding_model() {
    let dir = workspace(&[(
        "meow.star",
        &format!(r#"{TWO_KINDS}meow.agent(name = "helper", model = "embed", system = "s")"#),
    )]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("agent `helper`"), "{error}");
    assert!(error.contains("needs a chat model"), "{error}");
    assert!(error.contains("`embed` is an embedding model"), "{error}");
}

/// [R-STAR-035] an index says which model embeds the workspace
#[test]
fn an_index_declares_its_model_and_parameters() {
    let dir = workspace(&[(
        "meow.star",
        &format!(
            r#"{TWO_KINDS}meow.index(model = "embed", chunk_lines = 40, overlap = 8, max_bytes = 500000)"#
        ),
    )]);

    let declared = load_at(&dir).unwrap().registry.index().cloned().unwrap();

    assert_eq!(declared.model, "embed");
    assert_eq!(declared.chunk_lines, Some(40));
    assert_eq!(declared.overlap, Some(8));
    assert_eq!(declared.max_bytes, Some(500_000));
}

/// [R-STAR-035] an index on a chat model is refused, naming both
#[test]
fn an_index_may_not_use_a_chat_model() {
    let dir = workspace(&[(
        "meow.star",
        &format!(r#"{TWO_KINDS}meow.index(model = "fast")"#),
    )]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(error.contains("meow.index"), "{error}");
    assert!(error.contains("needs an embedding model"), "{error}");
    assert!(error.contains("`fast` is a chat model"), "{error}");
}

/// [R-STAR-035] an index is declared at most once
#[test]
fn an_index_may_be_declared_only_once() {
    let dir = workspace(&[(
        "meow.star",
        &format!(
            r#"{TWO_KINDS}
meow.index(model = "embed")
meow.index(model = "embed")
"#
        ),
    )]);

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(
        error.contains("index `workspace` is declared twice"),
        "{error}"
    );
}

/// A workspace that declares no index loads; only an index command fails.
#[test]
fn a_workspace_without_an_index_still_loads() {
    let dir = workspace(&[("meow.star", TWO_KINDS)]);
    let loaded = load_at(&dir).unwrap();
    assert!(loaded.registry.index().is_none());
}
