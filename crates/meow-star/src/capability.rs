// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The modules that reach outside the process.
//!
//! `@std//fs` and `@std//shell` do what a handler cannot do for itself, and
//! everything here is confined to the workspace.
//!
//! The confinement is not policy. `docs/spec/policy.md` records that policy
//! governs what a model decided, and a handler is code the workspace's own
//! author wrote. It is there because a path that escapes the workspace is a
//! mistake whichever of them made it, and the cheapest place to notice is
//! before the call.

use std::path::{Path, PathBuf};

use starlark::environment::GlobalsBuilder;
use starlark::eval::Evaluator;
use starlark::starlark_module;
use starlark::values::Value as StarValue;
use starlark::values::none::{NoneOr, NoneType};

use crate::run::running;

/// How much of a file may be read at once, unless a caller says otherwise.
///
/// A handler that reads a hundred-megabyte log into a Starlark string has
/// already lost; a limit it can raise deliberately is kinder than a machine
/// that stops responding.
const DEFAULT_MAX_BYTES: u32 = 4 * 1024 * 1024;

/// How long a command may run before it is killed.
const DEFAULT_TIMEOUT_SECS: u32 = 120;

fn oops(message: impl std::fmt::Display) -> starlark::Error {
    starlark::Error::new_other(anyhow::anyhow!("{message}"))
}

/// Resolve a path against the workspace, refusing one that leaves it.
///
/// Lexically, before touching the disk: a `..` that happens to land back
/// inside is still a mistake worth reporting, and resolving first would let a
/// symbolic link decide where the boundary is.
fn inside(root: &Path, given: &str) -> starlark::Result<PathBuf> {
    let path = Path::new(given);
    // `has_root` rather than `is_absolute`, because on Windows `/etc/hosts`
    // is neither absolute nor workspace-relative: it has no drive letter, so
    // `is_absolute` is false and joining it to the root would quietly produce
    // a path inside the workspace that the caller never asked for.
    if path.has_root() {
        let resolved = path.canonicalize().unwrap_or_else(|_| path.to_path_buf());
        if !resolved.starts_with(root) {
            return Err(oops(format!("`{given}` is outside the workspace")));
        }
        return Ok(resolved);
    }
    if path.components().any(|c| c.as_os_str() == "..") {
        return Err(oops(format!("`{given}` climbs out of the workspace")));
    }
    Ok(root.join(path))
}

