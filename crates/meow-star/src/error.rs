// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What can be wrong with a workspace.

use std::path::PathBuf;

/// Everything loading a workspace can fail with.
#[derive(Debug, thiserror::Error)]
pub enum StarError {
    /// There is no workspace here or above here.
    ///
    /// `[R-STAR-002]`: the directories that were searched are named, so a user
    /// can see whether they are simply in the wrong place.
    #[error(
        "no .meow/meow.star found. Searched:\n{}\nRun `meow init` to create one.",
        .searched.iter().map(|p| format!("  {}", p.display())).collect::<Vec<_>>().join("\n")
    )]
    NoWorkspace {
        /// Every directory that was looked in, nearest first.
        searched: Vec<PathBuf>,
    },

    /// A `load` named something that is not there.
    #[error("{message}")]
    Load {
        /// What went wrong, with the alternatives when there are any.
        message: String,
    },

    /// Files load each other in a circle.
    ///
    /// `[R-STAR-006]`: the cycle is listed in order, because "circular import"
    /// without the ring is a puzzle rather than a message.
    #[error("import cycle: {}", .cycle.join(" -> "))]
    Cycle {
        /// The ring, in the order it was walked.
        cycle: Vec<String>,
    },

    /// Something was declared twice.
    ///
    /// `[R-STAR-031]`: both sites are named. Knowing there is a duplicate
    /// without knowing where the other one is leaves you grepping.
    #[error("{kind} `{name}` is declared twice: {first} and {second}")]
    Duplicate {
        /// What kind of thing.
        kind: &'static str,
        /// Its name.
        name: String,
        /// Where it was first declared.
        first: String,
        /// Where it was declared again.
        second: String,
    },

    /// A declaration refers to something that does not exist.
    #[error("{kind} `{name}` is not declared{}", suggestion(.closest.as_deref()))]
    Unknown {
        /// What kind of thing was expected.
        kind: &'static str,
        /// What was named.
        name: String,
        /// The nearest declared name, when one is close.
        closest: Option<String>,
    },

    /// A model is the wrong kind for what named it.
    ///
    /// `[R-STAR-034]`: both the model and the kind it is, because the fix is
    /// either to declare another model or to change this one's kind and
    /// neither is obvious from the name alone.
    #[error(
        "{used_by} needs {} model and `{name}` is {} model",
        article(.wanted), article(.is)
    )]
    WrongKind {
        /// Which model.
        name: String,
        /// What was needed.
        wanted: &'static str,
        /// What it is.
        is: &'static str,
        /// What named it.
        used_by: String,
    },

    /// A command would shadow a built-in.
    ///
    /// `[R-STAR-033]`: refused rather than shadowed in either direction, so
    /// `meow session` never becomes ambiguous.
    #[error("`{name}` is a built-in command and cannot be redeclared")]
    Reserved {
        /// The name that collides.
        name: String,
    },

    /// A declaration was made somewhere it is not allowed.
    #[error("{what} can only be called while .meow/ is being loaded")]
    NotDeclaring {
        /// Which call.
        what: String,
    },

    /// A module was used during declaration.
    ///
    /// `[R-STAR-084]`.
    #[error("`{module}` is not available while .meow/ is being loaded")]
    ModuleUnavailable {
        /// Which module.
        module: String,
    },

    /// The Starlark evaluator said no.
    ///
    /// `starlark-rust` renders its own diagnostic with a call stack and a
    /// source span, which is why `[R-STAR-090]` needs nothing further here.
    #[error("{0}")]
    Starlark(String),

    /// A file could not be read.
    #[error("could not read {path}: {source}")]
    Io {
        /// Which file.
        path: PathBuf,
        /// What the operating system said.
        #[source]
        source: std::io::Error,
    },
}

/// `a` or `an`, so a message about a kind reads like a sentence.
fn article(word: &str) -> String {
    let first = word.chars().next().unwrap_or('x');
    let vowel = matches!(first, 'a' | 'e' | 'i' | 'o' | 'u');
    format!("{} {word}", if vowel { "an" } else { "a" })
}

/// The "did you mean" half of `[R-STAR-091]`.
fn suggestion(closest: Option<&str>) -> String {
    closest.map_or_else(String::new, |c| format!(". Did you mean `{c}`?"))
}

/// The closest declared name, when one is within a small edit distance.
///
/// `[R-STAR-091]`. The distance allowed grows with the length of what was
/// typed, because two edits in a four-letter name is a different word while
/// two edits in a twelve-letter one is a typo. A transposition counts as one
/// edit rather than two, since swapping adjacent letters is the mistake people
/// actually make, and plain Levenshtein puts `titel` two edits from `title`
/// and so never suggests it.
pub fn closest<'a>(name: &str, candidates: impl Iterator<Item = &'a str>) -> Option<String> {
    let limit = match name.chars().count() {
        0..=3 => 1,
        4..=8 => 2,
        _ => 3,
    };
    candidates
        .map(|c| (distance(name, c), c))
        .filter(|(d, _)| *d <= limit)
        .min_by_key(|(d, _)| *d)
        .map(|(_, c)| c.to_owned())
}

/// Damerau-Levenshtein distance, three rows at a time.
///
/// The third row is what makes a transposition cost one edit instead of two.
fn distance(a: &str, b: &str) -> usize {
    let (a, b): (Vec<char>, Vec<char>) = (a.chars().collect(), b.chars().collect());
    let mut before = vec![0; b.len() + 1];
    let mut prev: Vec<usize> = (0..=b.len()).collect();
    let mut cur = vec![0; b.len() + 1];

    for (i, ca) in a.iter().enumerate() {
        cur[0] = i + 1;
        for (j, cb) in b.iter().enumerate() {
            let cost = usize::from(ca != cb);
            let mut best = (prev[j + 1] + 1).min(cur[j] + 1).min(prev[j] + cost);
            if i > 0 && j > 0 && ca == &b[j - 1] && a[i - 1] == *cb {
                best = best.min(before[j - 1] + 1);
            }
            cur[j + 1] = best;
        }
        std::mem::swap(&mut before, &mut prev);
        std::mem::swap(&mut prev, &mut cur);
    }
    prev[b.len()]
}

/// The result type every fallible call in this crate returns.
pub type Result<T> = std::result::Result<T, StarError>;
