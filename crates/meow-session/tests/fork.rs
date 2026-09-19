// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Forking, retention, and export.
#![allow(clippy::unwrap_used)]

use meow_core::{EventKind, SessionId, StopReason, Usage};
use meow_session::{Redaction, Retention, Sessions};
use meow_store::Store;
use tempfile::TempDir;

/// Identifiers are minted by the caller, so a test can name the run it means.
fn id(n: u8) -> SessionId {
    SessionId::new(1_700_000_000_000 + u64::from(n), [n; 10])
}

fn sessions() -> (TempDir, Sessions) {
    let dir = tempfile::tempdir().unwrap();
    let store = Store::open(&dir.path().join("meow.db")).unwrap();
    (dir, Sessions::new(store))
}

/// A run with a task, a tool call with a decision, an answer, and totals.
fn record(sessions: &Sessions, id: &SessionId) {
    sessions
        .start(id, "review", "look at the diff", None)
        .unwrap();
    sessions
        .append(
            id,
            EventKind::Policy {
                id: "c1".to_owned(),
                decision: "allow".to_owned(),
                rule: Some("fs.read src/**".to_owned()),
                source: "rule".to_owned(),
            },
        )
        .unwrap();
    sessions
        .append(
            id,
            EventKind::ToolCall {
                id: "c1".to_owned(),
                name: "fs.read".to_owned(),
                args: r#"{"path":"src/a.rs","token":"hunter2"}"#.to_owned(),
            },
        )
        .unwrap();
    sessions
        .append(
            id,
            EventKind::ToolResult {
                id: "c1".to_owned(),
                output: "fn main() {}".to_owned(),
                duration_ms: 12,
                error: None,
            },
        )
        .unwrap();
    sessions
        .append(
            id,
            EventKind::Assistant {
                content: "the diff is fine".to_owned(),
                tool_calls: Vec::new(),
            },
        )
        .unwrap();
    sessions
        .append(
            id,
            EventKind::Usage(Usage {
                prompt: 100,
                completion: 20,
                cached: None,
                cost_micros: Some(210_000),
            }),
        )
        .unwrap();
    sessions.finish(id, StopReason::Finished, None).unwrap();
}

/// [R-SESSION-052] a fork copies the first n events and leaves the origin
/// alone
#[test]
fn a_fork_copies_the_prefix_and_does_not_touch_the_origin() {
    let (_dir, mut sessions) = sessions();
    let origin = id(1);
    record(&sessions, &origin);

    let before = sessions.events(&origin).unwrap();
    let forked = sessions.fork(&origin, 3, &id(2)).unwrap();

    let copied = sessions.events(&forked).unwrap();
    assert_eq!(copied.len(), 3);
    assert_eq!(copied[0].kind, before[0].kind);
    assert_eq!(copied[2].kind, before[2].kind);

    // The origin is exactly as it was.
    assert_eq!(sessions.events(&origin).unwrap().len(), before.len());
    assert_eq!(sessions.state(&origin).unwrap().as_str(), "finished");
}

/// [R-SESSION-052] the fork records where it came from
#[test]
fn a_fork_records_its_origin() {
    let (_dir, mut sessions) = sessions();
    let origin = id(1);
    record(&sessions, &origin);

    let forked = sessions.fork(&origin, 4, &id(2)).unwrap();
    let recorded = sessions.origin(&forked).unwrap().unwrap();

    assert_eq!(recorded.session, origin);
    assert_eq!(recorded.seq, 4);
    assert!(sessions.origin(&origin).unwrap().is_none());
}

