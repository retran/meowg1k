// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Every module, through the binary.
//!
//! The unit tests in `meow-star` drive each module against a runtime a test
//! built, with doubles for the ports. That leaves one thing untested: whether
//! this binary hands a handler the ports it should. It did not - a workspace
//! with no index got a search port that answered every query with an empty
//! list, and every module test passed while it did, because they used a double
//! that answered.
//!
//! So this asks for one real thing from each of the seventeen modules, in one
//! handler, through the process. It is not a second copy of the module tests:
//! what it checks is that the module is reachable and wired to something that
//! works.
#![allow(clippy::unwrap_used)]

use std::path::Path;
use std::process::Command;

use tempfile::TempDir;

/// A workspace that loads every module and exercises one call in each.
///
/// `http` is absent: it needs a server, and `running.rs` drives it against a
/// real socket already. `index` appears only as the call that must refuse,
/// because ranking needs a model and a credential.
const EVERY_MODULE: &str = r#"
load("@std//env", "get")
load("@std//re", re_match = "match")
load("@std//time", "now", "since")
load("@std//json", json_encode = "encode")
load("@std//yaml", yaml_parse = "parse")
load("@std//toml", toml_encode = "encode")
load("@std//csv", csv_parse = "parse")
load("@std//xml", xml_parse = "parse")
load("@std//path", "join")
load("@std//text", "truncate")
load("@std//store", store_put = "put", store_get = "get")
load("@std//fs", "read", "write")
load("@std//shell", "which")
load("@std//git", "branch")
load("@std//search", "files", "text")
load("@std//index", index_stats = "stats")

meow.provider(name = "p", kind = "anthropic", api_key = get("NOTHING", "x"))
meow.model(name = "m", provider = "p", id = "i", context = 1000, max_output = 10)

def _wired(ctx):
    bad = []
    def want(name, got, expected):
        if got != expected:
            bad.append("%s: got %s want %s" % (name, got, expected))

    want("re", re_match(r"(\w+)@", "a@b")[1], "a")
    want("time", since(now()) <= 1, True)
    want("json", json_encode({"b": 1, "a": 2}), '{"a":2,"b":1}')
    want("yaml+toml", toml_encode(yaml_parse("n: 1\n")), "n = 1\n")
    want("csv", csv_parse("a\n1\n"), [{"a": "1"}])
    want("xml", xml_parse("<r>t</r>").text, "t")
    want("path", join("a", "b"), "a/b")
    want("text", truncate("abcdefgh", 5), "ab...")
    want("env", get("NOTHING", "fallback"), "fallback")

    store_put("k", [1, 2])
    want("store", store_get("k"), [1, 2])

    write("made.txt", "body")
    want("fs", read("made.txt"), "body")
    want("shell", which("not-a-real-program") == None, True)
    want("git", type(branch()), "string")

    # The two that need no index, in a workspace that has none. Both are
    # checked by containment rather than equality: this handler wrote
    # `made.txt` a few lines up, and the word it searches for appears in
    # `.meow/meow.star` too, because that is where this line is. Both extra
    # hits are the modules working.
    want("search.files", "hay.txt" in files("**/*.txt"), True)
    want("search.text", [h.first_line for h in text("needle") if h.path == "hay.txt"], [2])

    for line in bad:
        ctx.out.warn(line)
    ctx.out.write("checked")
    return "false" if bad else "true"

meow.command(meow.tool(name = "wired", about = "every module", run = _wired))
"#;

fn meow() -> Command {
    Command::new(env!("CARGO_BIN_EXE_meow"))
}

fn workspace() -> TempDir {
    let dir = tempfile::tempdir().unwrap();
    let config = dir.path().join(".meow");
    std::fs::create_dir_all(&config).unwrap();
    std::fs::write(config.join("meow.star"), EVERY_MODULE).unwrap();
    std::fs::write(dir.path().join("hay.txt"), "one\ntwo needle three\n").unwrap();

    // `git.branch` asks git about the working tree, so the workspace has to be
    // one. Dropping the module from the check instead would leave the wiring
    // this file exists to test unchecked for exactly the module most likely to
    // behave differently outside a repository.
    let git = |args: &[&str]| {
        Command::new("git")
            .current_dir(dir.path())
            .args(args)
            .output()
            .map(|o| o.status.success())
            .unwrap_or(false)
    };
    assert!(git(&["init", "-q", "."]), "could not make a repository");
    assert!(
        git(&[
            "-c",
            "user.email=t@example.com",
            "-c",
            "user.name=t",
            "commit",
            "-q",
            "--allow-empty",
            "-m",
            "first"
        ]),
        "could not make a commit for `git.branch` to name"
    );

    dir
}

/// Run `meow`, having first agreed to the workspace - `[R-AUTH-030]`.
fn run(dir: &Path, args: &[&str]) -> std::process::Output {
    let _ = bare(dir, &["trust"]);
    bare(dir, args)
}

fn bare(dir: &Path, args: &[&str]) -> std::process::Output {
    meow()
        .current_dir(dir)
        .env("MEOW_HOME", dir.join(".meow").join(".data").join("home"))
        .env_remove("NO_COLOR")
        .args(args)
        .output()
        .unwrap()
}

/// [R-STAR-010] [R-STAR-011] every module is reachable from a handler this
/// binary ran, and wired to something that answers
#[test]
fn every_module_works_through_the_binary() {
    let dir = workspace();
    let out = run(dir.path(), &["wired"]);

    let said = String::from_utf8_lossy(&out.stdout);
    let complained = String::from_utf8_lossy(&out.stderr);

    assert!(
        said.contains("checked"),
        "the handler did not finish: {said}{complained}"
    );
    assert!(
        out.status.success(),
        "a module was reachable but not wired to something that works:\n{said}{complained}"
    );
}

/// [R-STAR-003] a module that does not exist is refused, and the error lists
/// the ones that do
#[test]
fn an_unknown_module_lists_the_real_ones() {
    let dir = tempfile::tempdir().unwrap();
    let config = dir.path().join(".meow");
    std::fs::create_dir_all(&config).unwrap();
    std::fs::write(config.join("meow.star"), "load(\"@std//nope\", \"x\")\n").unwrap();

    let out = run(dir.path(), &["check"]);
    let complained = String::from_utf8_lossy(&out.stderr);

    assert!(
        complained.contains("there is no module `@std//nope`"),
        "the error did not name the module: {complained}"
    );
    // The listing is what tells a user what they could have written, so every
    // module the binary has must be in it.
    for module in [
        "csv", "env", "fs", "git", "http", "index", "json", "path", "re", "search", "shell",
        "store", "text", "time", "toml", "xml", "yaml",
    ] {
        assert!(
            complained.contains(module),
            "`{module}` is missing from the listing: {complained}"
        );
    }
}
