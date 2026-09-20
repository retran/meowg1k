// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The credential store, at `~/.meow/auth.json`.
//!
//! Outside every workspace, by `[R-AUTH-010]`: a repository cannot carry a
//! credential and a contributor cannot commit one. Nothing in `.meow/` can
//! read it and no runtime module exposes it, by `[R-AUTH-004]` - a workspace
//! that could read the store would be a workspace that could exfiltrate it.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

/// What went wrong reaching the store.
#[derive(Debug, thiserror::Error)]
pub enum AuthError {
    /// There is no home directory to put it in.
    #[error("there is no home directory to keep credentials in")]
    NoHome,

    /// The file is readable by somebody other than its owner.
    ///
    /// `[R-AUTH-011]`: refused rather than repaired. A file that was
    /// world-readable has already been readable, and silently tightening the
    /// mode would hide that from the person who needs to know.
    #[error("`{path}` is readable by others (mode {mode:o}); it must be {expected:o}")]
    TooOpen {
        /// Which file.
        path: PathBuf,
        /// What it is.
        mode: u32,
        /// What it must be.
        expected: u32,
    },

    /// The file is there and is not a credential store.
    #[error("`{path}` is not a credential store: {message}")]
    Unreadable {
        /// Which file.
        path: PathBuf,
        /// What the parser said.
        message: String,
    },

    /// Reading or writing failed.
    #[error("could not {doing} `{path}`: {source}")]
    Io {
        /// What was being attempted.
        doing: &'static str,
        /// Which file.
        path: PathBuf,
        /// The underlying failure.
        source: std::io::Error,
    },
}

/// One provider's credential.
///
/// An API key and an OAuth token live in the same store, by `[R-AUTH-022]`,
/// because a `.star` file could hold neither and the person managing them
/// should not have to know which kind a provider uses.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Credential {
    /// A key the provider issued, used as it is.
    ApiKey {
        /// The key.
        key: String,
        /// When it was stored, as seconds since the epoch.
        stored: i64,
    },
    /// A token obtained by a device-code flow.
    OAuth {
        /// What is sent, until it expires.
        access: String,
        /// What obtains the next access token.
        refresh: String,
        /// When the access token stops working.
        expires: i64,
        /// When it was stored.
        stored: i64,
    },
}

impl Credential {
    /// When it was stored.
    pub fn stored(&self) -> i64 {
        match self {
            Self::ApiKey { stored, .. } | Self::OAuth { stored, .. } => *stored,
        }
    }

    /// What to describe it as, without describing it.
    ///
    /// `[R-AUTH-014]`: nothing here returns any part of a secret.
    pub fn describe(&self) -> &'static str {
        match self {
            Self::ApiKey { .. } => "api key",
            Self::OAuth { .. } => "oauth",
        }
    }
}

/// The file, parsed.
#[derive(Debug, Default, Serialize, Deserialize)]
struct Stored {
    /// One entry per provider name.
    #[serde(default)]
    providers: BTreeMap<String, Credential>,
}

/// The credential store.
#[derive(Debug)]
pub struct Store {
    path: PathBuf,
    entries: BTreeMap<String, Credential>,
}

/// What a store file must be, and nothing more.
#[cfg(unix)]
const OWNER_ONLY: u32 = 0o600;

impl Store {
    /// Where the store lives, whether or not it exists.
    ///
    /// # Errors
    ///
    /// [`AuthError::NoHome`] when there is no home directory.
    pub fn path() -> Result<PathBuf, AuthError> {
        let home = std::env::var_os("MEOW_HOME")
            .map(PathBuf::from)
            .or_else(home_dir)
            .ok_or(AuthError::NoHome)?;
        Ok(home.join(".meow").join("auth.json"))
    }

