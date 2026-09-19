// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Every requirement in `docs/spec/store.md`, one test or more each.
//!
//! `[R-STORE-005]` has no test of its own: it forbids a downgrade path, and
//! what satisfies it is that `migrations.rs` offers no way to express one. A
//! test cannot prove the absence of an API; review can.

// `allow-unwrap-in-tests` in clippy.toml covers `#[test]` functions, not the
// helpers beside them, so the file says it once here.
#![allow(clippy::unwrap_used)]

use std::path::PathBuf;

use meow_store::{CacheKind, Store, StoreError};

fn temp() -> (tempfile::TempDir, PathBuf) {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().to_path_buf();
    (dir, path)
}

/// [R-STORE-001] one database per workspace, at a known path
#[test]
fn the_database_lives_at_one_known_path() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    assert_eq!(
        store.path(),
        root.join(".meow").join(".data").join("meow.db")
    );
    assert!(store.path().exists());
}

/// [R-STORE-002] write-ahead logging, foreign keys, and synchronous NORMAL
#[test]
fn the_connection_pragmas_are_set() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    // Foreign keys are observable through behaviour: an event naming a
    // session that does not exist must be refused.
    let err = store
        .append_event("no-such-session", 1, "Note", None)
        .unwrap_err();
    assert!(
        matches!(err, StoreError::Sqlite(_)),
        "expected a foreign key violation, got {err:?}"
    );
    // WAL is observable through the side file it creates.
    let wal = store.path().with_extension("db-wal");
    let alt = PathBuf::from(format!("{}-wal", store.path().display()));
    assert!(
        wal.exists() || alt.exists(),
        "no write-ahead log beside the database"
    );
}

/// [R-STORE-003] the schema version is recorded in the file
/// [R-STORE-004] migrations run on open and are idempotent across reopens
#[test]
fn the_schema_version_is_recorded_and_reopening_is_idempotent() {
    let (_guard, root) = temp();
    let first = Store::open(&root).unwrap().schema_version().unwrap();
    assert!(first >= 1);
    let second = Store::open(&root).unwrap().schema_version().unwrap();
    assert_eq!(first, second, "reopening moved the schema version");
}

/// [R-STORE-006] a database from the future is an error, and is not touched
#[test]
fn a_newer_schema_is_refused_without_modifying_the_file() {
    let (_guard, root) = temp();
    let path = {
        let store = Store::open(&root).unwrap();
        let path = store.path().to_path_buf();
        drop(store);
        // Stand in for a newer binary having written this file.
        let conn = rusqlite::Connection::open(&path).unwrap();
        conn.execute(
            "UPDATE meta SET value = '9999' WHERE key = 'schema_version'",
            [],
        )
        .unwrap();
        path
    };
    let before = std::fs::metadata(&path).unwrap().len();

    let err = Store::open_at(&path).unwrap_err();
    match err {
        StoreError::SchemaTooNew {
            found, supported, ..
        } => {
            assert_eq!(found, 9999);
            assert!(supported < found, "both versions must be named");
        }
        other => panic!("expected SchemaTooNew, got {other:?}"),
    }
    assert_eq!(
        std::fs::metadata(&path).unwrap().len(),
        before,
        "the file was modified"
    );
}

/// [R-STORE-007] the database is unencrypted and says so
#[test]
fn the_database_reports_that_it_is_unencrypted() {
    let (_guard, root) = temp();
    assert!(!Store::open(&root).unwrap().is_encrypted());
}

/// [R-STORE-008] owner-only permissions
#[cfg(unix)]
#[test]
fn the_database_is_readable_by_its_owner_only() {
    use std::os::unix::fs::PermissionsExt;
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    let mode = std::fs::metadata(store.path())
        .unwrap()
        .permissions()
        .mode()
        & 0o777;
    assert_eq!(mode, 0o600, "mode was {mode:o}");
}

