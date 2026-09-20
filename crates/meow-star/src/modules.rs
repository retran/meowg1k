// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The `@std//` module table.
//!
//! One table, built once, and every consumer takes a module from it. That is
//! `[R-STAR-010]`, and it is the fix for the defect that cost v0.2.x the most:
//! `ctx_run.go` and `module_llm.go` each assembled a context by hand, they
//! drifted on UI nesting depth, and a module added to one was silently missing
//! from tools running inside an agent loop. There is nowhere here for a second
//! copy to live, which is also why `[R-STAR-011]` holds without a mechanism of
//! its own.
//!
//! Every module resolves in both phases. What differs is what its builtins do:
//! a call made while `.meow/` is being evaluated fails with
//! [`StarError::ModuleUnavailable`], except in `env`, which `[R-STAR-084]`
//! carves out because credentials are resolved in a declaration file.
//!
//! [`StarError::ModuleUnavailable`]: crate::error::StarError::ModuleUnavailable

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use starlark::environment::{FrozenModule, Globals, GlobalsBuilder, Module};
use starlark::eval::Evaluator;
use starlark::starlark_module;
use starlark::values::Value as StarValue;
use starlark::values::none::NoneOr;

use crate::error::{Result, StarError};
use crate::run::running;

/// The modules that exist, in the order `meow doctor` should list them.
pub const NAMES: &[&str] = &[
    "env", "fs", "git", "json", "path", "re", "search", "shell", "text", "time",
];

/// Every `@std//` module, built once per load.
#[derive(Debug)]
pub struct Modules(BTreeMap<String, FrozenModule>);

impl Modules {
    /// Build the table.
    ///
    /// # Errors
    ///
    /// [`StarError::Starlark`] if a module will not freeze, which would be a
    /// defect in this file rather than in anything a user wrote.
    pub fn build() -> Result<Self> {
        let mut table = BTreeMap::new();
        table.insert("env".to_owned(), freeze(env_module)?);
        table.insert("fs".to_owned(), freeze(crate::capability::fs_module)?);
        table.insert("git".to_owned(), freeze(crate::capability_git::git_module)?);
        table.insert("json".to_owned(), freeze(json_module)?);
        table.insert("path".to_owned(), freeze(path_module)?);
        table.insert("re".to_owned(), freeze(re_module)?);
        table.insert("search".to_owned(), freeze(search_module)?);
        table.insert("shell".to_owned(), freeze(crate::capability::shell_module)?);
        table.insert("text".to_owned(), freeze(text_module)?);
        table.insert("time".to_owned(), freeze(time_module)?);
        Ok(Self(table))
    }

    /// One module, if it exists.
    pub fn get(&self, name: &str) -> Option<&FrozenModule> {
        self.0.get(name)
    }

    /// Every name in the table.
    pub fn names(&self) -> impl Iterator<Item = &String> {
        self.0.keys()
    }
}

/// Turn a builder into a module a `load` can return.
fn freeze(build: impl Fn(&mut GlobalsBuilder)) -> Result<FrozenModule> {
    let globals: Globals = GlobalsBuilder::new().with(build).build();
    Module::with_temp_heap(|module| {
        module.frozen_heap().add_reference(globals.heap());
        for (name, value) in globals.iter() {
            module.set(name, value.to_value());
        }
        module
            .freeze()
            .map_err(|e| StarError::Starlark(format!("{e:?}")))
    })
}

fn oops(message: impl std::fmt::Display) -> starlark::Error {
    starlark::Error::new_other(anyhow::anyhow!("{message}"))
}

/// `@std//env`: the environment, readable in both phases.
#[starlark_module]
fn env_module(builder: &mut GlobalsBuilder) {
    /// Read a variable, or a default when it is not set.
    fn get(
        #[starlark(require = pos)] name: String,
        #[starlark(require = pos)] default: Option<String>,
    ) -> starlark::Result<NoneOr<String>> {
        Ok(match std::env::var(&name).ok().or(default) {
            Some(value) => NoneOr::Other(value),
            None => NoneOr::None,
        })
    }

    /// Read a variable that has to be set.
    ///
    /// The error names the variable, because "missing credentials" without a
    /// name is a support ticket.
    fn require(#[starlark(require = pos)] name: String) -> starlark::Result<String> {
        std::env::var(&name).map_err(|_| oops(format!("`{name}` is not set in the environment")))
    }
}

