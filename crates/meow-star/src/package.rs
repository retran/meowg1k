// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Starlark a workspace did not write.
//!
//! `@std//` is Rust in this binary and `//` is a file the workspace wrote.
//! `@<pkg>//` is neither, and the difference that matters is that it arrived
//! over a network from somebody else. So: a load never fetches, the contents
//! are verified before anything is evaluated, and the cache is keyed by what
//! the contents hash to rather than by what somebody called them.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

use crate::error::{Result, StarError};

/// Where the cache lives, under the directory nothing else walks.
pub const CACHE: &str = ".data/pkg";

/// The lockfile, beside `meow.star` and committed with it.
pub const LOCKFILE: &str = "meow.lock";

/// A package a workspace declares.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Package {
    /// What `@<name>//` refers to.
    pub name: String,
    /// Where it comes from.
    pub source: String,
    /// Which version is wanted.
    pub version: String,
    /// Where it was declared, for the error when two share a name.
    pub origin: crate::registry::Origin,
}

/// One package, pinned.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct Pin {
    /// Where it came from, recorded so a diff shows a source change.
    pub source: String,
    /// What version resolved to.
    pub version: String,
    /// SHA-256 over the contents, which is what is actually verified.
    pub hash: String,
}

/// The lockfile.
///
/// `[R-PKG-010]`: deterministic, because a diff should show a dependency
/// change and nothing else. A `BTreeMap` and a trailing newline are the whole
/// of what that takes.
#[derive(Debug, Default, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct Lock {
    /// One entry per package name.
    #[serde(default)]
    pub packages: BTreeMap<String, Pin>,
}

impl Lock {
    /// Read the lockfile, or an empty one when there is none.
    ///
    /// # Errors
    ///
    /// [`StarError::Load`] when it exists and will not parse.
    pub fn read(config_dir: &Path) -> Result<Self> {
        let path = config_dir.join(LOCKFILE);
        if !path.exists() {
            return Ok(Self::default());
        }
        let text = std::fs::read_to_string(&path).map_err(|e| StarError::Load {
            message: format!("could not read `{}`: {e}", path.display()),
        })?;
        toml::from_str(&text).map_err(|e| StarError::Load {
            message: format!("`{}` is not a lockfile: {e}", path.display()),
        })
    }

    /// Write it back.
    ///
    /// # Errors
    ///
    /// [`StarError::Load`] when it cannot be encoded or written.
    pub fn write(&self, config_dir: &Path) -> Result<()> {
        let path = config_dir.join(LOCKFILE);
        let text = toml::to_string_pretty(self).map_err(|e| StarError::Load {
            message: format!("could not encode the lockfile: {e}"),
        })?;
        std::fs::write(&path, text).map_err(|e| StarError::Load {
            message: format!("could not write `{}`: {e}", path.display()),
        })
    }
}

/// Where a package's contents sit once fetched.
///
/// `[R-PKG-021]`: keyed by the hash rather than by name and version. A version
/// is a label upstream controls and can move; a hash is the bytes. Two
/// workspaces that pinned the same hash are provably running the same code.
pub fn cached_at(config_dir: &Path, hash: &str) -> PathBuf {
    config_dir.join(CACHE).join(hash)
}

/// SHA-256 over a directory's contents.
///
/// Over the sorted relative paths and the bytes at each, so a file that moves
/// changes the hash and the order a filesystem happens to return entries in
/// does not. This is what `[R-PKG-011]` verifies, and it is recomputed at load
/// rather than trusted from a stamp file: a stamp is written by the same
/// process that would have been fooled.
///
/// # Errors
///
/// [`StarError::Load`] when the directory cannot be walked or a file read.
pub fn hash_tree(root: &Path) -> Result<String> {
    use sha2::{Digest, Sha256};

    let mut files = Vec::new();
    collect(root, root, &mut files)?;
    files.sort();

    let mut hasher = Sha256::new();
    for relative in &files {
        hasher.update(relative.as_bytes());
        hasher.update([0]);
        let bytes = std::fs::read(root.join(relative)).map_err(|e| StarError::Load {
            message: format!("could not read `{relative}`: {e}"),
        })?;
        hasher.update(bytes.len().to_le_bytes());
        hasher.update(&bytes);
    }
    Ok(format!("{:x}", hasher.finalize()))
}

/// Every file under a root, relative and with forward slashes.
fn collect(root: &Path, at: &Path, out: &mut Vec<String>) -> Result<()> {
    let entries = std::fs::read_dir(at).map_err(|e| StarError::Load {
        message: format!("could not read `{}`: {e}", at.display()),
    })?;
    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            collect(root, &path, out)?;
        } else if let Ok(relative) = path.strip_prefix(root) {
            out.push(relative.to_string_lossy().replace('\\', "/"));
        }
    }
    Ok(())
}

/// Where a `@<pkg>//<path>` load should read from, or why it cannot.
///
/// `[R-PKG-012]`: this never fetches. A load that reached the network without
/// being asked is a load that can behave differently between two runs of the
/// same commit.
///
/// # Errors
///
/// [`StarError::Load`] when the package is undeclared, unlocked, absent from
/// the cache, or does not hash to what the lockfile says.
pub fn resolve(
    config_dir: &Path,
    declared: Option<&Package>,
    lock: &Lock,
    name: &str,
    path: &str,
) -> Result<PathBuf> {
    // `[R-PKG-001]`: undeclared is refused before anything else, so a typo
    // reads as a typo rather than as a missing download.
    let Some(declared) = declared else {
        return Err(StarError::Load {
            message: format!(
                "`@{name}//{path}` names package `{name}`, which this workspace does not declare. \
                 Add `meow.package(name = \"{name}\", ...)` to .meow/meow.star."
            ),
        });
    };

    let Some(pin) = lock.packages.get(name) else {
        return Err(StarError::Load {
            message: format!(
                "`{name}` is declared and not locked. Run `meow pkg update` to write \
                 `.meow/{LOCKFILE}`."
            ),
        });
    };

    let root = cached_at(config_dir, &pin.hash);
    if !root.exists() {
        return Err(StarError::Load {
            message: format!(
                "`{name}` {} is locked and not in the cache. Run `meow pkg fetch`.",
                pin.version
            ),
        });
    }

    // `[R-PKG-011]`: before anything is evaluated, and recomputed rather than
    // read from a marker. A cache somebody edited is the case this exists for.
    let found = hash_tree(&root)?;
    if found != pin.hash {
        return Err(StarError::Load {
            message: format!(
                "`{name}` in the cache does not match `{LOCKFILE}`. Expected {}, found {found}. \
                 Run `meow pkg fetch` to replace it.",
                pin.hash
            ),
        });
    }

    // The declared source is recorded in the pin, so a source changed in
    // `meow.star` and not re-locked is caught here rather than silently
    // loading what the old source gave.
    if pin.source != declared.source {
        return Err(StarError::Load {
            message: format!(
                "`{name}` is declared from `{}` and locked from `{}`. Run `meow pkg update`.",
                declared.source, pin.source
            ),
        });
    }

    let file = root.join(path);
    // `[R-PKG-031]`: a package's own files and nothing else. A `..` that
    // happens to land back inside is still a mistake worth reporting.
    if path.contains("..") || !file.starts_with(&root) {
        return Err(StarError::Load {
            message: format!("`{path}` climbs out of package `{name}`"),
        });
    }
    if !file.exists() {
        return Err(StarError::Load {
            message: format!("package `{name}` has no `{path}`"),
        });
    }
    Ok(file)
}