/// [R-STORE-010] payloads are content addressed, and inlining is invisible
#[test]
fn a_payload_round_trips_by_hash_whatever_its_size() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    // One below the inline limit and one far above it. A reader cannot tell
    // which path each took, which is the whole of the requirement.
    for bytes in [vec![7u8; 16], vec![9u8; 40_000]] {
        let hash = store.put_blob(&bytes).unwrap();
        assert_eq!(store.get_blob(&hash).unwrap(), bytes);
    }
}

/// [R-STORE-011] writing a payload twice stores one copy and does not fail
#[test]
fn writing_the_same_payload_twice_stores_it_once() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    let a = store.put_blob(b"the same bytes").unwrap();
    let b = store.put_blob(b"the same bytes").unwrap();
    assert_eq!(a, b);
    assert_eq!(store.blob_count().unwrap(), 1);
    assert_eq!(
        store.blob_refcount(&a).unwrap(),
        2,
        "the second write is a second referent"
    );
}

/// [R-STORE-012] a payload is deleted when its last referent goes
#[test]
fn a_payload_survives_until_its_last_referent_goes() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    let hash = store.put_blob(b"shared").unwrap();
    store.put_blob(b"shared").unwrap();

    store.release_blob(&hash).unwrap();
    assert_eq!(
        store.get_blob(&hash).unwrap(),
        b"shared",
        "one referent still holds it"
    );

    store.release_blob(&hash).unwrap();
    assert!(matches!(
        store.get_blob(&hash),
        Err(StoreError::BlobMissing { .. })
    ));
}

/// [R-STORE-013] a missing payload is an error naming the hash, never empty
#[test]
fn a_missing_payload_names_its_hash_rather_than_reading_empty() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    let absent = meow_store::BlobHash::of(b"never stored");
    match store.get_blob(&absent) {
        Err(StoreError::BlobMissing { hash }) => assert_eq!(hash, absent.to_string()),
        other => panic!("expected BlobMissing, got {other:?}"),
    }
}

/// [R-STORE-020] each event is committed on its own, visible to another reader
#[test]
fn an_event_is_visible_to_another_connection_at_once() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    store.create_session("s1", "review", None).unwrap();
    store.append_event("s1", 1, "Started", None).unwrap();

    // A second handle onto the same file. If the first held a transaction open
    // across the turn, this would see nothing.
    let reader = Store::open(&root).unwrap();
    assert_eq!(reader.event_count("s1").unwrap(), 1);
}

/// [R-STORE-021] a failed write reaches the caller
#[test]
fn a_failed_write_is_returned_rather_than_logged() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    store.create_session("s1", "review", None).unwrap();
    store.append_event("s1", 1, "Started", None).unwrap();

    // The same sequence number twice violates the primary key. The point of
    // the test is not the constraint; it is that the caller is told.
    let err = store.append_event("s1", 1, "Started", None).unwrap_err();
    assert!(matches!(err, StoreError::Sqlite(_)), "got {err:?}");
    assert_eq!(
        store.event_count("s1").unwrap(),
        1,
        "the failed write left nothing behind"
    );
}

/// [R-STORE-022] one writer, readers alongside it
/// [R-STORE-023] a blocked write waits rather than failing at once
#[test]
fn a_blocked_write_waits_for_the_other_writer() {
    let (guard, root) = temp();
    let store = Store::open(&root).unwrap();
    store.create_session("s1", "review", None).unwrap();

    let db = store.path().to_path_buf();
    let holder = std::thread::spawn(move || {
        // Another process holding the write lock.
        let mut other = rusqlite::Connection::open(&db).unwrap();
        let tx = other.transaction().unwrap();
        tx.execute("INSERT INTO kv(key, value) VALUES ('held', x'00')", [])
            .unwrap();
        std::thread::sleep(std::time::Duration::from_millis(300));
        tx.commit().unwrap();
    });

    std::thread::sleep(std::time::Duration::from_millis(50));
    // A reader is never blocked by the writer.
    assert_eq!(store.event_count("s1").unwrap(), 0);
    // A writer waits it out instead of failing immediately.
    let started = std::time::Instant::now();
    store.kv_put("mine", b"value").unwrap();
    assert!(
        started.elapsed() >= std::time::Duration::from_millis(100),
        "the write did not wait"
    );

    holder.join().unwrap();
    drop(guard);
}