/// `@std//fs`: files, inside the workspace.
#[starlark_module]
pub(crate) fn fs_module(builder: &mut GlobalsBuilder) {
    /// Read a file as text.
    fn read<'v>(
        #[starlark(require = pos)] path: String,
        #[starlark(require = named, default = NoneOr::None)] max_bytes: NoneOr<u32>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        let state = running(eval, "fs.read")?;
        let file = inside(state.runtime.workspace().root(), &path)?;
        let limit = u64::from(max_bytes.into_option().unwrap_or(DEFAULT_MAX_BYTES));

        let size = std::fs::metadata(&file)
            .map_err(|e| oops(format!("could not read `{path}`: {e}")))?
            .len();
        if size > limit {
            return Err(oops(format!(
                "`{path}` is {size} bytes and the limit is {limit}; pass `max_bytes` to raise it"
            )));
        }

        std::fs::read_to_string(&file).map_err(|e| oops(format!("could not read `{path}`: {e}")))
    }

    /// Write a file, replacing what was there.
    fn write<'v>(
        #[starlark(require = pos)] path: String,
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let state = running(eval, "fs.write")?;
        let file = inside(state.runtime.workspace().root(), &path)?;

        if let Some(parent) = file.parent() {
            std::fs::create_dir_all(parent).map_err(oops)?;
        }
        std::fs::write(&file, text).map_err(|e| oops(format!("could not write `{path}`: {e}")))?;
        Ok(NoneType)
    }

    /// Add to the end of a file, creating it when it is not there.
    fn append<'v>(
        #[starlark(require = pos)] path: String,
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        use std::io::Write;

        let state = running(eval, "fs.append")?;
        let file = inside(state.runtime.workspace().root(), &path)?;
        if let Some(parent) = file.parent() {
            std::fs::create_dir_all(parent).map_err(oops)?;
        }

        let mut handle = std::fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(&file)
            .map_err(|e| oops(format!("could not open `{path}`: {e}")))?;
        handle.write_all(text.as_bytes()).map_err(oops)?;
        Ok(NoneType)
    }

    /// Whether a path is there.
    fn exists<'v>(
        #[starlark(require = pos)] path: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<bool> {
        let state = running(eval, "fs.exists")?;
        // A path that leaves the workspace does not exist as far as this is
        // concerned, which is a truthful answer and not a refusal: asking
        // whether something is there should not be able to probe the machine.
        let Ok(file) = inside(state.runtime.workspace().root(), &path) else {
            return Ok(false);
        };
        Ok(file.exists())
    }

    /// Every path matching a glob, relative to the workspace root.
    fn glob<'v>(
        #[starlark(require = pos)] pattern: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<Vec<String>> {
        let state = running(eval, "fs.glob")?;
        let root = state.runtime.workspace().root().to_path_buf();

        let matcher = globset::Glob::new(&pattern)
            .map_err(|e| oops(format!("`{pattern}` is not a valid glob: {e}")))?
            .compile_matcher();

        let mut out = Vec::new();
        let mut stack = vec![root.clone()];
        while let Some(dir) = stack.pop() {
            let Ok(entries) = std::fs::read_dir(&dir) else {
                continue;
            };
            for entry in entries.flatten() {
                let path = entry.path();
                // The store is never walked. Indexing it or globbing it both
                // end in the same place: a workspace describing itself.
                if path.file_name().is_some_and(|n| n == ".data") {
                    continue;
                }
                if path.is_dir() {
                    stack.push(path);
                    continue;
                }
                let Ok(relative) = path.strip_prefix(&root) else {
                    continue;
                };
                let relative = relative.to_string_lossy().replace('\\', "/");
                if matcher.is_match(relative.as_str()) {
                    out.push(relative);
                }
            }
        }

        // A stable order, so a handler that writes its results to a file
        // produces the same file twice.
        out.sort();
        Ok(out)
    }

    /// Make a directory, and its parents.
    fn mkdir<'v>(
        #[starlark(require = pos)] path: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let state = running(eval, "fs.mkdir")?;
        let dir = inside(state.runtime.workspace().root(), &path)?;
        std::fs::create_dir_all(&dir)
            .map_err(|e| oops(format!("could not create `{path}`: {e}")))?;
        Ok(NoneType)
    }

    /// Remove a file, or a directory and everything under it.
    fn remove<'v>(
        #[starlark(require = pos)] path: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let state = running(eval, "fs.remove")?;
        let target = inside(state.runtime.workspace().root(), &path)?;

        // The workspace root itself, and the store, are not removable by a
        // path. A handler that means to delete its own project should say so
        // with a shell command, where the intent is unmistakable.
        if target == state.runtime.workspace().root()
            || target.starts_with(state.runtime.workspace().data_dir())
        {
            return Err(oops(format!(
                "`{path}` is not something `fs.remove` will delete"
            )));
        }

        let result = if target.is_dir() {
            std::fs::remove_dir_all(&target)
        } else {
            std::fs::remove_file(&target)
        };
        result.map_err(|e| oops(format!("could not remove `{path}`: {e}")))?;
        Ok(NoneType)
    }
}

/// `@std//shell`: running something else.
#[starlark_module]
pub(crate) fn shell_module(builder: &mut GlobalsBuilder) {
    /// Run a command and return what it wrote, failing if it fails.
    fn run<'v>(
        #[starlark(require = pos)] command: StarValue<'v>,
        #[starlark(require = named, default = NoneOr::None)] timeout_secs: NoneOr<u32>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        let (code, out, err) = capture_inner(eval, "shell.run", command, timeout_secs)?;
        if code != 0 {
            let said = if err.trim().is_empty() { out } else { err };
            return Err(oops(format!("the command exited {code}:\n{said}")));
        }
        Ok(out)
    }

    /// Run a command and return its status and output, whatever happened.
    ///
    /// The one a handler wants when a non-zero exit is an answer rather than a
    /// failure, which is most of what a linter or a test runner is for.
    fn capture<'v>(
        #[starlark(require = pos)] command: StarValue<'v>,
        #[starlark(require = named, default = NoneOr::None)] timeout_secs: NoneOr<u32>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        let (code, out, err) = capture_inner(eval, "shell.capture", command, timeout_secs)?;
        let heap = eval.heap();
        Ok(heap.alloc(starlark::values::structs::AllocStruct([
            ("code", heap.alloc(code)),
            ("stdout", heap.alloc(out)),
            ("stderr", heap.alloc(err)),
            ("ok", heap.alloc(code == 0)),
        ])))
    }

    /// Where a program is, or nothing when it is not on the path.
    fn which<'v>(
        #[starlark(require = pos)] program: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneOr<String>> {
        running(eval, "shell.which")?;

        let Some(path) = std::env::var_os("PATH") else {
            return Ok(NoneOr::None);
        };
        for dir in std::env::split_paths(&path) {
            let candidate = dir.join(&program);
            if candidate.is_file() {
                return Ok(NoneOr::Other(candidate.to_string_lossy().into_owned()));
            }
            // Windows puts the extension on, and a handler asking for `git`
            // means the program rather than the exact filename.
            for extension in ["exe", "cmd", "bat"] {
                let candidate = dir.join(format!("{program}.{extension}"));
                if candidate.is_file() {
                    return Ok(NoneOr::Other(candidate.to_string_lossy().into_owned()));
                }
            }
        }
        Ok(NoneOr::None)
    }
}

