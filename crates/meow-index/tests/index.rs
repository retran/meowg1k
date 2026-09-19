// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Embedding, keeping the index current, and answering from it.
#![allow(clippy::unwrap_used)]

use std::sync::Mutex;

use meow_index::embed::{self, Rejected};
use meow_index::{Embed, Index, IndexError, Query};
use meow_store::Store;
use tempfile::TempDir;

/// An embedder with no network behind it.
///
/// The vector is a count of a few marker words, so two texts about the same
/// thing land near each other and a test can say which result should win
/// without depending on a model.
#[derive(Debug)]
struct Counting {
    model: String,
    calls: Mutex<Vec<usize>>,
    /// Refuse a batch larger than this, the way a provider does.
    max_batch: usize,
    /// Refuse any text longer than this, however small the batch.
    max_chars: usize,
}

impl Counting {
    fn new(model: &str) -> Self {
        Self {
            model: model.to_owned(),
            calls: Mutex::new(Vec::new()),
            max_batch: usize::MAX,
            max_chars: usize::MAX,
        }
    }

    fn batches(&self) -> Vec<usize> {
        self.calls.lock().unwrap().clone()
    }
}

const MARKERS: [&str; 4] = ["retry", "budget", "session", "colour"];

impl Embed for Counting {
    fn model(&self) -> &str {
        &self.model
    }

    fn embed(&self, texts: &[String]) -> Result<Vec<Vec<f32>>, Rejected> {
        self.calls.lock().unwrap().push(texts.len());

        if texts.len() > self.max_batch {
            return Err(Rejected::TooLarge);
        }
        if texts.iter().any(|t| t.chars().count() > self.max_chars) {
            return Err(Rejected::TooLarge);
        }

        Ok(texts
            .iter()
            .map(|text| {
                let lower = text.to_lowercase();
                MARKERS
                    .iter()
                    .map(|m| lower.matches(m).count() as f32)
                    .collect()
            })
            .collect())
    }
}

fn workspace(files: &[(&str, &str)]) -> TempDir {
    let dir = tempfile::tempdir().unwrap();
    for (name, body) in files {
        let path = dir.path().join(name);
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, body).unwrap();
    }
    dir
}

fn index(dir: &TempDir) -> Index {
    let store = Store::open(dir.path()).unwrap();
    Index::new(store, dir.path())
}

const FILES: &[(&str, &str)] = &[
    (
        "src/retry.rs",
        "// the retry budget bounds how often we retry\nfn retry() {}\n",
    ),
    (
        "src/colour.rs",
        "// the colour palette is quantised\nfn colour() {}\n",
    ),
    ("docs/sessions.md", "# Sessions\n\nA session is a log.\n"),
];

/// [R-INDEX-020] a batch a provider refuses is split and retried
#[test]
fn a_refused_batch_is_split_and_retried() {
    let mut embedder = Counting::new("small");
    embedder.max_batch = 2;

    let texts: Vec<String> = (1..=8).map(|n| format!("text {n}")).collect();
    let vectors = embed::in_batches(&embedder, &texts, 8).unwrap();

    assert_eq!(vectors.len(), 8);

    let batches = embedder.batches();
    assert_eq!(batches[0], 8, "the first attempt is the whole batch");
    assert!(
        batches.iter().skip(1).all(|n| *n <= 4),
        "the batch was not halved: {batches:?}"
    );
    assert_eq!(
        batches.iter().skip(1).filter(|n| **n == 2).count() * 2,
        8,
        "not every text was embedded exactly once: {batches:?}"
    );
}

/// [R-INDEX-021] one chunk a provider refuses names the file and the lines
#[test]
fn an_oversized_chunk_names_the_file_and_the_lines() {
    let long = "retry ".repeat(200);
    let dir = workspace(&[("src/big.rs", &format!("// short\n{long}\n// short\n"))]);

    let mut index = index(&dir);
    index.update().unwrap();

    let mut embedder = Counting::new("small");
    embedder.max_chars = 100;

    let error = index.embed(&embedder, 8).unwrap_err();
    match error {
        IndexError::ChunkTooLarge {
            path, first, last, ..
        } => {
            assert!(path.ends_with("big.rs"), "{path:?}");
            assert!(first >= 1 && last >= first, "{first}-{last}");
        }
        other => panic!("expected a named chunk, got {other}"),
    }

    // And the message says where, which is the whole point.
    let shown = index.embed(&embedder, 8).unwrap_err().to_string();
    assert!(shown.contains("big.rs"), "{shown}");
}

/// [R-INDEX-022] an interrupted build does not re-embed what it already paid
/// for
#[test]
fn embedding_is_resumable() {
    let dir = workspace(FILES);
    let mut index = index(&dir);
    index.update().unwrap();

    let embedder = Counting::new("small");
    let first = index.embed(&embedder, 8).unwrap();
    assert!(first > 0);

    let (total, embedded) = index.counts().unwrap();
    assert_eq!(total, embedded, "not everything was embedded");

    // A second run finds nothing left to do and asks the provider nothing.
    let before = embedder.batches().len();
    let second = index.embed(&embedder, 8).unwrap();

    assert_eq!(second, 0);
    assert_eq!(embedder.batches().len(), before, "it asked again");
}