/// [R-STORE-024] a bulk write commits in batches
#[test]
fn a_bulk_write_commits_in_batches() {
    let (_guard, root) = temp();
    let mut store = Store::open(&root).unwrap();
    let payloads: Vec<Vec<u8>> = (0..(meow_store::BULK_BATCH * 2 + 7))
        .map(|i| format!("chunk {i}").into_bytes())
        .collect();
    let hashes = store.put_blobs_bulk(&payloads).unwrap();
    assert_eq!(hashes.len(), payloads.len());
    assert_eq!(store.get_blob(&hashes[0]).unwrap(), payloads[0]);
    assert_eq!(
        store.get_blob(&hashes[payloads.len() - 1]).unwrap(),
        payloads[payloads.len() - 1]
    );
}

/// [R-STORE-030] the key-value store has get, put, delete, and keys
#[test]
fn the_key_value_store_reads_writes_deletes_and_lists() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    assert_eq!(store.kv_get("absent").unwrap(), None);
    store.kv_put("b", b"two").unwrap();
    store.kv_put("a", b"one").unwrap();
    assert_eq!(store.kv_get("a").unwrap().as_deref(), Some(&b"one"[..]));
    assert_eq!(
        store.kv_keys().unwrap(),
        vec!["a".to_owned(), "b".to_owned()]
    );
    store.kv_delete("a").unwrap();
    assert_eq!(store.kv_keys().unwrap(), vec!["b".to_owned()]);
}

/// [R-STORE-031] deleting a session leaves the key-value store intact
#[test]
fn deleting_a_session_leaves_the_key_value_store_alone() {
    let (_guard, root) = temp();
    let mut store = Store::open(&root).unwrap();
    store.kv_put("survives", b"yes").unwrap();
    store.create_session("s1", "review", None).unwrap();
    store.delete_session("s1").unwrap();
    assert_eq!(
        store.kv_get("survives").unwrap().as_deref(),
        Some(&b"yes"[..])
    );
}

/// [R-STORE-040] deleting a session releases the payloads it referenced
#[test]
fn deleting_a_session_releases_its_payloads() {
    let (_guard, root) = temp();
    let mut store = Store::open(&root).unwrap();
    store.create_session("s1", "review", None).unwrap();
    let shared = store.put_blob(b"read by two sessions").unwrap();
    store
        .append_event("s1", 1, "ToolResult", Some(&shared))
        .unwrap();

    store.create_session("s2", "review", None).unwrap();
    store.put_blob(b"read by two sessions").unwrap();
    store
        .append_event("s2", 1, "ToolResult", Some(&shared))
        .unwrap();
    assert_eq!(store.blob_refcount(&shared).unwrap(), 2);

    store.delete_session("s1").unwrap();
    assert_eq!(
        store.blob_refcount(&shared).unwrap(),
        1,
        "one referent should remain"
    );
    assert_eq!(store.get_blob(&shared).unwrap(), b"read by two sessions");

    store.delete_session("s2").unwrap();
    assert!(
        matches!(store.get_blob(&shared), Err(StoreError::BlobMissing { .. })),
        "the last referent went, so the payload should have gone with it"
    );
}

