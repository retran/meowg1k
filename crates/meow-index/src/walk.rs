// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Deciding which files exist as far as the index is concerned.

use std::path::{Path, PathBuf};

use ignore::WalkBuilder;
use ignore::overrides::OverrideBuilder;

use crate::error::{IndexError, Result};

/// How big a file may be before it is skipped.
///
/// `[R-INDEX-003]`. A megabyte of source is a generated file, a vendored
/// bundle, or a data fixture, and embedding it buys a result nobody wanted at
/// a cost everybody pays.
pub const DEFAULT_MAX_BYTES: u64 = 1024 * 1024;

/// How much of a file is read to decide whether it is text.
///
/// The first few kilobytes: a file that is text at the start and binary in the
/// middle is a file somebody generated, and reading all of it to find out
/// costs the whole walk.
const SNIFF_BYTES: usize = 8192;

/// The directory the index must never read, whatever an ignore file says.
///
/// `[R-INDEX-001]`: not re-includable. Indexing the store would embed the
/// index, which grows without bound and answers every query with itself.
const DATA_DIR: &str = ".meow/.data";

/// What the walk decided about the files it saw.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Walked {
    /// The files to index, in a stable order.
    pub files: Vec<PathBuf>,
    /// The files skipped for being binary.
    ///
    /// `[R-INDEX-002]`: counted, because "why is this not in the index" is
    /// the question an index has to be able to answer.
    pub binary: Vec<PathBuf>,
    /// The files skipped for being too large, with their sizes.
    ///
    /// `[R-INDEX-003]`: each one reported, not just counted, because the
    /// answer to "why is this missing" is a path.
    pub too_large: Vec<(PathBuf, u64)>,
    /// Symbolic links that pointed outside the workspace.
    pub escaping: Vec<PathBuf>,
}

impl Walked {
    /// How many files were skipped, for a one-line summary.
    pub fn skipped(&self) -> usize {
        self.binary.len() + self.too_large.len() + self.escaping.len()
    }
}

/// How to walk.
#[derive(Debug, Clone, Copy)]
pub struct Walk {
    /// The size at which a file is skipped.
    pub max_bytes: u64,
}

impl Default for Walk {
    fn default() -> Self {
        Self {
            max_bytes: DEFAULT_MAX_BYTES,
        }
    }
}

impl Walk {
    /// Find everything under a root that should be indexed.
    ///
    /// Satisfies `[R-INDEX-001]` through `.gitignore` and `.meowignore`, with
    /// `.meow/.data/` excluded last so no negation can bring it back;
    /// `[R-INDEX-002]` and `[R-INDEX-003]` by recording what was skipped and
    /// why; `[R-INDEX-004]` by refusing a symbolic link that leaves the root;
    /// and `[R-INDEX-005]` by having no rule about prose at all - markdown is
    /// a text file like any other.
    ///
    /// # Errors
    ///
    /// [`IndexError::Walk`] when the root cannot be read.
    pub fn run(&self, root: &Path) -> Result<Walked> {
        let root = root.canonicalize().map_err(|source| IndexError::Walk {
            root: root.to_path_buf(),
            message: source.to_string(),
        })?;

        let mut overrides = OverrideBuilder::new(&root);
        // Last wins in an override set, and this is added after nothing else,
        // so `.meowignore` cannot negate it.
        overrides
            .add(&format!("!{DATA_DIR}/**"))
            .map_err(|e| IndexError::Walk {
                root: root.clone(),
                message: e.to_string(),
            })?;
        let overrides = overrides.build().map_err(|e| IndexError::Walk {
            root: root.clone(),
            message: e.to_string(),
        })?;

        let mut out = Walked::default();
        let reclaimed = self.reclaimed(&root, &mut out)?;

        let mut walker = WalkBuilder::new(&root);
        walker
            .hidden(false)
            .parents(false)
            .git_ignore(true)
            .git_global(false)
            .git_exclude(true)
            // A workspace need not be a repository, and a `.gitignore` in one
            // that is not still says what does not belong in the index.
            .require_git(false)
            .follow_links(true)
            .overrides(overrides)
            // `.meowignore` is added after `.gitignore`, so it wins, which is
            // what makes a negation in it able to re-include something git
            // excludes.
            .add_custom_ignore_filename(".meowignore");

        for entry in walker.build() {
            let entry = match entry {
                Ok(entry) => entry,
                // One unreadable directory is not a reason to abandon the
                // walk: the index is allowed to be incomplete and has to say
                // so, which is what the skip lists are for.
                Err(_) => continue,
            };

            if !entry.file_type().is_some_and(|t| t.is_file()) {
                continue;
            }
            // [R-INDEX-004]: a link that resolves outside the workspace is
            // somebody else's content, and following it would index a home
            // directory from a repository.
            if let Some(path) = self.judge(&root, entry.path(), &mut out) {
                out.files.push(path);
            }
        }

        out.files.extend(reclaimed);
        out.files.dedup();

        // A stable order, so two builds of the same tree produce the same
        // work list and a diff between two runs is about content.
        out.files.sort();
        out.binary.sort();
        out.too_large.sort();
        out.escaping.sort();

        Ok(out)
    }
}