    /// Read the store, or an empty one when the file is not there.
    ///
    /// # Errors
    ///
    /// [`AuthError`] when the file exists and cannot be read, is too open, or
    /// does not parse. A missing file is not an error: a machine with no
    /// credentials stored is the ordinary starting state.
    pub fn open() -> Result<Self, AuthError> {
        let path = Self::path()?;
        if !path.exists() {
            return Ok(Self {
                path,
                entries: BTreeMap::new(),
            });
        }

        check_mode(&path)?;

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
            entries: stored.providers,
        })
    }

    /// One provider's credential, if it has one.
    pub fn get(&self, provider: &str) -> Option<&Credential> {
        self.entries.get(provider)
    }

    /// Every provider with a credential, and what kind it is.
    ///
    /// `[R-AUTH-013]`: what a listing may say. Never the credential itself.
    pub fn list(&self) -> Vec<(&str, &Credential)> {
        self.entries
            .iter()
            .map(|(name, credential)| (name.as_str(), credential))
            .collect()
    }

    /// Store one, replacing whatever was there.
    pub fn put(&mut self, provider: impl Into<String>, credential: Credential) {
        self.entries.insert(provider.into(), credential);
    }

    /// Remove one, and say whether it was there.
    pub fn remove(&mut self, provider: &str) -> bool {
        self.entries.remove(provider).is_some()
    }

    /// Write the store back.
    ///
    /// `[R-AUTH-012]`: through a temporary file in the same directory and a
    /// rename, so a process that dies mid-write leaves the old store rather
    /// than half of the new one. A truncated credential store locks the user
    /// out of every provider at once.
    ///
    /// # Errors
    ///
    /// [`AuthError::Io`] when the directory cannot be made or the file cannot
    /// be written or renamed.
    pub fn save(&self) -> Result<(), AuthError> {
        write_json(
            &self.path,
            &Stored {
                providers: self.entries.clone(),
            },
        )
    }
}

/// Write a JSON file under `~/.meow/`, privately and atomically.
///
/// `[R-AUTH-012]`: through a temporary beside the target and a rename, so a
/// process that dies mid-write leaves the old file rather than half of the new
/// one. A truncated credential store locks the user out of every provider at
/// once, and a truncated trust record asks every question again.
///
/// Beside the target rather than in a temporary directory, because a rename
/// across filesystems is not atomic and `/tmp` is often one.
///
/// # Errors
///
/// [`AuthError::Io`] when the directory cannot be made or the file cannot be
/// written or renamed.
pub fn write_json<T: Serialize>(path: &Path, value: &T) -> Result<(), AuthError> {
    let directory = path.parent().ok_or_else(|| AuthError::Io {
        doing: "find the directory of",
        path: path.to_path_buf(),
        source: std::io::Error::other("no parent"),
    })?;
    std::fs::create_dir_all(directory).map_err(|source| AuthError::Io {
        doing: "create",
        path: directory.to_path_buf(),
        source,
    })?;

    let text = serde_json::to_string_pretty(value).map_err(|error| AuthError::Io {
        doing: "encode",
        path: path.to_path_buf(),
        source: std::io::Error::other(error),
    })?;

    let temporary = path.with_extension("json.new");
    write_private(&temporary, &text)?;
    std::fs::rename(&temporary, path).map_err(|source| AuthError::Io {
        doing: "replace",
        path: path.to_path_buf(),
        source,
    })
}

/// The home directory, without a dependency for it.
fn home_dir() -> Option<PathBuf> {
    #[cfg(unix)]
    {
        std::env::var_os("HOME").map(PathBuf::from)
    }
    #[cfg(windows)]
    {
        std::env::var_os("USERPROFILE").map(PathBuf::from)
    }
}

/// Refuse a store anybody else can read - `[R-AUTH-011]`.
#[cfg(unix)]
fn check_mode(path: &Path) -> Result<(), AuthError> {
    use std::os::unix::fs::PermissionsExt;

    let meta = std::fs::metadata(path).map_err(|source| AuthError::Io {
        doing: "stat",
        path: path.to_path_buf(),
        source,
    })?;
    let mode = meta.permissions().mode() & 0o777;
    if mode & 0o077 != 0 {
        return Err(AuthError::TooOpen {
            path: path.to_path_buf(),
            mode,
            expected: OWNER_ONLY,
        });
    }
    Ok(())
}

/// Windows has no mode to check; its files inherit the user's ACL.
#[cfg(not(unix))]
fn check_mode(_path: &Path) -> Result<(), AuthError> {
    Ok(())
}

/// Write a file only its owner can read.
#[cfg(unix)]
fn write_private(path: &Path, text: &str) -> Result<(), AuthError> {
    use std::io::Write;
    use std::os::unix::fs::OpenOptionsExt;

    let mut file = std::fs::OpenOptions::new()
        .write(true)
        .create(true)
        .truncate(true)
        .mode(OWNER_ONLY)
        .open(path)
        .map_err(|source| AuthError::Io {
            doing: "create",
            path: path.to_path_buf(),
            source,
        })?;
    file.write_all(text.as_bytes())
        .and_then(|()| file.sync_all())
        .map_err(|source| AuthError::Io {
            doing: "write",
            path: path.to_path_buf(),
            source,
        })
}

#[cfg(not(unix))]
fn write_private(path: &Path, text: &str) -> Result<(), AuthError> {
    std::fs::write(path, text).map_err(|source| AuthError::Io {
        doing: "write",
        path: path.to_path_buf(),
        source,
    })
}