/// [R-INDEX-030] a file is re-chunked only when its hash changes, and the hash
/// covers the chunking parameters
#[test]
fn only_a_changed_file_is_re_chunked() {
    let dir = workspace(FILES);
    let mut index = index(&dir);

    let first = index.update().unwrap();
    assert_eq!(first.added, 3);
    assert_eq!(first.unchanged, 0);

    let again = index.update().unwrap();
    assert_eq!(again.added, 0);
    assert_eq!(again.changed, 0);
    assert_eq!(again.unchanged, 3, "{again:?}");

    std::fs::write(dir.path().join("src/retry.rs"), "// different\n").unwrap();
    let after = index.update().unwrap();
    assert_eq!(after.changed, 1);
    assert_eq!(after.unchanged, 2);
}

/// [R-INDEX-030] changing the chunking makes every file stale
#[test]
fn changing_the_chunking_makes_everything_stale() {
    let dir = workspace(FILES);

    let store = Store::open(dir.path()).unwrap();
    let mut index = Index::new(store, dir.path());
    assert_eq!(index.update().unwrap().added, 3);

    let store = Store::open(dir.path()).unwrap();
    let mut wider = Index::new(store, dir.path()).with_chunking(meow_index::Chunking {
        lines: 5,
        overlap: 1,
        max_chars: 4000,
    });

    let after = wider.update().unwrap();
    assert_eq!(
        after.changed, 3,
        "chunks built by other parameters looked current: {after:?}"
    );
    assert_eq!(after.unchanged, 0);
}

/// [R-INDEX-031] a file that is gone, or newly excluded, leaves nothing behind
#[test]
fn a_removed_or_excluded_file_leaves_nothing() {
    let dir = workspace(FILES);
    let mut index = index(&dir);
    index.update().unwrap();

    std::fs::remove_file(dir.path().join("docs/sessions.md")).unwrap();
    let gone = index.update().unwrap();
    assert_eq!(gone.removed, 1, "{gone:?}");

    // Excluding one is the same case: the walk no longer sees it.
    std::fs::write(dir.path().join(".gitignore"), "src/colour.rs\n").unwrap();
    let excluded = index.update().unwrap();
    assert_eq!(excluded.removed, 1, "{excluded:?}");

    // The `.gitignore` written above is a text file like any other, so it is
    // in the index too.
    let paths = index.store().index_paths().unwrap();
    assert_eq!(paths, [".gitignore", "src/retry.rs"]);
}

/// [R-INDEX-032] an update reports all four outcomes
#[test]
fn an_update_reports_what_it_did() {
    let dir = workspace(FILES);
    let mut index = index(&dir);
    index.update().unwrap();

    std::fs::write(dir.path().join("src/retry.rs"), "// changed\n").unwrap();
    std::fs::write(dir.path().join("src/new.rs"), "// new\n").unwrap();
    std::fs::remove_file(dir.path().join("docs/sessions.md")).unwrap();

    let built = index.update().unwrap();

    assert_eq!(built.added, 1, "{built:?}");
    assert_eq!(built.changed, 1, "{built:?}");
    assert_eq!(built.removed, 1, "{built:?}");
    assert_eq!(built.unchanged, 1, "{built:?}");
}

/// [R-INDEX-040] results are ranked by descending similarity and can be cited
#[test]
fn results_are_ranked_and_citable() {
    let dir = workspace(FILES);
    let mut index = index(&dir);
    index.update().unwrap();

    let embedder = Counting::new("small");
    index.embed(&embedder, 8).unwrap();

    let hits = index
        .query(&embedder, "retry budget", &Query::default())
        .unwrap();

    assert!(!hits.is_empty());
    assert_eq!(hits[0].path, "src/retry.rs", "{hits:?}");
    assert!(hits[0].first_line >= 1);
    assert!(hits[0].last_line >= hits[0].first_line);
    assert!(hits[0].text.contains("retry"));

    for pair in hits.windows(2) {
        assert!(
            pair[0].score >= pair[1].score,
            "results are not in descending order: {hits:?}"
        );
    }
}

/// [R-INDEX-041] an empty index says so and builds nothing
#[test]
fn an_empty_index_says_so() {
    let dir = workspace(FILES);
    let index = index(&dir);
    let embedder = Counting::new("small");

    let error = index
        .query(&embedder, "retry", &Query::default())
        .unwrap_err();

    assert!(matches!(error, IndexError::Empty), "{error}");
    assert!(error.to_string().contains("meow index build"), "{error}");

    // And nothing was built on the way past.
    assert_eq!(index.counts().unwrap(), (0, 0));
    assert!(embedder.batches().is_empty(), "it embedded something");
}

