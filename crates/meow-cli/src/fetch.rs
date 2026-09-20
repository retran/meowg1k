// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Getting a package, which is the half that touches the network.
//!
//! The loader never does this - `[R-PKG-012]`. `meow pkg fetch` downloads what
//! the lockfile pins and changes nothing; `meow pkg update` re-resolves and
//! rewrites it. Keeping them apart is what makes a run depend on the commit
//! rather than on when it happened.

use std::path::{Path, PathBuf};
use std::time::Duration;

use meow_star::package::{Lock, Package, Pin, cached_at, hash_tree};
use tokio_util::sync::CancellationToken;

/// How long a download may take.
const TIMEOUT: Duration = Duration::from_secs(120);

/// How much of an archive will be read.
///
/// A package is Starlark. Something claiming to be one and arriving as a
/// gigabyte is not a package, and refusing it here is cheaper than filling a
/// disk to find out.
const MAX_BYTES: usize = 64 * 1024 * 1024;

/// What went wrong.
#[derive(Debug, thiserror::Error)]
pub enum FetchError {
    /// The source could not be read.
    #[error("`{name}`: {message}")]
    Source {
        /// Which package.
        name: String,
        /// What went wrong.
        message: String,
    },

    /// What arrived is not what the lockfile pins.
    ///
    /// `[R-PKG-011]` verifies at load; this verifies at fetch, so a mirror
    /// that served something else is caught when it is downloaded rather than
    /// the next time somebody runs a command.
    #[error("`{name}` hashes to {found}, and `meow.lock` pins {expected}")]
    Mismatch {
        /// Which package.
        name: String,
        /// What it pins.
        expected: String,
        /// What arrived.
        found: String,
    },

    /// Writing the cache failed.
    #[error("could not {doing} `{path}`: {source}")]
    Io {
        /// What was being attempted.
        doing: &'static str,
        /// Which path.
        path: PathBuf,
        /// The underlying failure.
        source: std::io::Error,
    },

    /// Ctrl-C.
    #[error("cancelled")]
    Cancelled,
}

/// Where a package's archive is, with its version substituted in.
///
/// `{version}` in the source is replaced. A source with no placeholder is
/// used as it is, which is what a package that publishes one URL per release
/// needs and what a mirror of a single file needs.
pub fn url_for(package: &Package) -> String {
    package.source.replace("{version}", &package.version)
}

/// Download a package and put it in the cache, returning what it hashed to.
///
/// # Errors
///
/// [`FetchError`] when the source cannot be read, the archive will not
/// extract, or the cache cannot be written.
pub async fn fetch(
    config_dir: &Path,
    package: &Package,
    cancel: &CancellationToken,
) -> Result<String, FetchError> {
    let url = url_for(package);
    let client = reqwest::Client::builder()
        .timeout(TIMEOUT)
        .user_agent(concat!("meowg1k/", env!("CARGO_PKG_VERSION")))
        .build()
        .map_err(|e| FetchError::Source {
            name: package.name.clone(),
            message: e.to_string(),
        })?;

    let response = tokio::select! {
        () = cancel.cancelled() => return Err(FetchError::Cancelled),
        response = client.get(&url).send() => response.map_err(|e| FetchError::Source {
            name: package.name.clone(),
            message: format!("could not reach `{url}`: {e}"),
        })?,
    };

    if !response.status().is_success() {
        return Err(FetchError::Source {
            name: package.name.clone(),
            message: format!("`{url}` answered {}", response.status()),
        });
    }

    let bytes = tokio::select! {
        () = cancel.cancelled() => return Err(FetchError::Cancelled),
        bytes = response.bytes() => bytes.map_err(|e| FetchError::Source {
            name: package.name.clone(),
            message: format!("could not read `{url}`: {e}"),
        })?,
    };

    if bytes.len() > MAX_BYTES {
        return Err(FetchError::Source {
            name: package.name.clone(),
            message: format!(
                "`{url}` is {} bytes, and a package may be {MAX_BYTES}",
                bytes.len()
            ),
        });
    }

    // Into a staging directory first, hashed there, then moved into place
    // under its hash. `[R-PKG-022]`: an interrupted download leaves staging
    // behind and never something the next run mistakes for a package.
    let staging = config_dir
        .join(meow_star::package::CACHE)
        .join(format!(".staging-{}", package.name));
    let _ = std::fs::remove_dir_all(&staging);
    std::fs::create_dir_all(&staging).map_err(|source| FetchError::Io {
        doing: "create",
        path: staging.clone(),
        source,
    })?;

    let result = unpack(&bytes, &staging).and_then(|()| {
        hash_tree(&staging).map_err(|e| FetchError::Source {
            name: package.name.clone(),
            message: e.to_string(),
        })
    });

    let hash = match result {
        Ok(hash) => hash,
        Err(error) => {
            let _ = std::fs::remove_dir_all(&staging);
            return Err(error);
        }
    };

    let destination = cached_at(config_dir, &hash);
    if destination.exists() {
        // `[R-PKG-021]`: keyed by hash, so this is already the same bytes.
        let _ = std::fs::remove_dir_all(&staging);
        return Ok(hash);
    }

    std::fs::rename(&staging, &destination).map_err(|source| FetchError::Io {
        doing: "move into place",
        path: destination,
        source,
    })?;

    Ok(hash)
}

