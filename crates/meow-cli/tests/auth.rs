// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The credential store, driven as a binary.
//!
//! A store is a file with a mode, written by a process that can die halfway.
//! None of that is visible from a function call, so these run `meow`.
#![allow(clippy::unwrap_used)]

use std::path::Path;
use std::process::{Command, Output};

use tempfile::TempDir;

fn meow() -> Command {
    Command::new(env!("CARGO_BIN_EXE_meow"))
}

/// A home directory of its own, so a test never reads the developer's store.
fn home() -> TempDir {
    tempfile::tempdir().unwrap()
}

fn run(home: &Path, at: &Path, args: &[&str]) -> Output {
    meow()
        .current_dir(at)
        .env("MEOW_HOME", home)
        .env_remove("ANTHROPIC_API_KEY")
        .env_remove("NO_COLOR")
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

/// A workspace declaring one provider with no key of its own.
fn workspace() -> TempDir {
    let dir = tempfile::tempdir().unwrap();
    let config = dir.path().join(".meow");
    std::fs::create_dir_all(&config).unwrap();
    std::fs::write(
        config.join("meow.star"),
        r#"
meow.provider(name = "anthropic", kind = "anthropic")
meow.model(name = "m", provider = "anthropic", id = "i", context = 1000, max_output = 10)
"#,
    )
    .unwrap();
    dir
}

/// [R-AUTH-013] login stores one, list says so, logout removes it
#[test]
fn a_credential_can_be_stored_listed_and_removed() {
    let home = home();
    let at = workspace();

    assert!(
        stdout(&run(home.path(), at.path(), &["auth", "list"])).contains("no credentials"),
        "a fresh machine must start with none"
    );

    let stored = run(
        home.path(),
        at.path(),
        &["auth", "login", "anthropic", "--key", "k"],
    );
    assert!(stored.status.success(), "{}", stderr(&stored));

    let listed = stdout(&run(home.path(), at.path(), &["auth", "list"]));
    assert!(
        listed.contains("anthropic"),
        "the listing omits it: {listed}"
    );

    let gone = run(home.path(), at.path(), &["auth", "logout", "anthropic"]);
    assert!(gone.status.success(), "{}", stderr(&gone));
    assert!(
        stdout(&run(home.path(), at.path(), &["auth", "list"])).contains("no credentials"),
        "logout left it behind"
    );
}

/// [R-AUTH-014] no command prints a credential
#[test]
fn nothing_ever_prints_the_credential() {
    let home = home();
    let at = workspace();
    const SECRET: &str = "sk-do-not-print-this";

    run(
        home.path(),
        at.path(),
        &["auth", "login", "anthropic", "--key", SECRET],
    );

    // Every command that touches the store, plus the two that report on
    // providers, because a listing is not the only place a secret could leak.
    for args in [
        vec!["auth", "list"],
        vec!["auth", "login", "anthropic", "--key", SECRET],
        vec!["providers"],
        vec!["doctor"],
        vec!["check"],
    ] {
        let out = run(home.path(), at.path(), &args);
        let said = format!("{}{}", stdout(&out), stderr(&out));
        assert!(
            !said.contains(SECRET),
            "`meow {}` printed the credential: {said}",
            args.join(" ")
        );
    }
}

/// [R-AUTH-001] the store is consulted when the declaration has no key
#[test]
fn the_store_supplies_a_key_the_declaration_does_not() {
    let home = home();
    let at = workspace();

    let before = stdout(&run(home.path(), at.path(), &["doctor"]));
    assert!(
        before.contains("missing for anthropic"),
        "with nothing stored the provider must be reported missing: {before}"
    );

    run(
        home.path(),
        at.path(),
        &["auth", "login", "anthropic", "--key", "k"],
    );

    let after = stdout(&run(home.path(), at.path(), &["doctor"]));
    assert!(
        after.contains("all 1 providers have one"),
        "the store was not consulted: {after}"
    );
}

/// [R-AUTH-001] a declared key wins over the store
#[test]
fn a_declared_key_is_preferred_to_the_stored_one() {
    let home = home();
    let at = tempfile::tempdir().unwrap();
    let config = at.path().join(".meow");
    std::fs::create_dir_all(&config).unwrap();
    std::fs::write(
        config.join("meow.star"),
        r#"
meow.provider(name = "anthropic", kind = "anthropic", api_key = "declared")
meow.model(name = "m", provider = "anthropic", id = "i", context = 1000, max_output = 10)
"#,
    )
    .unwrap();

    // Nothing stored, and the workspace still has a credential: the
    // declaration is the first place looked at.
    let said = stdout(&run(home.path(), at.path(), &["doctor"]));
    assert!(
        said.contains("all 1 providers have one"),
        "a declared key must be enough on its own: {said}"
    );
}

/// [R-AUTH-011] a store anybody can read is refused, naming the mode
#[cfg(unix)]
#[test]
fn a_store_readable_by_others_is_refused() {
    use std::os::unix::fs::PermissionsExt;

    let home = home();
    let at = workspace();
    run(
        home.path(),
        at.path(),
        &["auth", "login", "anthropic", "--key", "k"],
    );

    let path = home.path().join(".meow").join("auth.json");
    std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o644)).unwrap();

    let out = run(home.path(), at.path(), &["auth", "list"]);
    let complained = stderr(&out);

    assert!(!out.status.success(), "a readable store must not be used");
    assert!(
        complained.contains("644") && complained.contains("600"),
        "the complaint must name what the mode is and what it must be: {complained}"
    );
}