/// `@std//json`: parse and encode.
#[starlark_module]
fn json_module(builder: &mut GlobalsBuilder) {
    /// Parse JSON text into Starlark values.
    fn parse<'v>(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "json.parse")?;
        let value: serde_json::Value = serde_json::from_str(&text).map_err(oops)?;
        Ok(eval.heap().alloc(value))
    }

    /// Encode Starlark values as JSON text.
    fn encode<'v>(
        #[starlark(require = pos)] value: StarValue<'v>,
        #[starlark(require = named, default = false)] indent: bool,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "json.encode")?;
        let value = value.to_json_value().map_err(oops)?;
        if indent {
            serde_json::to_string_pretty(&value).map_err(oops)
        } else {
            serde_json::to_string(&value).map_err(oops)
        }
    }
}

/// `@std//path`: paths as text, with no disk behind them.
#[starlark_module]
fn path_module(builder: &mut GlobalsBuilder) {
    /// Join segments.
    fn join<'v>(
        #[starlark(args)] parts: starlark::values::tuple::UnpackTuple<String>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "path.join")?;
        let mut out = PathBuf::new();
        for part in parts.items {
            out.push(part);
        }
        Ok(out.to_string_lossy().into_owned())
    }

    /// Everything before the last separator.
    fn dir<'v>(
        #[starlark(require = pos)] path: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "path.dir")?;
        Ok(Path::new(&path)
            .parent()
            .map(|p| p.to_string_lossy().into_owned())
            .unwrap_or_default())
    }

    /// Everything after the last separator.
    fn base<'v>(
        #[starlark(require = pos)] path: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "path.base")?;
        Ok(Path::new(&path)
            .file_name()
            .map(|p| p.to_string_lossy().into_owned())
            .unwrap_or_default())
    }

    /// The extension, without its dot.
    fn ext<'v>(
        #[starlark(require = pos)] path: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "path.ext")?;
        Ok(Path::new(&path)
            .extension()
            .map(|p| p.to_string_lossy().into_owned())
            .unwrap_or_default())
    }

    /// A path relative to a base.
    ///
    /// Purely lexical, like the rest of this module: it does not ask the disk
    /// what anything resolves to, so it behaves the same whether or not the
    /// file exists.
    fn rel<'v>(
        #[starlark(require = pos)] path: String,
        #[starlark(require = pos)] base: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "path.rel")?;
        Ok(Path::new(&path)
            .strip_prefix(Path::new(&base))
            .map(|p| p.to_string_lossy().into_owned())
            .unwrap_or(path))
    }

    /// A path against the workspace root.
    fn abs<'v>(
        #[starlark(require = pos)] path: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        let state = running(eval, "path.abs")?;
        let given = Path::new(&path);
        let out = if given.is_absolute() {
            given.to_path_buf()
        } else {
            state.runtime.workspace().root().join(given)
        };
        Ok(out.to_string_lossy().into_owned())
    }
}

/// `@std//search`: asking the index a question.
#[starlark_module]
fn search_module(builder: &mut GlobalsBuilder) {
    /// Rank the workspace against a question, by meaning.
    ///
    /// Returns a list of structs carrying the path, the line range, the text,
    /// and the score, so a result can be cited rather than only read.
    fn code<'v>(
        #[starlark(require = pos)] query: String,
        #[starlark(require = named, default = 10)] limit: u32,
        #[starlark(require = named)] paths: Option<StarValue<'v>>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        let state = running(eval, "search.code")?;

        let paths = match paths {
            None => Vec::new(),
            Some(value) if value.is_none() => Vec::new(),
            Some(value) => match value.to_json_value().map_err(oops)? {
                serde_json::Value::Array(items) => items
                    .into_iter()
                    .filter_map(|item| item.as_str().map(str::to_owned))
                    .collect(),
                other => return Err(oops(format!("`paths` must be a list, and is {other}"))),
            },
        };

        // `[R-STAR-081]`: the call blocks this thread. The index reads a file
        // and may reach a provider, and neither is something Starlark can
        // wait on itself.
        let found = state
            .runtime
            .search()
            .code(&query, limit as usize, &paths)
            .map_err(oops)?;

        let heap = eval.heap();
        let results: Vec<StarValue<'v>> = found
            .into_iter()
            .map(|hit| {
                heap.alloc(starlark::values::structs::AllocStruct([
                    ("path", heap.alloc(hit.path)),
                    ("first_line", heap.alloc(hit.first_line as u32)),
                    ("last_line", heap.alloc(hit.last_line as u32)),
                    ("text", heap.alloc(hit.text)),
                    ("score", heap.alloc(f64::from(hit.score))),
                ]))
            })
            .collect();

        Ok(heap.alloc(results))
    }
}

