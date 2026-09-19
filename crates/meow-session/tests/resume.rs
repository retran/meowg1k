// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Continuing a session rather than starting one.
#![allow(clippy::unwrap_used)]

use meow_core::{EventKind, SessionId, StopReason};
use meow_session::Sessions;
use meow_store::Store;
use tempfile::TempDir;

fn id(n: u8) -> SessionId {
    SessionId::new(1_700_000_000_000 + u64::from(n), [n; 10])
}

fn sessions() -> (TempDir, Sessions) {
    let dir = tempfile::tempdir().unwrap();
    let store = Store::open(&dir.path().join("meow.db")).unwrap();
    (dir, Sessions::new(store))
}

/// [R-SESSION-050] resuming appends to the same session and the same sequence
#[test]
fn resuming_grows_one_session_rather_than_making_another() {
    let (_dir, sessions) = sessions();
    let session = id(1);

    sessions
        .start(&session, "review", "first question", None)
        .unwrap();
    sessions
        .append(
            &session,
            EventKind::Assistant {
                content: "first answer".to_owned(),
                tool_calls: Vec::new(),
            },
        )
        .unwrap();
    sessions
        .finish(&session, StopReason::Finished, None)
        .unwrap();

    let before = sessions.events(&session).unwrap();
    sessions.resume(&session, "second question").unwrap();
    let after = sessions.events(&session).unwrap();

    assert_eq!(after.len(), before.len() + 1);
    assert_eq!(after[before.len()].seq, before.len() as u64 + 1);
    assert_eq!(sessions.list(None, 10).unwrap().len(), 1);

    // The event that ended the first run is untouched.
    assert_eq!(after[before.len() - 1].kind, before[before.len() - 1].kind);
}

/// [R-SESSION-051] a resumed run sees the conversation the way the original
/// did, summarised ranges and all
#[test]
fn a_resumed_run_sees_what_the_original_saw() {
    let (_dir, sessions) = sessions();
    let session = id(1);

    sessions
        .start(&session, "review", "look at it", None)
        .unwrap();
    for n in 1..=3 {
        sessions
            .append(
                &session,
                EventKind::Assistant {
                    content: format!("thought {n}"),
                    tool_calls: Vec::new(),
                },
            )
            .unwrap();
    }
    sessions
        .compact(&session, 2..=3, "the model thought twice", 40)
        .unwrap();

    let rebuilt = sessions.events_for_model(&session).unwrap();
    let said: Vec<String> = rebuilt
        .iter()
        .filter_map(|e| match &e.kind {
            EventKind::Assistant { content, .. } => Some(content.clone()),
            EventKind::Compaction { summary, .. } => Some(summary.clone()),
            _ => None,
        })
        .collect();

    assert_eq!(said, ["the model thought twice", "thought 3"]);
    assert!(
        !said.iter().any(|s| s == "thought 1"),
        "a superseded event reached the rebuild: {said:?}"
    );

    // And the full log still has everything, which is what makes the
    // supersession reversible.
    assert_eq!(sessions.events(&session).unwrap().len(), 5);
}

/// [R-SESSION-054] nothing infers a continuation
#[test]
fn a_run_that_names_no_session_starts_one() {
    let (_dir, sessions) = sessions();

    sessions.start(&id(1), "review", "first", None).unwrap();
    sessions.start(&id(2), "review", "second", None).unwrap();

    let all = sessions.list(Some("review"), 10).unwrap();
    assert_eq!(all.len(), 2, "the second run joined the first");

    // The newest is first, which is what `--continue` picks up.
    assert_eq!(all[0].id, id(2));
}
