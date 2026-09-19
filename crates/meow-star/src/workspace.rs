// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Finding the workspace.

use std::path::{Path, PathBuf};

use crate::error::{Result, StarError};

/// The names a workspace is recognised by.
const DIR: &str = ".meow";
const ENTRY: &str = "meow.star";

/// Where a run's configuration and data live.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Workspace {
    root: PathBuf,
}

impl Workspace {
    /// Find the workspace containing a directory.
    ///
    /// Satisfies `[R-STAR-001]`: the nearest ancestor with `.meow/meow.star`,
    /// stopping at the first match and merging with no global configuration.
    /// The project wins outright, which is the decision `0.3.0-starlark-api.md`
    /// section 2 records: merging would mean a run behaves differently
    /// depending on a file outside the repository, and no amount of
    /// documentation makes that debuggable.
    ///
    /// # Errors
    ///
    /// [`StarError::NoWorkspace`] naming every directory searched, per
    /// `[R-STAR-002]`.
    pub fn discover(from: &Path) -> Result<Self> {
        let mut searched = Vec::new();
        let mut here = Some(from);
        while let Some(dir) = here {
            searched.push(dir.to_path_buf());
            if dir.join(DIR).join(ENTRY).is_file() {
                return Ok(Self {
                    root: dir.to_path_buf(),
                });
            }
            here = dir.parent();
        }
        Err(StarError::NoWorkspace { searched })
    }

    /// Treat a directory as the workspace without searching.
    pub fn at(root: impl Into<PathBuf>) -> Self {
        Self { root: root.into() }
    }

    /// The directory the workspace is rooted at.
    pub fn root(&self) -> &Path {
        &self.root
    }

    /// The `.meow/` directory.
    pub fn config_dir(&self) -> PathBuf {
        self.root.join(DIR)
    }

    /// The file loading starts from.
    pub fn entry(&self) -> PathBuf {
        self.config_dir().join(ENTRY)
    }

    /// Where sessions, the cache, and the index live.
    pub fn data_dir(&self) -> PathBuf {
        self.config_dir().join(".data")
    }

    /// Turn a `//` path into a file, refusing one that escapes.
    ///
    /// Satisfies `[R-STAR-004]`. Refusing at the lexical level rather than
    /// after resolving: a `..` that happens to land back inside is still a
    /// mistake worth reporting, and resolving first would let a symlink decide.
    ///
    /// # Errors
    ///
    /// [`StarError::Load`] when the path leaves `.meow/`.
    pub fn resolve_local(&self, path: &str) -> Result<PathBuf> {
        if path.starts_with('/') || path.split('/').any(|part| part == ".." || part == ".") {
            return Err(StarError::Load {
                message: format!("`//{path}` leaves .meow/, which a load may not do"),
            });
        }
        Ok(self.config_dir().join(path))
    }
}