/// Run a command given as words already split.
///
/// The one `@std//git` uses: it builds its own argument list and has no
/// Starlark value to unpack.
pub(crate) fn run_words(
    eval: &mut Evaluator<'_, '_, '_>,
    what: &str,
    words: &[String],
    timeout_secs: Option<u32>,
) -> starlark::Result<(i32, String, String)> {
    let state = running(eval, what)?;
    let Some((program, arguments)) = words.split_first() else {
        return Err(oops(format!("`{what}` was given no command")));
    };
    spawn(
        state,
        program.clone(),
        arguments.to_vec(),
        timeout_secs.unwrap_or(DEFAULT_TIMEOUT_SECS),
    )
}

/// Run a command in the workspace and collect what it produced.
///
/// The command is a list of words, never a string for a shell to split.
/// `[R-POLICY-006]` judges a command line, and a string handed to `sh -c` is a
/// command line the policy never saw the real shape of.
fn capture_inner(
    eval: &mut Evaluator<'_, '_, '_>,
    what: &str,
    command: StarValue<'_>,
    timeout_secs: NoneOr<u32>,
) -> starlark::Result<(i32, String, String)> {
    let state = running(eval, what)?;

    let words: Vec<String> = match command.to_json_value().map_err(oops)? {
        serde_json::Value::Array(items) => items
            .into_iter()
            .map(|item| match item {
                serde_json::Value::String(word) => Ok(word),
                other => Err(oops(format!("a word must be a string, and one is {other}"))),
            })
            .collect::<starlark::Result<Vec<String>>>()?,
        other => {
            return Err(oops(format!(
                "`{what}` takes a list of words, and was given {other}. A single string would be \
                 split by a shell, and a policy cannot judge a command line it never saw the \
                 shape of."
            )));
        }
    };

    let Some((program, arguments)) = words.split_first() else {
        return Err(oops(format!("`{what}` was given no command")));
    };

    spawn(
        state,
        program.clone(),
        arguments.to_vec(),
        timeout_secs.into_option().unwrap_or(DEFAULT_TIMEOUT_SECS),
    )
}

/// Start a program in the workspace and wait for it.
///
/// `[R-STAR-081]`: the thread blocks. A command is the clearest case for it:
/// there is nothing useful a handler could do with a future here.
fn spawn(
    state: &crate::state::Running,
    program: String,
    arguments: Vec<String>,
    timeout_secs: u32,
) -> starlark::Result<(i32, String, String)> {
    let timeout = std::time::Duration::from_secs(u64::from(timeout_secs));
    let root = state.runtime.workspace().root().to_path_buf();
    let cancel = state.runtime.cancel_token().clone();

    state.runtime.block_on(async move {
        let child = tokio::process::Command::new(&program)
            .args(&arguments)
            .current_dir(&root)
            .stdin(std::process::Stdio::null())
            .stdout(std::process::Stdio::piped())
            .stderr(std::process::Stdio::piped())
            .kill_on_drop(true)
            .spawn()
            .map_err(|e| oops(format!("could not run `{program}`: {e}")))?;

        let output = tokio::select! {
            () = cancel.cancelled() => return Err(oops("the run was cancelled")),
            () = tokio::time::sleep(timeout) => {
                return Err(oops(format!(
                    "`{program}` did not finish in {} seconds",
                    timeout.as_secs()
                )));
            }
            output = child.wait_with_output() => output,
        };

        let output = output.map_err(|e| oops(format!("`{program}` failed: {e}")))?;
        Ok((
            output.status.code().unwrap_or(-1),
            String::from_utf8_lossy(&output.stdout).into_owned(),
            String::from_utf8_lossy(&output.stderr).into_owned(),
        ))
    })
}