/// `@std//re`: regular expressions.
///
/// Every call compiles its pattern, which costs microseconds and buys the
/// property that a bad pattern is reported at the call that wrote it rather
/// than at a load that mentioned it.
#[starlark_module]
fn re_module(builder: &mut GlobalsBuilder) {
    /// The first match, as a list of groups, or `None`.
    ///
    /// Group 0 is the whole match and the rest follow in the order the pattern
    /// opens them. A group that took part in no match is `None`, which is the
    /// only way to tell "matched empty" from "did not match" - `[R-STAR-013]`.
    fn r#match<'v>(
        #[starlark(require = pos)] pattern: String,
        #[starlark(require = pos)] subject: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "re.match")?;
        let re = compile(&pattern)?;
        let heap = eval.heap();
        match re.captures(&subject) {
            None => Ok(StarValue::new_none()),
            Some(caps) => Ok(heap.alloc(groups(&heap, &caps))),
        }
    }

    /// Every match, each as its own list of groups.
    fn find_all<'v>(
        #[starlark(require = pos)] pattern: String,
        #[starlark(require = pos)] subject: String,
        #[starlark(require = named, default = 0)] limit: u32,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "re.find_all")?;
        let re = compile(&pattern)?;
        let heap = eval.heap();
        // A pattern that can match empty makes `captures_iter` unbounded in
        // the number of matches it reports for a long subject, so `limit`
        // exists to put a ceiling on it. Zero means no ceiling, which is what
        // a caller who has not thought about it wants.
        let cap = if limit == 0 {
            usize::MAX
        } else {
            limit as usize
        };
        let found: Vec<StarValue<'v>> = re
            .captures_iter(&subject)
            .take(cap)
            .map(|caps| heap.alloc(groups(&heap, &caps)))
            .collect();
        Ok(heap.alloc(found))
    }

    /// Replace every match.
    ///
    /// `$1` and `${name}` in the replacement refer to groups, per the `regex`
    /// crate's syntax. A `$` that means itself is written `$$`.
    fn replace<'v>(
        #[starlark(require = pos)] pattern: String,
        #[starlark(require = pos)] subject: String,
        #[starlark(require = pos)] replacement: String,
        #[starlark(require = named, default = false)] first_only: bool,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "re.replace")?;
        let re = compile(&pattern)?;
        Ok(if first_only {
            re.replace(&subject, replacement.as_str()).into_owned()
        } else {
            re.replace_all(&subject, replacement.as_str()).into_owned()
        })
    }

    /// Split on every match.
    fn split<'v>(
        #[starlark(require = pos)] pattern: String,
        #[starlark(require = pos)] subject: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "re.split")?;
        let re = compile(&pattern)?;
        let heap = eval.heap();
        let parts: Vec<StarValue<'v>> = re.split(&subject).map(|part| heap.alloc(part)).collect();
        Ok(heap.alloc(parts))
    }
}

/// Compile a pattern, saying what was wrong with it rather than that something
/// was.
fn compile(pattern: &str) -> starlark::Result<regex::Regex> {
    regex::Regex::new(pattern)
        .map_err(|error| oops(format!("`{pattern}` is not a regular expression: {error}")))
}

/// One match's groups, with a group that did not participate as `None`.
fn groups<'v>(heap: &starlark::values::Heap<'v>, caps: &regex::Captures<'_>) -> Vec<StarValue<'v>> {
    caps.iter()
        .map(|group| match group {
            Some(m) => heap.alloc(m.as_str()),
            None => StarValue::new_none(),
        })
        .collect()
}

