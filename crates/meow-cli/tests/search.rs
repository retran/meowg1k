// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Searching a workspace with no index, over the `Search` port directly.
//!
//! `surface.rs` drives the binary, which is the right test for most things and
//! the wrong one here: a process is started with `current_dir`, and on Unix the
//! kernel hands back a resolved path, so the binary never sees the
//! non-canonical root that this file is about.
#![allow(clippy::unwrap_used)]

use meow_cli::index::Unindexed;
use meow_star::port::Search;

/// [R-STAR-019] a root that is not in its canonical spelling still searches
///
/// `Walk::run` canonicalises before it walks and `Workspace::at` does not, so
/// stripping the given root off a walked path matches nothing whenever the two
/// spellings differ. Every file was skipped and the search came back empty -
/// not an error, an empty list.
///
/// A macOS temporary directory is `/var/folders/…`, which resolves to
/// `/private/var/folders/…`, so this reproduces here. On Windows every
/// canonical path differs, by the `\\?\` prefix, which is where CI found it.
#[test]
fn a_root_that_is_not_canonical_still_finds_its_files() {
    let dir = tempfile::tempdir().unwrap();
    std::fs::write(dir.path().join("hay.txt"), "one\ntwo needle three\n").unwrap();

    let search = Unindexed::new(dir.path(), "no index here");

    assert_eq!(
        search.files("**/*.txt", 10).unwrap(),
        vec!["hay.txt".to_owned()],
        "the glob matched nothing under a root spelled differently from the walk's"
    );

    let hits = search.text("needle", false, 10, &[]).unwrap();
    assert_eq!(hits.len(), 1, "the literal matched nothing: {hits:?}");
    assert_eq!(hits[0].path, "hay.txt");
    assert_eq!(hits[0].first_line, 2, "the line number is one-based");
}

/// [R-STAR-025] the searches that need an index still say they have none
#[test]
fn ranking_says_why_it_cannot_rather_than_returning_nothing() {
    let dir = tempfile::tempdir().unwrap();
    let search = Unindexed::new(dir.path(), "this workspace declares no index");

    for outcome in [
        search.code("q", 1, &[]).err(),
        search.query("q", 1, &[], 0.0).err(),
        search.build().err(),
        search.update().err(),
        search.stats().err(),
    ] {
        assert_eq!(
            outcome.as_deref(),
            Some("this workspace declares no index"),
            "a call that needs an index must carry the reason it has none"
        );
    }
}