/// [R-AUTH-010] the store is written with owner-only permissions
#[cfg(unix)]
#[test]
fn the_store_is_created_private() {
    use std::os::unix::fs::PermissionsExt;

    let home = home();
    let at = workspace();
    run(
        home.path(),
        at.path(),
        &["auth", "login", "anthropic", "--key", "k"],
    );

    let path = home.path().join(".meow").join("auth.json");
    let mode = std::fs::metadata(&path).unwrap().permissions().mode() & 0o777;
    assert_eq!(
        mode, 0o600,
        "the store was created readable by somebody else"
    );
}

/// [R-AUTH-012] a write that is interrupted leaves the old store
#[test]
fn a_failed_write_does_not_truncate_what_was_there() {
    let home = home();
    let at = workspace();
    run(
        home.path(),
        at.path(),
        &["auth", "login", "anthropic", "--key", "first"],
    );

    let path = home.path().join(".meow").join("auth.json");
    let before = std::fs::read_to_string(&path).unwrap();

    // The temporary the writer uses, left behind by a previous death. A write
    // that reuses it must still end with a complete store rather than with
    // whatever this held.
    std::fs::write(path.with_extension("json.new"), "{ truncated").unwrap();

    let out = run(
        home.path(),
        at.path(),
        &["auth", "login", "openai", "--key", "second"],
    );
    assert!(out.status.success(), "{}", stderr(&out));

    let after = std::fs::read_to_string(&path).unwrap();
    assert!(
        serde_json::from_str::<serde_json::Value>(&after).is_ok(),
        "the store is no longer valid JSON: {after}"
    );
    assert!(
        after.contains("anthropic") && after.contains("openai"),
        "the earlier credential was lost:\nbefore {before}\nafter {after}"
    );
}

/// [R-AUTH-002] a provider with no credential names all three places
#[test]
fn a_missing_credential_names_where_it_was_looked_for() {
    let home = home();
    let at = workspace();

    // `meow index build` needs a provider it can build, which is the path
    // that reports a credential it could not find.
    let config = at.path().join(".meow");
    std::fs::write(
        config.join("meow.star"),
        r#"
meow.provider(name = "anthropic", kind = "anthropic")
meow.model(name = "e", provider = "anthropic", id = "i", context = 1000, max_output = 0, kind = "embedding")
meow.index(model = "e")
"#,
    )
    .unwrap();

    let out = run(home.path(), at.path(), &["index", "build"]);
    let complained = stderr(&out);

    assert!(
        complained.contains("api_key")
            && complained.contains("meow auth login")
            && complained.contains("ANTHROPIC_API_KEY"),
        "the complaint must name the declaration, the store, and the variable: {complained}"
    );
}