/// `@std//time`: one scale, and it is seconds in UTC.
///
/// `[R-STAR-014]`. A handler that measures a duration gets the same number on
/// every machine, and a zone enters only at `format`, where a human is about
/// to read the result.
#[starlark_module]
fn time_module(builder: &mut GlobalsBuilder) {
    /// Now, in seconds since the Unix epoch.
    fn now<'v>(eval: &mut Evaluator<'v, '_, '_>) -> starlark::Result<i64> {
        running(eval, "time.now")?;
        Ok(jiff::Timestamp::now().as_second())
    }

    /// Read an RFC 3339 timestamp, in seconds since the epoch.
    fn parse<'v>(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<i64> {
        running(eval, "time.parse")?;
        let stamp: jiff::Timestamp = text
            .parse()
            .map_err(|error| oops(format!("`{text}` is not an RFC 3339 timestamp: {error}")))?;
        Ok(stamp.as_second())
    }

    /// Write seconds since the epoch as text.
    ///
    /// The default is RFC 3339 in UTC, which is what another program should
    /// be given. `layout` takes `strftime` fields for the case where a person
    /// is going to read it.
    fn format<'v>(
        #[starlark(require = pos)] seconds: i64,
        #[starlark(require = named, default = NoneOr::None)] layout: NoneOr<String>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "time.format")?;
        let stamp = jiff::Timestamp::from_second(seconds)
            .map_err(|error| oops(format!("{seconds} is not a timestamp: {error}")))?;
        match layout.into_option() {
            None => Ok(stamp.to_string()),
            Some(layout) => jiff::fmt::strtime::format(&layout, stamp)
                .map_err(|error| oops(format!("`{layout}` is not a layout: {error}"))),
        }
    }

    /// How many seconds have passed since an instant.
    ///
    /// Negative if the instant is in the future, which is a fact about the
    /// argument rather than an error.
    fn since<'v>(
        #[starlark(require = pos)] seconds: i64,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<i64> {
        running(eval, "time.since")?;
        Ok(jiff::Timestamp::now().as_second().saturating_sub(seconds))
    }
}

/// `@std//text`: shaping text for a terminal or a prompt.
#[starlark_module]
fn text_module(builder: &mut GlobalsBuilder) {
    /// Wrap to a width, breaking between words.
    fn wrap<'v>(
        #[starlark(require = pos)] text: String,
        #[starlark(require = pos)] width: u32,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "text.wrap")?;
        if width == 0 {
            return Err(oops("`width` must be at least 1"));
        }
        Ok(wrap_to(&text, width as usize))
    }

    /// Remove the indentation every line shares.
    fn dedent<'v>(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "text.dedent")?;
        Ok(dedent_all(&text))
    }

    /// Cut to a length, marking that something was cut.
    fn truncate<'v>(
        #[starlark(require = pos)] text: String,
        #[starlark(require = pos)] max: u32,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "text.truncate")?;
        let max = max as usize;
        if text.chars().count() <= max {
            return Ok(text);
        }
        // The marker counts towards the limit, so the result is never longer
        // than what was asked for. A truncation that overruns its own bound is
        // how a prompt ends up one token over a context window.
        let keep = max.saturating_sub(3);
        let head: String = text.chars().take(keep).collect();
        Ok(format!("{head}..."))
    }

    /// About how many tokens a model will see.
    ///
    /// An estimate, and the same one the engine's compaction uses, so a
    /// handler that budgets a prompt against this number and an engine that
    /// decides to compact cannot disagree about how big the prompt is.
    fn tokens<'v>(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<u32> {
        running(eval, "text.tokens")?;
        Ok(u32::try_from(text.chars().count().div_ceil(4)).unwrap_or(u32::MAX))
    }
}

fn wrap_to(text: &str, width: usize) -> String {
    let mut out = String::with_capacity(text.len());
    for (i, line) in text.lines().enumerate() {
        if i > 0 {
            out.push('\n');
        }
        let mut column = 0;
        for (j, word) in line.split_whitespace().enumerate() {
            let len = word.chars().count();
            if j > 0 && column + 1 + len > width {
                out.push('\n');
                column = 0;
            } else if j > 0 {
                out.push(' ');
                column += 1;
            }
            out.push_str(word);
            column += len;
        }
    }
    out
}

fn dedent_all(text: &str) -> String {
    let indent = text
        .lines()
        .filter(|line| !line.trim().is_empty())
        .map(|line| line.len() - line.trim_start().len())
        .min()
        .unwrap_or(0);

    text.lines()
        .map(|line| {
            if line.len() >= indent {
                &line[indent..]
            } else {
                line.trim_start()
            }
        })
        .collect::<Vec<_>>()
        .join("\n")
}