/// [R-SESSION-052] a blob the fork points at gains a referent, so collecting
/// the origin cannot delete it
#[test]
fn forking_retains_the_blobs_the_copy_points_at() {
    let (_dir, mut sessions) = sessions();
    let origin = id(1);
    sessions.start(&origin, "review", "task", None).unwrap();

    // An event with a payload, written through the store, which is the shape
    // a large tool result takes.
    let hash = sessions.store().put_blob(b"a large tool result").unwrap();
    let body = serde_json::to_string(&EventKind::ToolResult {
        id: "c1".to_owned(),
        output: "a large tool result".to_owned(),
        duration_ms: 3,
        error: None,
    })
    .unwrap();
    sessions
        .store()
        .append_with_payload(origin.as_str(), 2, "ToolResult", &body, Some(&hash))
        .unwrap();
    assert_eq!(sessions.store().blob_refcount(&hash).unwrap(), 1);

    sessions.fork(&origin, 2, &id(2)).unwrap();
    assert_eq!(
        sessions.store().blob_refcount(&hash).unwrap(),
        2,
        "the fork does not hold a referent, so collecting the origin would delete its content"
    );
}

/// [R-SESSION-053] a sequence that does not exist fails, naming what would
/// work
#[test]
fn forking_outside_the_range_names_the_range() {
    let (_dir, mut sessions) = sessions();
    let origin = id(1);
    record(&sessions, &origin);

    let error = sessions.fork(&origin, 99, &id(2)).unwrap_err().to_string();
    assert!(error.contains("cannot fork at 99"), "{error}");
    assert!(
        error.contains("1 to 7"),
        "the valid range is missing: {error}"
    );

    let zero = sessions.fork(&origin, 0, &id(3)).unwrap_err().to_string();
    assert!(zero.contains("cannot fork at 0"), "{zero}");
}

/// [R-SESSION-053] a sequence inside a summarised range fails, naming the
/// range
#[test]
fn forking_inside_a_summarised_range_fails() {
    let (_dir, mut sessions) = sessions();
    let origin = id(1);
    record(&sessions, &origin);

    sessions
        .compact(&origin, 2..=4, "the model read a file", 40)
        .unwrap();

    let error = sessions.fork(&origin, 3, &id(2)).unwrap_err().to_string();
    assert!(error.contains("2 to 4 were summarised"), "{error}");
    assert!(error.contains("fork at"), "{error}");

    // Outside the range still works.
    assert!(sessions.fork(&origin, 5, &id(3)).is_ok());
}

/// [R-SESSION-080] retention deletes whole sessions, and a parent only with
/// its descendants
#[test]
fn a_sweep_takes_a_parent_together_with_its_children() {
    let (_dir, mut sessions) = sessions();
    let parent = id(1);
    let child = id(2);

    sessions.start(&parent, "outer", "task", None).unwrap();
    sessions
        .start(&child, "inner", "subtask", Some(&parent))
        .unwrap();

    let swept = sessions
        .sweep(
            Retention {
                max_age_secs: Some(0),
                ..Retention::default()
            },
            i64::MAX / 2,
        )
        .unwrap();

    assert_eq!(swept.deleted.len(), 2, "{swept:?}");
    assert!(swept.deleted.contains(&parent));
    assert!(swept.deleted.contains(&child));
    assert!(sessions.session(&parent).is_err());
}

/// [R-SESSION-081] a named session survives unless it is asked for
#[test]
fn a_named_session_is_kept_unless_it_is_asked_for() {
    let (_dir, mut sessions) = sessions();
    let kept = id(1);
    let gone = id(2);

    sessions.start(&kept, "review", "task", None).unwrap();
    sessions.start(&gone, "review", "task", None).unwrap();
    sessions.name_session(&kept, "nightly").unwrap();

    let old = Retention {
        max_age_secs: Some(0),
        ..Retention::default()
    };

    let swept = sessions.sweep(old, i64::MAX / 2).unwrap();
    assert_eq!(swept.deleted, std::slice::from_ref(&gone));
    assert_eq!(swept.kept, std::slice::from_ref(&kept));
    assert!(sessions.session(&kept).is_ok());

    let asked = sessions
        .sweep(
            Retention {
                include_named: true,
                ..old
            },
            i64::MAX / 2,
        )
        .unwrap();
    assert_eq!(asked.deleted, [kept]);
}