/// Extract a gzipped tar into a directory, refusing anything that leaves it.
///
/// The `tar` crate refuses a path that escapes, and this checks as well
/// rather than instead: an archive that writes outside the directory it was
/// unpacked into is the oldest vulnerability in the format, and two cheap
/// checks are better than one relied upon.
fn unpack(bytes: &[u8], into: &Path) -> Result<(), FetchError> {
    let decoder = flate2::read::GzDecoder::new(bytes);
    let mut archive = tar::Archive::new(decoder);

    let entries = archive.entries().map_err(|source| FetchError::Io {
        doing: "read the archive in",
        path: into.to_path_buf(),
        source,
    })?;

    for entry in entries {
        let mut entry = entry.map_err(|source| FetchError::Io {
            doing: "read an entry of the archive in",
            path: into.to_path_buf(),
            source,
        })?;

        let path = entry
            .path()
            .map_err(|source| FetchError::Io {
                doing: "read an entry's name in",
                path: into.to_path_buf(),
                source,
            })?
            .into_owned();

        if path.components().any(|c| {
            matches!(
                c,
                std::path::Component::ParentDir | std::path::Component::RootDir
            )
        }) {
            return Err(FetchError::Io {
                doing: "unpack",
                path: into.join(&path),
                source: std::io::Error::other(format!(
                    "`{}` would be written outside the package",
                    path.display()
                )),
            });
        }

        // `unpack_in` answers `false` for an entry it declined to write
        // rather than failing, which is how an escaping archive becomes an
        // empty package that pins and loads with nothing saying why. The
        // check above catches the ordinary `..`; this catches whatever it
        // did not, and neither is redundant while the failure mode is
        // silence.
        let written = entry.unpack_in(into).map_err(|source| FetchError::Io {
            doing: "unpack into",
            path: into.to_path_buf(),
            source,
        })?;
        if !written {
            return Err(FetchError::Io {
                doing: "unpack",
                path: into.to_path_buf(),
                source: std::io::Error::other(format!(
                    "`{}` would be written outside the package",
                    path.display()
                )),
            });
        }
    }

    Ok(())
}

/// Fetch every declared package and write the lockfile.
///
/// `[R-PKG-013]`: `update` re-resolves and rewrites; `fetch` downloads what is
/// already pinned and leaves the lockfile alone.
///
/// # Errors
///
/// [`FetchError`] from the first package that cannot be obtained.
pub async fn update(
    config_dir: &Path,
    packages: &[Package],
    cancel: &CancellationToken,
) -> Result<Lock, FetchError> {
    let mut lock = Lock::default();
    for package in packages {
        let hash = fetch(config_dir, package, cancel).await?;
        lock.packages.insert(
            package.name.clone(),
            Pin {
                source: package.source.clone(),
                version: package.version.clone(),
                hash,
            },
        );
    }
    Ok(lock)
}

/// Download what the lockfile already pins, changing nothing.
///
/// # Errors
///
/// [`FetchError`] when a package cannot be obtained, or when what arrives is
/// not what is pinned.
pub async fn ensure(
    config_dir: &Path,
    packages: &[Package],
    lock: &Lock,
    cancel: &CancellationToken,
) -> Result<usize, FetchError> {
    let mut got = 0;
    for package in packages {
        let Some(pin) = lock.packages.get(&package.name) else {
            return Err(FetchError::Source {
                name: package.name.clone(),
                message: "is not in `meow.lock`; run `meow pkg update`".to_owned(),
            });
        };
        if cached_at(config_dir, &pin.hash).exists() {
            continue;
        }

        let found = fetch(config_dir, package, cancel).await?;
        if found != pin.hash {
            return Err(FetchError::Mismatch {
                name: package.name.clone(),
                expected: pin.hash.clone(),
                found,
            });
        }
        got += 1;
    }
    Ok(got)
}
