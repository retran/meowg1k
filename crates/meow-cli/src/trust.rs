// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Whether this machine has agreed to run the scripts in a workspace.
//!
//! A `.meow/` directory in a repository you just cloned is executable code
//! with tool access - `[R-AUTH-030]`. Nothing about `git clone` asks whether
//! you meant to run it, so the first invocation does.
//!
//! Asking is safe because asking costs nothing: `[R-STAR-084]` refuses every
//! module during declaration, so loading `.meow/` far enough to say what it
//! declares reaches no file, no program, and no network. That is what makes
//! `[R-AUTH-033]` true rather than aspirational.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use meow_star::Registry;
use serde::{Deserialize, Serialize};

use crate::auth::AuthError;

/// What a workspace declares, as one line per thing.
///
/// This is both what the person is shown and what is remembered, which is the
/// point: `[R-AUTH-034]` says a workspace whose declarations changed asks
/// again, and the only honest way to mean that is to record what was shown.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Declared(Vec<String>);

impl Declared {
    /// Read what a loaded workspace declares.
    pub fn of(registry: &Registry) -> Self {
        let mut lines = Vec::new();

        for agent in registry.agents() {
            lines.push(format!("agent {}", agent.name));
        }
        for tool in registry.tools() {
            lines.push(format!("tool {}", tool.name));
        }
        for command in registry.commands() {
            lines.push(format!("command {command}"));
        }
        match registry.policy() {
            Some(policy) => {
                for (decision, rule) in policy.rules() {
                    lines.push(format!("policy {} {}", decision.as_str(), rule.describe()));
                }
            }
            // A workspace with no policy is not a workspace that asks for
            // nothing: it is one where every tool call needs a grant. Saying
            // so is part of what the person is agreeing to.
            None => lines.push("policy none, so every tool call is asked".to_owned()),
        }

        // Sorted, so that reordering a declaration is not a change and a
        // person is not re-asked for moving a function.
        lines.sort();
        Self(lines)
    }

    /// What to show, in the order it will be read.
    pub fn lines(&self) -> &[String] {
        &self.0
    }

    /// A short, stable name for exactly this set.
    ///
    /// SHA-256 over the sorted lines. What matters is that it changes when a
    /// tool, an agent, a command, or a rule does, and not when anything else
    /// in the workspace does - a comment, a handler's body, a model's name.
    /// Trust is about authority, and a handler's body cannot widen it without
    /// widening one of these.
    pub fn fingerprint(&self) -> String {
        use sha2::{Digest, Sha256};

        let mut hasher = Sha256::new();
        for line in &self.0 {
            hasher.update(line.as_bytes());
            hasher.update([0]);
        }
        format!("{:x}", hasher.finalize())
    }
}

/// One workspace this machine has agreed to.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
struct Agreement {
    /// The fingerprint that was shown and agreed to.
    fingerprint: String,
    /// When, as seconds since the epoch.
    agreed: i64,
}

#[derive(Debug, Default, Serialize, Deserialize)]
struct Stored {
    #[serde(default)]
    workspaces: BTreeMap<String, Agreement>,
}

/// What this machine has agreed to run.
#[derive(Debug)]
pub struct Trust {
    path: PathBuf,
    entries: BTreeMap<String, Agreement>,
}

/// Whether a workspace may run, and why not when it may not.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Standing {
    /// Agreed to, and unchanged since.
    Trusted,
    /// Never agreed to.
    Unknown,
    /// Agreed to, but it declares something else now - `[R-AUTH-034]`.
    Changed,
}

impl Trust {
    /// Where the record lives.
    ///
    /// # Errors
    ///
    /// [`AuthError::NoHome`] when there is no home directory.
    pub fn path() -> Result<PathBuf, AuthError> {
        let store = crate::auth::Store::path()?;
        Ok(store.with_file_name("trust.json"))
    }

    /// Read it, or an empty one when there is none.
    ///
    /// # Errors
    ///
    /// [`AuthError`] when the file exists and cannot be read or parsed.
    pub fn open() -> Result<Self, AuthError> {
        let path = Self::path()?;
        if !path.exists() {
            return Ok(Self {
                path,
                entries: BTreeMap::new(),
            });
        }

        let text = std::fs::read_to_string(&path).map_err(|source| AuthError::Io {
            doing: "read",
            path: path.clone(),
            source,
        })?;
        let stored: Stored =
            serde_json::from_str(&text).map_err(|error| AuthError::Unreadable {
                path: path.clone(),
                message: error.to_string(),
            })?;

        Ok(Self {
            path,
            entries: stored.workspaces,
        })
    }

    /// Where this workspace stands.
    pub fn standing(&self, root: &Path, declared: &Declared) -> Standing {
        match self.entries.get(&key(root)) {
            None => Standing::Unknown,
            Some(agreement) if agreement.fingerprint == declared.fingerprint() => Standing::Trusted,
            Some(_) => Standing::Changed,
        }
    }

    /// Record an agreement to what this workspace declares now.
    pub fn agree(&mut self, root: &Path, declared: &Declared, when: i64) {
        self.entries.insert(
            key(root),
            Agreement {
                fingerprint: declared.fingerprint(),
                agreed: when,
            },
        );
    }

    /// Withdraw it, and say whether there was one - `[R-AUTH-032]`.
    pub fn withdraw(&mut self, root: &Path) -> bool {
        self.entries.remove(&key(root)).is_some()
    }

    /// Every workspace agreed to, with when.
    pub fn list(&self) -> Vec<(&str, i64)> {
        self.entries
            .iter()
            .map(|(path, agreement)| (path.as_str(), agreement.agreed))
            .collect()
    }

    /// Write it back, atomically and privately, as the credential store is.
    ///
    /// # Errors
    ///
    /// [`AuthError::Io`] when the directory or the file cannot be written.
    pub fn save(&self) -> Result<(), AuthError> {
        let stored = Stored {
            workspaces: self.entries.clone(),
        };
        crate::auth::write_json(&self.path, &stored)
    }
}

/// How a workspace is named in the record.
///
/// Canonical where that is possible, because `/var` and `/private/var` are the
/// same directory and agreeing to one is agreeing to the other. A path that
/// will not canonicalise is used as given rather than dropped: failing to
/// record an agreement would mean asking again every time.
fn key(root: &Path) -> String {
    root.canonicalize()
        .unwrap_or_else(|_| root.to_path_buf())
        .to_string_lossy()
        .into_owned()
}
