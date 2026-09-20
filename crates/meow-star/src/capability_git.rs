// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! `@std//git`: the repository, through the program that owns it.
//!
//! Shelling out to `git` rather than linking a library. `libgit2` would avoid
//! a process per call and would also disagree with the user's `git` about
//! configuration, hooks, credential helpers, and worktrees - and the diff a
//! reviewer sees has to be the diff they would see themselves, or the review
//! is about a different repository.

use starlark::environment::GlobalsBuilder;
use starlark::eval::Evaluator;
use starlark::starlark_module;
use starlark::values::Value as StarValue;
use starlark::values::none::{NoneOr, NoneType};

use crate::capability::run_words;

fn oops(message: impl std::fmt::Display) -> starlark::Error {
    starlark::Error::new_other(anyhow::anyhow!("{message}"))
}

/// Run `git` with these arguments and return what it wrote.
fn git(
    eval: &mut Evaluator<'_, '_, '_>,
    what: &str,
    arguments: &[String],
) -> starlark::Result<String> {
    let mut words = vec!["git".to_owned()];
    words.extend_from_slice(arguments);

    let (code, out, err) = run_words(eval, what, &words, None)?;
    if code != 0 {
        let said = if err.trim().is_empty() { out } else { err };
        return Err(oops(format!(
            "`git {}` exited {code}:\n{}",
            arguments.join(" "),
            said.trim()
        )));
    }
    Ok(out)
}

/// Refuse a value that would be read as an option.
///
/// A branch called `--upload-pack=rm` is not a branch, and `git` would take it
/// as one more flag. Every value a caller supplies goes through here, so a
/// name from a model cannot turn one command into another.
fn plain(what: &str, value: &str) -> starlark::Result<String> {
    if value.starts_with('-') {
        return Err(oops(format!(
            "`{what}` may not begin with `-`, because `git` would read `{value}` as an option"
        )));
    }
    Ok(value.to_owned())
}

/// `@std//git`.
#[starlark_module]
pub(crate) fn git_module(builder: &mut GlobalsBuilder) {
    /// What has changed, as a unified diff.
    ///
    /// Staged by default, because that is what a review of a commit is about
    /// and the alternative is reviewing work somebody is still in the middle
    /// of.
    fn diff<'v>(
        #[starlark(require = named, default = true)] staged: bool,
        #[starlark(require = named, default = NoneOr::None)] revision: NoneOr<String>,
        #[starlark(require = named, default = NoneOr::None)] paths: NoneOr<StarValue<'v>>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        let mut arguments = vec!["--no-pager".to_owned(), "diff".to_owned()];
        if staged {
            arguments.push("--cached".to_owned());
        }
        if let Some(revision) = revision.into_option() {
            arguments.push(plain("revision", &revision)?);
        }
        if let Some(paths) = paths.into_option() {
            arguments.push("--".to_owned());
            arguments.extend(strings(paths, "paths")?);
        }
        git(eval, "git.diff", &arguments)
    }

    /// What is changed, staged, and untracked, one path per line.
    ///
    /// The porcelain format, which is the one `git` promises not to change
    /// between versions. The human format is prettier and is not a contract.
    fn status<'v>(eval: &mut Evaluator<'v, '_, '_>) -> starlark::Result<String> {
        git(
            eval,
            "git.status",
            &["status".to_owned(), "--porcelain=v1".to_owned()],
        )
    }

    /// Recent commits, one per line.
    fn log<'v>(
        #[starlark(require = named, default = 20)] limit: u32,
        #[starlark(require = named, default = NoneOr::None)] revision: NoneOr<String>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        let mut arguments = vec![
            "--no-pager".to_owned(),
            "log".to_owned(),
            format!("-{}", limit.max(1)),
            "--format=%h %s".to_owned(),
        ];
        if let Some(revision) = revision.into_option() {
            arguments.push(plain("revision", &revision)?);
        }
        git(eval, "git.log", &arguments)
    }

    /// One commit, with its message and its diff.
    fn show<'v>(
        #[starlark(require = pos)] revision: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        git(
            eval,
            "git.show",
            &[
                "--no-pager".to_owned(),
                "show".to_owned(),
                plain("revision", &revision)?,
            ],
        )
    }

    /// Which branch is checked out.
    fn branch<'v>(eval: &mut Evaluator<'v, '_, '_>) -> starlark::Result<String> {
        Ok(git(
            eval,
            "git.branch",
            &[
                "rev-parse".to_owned(),
                "--abbrev-ref".to_owned(),
                "HEAD".to_owned(),
            ],
        )?
        .trim()
        .to_owned())
    }

    /// Stage paths.
    fn stage<'v>(
        #[starlark(require = pos)] paths: StarValue<'v>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let paths = strings(paths, "paths")?;
        if paths.is_empty() {
            return Err(oops("`git.stage` was given no paths"));
        }

        let mut arguments = vec!["add".to_owned(), "--".to_owned()];
        arguments.extend(paths);
        git(eval, "git.stage", &arguments)?;
        Ok(NoneType)
    }

    /// Commit what is staged.
    ///
    /// Only what is staged: a commit that also picked up whatever happened to
    /// be in the working tree is a commit nobody reviewed.
    fn commit<'v>(
        #[starlark(require = pos)] message: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        if message.trim().is_empty() {
            return Err(oops("`git.commit` needs a message"));
        }

        // `--message` with the text as its own word, and `--` after it, so a
        // message beginning with a dash is a message rather than one more
        // option. The words never touch a shell, so a newline in it is a
        // newline.
        git(
            eval,
            "git.commit",
            &[
                "commit".to_owned(),
                "--message".to_owned(),
                message,
                "--".to_owned(),
            ],
        )?;
        Ok(git(
            eval,
            "git.commit",
            &[
                "rev-parse".to_owned(),
                "--short".to_owned(),
                "HEAD".to_owned(),
            ],
        )?
        .trim()
        .to_owned())
    }
}

/// Read a list of strings from a Starlark value.
fn strings(value: StarValue<'_>, what: &str) -> starlark::Result<Vec<String>> {
    match value.to_json_value().map_err(oops)? {
        serde_json::Value::Array(items) => items
            .into_iter()
            .map(|item| match item {
                serde_json::Value::String(text) => plain(what, &text),
                other => Err(oops(format!(
                    "`{what}` must hold strings, and carries {other}"
                ))),
            })
            .collect(),
        serde_json::Value::Null => Ok(Vec::new()),
        other => Err(oops(format!("`{what}` must be a list, and is {other}"))),
    }
}
