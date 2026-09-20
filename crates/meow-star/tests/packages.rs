// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Loading Starlark a workspace did not write.
//!
//! Every test here puts the package in the cache by hand. That is deliberate:
//! `[R-PKG-012]` says a load never fetches, so a load is testable with no
//! network at all, and a test that needed one would be testing the wrong half.
#![allow(clippy::unwrap_used)]

use std::path::Path;

use meow_star::package::{Lock, Pin, hash_tree};
use meow_star::{StarError, Workspace, load};
use tempfile::TempDir;

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

fn write_files(root: &Path, files: &[(&str, &str)]) {
    for (name, body) in files {
        let path = root.join(name);
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, body).unwrap();
    }
}

/// A workspace, and a package put in its cache as if it had been fetched.
///
/// Returns the hash the contents came to, so a test can lock the right one or
/// deliberately lock the wrong one.
fn with_package(config: &[(&str, &str)], package: &[(&str, &str)]) -> (TempDir, String) {
    let dir = tempfile::tempdir().unwrap();
    write_files(&dir.path().join(".meow"), config);

    let staging = dir.path().join("staging");
    write_files(&staging, package);
    let hash = hash_tree(&staging).unwrap();

    let cached = dir
        .path()
        .join(".meow")
        .join(".data")
        .join("pkg")
        .join(&hash);
    std::fs::create_dir_all(cached.parent().unwrap()).unwrap();
    std::fs::rename(&staging, &cached).unwrap();

    (dir, hash)
}

fn lock(dir: &TempDir, name: &str, source: &str, version: &str, hash: &str) {
    let mut lock = Lock::default();
    lock.packages.insert(
        name.to_owned(),
        Pin {
            source: source.to_owned(),
            version: version.to_owned(),
            hash: hash.to_owned(),
        },
    );
    lock.write(&dir.path().join(".meow")).unwrap();
}

fn load_at(dir: &TempDir) -> Result<meow_star::Loaded, StarError> {
    load(&Workspace::at(dir.path()))
}

const DECLARING: &str = r#"
meow.package(name = "acme", source = "https://example.invalid/acme.tar.gz", version = "1.0.0")
load("@acme//models.star", "setup")
"#;

/// [R-PKG-001] a declared, locked, cached package loads
#[test]
fn a_pinned_package_loads() {
    let (dir, hash) = with_package(
        &[("meow.star", &format!("{DECLARING}\nsetup()"))],
        &[("models.star", &format!("def setup():\n    pass\n{PRELUDE}"))],
    );
    lock(
        &dir,
        "acme",
        "https://example.invalid/acme.tar.gz",
        "1.0.0",
        &hash,
    );

    let loaded = load_at(&dir).unwrap();
    assert!(
        loaded.registry.model("fast").is_some(),
        "the package's declarations did not take effect"
    );
}

/// [R-PKG-011] contents that do not hash to what is locked are refused, and
/// nothing is evaluated
///
/// This is the whole point of a lockfile. A cache somebody edited, or a
/// mirror that served something else, must not run.
#[test]
fn contents_that_do_not_match_the_lockfile_are_refused() {
    let (dir, hash) = with_package(
        &[("meow.star", DECLARING)],
        &[("models.star", "def setup():\n    pass")],
    );
    lock(
        &dir,
        "acme",
        "https://example.invalid/acme.tar.gz",
        "1.0.0",
        &hash,
    );

    // Edit the cached package after it was locked.
    let cached = dir
        .path()
        .join(".meow")
        .join(".data")
        .join("pkg")
        .join(&hash)
        .join("models.star");
    std::fs::write(&cached, "def setup():\n    pass\nSOMETHING_ELSE = 1\n").unwrap();

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(
        error.contains("does not match") && error.contains(&hash),
        "the refusal must name the hash that was expected: {error}"
    );
}

/// [R-PKG-012] a declared package that is not locked does not fetch
#[test]
fn a_package_that_is_not_locked_says_to_lock_it() {
    let (dir, _) = with_package(
        &[("meow.star", DECLARING)],
        &[("models.star", "def setup():\n    pass")],
    );
    // No lockfile written.

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(
        error.contains("not locked") && error.contains("meow pkg update"),
        "the refusal must name the command that writes one: {error}"
    );
}

/// [R-PKG-012] a locked package that is not cached says to fetch it
#[test]
fn a_package_that_is_not_cached_says_to_fetch_it() {
    let dir = tempfile::tempdir().unwrap();
    write_files(&dir.path().join(".meow"), &[("meow.star", DECLARING)]);
    lock(
        &dir,
        "acme",
        "https://example.invalid/acme.tar.gz",
        "1.0.0",
        "0000000000000000000000000000000000000000000000000000000000000000",
    );

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(
        error.contains("not in the cache") && error.contains("meow pkg fetch"),
        "the refusal must name the command that downloads it: {error}"
    );
}

