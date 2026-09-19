// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Which files the index sees, and how they are cut up.
#![allow(clippy::unwrap_used)]

use std::path::{Path, PathBuf};

use meow_index::{Chunking, Walk};
use tempfile::TempDir;

/// A tree with the files given, relative to its root.
fn tree(files: &[(&str, &[u8])]) -> TempDir {
    let dir = tempfile::tempdir().unwrap();
    for (name, body) in files {
        let path = dir.path().join(name);
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, body).unwrap();
    }
    dir
}

/// The indexed paths, relative to the root and sorted.
fn indexed(dir: &TempDir) -> Vec<String> {
    let walked = Walk::default().run(dir.path()).unwrap();
    relative(dir.path(), &walked.files)
}

fn relative(root: &Path, paths: &[PathBuf]) -> Vec<String> {
    let root = root.canonicalize().unwrap();
    let mut out: Vec<String> = paths
        .iter()
        .map(|p| {
            p.strip_prefix(&root)
                .unwrap_or(p)
                .to_string_lossy()
                .replace('\\', "/")
        })
        .collect();
    out.sort();
    out
}

/// [R-INDEX-001] .gitignore is respected
#[test]
fn a_gitignored_file_is_not_indexed() {
    let dir = tree(&[
        (".gitignore", b"target/\n*.log\n"),
        ("src/main.rs", b"fn main() {}\n"),
        ("target/debug/thing.rs", b"generated\n"),
        ("run.log", b"noise\n"),
    ]);

    assert_eq!(indexed(&dir), [".gitignore", "src/main.rs"]);
}

/// [R-INDEX-001] .meowignore can bring back something .gitignore excludes
#[test]
fn meowignore_can_re_include_what_git_excludes() {
    let dir = tree(&[
        (".gitignore", b"generated/\n"),
        (".meowignore", b"!generated/api.rs\n"),
        ("generated/api.rs", b"pub fn call() {}\n"),
        ("generated/other.rs", b"pub fn skip() {}\n"),
    ]);

    let files = indexed(&dir);
    assert!(
        files.contains(&"generated/api.rs".to_owned()),
        "the negation did not take: {files:?}"
    );
    assert!(
        !files.contains(&"generated/other.rs".to_owned()),
        "the negation took too much: {files:?}"
    );
}

/// [R-INDEX-001] .meow/.data/ is never indexed, whatever an ignore file says
#[test]
fn the_data_directory_cannot_be_re_included() {
    let dir = tree(&[
        (".meowignore", b"!.meow/.data/**\n"),
        (".meow/.data/meow.db", b"not really a database\n"),
        ("src/main.rs", b"fn main() {}\n"),
    ]);

    let files = indexed(&dir);
    assert!(
        !files.iter().any(|f| f.contains(".data")),
        "the store was indexed: {files:?}"
    );
}

/// [R-INDEX-002] a binary file is skipped and counted
#[test]
fn a_binary_file_is_skipped_and_recorded() {
    let dir = tree(&[
        ("src/main.rs", b"fn main() {}\n"),
        ("assets/logo.png", &[0x89, b'P', b'N', b'G', 0x00, 0x01]),
    ]);

    let walked = Walk::default().run(dir.path()).unwrap();

    assert_eq!(relative(dir.path(), &walked.files), ["src/main.rs"]);
    assert_eq!(relative(dir.path(), &walked.binary), ["assets/logo.png"]);
    assert_eq!(walked.skipped(), 1);
}

/// [R-INDEX-003] a file over the limit is skipped and reported by name
#[test]
fn a_large_file_is_skipped_and_named() {
    let big = vec![b'x'; 200];
    let dir = tree(&[("src/main.rs", b"fn main() {}\n"), ("data/big.txt", &big)]);

    let walk = Walk { max_bytes: 100 };
    let walked = walk.run(dir.path()).unwrap();

    assert_eq!(relative(dir.path(), &walked.files), ["src/main.rs"]);
    assert_eq!(walked.too_large.len(), 1);
    assert!(
        walked.too_large[0].0.ends_with("big.txt"),
        "{:?}",
        walked.too_large
    );
    assert_eq!(walked.too_large[0].1, 200, "the size is what explains it");
}

/// [R-INDEX-004] a symbolic link out of the workspace is not followed
#[cfg(unix)]
#[test]
fn a_link_out_of_the_workspace_is_refused() {
    let outside = tree(&[("secret.txt", b"not yours\n")]);
    let dir = tree(&[("src/main.rs", b"fn main() {}\n")]);

    std::os::unix::fs::symlink(
        outside.path().join("secret.txt"),
        dir.path().join("link.txt"),
    )
    .unwrap();

    let walked = Walk::default().run(dir.path()).unwrap();

    assert_eq!(relative(dir.path(), &walked.files), ["src/main.rs"]);
    assert_eq!(relative(dir.path(), &walked.escaping), ["link.txt"]);
}

/// [R-INDEX-005] prose is indexed on the same terms as code
#[test]
fn prose_is_indexed_alongside_code() {
    let dir = tree(&[
        ("src/main.rs", b"fn main() {}\n"),
        ("README.md", b"# A project\n"),
        ("notes.txt", b"a note\n"),
        ("docs/design.md", b"# Design\n"),
    ]);

    assert_eq!(
        indexed(&dir),
        ["README.md", "docs/design.md", "notes.txt", "src/main.rs"]
    );
}