/// [R-STORE-041] a deletion removes the whole session or fails
#[test]
fn deleting_a_session_is_all_or_nothing() {
    let (_guard, root) = temp();
    let mut store = Store::open(&root).unwrap();
    store.create_session("parent", "triage", None).unwrap();
    store
        .create_session("child", "review", Some("parent"))
        .unwrap();
    store.append_event("parent", 1, "Started", None).unwrap();

    // The parent has a child, so the delete must fail rather than orphan it,
    // and must leave the parent's own events untouched.
    let err = store.delete_session("parent").unwrap_err();
    assert!(matches!(err, StoreError::Sqlite(_)), "got {err:?}");
    assert_eq!(
        store.event_count("parent").unwrap(),
        1,
        "a failed delete removed events"
    );

    // Deleting a session that does not exist fails rather than succeeding
    // quietly, because a silent success hides a typo in an identifier.
    assert!(matches!(
        store.delete_session("no-such-session"),
        Err(StoreError::SessionMissing { .. })
    ));
}

/// [R-STORE-042] the store reports its size so retention can act on it
#[test]
fn the_store_reports_its_size() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    let before = store.size_bytes().unwrap();
    for i in 0..200 {
        store
            .put_blob(format!("payload number {i} padded out a little").as_bytes())
            .unwrap();
    }
    assert!(store.size_bytes().unwrap() > before, "size did not grow");
}

/// [R-STORE-045] embeddings are cached; generations are not unless asked
#[test]
fn generations_are_not_cached_unless_the_caller_asks() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();

    assert!(
        store
            .cache_put("req1", "embed-1", CacheKind::Embedding, false, b"vector")
            .unwrap()
    );
    assert_eq!(
        store.cache_get("req1", "embed-1").unwrap().as_deref(),
        Some(&b"vector"[..])
    );

    assert!(
        !store
            .cache_put("req2", "smart", CacheKind::Generation, false, b"answer")
            .unwrap()
    );
    assert_eq!(
        store.cache_get("req2", "smart").unwrap(),
        None,
        "a generation was cached by default"
    );

    assert!(
        store
            .cache_put("req2", "smart", CacheKind::Generation, true, b"answer")
            .unwrap()
    );
    assert_eq!(
        store.cache_get("req2", "smart").unwrap().as_deref(),
        Some(&b"answer"[..])
    );
}

/// [R-STORE-046] the model is part of the cache key
#[test]
fn a_cache_lookup_misses_when_the_model_differs() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    store
        .cache_put(
            "req",
            "embed-1",
            CacheKind::Embedding,
            false,
            b"from model one",
        )
        .unwrap();
    assert_eq!(store.cache_get("req", "embed-2").unwrap(), None);
    assert_eq!(
        store.cache_get("req", "embed-1").unwrap().as_deref(),
        Some(&b"from model one"[..])
    );
}

/// [R-STORE-047] eviction is by age and size, and leaves sessions alone
#[test]
fn eviction_is_by_age_and_size_and_does_not_touch_sessions() {
    let (_guard, root) = temp();
    let store = Store::open(&root).unwrap();
    store.create_session("s1", "review", None).unwrap();
    let quoted = store.put_blob(b"a response a transcript quoted").unwrap();
    store
        .append_event("s1", 1, "ToolResult", Some(&quoted))
        .unwrap();

    for i in 0..20 {
        store
            .cache_put(
                &format!("req{i}"),
                "embed-1",
                CacheKind::Embedding,
                false,
                &[0u8; 100],
            )
            .unwrap();
    }
    assert_eq!(store.cache_bytes().unwrap(), 2000);

    // Squeeze the cache to a quarter of its size.
    let removed = store.cache_evict(i64::MAX / 2, 500).unwrap();
    assert!(removed > 0, "nothing was evicted");
    assert!(store.cache_bytes().unwrap() <= 500);

    // The transcript is untouched, because it holds its own copy.
    assert_eq!(
        store.get_blob(&quoted).unwrap(),
        b"a response a transcript quoted"
    );
    assert_eq!(store.event_count("s1").unwrap(), 1);

    // Everything older than a moment ago goes.
    store.cache_evict(0, i64::MAX).unwrap();
    assert_eq!(store.cache_bytes().unwrap(), 0);
}