/// [R-INDEX-042] both the limit and the minimum score apply
#[test]
fn a_query_applies_the_limit_and_the_minimum() {
    let dir = workspace(FILES);
    let mut index = index(&dir);
    index.update().unwrap();

    let embedder = Counting::new("small");
    index.embed(&embedder, 8).unwrap();

    let limited = index
        .query(
            &embedder,
            "retry budget session colour",
            &Query {
                limit: 1,
                ..Query::default()
            },
        )
        .unwrap();
    assert_eq!(limited.len(), 1);

    let strict = index
        .query(
            &embedder,
            "retry budget",
            &Query {
                limit: 10,
                min_score: 0.99,
                ..Query::default()
            },
        )
        .unwrap();
    assert!(
        strict.iter().all(|h| h.score >= 0.99),
        "the minimum was not applied: {strict:?}"
    );
    assert!(strict.len() < 3, "nothing was filtered: {strict:?}");
}

/// [R-INDEX-043] the same query twice costs one request
#[test]
fn a_repeated_query_needs_no_second_request() {
    let dir = workspace(FILES);
    let mut index = index(&dir);
    index.update().unwrap();

    let embedder = Counting::new("small");
    index.embed(&embedder, 8).unwrap();

    index.query(&embedder, "retry", &Query::default()).unwrap();
    let after_first = embedder.batches().len();

    index.query(&embedder, "retry", &Query::default()).unwrap();
    assert_eq!(
        embedder.batches().len(),
        after_first,
        "the query was embedded twice"
    );

    // A different query does ask.
    index.query(&embedder, "colour", &Query::default()).unwrap();
    assert_eq!(embedder.batches().len(), after_first + 1);
}

/// [R-INDEX-044] a path filter applies before ranking
#[test]
fn a_path_filter_applies_before_ranking() {
    let dir = workspace(FILES);
    let mut index = index(&dir);
    index.update().unwrap();

    let embedder = Counting::new("small");
    index.embed(&embedder, 8).unwrap();

    let hits = index
        .query(
            &embedder,
            "retry budget",
            &Query {
                limit: 1,
                paths: vec!["docs/**".to_owned()],
                ..Query::default()
            },
        )
        .unwrap();

    // The best match overall is in `src/`, so a filter applied afterwards
    // would have left nothing.
    assert_eq!(hits.len(), 1, "{hits:?}");
    assert!(hits[0].path.starts_with("docs/"), "{hits:?}");
}

/// [R-INDEX-050] the vectors live in the same database as everything else
#[test]
fn the_index_shares_one_database() {
    let dir = workspace(FILES);
    let mut index = index(&dir);
    index.update().unwrap();
    index.embed(&Counting::new("small"), 8).unwrap();

    // The same file the sessions and the cache use.
    let path = dir.path().join(".meow").join(".data");
    let files: Vec<String> = std::fs::read_dir(&path)
        .unwrap()
        .filter_map(|e| Some(e.ok()?.file_name().to_string_lossy().into_owned()))
        .filter(|name| name.ends_with(".db"))
        .collect();

    assert_eq!(files.len(), 1, "{files:?}");
}

/// [R-INDEX-051] a query against an index another model built fails, naming
/// both
#[test]
fn a_query_by_the_wrong_model_names_both() {
    let dir = workspace(FILES);
    let mut index = index(&dir);
    index.update().unwrap();
    index.embed(&Counting::new("small"), 8).unwrap();

    let other = Counting::new("large");
    let error = index.query(&other, "retry", &Query::default()).unwrap_err();

    match &error {
        IndexError::WrongModel { built_by, asked_by } => {
            assert_eq!(built_by, "small");
            assert_eq!(asked_by, "large");
        }
        other => panic!("expected a model mismatch, got {other}"),
    }
    assert!(error.to_string().contains("small"), "{error}");
    assert!(error.to_string().contains("large"), "{error}");
}

/// [R-INDEX-052] clear removes the index and leaves the rest alone
#[test]
fn clear_removes_the_index_and_nothing_else() {
    let dir = workspace(FILES);
    let mut index = index(&dir);
    index.update().unwrap();
    index.embed(&Counting::new("small"), 8).unwrap();

    // Something else in the same database.
    index.store().kv_put("plan", b"keep me").unwrap();
    let session = meow_core::SessionId::new(1_700_000_000_000, [7; 10]);
    index
        .store()
        .create_session(session.as_str(), "review", None)
        .unwrap();

    index.clear().unwrap();

    assert_eq!(index.counts().unwrap(), (0, 0));
    assert!(index.store().index_paths().unwrap().is_empty());
    assert!(index.model().unwrap().is_none());

    assert_eq!(
        index.store().kv_get("plan").unwrap().as_deref(),
        Some(b"keep me".as_slice()),
        "the key-value store was cleared too"
    );
    assert_eq!(
        index.store().event_count(session.as_str()).unwrap(),
        0,
        "the session should still exist to be counted"
    );
    assert!(
        !index.store().list_sessions(None, 10).unwrap().is_empty(),
        "the sessions were cleared too"
    );
}
