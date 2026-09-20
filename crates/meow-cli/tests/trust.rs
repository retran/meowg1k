// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Agreeing to run a workspace.
//!
//! Driven as a binary, because the claim is about what happens between
//! loading and running, and both of those are the process.
#![allow(clippy::unwrap_used)]

use std::path::Path;
use std::process::{Command, Output};

use tempfile::TempDir;

fn meow() -> Command {
    Command::new(env!("CARGO_BIN_EXE_meow"))
}

fn home() -> TempDir {
    tempfile::tempdir().unwrap()
}

/// Run with stdin closed, which is the unattended case unless a test says
/// otherwise.
fn run(home: &Path, at: &Path, args: &[&str]) -> Output {
    meow()
        .current_dir(at)
        .env("MEOW_HOME", home)
        .env_remove("NO_COLOR")
        .stdin(std::process::Stdio::null())
        .args(args)
        .output()
        .unwrap()
}

fn stdout(output: &Output) -> String {
    String::from_utf8_lossy(&output.stdout).into_owned()
}

fn stderr(output: &Output) -> String {
    String::from_utf8_lossy(&output.stderr).into_owned()
}

const DECLARING: &str = r#"
meow.provider(name = "p", kind = "anthropic", api_key = "k")
meow.model(name = "m", provider = "p", id = "i", context = 1000, max_output = 10)
meow.policy(rules = [
    {"tools": ["read_file"], "decision": "allow"},
    {"tools": ["*"], "decision": "ask"},
])

def handler(ctx):
    ctx.out.write("ran")
    return "true"

meow.command(meow.tool(name = "go", about = "do a thing", run = handler))
"#;

fn workspace(source: &str) -> TempDir {
    let dir = tempfile::tempdir().unwrap();
    let config = dir.path().join(".meow");
    std::fs::create_dir_all(&config).unwrap();
    std::fs::write(config.join("meow.star"), source).unwrap();
    dir
}

/// [R-AUTH-030] [R-AUTH-031] an untrusted workspace does not run, and says how
/// to agree
#[test]
fn an_untrusted_workspace_does_not_run() {
    let home = home();
    let at = workspace(DECLARING);

    let out = run(home.path(), at.path(), &["go"]);
    let complained = stderr(&out);

    assert!(!out.status.success(), "an untrusted workspace ran");
    assert!(
        !stdout(&out).contains("ran"),
        "the handler ran: {}",
        stdout(&out)
    );
    assert!(
        complained.contains("meow trust"),
        "the refusal must name the command that agrees: {complained}"
    );
}

/// [R-AUTH-030] what is shown is what is being agreed to
#[test]
fn the_question_shows_the_agents_tools_and_policy() {
    let home = home();
    let at = workspace(DECLARING);

    let complained = stderr(&run(home.path(), at.path(), &["go"]));

    for expected in [
        "tool go",
        "command go",
        "policy allow read_file",
        "policy ask *",
    ] {
        assert!(
            complained.contains(expected),
            "`{expected}` was not shown: {complained}"
        );
    }
}

/// [R-AUTH-030] agreeing lets it run
#[test]
fn a_trusted_workspace_runs() {
    let home = home();
    let at = workspace(DECLARING);

    let agreed = run(home.path(), at.path(), &["trust"]);
    assert!(agreed.status.success(), "{}", stderr(&agreed));

    let out = run(home.path(), at.path(), &["go"]);
    assert!(out.status.success(), "{}", stderr(&out));
    assert!(stdout(&out).contains("ran"), "{}", stdout(&out));
}

/// [R-AUTH-034] widening the policy asks again
///
/// This is the case the whole feature is for. Agreeing once and forever means
/// agreeing to whatever the repository becomes, and a `.meow/` that changes
/// `ask` to `allow` has changed what it may do without asking anyone.
#[test]
fn widening_the_policy_withdraws_the_agreement() {
    let home = home();
    let at = workspace(DECLARING);
    run(home.path(), at.path(), &["trust"]);
    assert!(run(home.path(), at.path(), &["go"]).status.success());

    std::fs::write(
        at.path().join(".meow").join("meow.star"),
        DECLARING.replace(
            r#"{"tools": ["*"], "decision": "ask"}"#,
            r#"{"tools": ["*"], "decision": "allow"}"#,
        ),
    )
    .unwrap();

    let out = run(home.path(), at.path(), &["go"]);
    assert!(!out.status.success(), "a widened policy kept its trust");
    assert!(
        stderr(&out).contains("different from what you agreed to"),
        "the refusal must say what changed: {}",
        stderr(&out)
    );
}

/// [R-AUTH-034] a new tool asks again
#[test]
fn a_new_tool_withdraws_the_agreement() {
    let home = home();
    let at = workspace(DECLARING);
    run(home.path(), at.path(), &["trust"]);

    std::fs::write(
        at.path().join(".meow").join("meow.star"),
        format!(
            r#"{DECLARING}
def second(ctx):
    return "true"

meow.command(meow.tool(name = "another", about = "something else", run = second))
"#
        ),
    )
    .unwrap();

    assert!(
        !run(home.path(), at.path(), &["go"]).status.success(),
        "a workspace that grew a tool kept its trust"
    );
}

/// [R-AUTH-034] editing a handler's body does not ask again
///
/// The record is what a workspace may do, not what it says. Re-asking for
/// every edit would make the question a formality people click through, which
/// is the failure mode a trust prompt has to avoid.
#[test]
fn editing_a_handler_does_not_ask_again() {
    let home = home();
    let at = workspace(DECLARING);
    run(home.path(), at.path(), &["trust"]);

    std::fs::write(
        at.path().join(".meow").join("meow.star"),
        DECLARING.replace(
            r#"ctx.out.write("ran")"#,
            r#"ctx.out.write("ran differently")"#,
        ),
    )
    .unwrap();

    let out = run(home.path(), at.path(), &["go"]);
    assert!(out.status.success(), "{}", stderr(&out));
    assert!(stdout(&out).contains("ran differently"), "{}", stdout(&out));
}

/// [R-AUTH-032] trust can be withdrawn, and listed
#[test]
fn trust_can_be_listed_and_withdrawn() {
    let home = home();
    let at = workspace(DECLARING);

    assert!(
        stdout(&run(home.path(), at.path(), &["trust", "--list"])).contains("no workspaces"),
        "a fresh machine must trust nothing"
    );

    run(home.path(), at.path(), &["trust"]);
    assert!(
        stdout(&run(home.path(), at.path(), &["trust", "--list"])).contains(".meow")
            || !stdout(&run(home.path(), at.path(), &["trust", "--list"]))
                .contains("no workspaces"),
        "the listing does not show the workspace"
    );

    assert!(
        run(home.path(), at.path(), &["trust", "--withdraw"])
            .status
            .success()
    );
    assert!(
        !run(home.path(), at.path(), &["go"]).status.success(),
        "a withdrawn workspace still ran"
    );
}

/// [R-AUTH-030] the commands that describe a workspace do not need trust
///
/// They are how a person decides whether to trust one. Requiring trust before
/// they run would make the decision impossible to inform.
#[test]
fn describing_a_workspace_needs_no_trust() {
    let home = home();
    let at = workspace(DECLARING);

    for args in [
        vec!["check"],
        vec!["doctor"],
        vec!["policy", "show"],
        vec!["models"],
        vec!["providers"],
    ] {
        let out = run(home.path(), at.path(), &args);
        assert!(
            !stderr(&out).contains("has not been run on this machine"),
            "`meow {}` asked for trust: {}",
            args.join(" "),
            stderr(&out)
        );
    }
}
