// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! `meow pkg`, and what a package may reach, through the binary.
//!
//! `fetch.rs` drives the fetching over its own API and `packages.rs` in
//! `meow-star` drives the loading. What neither covers is the command a person
//! types and the wiring behind it.
#![allow(clippy::unwrap_used)]

use std::io::{Read, Write};
use std::path::Path;
use std::process::{Command, Output};
use std::sync::Arc;

use tempfile::TempDir;

fn meow() -> Command {
    Command::new(env!("CARGO_BIN_EXE_meow"))
}

/// Run `meow`, having first agreed to the workspace - `[R-AUTH-030]`.
fn run(dir: &Path, args: &[&str]) -> Output {
    let _ = bare(dir, &["trust"]);
    bare(dir, args)
}

fn bare(dir: &Path, args: &[&str]) -> Output {
    meow()
        .current_dir(dir)
        .env("MEOW_HOME", dir.join(".meow").join(".data").join("home"))
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

/// A server handing out one archive.
struct Serving {
    port: u16,
    stop: Arc<std::sync::atomic::AtomicBool>,
}

impl Serving {
    fn new(body: Vec<u8>) -> Self {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        let stop = Arc::new(std::sync::atomic::AtomicBool::new(false));

        let halt = Arc::clone(&stop);
        std::thread::spawn(move || {
            for stream in listener.incoming() {
                if halt.load(std::sync::atomic::Ordering::Relaxed) {
                    return;
                }
                let Ok(mut stream) = stream else { continue };
                let mut buffer = [0_u8; 4096];
                let _ = stream.read(&mut buffer);
                let head = format!(
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    body.len()
                );
                let _ = stream.write_all(head.as_bytes());
                let _ = stream.write_all(&body);
                let _ = stream.flush();
            }
        });

        Self { port, stop }
    }

    fn url(&self) -> String {
        format!("http://127.0.0.1:{}/acme.tar.gz", self.port)
    }
}

impl Drop for Serving {
    fn drop(&mut self) {
        self.stop.store(true, std::sync::atomic::Ordering::Relaxed);
        let _ = std::net::TcpStream::connect(("127.0.0.1", self.port));
    }
}

fn archive(files: &[(&str, &str)]) -> Vec<u8> {
    let mut builder = tar::Builder::new(Vec::new());
    for (name, body) in files {
        let mut header = tar::Header::new_gnu();
        header.set_size(body.len() as u64);
        header.set_mode(0o644);
        header.set_cksum();
        builder
            .append_data(&mut header, name, body.as_bytes())
            .unwrap();
    }
    let tar = builder.into_inner().unwrap();
    let mut encoder = flate2::write::GzEncoder::new(Vec::new(), flate2::Compression::default());
    encoder.write_all(&tar).unwrap();
    encoder.finish().unwrap()
}

fn workspace(source: &str) -> TempDir {
    let dir = tempfile::tempdir().unwrap();
    let config = dir.path().join(".meow");
    std::fs::create_dir_all(&config).unwrap();
    std::fs::write(config.join("meow.star"), source).unwrap();
    dir
}

const PRELUDE: &str = r#"
meow.provider(name = "anthropic", kind = "anthropic", api_key = "k")
meow.model(name = "fast", provider = "anthropic", id = "i", context = 1000, max_output = 10)
"#;

/// [R-PKG-013] update pins, fetch is a no-op after it, and the package loads
#[test]
fn update_then_load_is_the_whole_story() {
    let server = Serving::new(archive(&[(
        "models.star",
        &format!("def setup():\n    pass\n{PRELUDE}"),
    )]));
    let at = workspace(&format!(
        r#"
meow.package(name = "acme", source = "{}", version = "1.0.0")
load("@acme//models.star", "setup")
"#,
        server.url()
    ));

    // Before `update` there is no lockfile, so loading refuses and says so.
    let before = bare(at.path(), &["check"]);
    assert!(
        stderr(&before).contains("meow pkg update"),
        "an unlocked package must name the command: {}",
        stderr(&before)
    );

    let updated = run(at.path(), &["pkg", "update"]);
    assert!(updated.status.success(), "{}", stderr(&updated));
    assert!(
        at.path().join(".meow").join("meow.lock").exists(),
        "no lockfile was written"
    );

    // Now it loads.
    let checked = run(at.path(), &["check"]);
    assert!(checked.status.success(), "{}", stderr(&checked));

    // And `fetch` has nothing to do.
    let fetched = run(at.path(), &["pkg", "fetch"]);
    assert!(fetched.status.success(), "{}", stderr(&fetched));
    assert!(
        stdout(&fetched).contains("fetched\t0"),
        "a cached package was fetched again: {}",
        stdout(&fetched)
    );
}

/// [R-PKG-020] a workspace that has fetched once loads with no network
#[test]
fn a_fetched_workspace_loads_offline() {
    let server = Serving::new(archive(&[(
        "models.star",
        &format!("def setup():\n    pass\n{PRELUDE}"),
    )]));
    let at = workspace(&format!(
        r#"
meow.package(name = "acme", source = "{}", version = "1.0.0")
load("@acme//models.star", "setup")
"#,
        server.url()
    ));

    assert!(run(at.path(), &["pkg", "update"]).status.success());

    // Nothing is listening from here on.
    drop(server);

    let checked = run(at.path(), &["check"]);
    assert!(
        checked.status.success(),
        "a fetched workspace did not load offline: {}",
        stderr(&checked)
    );
}

/// [R-PKG-013] `list` says what is declared and whether it is pinned
#[test]
fn list_says_what_is_declared_and_what_is_pinned() {
    let server = Serving::new(archive(&[("models.star", PRELUDE)]));
    let at = workspace(&format!(
        r#"meow.package(name = "acme", source = "{}", version = "1.0.0")"#,
        server.url()
    ));

    let before = stdout(&run(at.path(), &["pkg", "list"]));
    assert!(
        before.contains("acme") && before.contains("not pinned"),
        "an unpinned package must say so: {before}"
    );

    run(at.path(), &["pkg", "update"]);

    let after = stdout(&run(at.path(), &["pkg", "list"]));
    assert!(
        after.contains("acme") && !after.contains("not pinned"),
        "a pinned package must show its hash: {after}"
    );
}

/// [R-PKG-032] a package cannot pull in a package the workspace did not declare
///
/// Dependencies are the workspace's to state, so that `meow.lock` is the whole
/// list of what runs. A package that could name its own would make the
/// lockfile a partial answer.
#[test]
fn a_package_cannot_reach_a_package_the_workspace_did_not_declare() {
    let server = Serving::new(archive(&[(
        "models.star",
        // The package tries to load one the workspace never declared.
        "load(\"@other//thing.star\", \"x\")\n\ndef setup():\n    pass\n",
    )]));
    let at = workspace(&format!(
        r#"
meow.package(name = "acme", source = "{}", version = "1.0.0")
load("@acme//models.star", "setup")
"#,
        server.url()
    ));

    assert!(run(at.path(), &["pkg", "update"]).status.success());

    let checked = run(at.path(), &["check"]);
    let complained = stderr(&checked);

    assert!(!checked.status.success(), "the undeclared package loaded");
    assert!(
        complained.contains("other") && complained.contains("does not declare"),
        "the refusal must name the package the workspace never declared: {complained}"
    );
}

/// [R-PKG-013] `pkg` in a workspace with no packages says so rather than
/// failing
#[test]
fn a_workspace_with_no_packages_says_so() {
    let at = workspace(PRELUDE);

    for args in [
        vec!["pkg", "list"],
        vec!["pkg", "update"],
        vec!["pkg", "fetch"],
    ] {
        let out = run(at.path(), &args);
        assert!(
            out.status.success(),
            "`meow {}` failed on a workspace with no packages: {}",
            args.join(" "),
            stderr(&out)
        );
        assert!(
            stdout(&out).contains("no packages"),
            "`meow {}` said nothing useful: {}",
            args.join(" "),
            stdout(&out)
        );
    }
}