/// [R-SESSION-082] the strictest configured limit wins
#[test]
fn the_strictest_limit_wins() {
    let (_dir, mut sessions) = sessions();
    for n in 1..=4 {
        sessions.start(&id(n), "review", "task", None).unwrap();
    }

    // Age would keep everything; count keeps two. The stricter one decides.
    let swept = sessions
        .sweep(
            Retention {
                max_age_secs: Some(i64::MAX / 4),
                max_count: Some(2),
                ..Retention::default()
            },
            1_700_000_100,
        )
        .unwrap();

    assert_eq!(swept.deleted.len(), 2, "{swept:?}");
    assert_eq!(swept.deleted, [id(1), id(2)], "the oldest should go first");
}

/// [R-SESSION-090] a JSON export is the same schema the live renderer writes
#[test]
fn a_json_export_uses_the_live_schema() {
    let (_dir, sessions) = sessions();
    let session = id(1);
    record(&sessions, &session);

    let export = sessions
        .export_json(&session, &Redaction::default())
        .unwrap();

    let lines: Vec<serde_json::Value> = export
        .text
        .lines()
        .map(|line| serde_json::from_str(line).unwrap())
        .collect();

    assert_eq!(lines[0]["type"], "Schema");
    assert_eq!(lines[0]["version"], meow_core::view::SCHEMA_VERSION);

    // Every line is a kind the log holds, and nothing live-only appears.
    for line in &lines[1..] {
        let kind = line["type"].as_str().unwrap();
        assert!(
            EventKind::NAMES.contains(&kind),
            "`{kind}` is not a persisted kind"
        );
    }

    // And one of them is byte-identical to what the live stream would write.
    let tool_call = lines.iter().find(|l| l["type"] == "ToolCall").unwrap();
    assert_eq!(tool_call["name"], "fs.read");
}

/// [R-SESSION-091] a markdown export carries the transcript, the calls with
/// their decisions, and the totals
#[test]
fn a_markdown_export_carries_the_transcript_and_the_totals() {
    let (_dir, sessions) = sessions();
    let session = id(1);
    record(&sessions, &session);

    let export = sessions
        .export_markdown(&session, &Redaction::default())
        .unwrap();
    let text = export.text;

    assert!(text.contains("look at the diff"), "no task: {text}");
    assert!(text.contains("the diff is fine"), "no transcript: {text}");
    assert!(text.contains("`fs.read`"), "no tool call: {text}");
    assert!(
        text.contains("allow (fs.read src/**)"),
        "no policy decision: {text}"
    );
    assert!(
        text.contains("100 prompt + 20 completion"),
        "no totals: {text}"
    );
    assert!(text.contains("$0.21"), "no cost: {text}");
    assert!(text.contains("**finished**"), "no ending: {text}");
}

/// [R-SESSION-092] a sensitive value is redacted in both formats
#[test]
fn a_sensitive_value_is_redacted_in_both_formats() {
    let (_dir, sessions) = sessions();
    let session = id(1);
    record(&sessions, &session);

    let redaction = Redaction {
        arguments: vec!["token".to_owned()],
        include_thinking: false,
    };

    for text in [
        sessions.export_json(&session, &redaction).unwrap().text,
        sessions.export_markdown(&session, &redaction).unwrap().text,
    ] {
        assert!(!text.contains("hunter2"), "the secret survived: {text}");
        assert!(text.contains("src/a.rs"), "the rest was lost too: {text}");
    }
}

/// [R-SESSION-093] reasoning is left out unless it is asked for
#[test]
fn reasoning_is_omitted_unless_it_is_asked_for() {
    let (_dir, sessions) = sessions();
    let session = id(1);
    sessions.start(&session, "review", "task", None).unwrap();
    sessions
        .append(
            &session,
            EventKind::Note {
                level: "thinking".to_owned(),
                message: "first I will look at the retry loop".to_owned(),
            },
        )
        .unwrap();

    let quiet = sessions
        .export_markdown(&session, &Redaction::default())
        .unwrap();
    assert!(!quiet.text.contains("retry loop"), "{}", quiet.text);

    let asked = sessions
        .export_markdown(
            &session,
            &Redaction {
                include_thinking: true,
                ..Redaction::default()
            },
        )
        .unwrap();
    assert!(asked.text.contains("retry loop"), "{}", asked.text);
}