impl Walk {
    /// Files a `.meowignore` negation asks for back.
    ///
    /// `[R-INDEX-001]` wants a path `.gitignore` excludes to be indexable
    /// deliberately, in one line. Git's own rule is that a file inside an
    /// excluded directory cannot be re-included, so `!generated/api.rs` alone
    /// does nothing when `.gitignore` holds `generated/` - the walk never
    /// descends far enough to see the negation.
    ///
    /// So the negations are collected and walked separately, with the ignore
    /// files switched off, and merged back in. `.meow/.data/` is excluded here
    /// too, which is what makes it un-reincludable by any route.
    fn reclaimed(&self, root: &Path, out: &mut Walked) -> Result<Vec<PathBuf>> {
        let Ok(text) = std::fs::read_to_string(root.join(".meowignore")) else {
            return Ok(Vec::new());
        };

        let negations: Vec<&str> = text
            .lines()
            .map(str::trim)
            .filter_map(|line| line.strip_prefix('!'))
            .filter(|line| !line.is_empty())
            .collect();

        if negations.is_empty() {
            return Ok(Vec::new());
        }

        let mut overrides = OverrideBuilder::new(root);
        for pattern in negations {
            overrides.add(pattern).map_err(|e| IndexError::Walk {
                root: root.to_path_buf(),
                message: e.to_string(),
            })?;
        }
        let overrides = overrides.build().map_err(|e| IndexError::Walk {
            root: root.to_path_buf(),
            message: e.to_string(),
        })?;

        let mut walker = WalkBuilder::new(root);
        walker
            .hidden(false)
            .parents(false)
            .git_ignore(false)
            .git_global(false)
            .git_exclude(false)
            .ignore(false)
            .follow_links(true)
            .overrides(overrides);

        let mut found = Vec::new();
        for entry in walker.build().flatten() {
            if !entry.file_type().is_some_and(|t| t.is_file()) {
                continue;
            }
            let path = entry.path().to_path_buf();

            if path.strip_prefix(root).is_ok_and(under_data_dir) {
                continue;
            }
            match self.judge(root, &path, out) {
                Some(path) => found.push(path),
                None => continue,
            }
        }
        Ok(found)
    }

    /// Apply the size, binary, and escape rules to one path.
    ///
    /// Shared by both walks, so a file that arrives through a negation is held
    /// to the same rules as one that arrives normally.
    fn judge(&self, root: &Path, path: &Path, out: &mut Walked) -> Option<PathBuf> {
        match path.canonicalize() {
            Ok(resolved) if !resolved.starts_with(root) => {
                out.escaping.push(path.to_path_buf());
                return None;
            }
            Ok(_) => {}
            Err(_) => return None,
        }

        let size = std::fs::metadata(path).map(|m| m.len()).unwrap_or_default();
        if size > self.max_bytes {
            out.too_large.push((path.to_path_buf(), size));
            return None;
        }

        match is_text(path) {
            Ok(true) => Some(path.to_path_buf()),
            Ok(false) => {
                out.binary.push(path.to_path_buf());
                None
            }
            Err(_) => None,
        }
    }
}

/// Whether a relative path is inside the store.
fn under_data_dir(relative: &Path) -> bool {
    relative
        .to_string_lossy()
        .replace('\\', "/")
        .starts_with(DATA_DIR)
}

/// Whether a file looks like text.
///
/// A null byte in the first few kilobytes. It is the test `git` uses, it costs
/// one short read, and it is right about everything except a UTF-16 file,
/// which it calls binary - correctly, as far as an embedding model is
/// concerned.
fn is_text(path: &Path) -> std::io::Result<bool> {
    use std::io::Read;

    let mut file = std::fs::File::open(path)?;
    let mut head = [0_u8; SNIFF_BYTES];
    let read = file.read(&mut head)?;
    Ok(!head[..read].contains(&0))
}