/// [R-PKG-010] a source changed in the declaration and not re-locked is caught
#[test]
fn a_source_the_lockfile_disagrees_with_is_refused() {
    let (dir, hash) = with_package(
        &[("meow.star", DECLARING)],
        &[("models.star", "def setup():\n    pass")],
    );
    lock(
        &dir,
        "acme",
        "https://elsewhere.invalid/acme.tar.gz",
        "1.0.0",
        &hash,
    );

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(
        error.contains("elsewhere.invalid") && error.contains("example.invalid"),
        "the refusal must name both sources: {error}"
    );
}

/// [R-PKG-002] two packages with one name are refused, naming both
#[test]
fn two_packages_with_the_same_name_are_refused() {
    let dir = tempfile::tempdir().unwrap();
    write_files(
        &dir.path().join(".meow"),
        &[(
            "meow.star",
            r#"
meow.package(name = "acme", source = "https://a.invalid/x.tar.gz", version = "1")
meow.package(name = "acme", source = "https://b.invalid/x.tar.gz", version = "2")
"#,
        )],
    );

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(
        error.to_lowercase().contains("acme"),
        "the duplicate must be named: {error}"
    );
}

/// [R-PKG-003] a package may not be called `std`
#[test]
fn a_package_may_not_shadow_the_runtime_modules() {
    let dir = tempfile::tempdir().unwrap();
    write_files(
        &dir.path().join(".meow"),
        &[(
            "meow.star",
            r#"meow.package(name = "std", source = "https://a.invalid/x.tar.gz", version = "1")"#,
        )],
    );

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(
        error.contains("may not be called `std`"),
        "the refusal must say why: {error}"
    );
}

/// [R-PKG-030] a package's file can load `@std//` and another of its own files
#[test]
fn a_package_reaches_the_runtime_modules_and_its_own_files() {
    let (dir, hash) = with_package(
        &[(
            "meow.star",
            r#"
meow.package(name = "acme", source = "https://example.invalid/acme.tar.gz", version = "1.0.0")
load("@acme//models.star", "setup")
"#,
        )],
        &[
            (
                "models.star",
                &format!(
                    r#"
load("@std//path", "join")
load("@acme//inner.star", "helper")

def setup():
    # Reached at run time. Calling either of these here, or calling `setup`
    # from `meow.star`, would be refused by `[R-STAR-084]` - which is
    # `[R-PKG-033]` working, and has a test of its own below.
    return join("a", helper())
{PRELUDE}
"#
                ),
            ),
            ("inner.star", "def helper():\n    return \"b\""),
        ],
    );
    lock(
        &dir,
        "acme",
        "https://example.invalid/acme.tar.gz",
        "1.0.0",
        &hash,
    );

    let loaded = load_at(&dir).unwrap();
    assert!(loaded.registry.model("fast").is_some());
}

/// [R-PKG-031] a package cannot climb out of itself
#[test]
fn a_package_path_cannot_climb_out() {
    let (dir, hash) = with_package(
        &[(
            "meow.star",
            r#"
meow.package(name = "acme", source = "https://example.invalid/acme.tar.gz", version = "1.0.0")
load("@acme//../../meow.star", "anything")
"#,
        )],
        &[("models.star", "x = 1")],
    );
    lock(
        &dir,
        "acme",
        "https://example.invalid/acme.tar.gz",
        "1.0.0",
        &hash,
    );

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(
        error.contains("climbs out"),
        "a path leaving the package must be refused: {error}"
    );
}

/// [R-PKG-010] a hash covers where a file is, not only what is in it
#[test]
fn moving_a_file_changes_the_hash() {
    let dir = tempfile::tempdir().unwrap();
    let one = dir.path().join("one");
    let two = dir.path().join("two");
    write_files(&one, &[("a/x.star", "x = 1")]);
    write_files(&two, &[("b/x.star", "x = 1")]);

    assert_ne!(
        hash_tree(&one).unwrap(),
        hash_tree(&two).unwrap(),
        "the same bytes at a different path must hash differently"
    );
}

/// [R-PKG-033] a package runs under the same rules as the workspace's own code
///
/// The declaration phase reaches nothing, for a package as much as for
/// `meow.star`. A package that could call `fs.read` while being loaded would
/// be a package that runs on `meow check`.
#[test]
fn a_package_cannot_reach_the_world_while_declaring() {
    let (dir, hash) = with_package(
        &[(
            "meow.star",
            r#"
meow.package(name = "acme", source = "https://example.invalid/acme.tar.gz", version = "1.0.0")
load("@acme//models.star", "setup")
"#,
        )],
        &[(
            "models.star",
            r#"
load("@std//fs", "read")

_ = read("anything")

def setup():
    pass
"#,
        )],
    );
    lock(
        &dir,
        "acme",
        "https://example.invalid/acme.tar.gz",
        "1.0.0",
        &hash,
    );

    let error = load_at(&dir).unwrap_err().to_string();
    assert!(
        error.contains("not available while .meow/ is being loaded"),
        "a package must be refused the world during declaration: {error}"
    );
}