/// [R-INDEX-010] the same content produces the same chunks every time
#[test]
fn chunking_is_deterministic() {
    let text: String = (1..=200).map(|n| format!("line {n}\n")).collect();
    let chunking = Chunking::default();

    let first = chunking.split(Path::new("a.rs"), &text).unwrap();
    let second = chunking.split(Path::new("a.rs"), &text).unwrap();

    assert_eq!(first, second);
    assert!(first.len() > 1, "one chunk proves nothing about boundaries");
}

/// [R-INDEX-011] a chunk can be cited: path, bytes, and lines
#[test]
fn a_chunk_carries_where_it_came_from() {
    let text = "alpha\nbravo\ncharlie\ndelta\n";
    let chunks = Chunking {
        lines: 2,
        overlap: 0,
        ..Chunking::default()
    }
    .split(Path::new("src/a.rs"), text)
    .unwrap();

    assert_eq!(chunks.len(), 2);
    assert_eq!(chunks[0].path, Path::new("src/a.rs"));
    assert_eq!(chunks[0].first_line, 1);
    assert_eq!(chunks[0].last_line, 2);
    assert_eq!(&text[chunks[0].start..chunks[0].end], "alpha\nbravo\n");
    assert_eq!(chunks[1].first_line, 3);
    assert_eq!(chunks[1].last_line, 4);
    assert_eq!(&text[chunks[1].start..chunks[1].end], "charlie\ndelta\n");
}

/// [R-INDEX-012] neighbouring chunks share the configured number of lines
#[test]
fn neighbouring_chunks_overlap() {
    let text: String = (1..=10).map(|n| format!("line {n}\n")).collect();
    let chunks = Chunking {
        lines: 4,
        overlap: 2,
        ..Chunking::default()
    }
    .split(Path::new("a.rs"), &text)
    .unwrap();

    assert!(chunks.len() >= 3, "{chunks:?}");
    assert_eq!(chunks[0].first_line, 1);
    assert_eq!(chunks[0].last_line, 4);
    assert_eq!(chunks[1].first_line, 3, "the overlap is two lines");
    assert_eq!(chunks[1].last_line, 6);

    // A definition on lines 4 and 5 is whole in the second chunk, which is
    // what the overlap buys.
    assert!(chunks[1].text.contains("line 4"));
    assert!(chunks[1].text.contains("line 5"));
}

/// [R-INDEX-013] no chunk exceeds the limit
#[test]
fn no_chunk_exceeds_the_limit() {
    let text: String = (1..=100)
        .map(|n| format!("{n} {}\n", "x".repeat(50)))
        .collect();
    let chunking = Chunking {
        lines: 60,
        overlap: 10,
        max_chars: 400,
    };

    for chunk in chunking.split(Path::new("a.rs"), &text).unwrap() {
        assert!(
            chunk.text.chars().count() <= 400,
            "{}:{}-{} is {} characters",
            chunk.path.display(),
            chunk.first_line,
            chunk.last_line,
            chunk.text.chars().count()
        );
    }
}

/// [R-INDEX-014] a chunk begins and ends on a line boundary
#[test]
fn chunks_fall_on_line_boundaries() {
    let text: String = (1..=50).map(|n| format!("line {n}\n")).collect();
    let chunking = Chunking {
        lines: 7,
        overlap: 2,
        max_chars: 60,
    };

    for chunk in chunking.split(Path::new("a.rs"), &text).unwrap() {
        assert!(!chunk.split_line);
        assert!(
            chunk.start == 0 || text.as_bytes()[chunk.start - 1] == b'\n',
            "a chunk starts part-way through a line: {chunk:?}"
        );
        assert!(
            chunk.end == text.len() || text.as_bytes()[chunk.end - 1] == b'\n',
            "a chunk ends part-way through a line: {chunk:?}"
        );
    }
}

/// [R-INDEX-014] a line longer than the whole limit is split, and says so
#[test]
fn an_overlong_line_is_split_and_reported() {
    let text = format!("short\n{}\nshort\n", "y".repeat(500));
    let chunking = Chunking {
        lines: 10,
        overlap: 0,
        max_chars: 100,
    };

    let chunks = chunking.split(Path::new("min.js"), &text).unwrap();

    let split: Vec<_> = chunks.iter().filter(|c| c.split_line).collect();
    assert!(!split.is_empty(), "the long line was not split: {chunks:?}");
    assert!(
        split.iter().all(|c| c.first_line == 2 && c.last_line == 2),
        "the split pieces do not name the line they came from: {split:?}"
    );
    assert!(
        chunks.iter().all(|c| c.text.chars().count() <= 100),
        "a piece is still over the limit"
    );

    // Everything is still there, in order.
    let rebuilt: String = chunks
        .iter()
        .filter(|c| c.split_line)
        .map(|c| c.text.as_str())
        .collect();
    assert_eq!(rebuilt, format!("{}\n", "y".repeat(500)));
}

/// [R-INDEX-030] the fingerprint changes when the parameters do
#[test]
fn the_fingerprint_covers_the_parameters() {
    let a = Chunking::default();
    let b = Chunking {
        lines: a.lines + 1,
        ..a
    };
    let c = Chunking {
        overlap: a.overlap + 1,
        ..a
    };

    assert_ne!(a.fingerprint(), b.fingerprint());
    assert_ne!(a.fingerprint(), c.fingerprint());
    assert_eq!(a.fingerprint(), Chunking::default().fingerprint());
}

/// An empty file produces nothing rather than an empty chunk.
#[test]
fn an_empty_file_produces_no_chunks() {
    assert!(
        Chunking::default()
            .split(Path::new("a.rs"), "")
            .unwrap()
            .is_empty()
    );
}
