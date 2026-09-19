// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Every requirement in `docs/spec/session.md` that M2 covers, one test or
//! more each.
//!
//! `[R-SESSION-030]` is withdrawn; `[R-SESSION-034]` replaces it and is tested
//! in `meow-core`, where the identifier lives.

// `allow-unwrap-in-tests` in clippy.toml covers `#[test]` functions, not the
// helpers beside them.
#![allow(clippy::unwrap_used)]

use meow_core::{EventKind, SessionId, StopReason, Usage};
use meow_session::{SessionError, Sessions, State};
use meow_store::Store;

struct Fixture {
    _dir: tempfile::TempDir,
    sessions: Sessions,
    next: std::cell::Cell<u64>,
}

impl Fixture {
    fn new() -> Self {
        let dir = tempfile::tempdir().unwrap();
        let store = Store::open(dir.path()).unwrap();
        Self {
            _dir: dir,
            sessions: Sessions::new(store),
            next: std::cell::Cell::new(1_700_000_000_000),
        }
    }

    /// A fresh identifier, one millisecond later each time, so that ordering
    /// in a test is the ordering a real run would produce.
    fn id(&self, entropy: u8) -> SessionId {
        let t = self.next.get();
        self.next.set(t + 1);
        SessionId::new(t, [entropy; 10])
    }

    fn start(&self, entropy: u8, agent: &str) -> SessionId {
        let id = self.id(entropy);
        self.sessions.start(&id, agent, "a task", None).unwrap();
        id
    }
}

/// [R-SESSION-001] the log only grows
#[test]
fn the_log_offers_no_way_to_change_what_it_holds() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    f.sessions
        .append(
            &id,
            EventKind::Note {
                level: "info".into(),
                message: "one".into(),
            },
        )
        .unwrap();
    let before = f.sessions.events(&id).unwrap();

    f.sessions
        .append(
            &id,
            EventKind::Note {
                level: "info".into(),
                message: "two".into(),
            },
        )
        .unwrap();
    let after = f.sessions.events(&id).unwrap();

    assert_eq!(after.len(), before.len() + 1);
    for (a, b) in before.iter().zip(after.iter()) {
        assert_eq!(a.seq, b.seq);
        assert_eq!(a.kind, b.kind, "an existing event changed");
    }
}

/// [R-SESSION-002] the sequence is monotonic and gapless from one
#[test]
fn the_sequence_starts_at_one_and_has_no_gaps() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    for i in 0..5 {
        f.sessions
            .append(
                &id,
                EventKind::Note {
                    level: "info".into(),
                    message: format!("{i}"),
                },
            )
            .unwrap();
    }
    let seqs: Vec<u64> = f
        .sessions
        .events(&id)
        .unwrap()
        .iter()
        .map(|e| e.seq)
        .collect();
    assert_eq!(seqs, (1..=6).collect::<Vec<_>>());
}

/// [R-SESSION-003] the event kinds are exactly ten
#[test]
fn there_are_exactly_ten_event_kinds() {
    assert_eq!(EventKind::NAMES.len(), 10);
    let unique: std::collections::BTreeSet<&str> = EventKind::NAMES.into_iter().collect();
    assert_eq!(unique.len(), 10, "a kind is listed twice");
}

/// [R-SESSION-004] every event carries when it happened
#[test]
fn every_event_carries_a_timestamp() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    for e in f.sessions.events(&id).unwrap() {
        assert!(e.at > 0, "event {} has no timestamp", e.seq);
    }
}

/// [R-SESSION-005] a run opens with Started and closes with exactly one Finished
#[test]
fn a_run_opens_and_closes_exactly_once() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    f.sessions.finish(&id, StopReason::Finished, None).unwrap();

    // A second finish has no open run to close.
    let err = f
        .sessions
        .finish(&id, StopReason::Finished, None)
        .unwrap_err();
    assert!(matches!(err, SessionError::Lifecycle(_)), "got {err:?}");

    let kinds: Vec<&str> = f
        .sessions
        .events(&id)
        .unwrap()
        .iter()
        .map(|e| e.kind.name())
        .collect();
    assert_eq!(kinds, vec!["Started", "Finished"]);
}

/// [R-SESSION-006] resuming opens a new run without rewriting the old ending
#[test]
fn resuming_appends_a_new_run_and_leaves_the_old_ending_alone() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    f.sessions.finish(&id, StopReason::Finished, None).unwrap();
    f.sessions.resume(&id, "a follow-up").unwrap();

    let kinds: Vec<&str> = f
        .sessions
        .events(&id)
        .unwrap()
        .iter()
        .map(|e| e.kind.name())
        .collect();
    assert_eq!(kinds, vec!["Started", "Finished", "Started"]);
    assert_eq!(f.sessions.state(&id).unwrap(), State::Running);

    // Resuming a run that is still open is a mistake worth reporting.
    assert!(matches!(
        f.sessions.resume(&id, "again"),
        Err(SessionError::Lifecycle(_))
    ));
}

/// [R-SESSION-010] compaction supersedes and does not delete
/// [R-SESSION-012] the rebuild for a person returns the originals
#[test]
fn compaction_supersedes_without_deleting() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    for i in 0..4 {
        f.sessions
            .append(
                &id,
                EventKind::UserMessage {
                    content: format!("message {i}"),
                },
            )
            .unwrap();
    }
    f.sessions
        .compact(&id, 2..=4, "three messages about the diff", 900)
        .unwrap();

    let all = f.sessions.events(&id).unwrap();
    assert_eq!(all.len(), 6, "an event was removed");
    assert!(
        all.iter().any(|e| e.kind
            == EventKind::UserMessage {
                content: "message 1".into()
            }),
        "a superseded event is gone from the human rebuild"
    );
}

/// [R-SESSION-011] the rebuild for a model skips what was superseded
#[test]
fn the_model_rebuild_skips_the_superseded_range() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    for i in 0..4 {
        f.sessions
            .append(
                &id,
                EventKind::UserMessage {
                    content: format!("message {i}"),
                },
            )
            .unwrap();
    }
    f.sessions.compact(&id, 2..=4, "a summary", 900).unwrap();

    let seqs: Vec<u64> = f
        .sessions
        .events_for_model(&id)
        .unwrap()
        .iter()
        .map(|e| e.seq)
        .collect();
    // The summary stands where the range stood, so event 6 comes before event
    // 5: it substitutes for sequences 2 to 4, which preceded 5. A rebuild that
    // put it last would show a summary of the early conversation after the
    // messages that followed it.
    assert_eq!(
        seqs,
        vec![1, 6, 5],
        "the summary must stand where the range did"
    );

    let for_model = f.sessions.events_for_model(&id).unwrap();
    assert!(
        for_model
            .iter()
            .any(|e| matches!(&e.kind, EventKind::Compaction { summary, .. }
            if summary == "a summary")),
        "the summary must stand in for the range"
    );
}

/// [R-SESSION-013] a range cannot be superseded twice
#[test]
fn a_range_cannot_be_superseded_twice() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    for i in 0..6 {
        f.sessions
            .append(
                &id,
                EventKind::UserMessage {
                    content: format!("m{i}"),
                },
            )
            .unwrap();
    }
    f.sessions.compact(&id, 2..=4, "first", 100).unwrap();

    match f.sessions.compact(&id, 4..=6, "overlapping", 100) {
        Err(SessionError::AlreadySuperseded { existing, .. }) => assert_eq!(existing, 8),
        other => panic!("expected AlreadySuperseded, got {other:?}"),
    }
    // A range beside it is fine.
    f.sessions.compact(&id, 5..=7, "second", 100).unwrap();
}

/// [R-SESSION-020] usage is typed, with cached as its own field
#[test]
fn usage_is_typed_and_cached_tokens_are_their_own_field() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    f.sessions
        .append(
            &id,
            EventKind::Usage(Usage {
                prompt: 1000,
                completion: 200,
                cached: Some(800),
                cost_micros: Some(4200),
            }),
        )
        .unwrap();
    let u = f.sessions.usage(&id).unwrap();
    assert_eq!(
        (u.prompt, u.completion, u.cached, u.cost_micros),
        (1000, 200, Some(800), Some(4200))
    );
    assert_eq!(u.total(), 1200);
}

/// [R-SESSION-021] an unpriced model records an absent cost, never zero
#[test]
fn an_unpriced_call_records_no_cost_rather_than_a_free_one() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    f.sessions
        .append(
            &id,
            EventKind::Usage(Usage {
                prompt: 10,
                completion: 5,
                cached: None,
                cost_micros: Some(50),
            }),
        )
        .unwrap();
    f.sessions
        .append(
            &id,
            EventKind::Usage(Usage {
                prompt: 10,
                completion: 5,
                cached: None,
                cost_micros: None,
            }),
        )
        .unwrap();

    let u = f.sessions.usage(&id).unwrap();
    assert_eq!(u.prompt, 20);
    assert_eq!(
        u.cost_micros, None,
        "a total that omits a cost must not look complete"
    );
}

/// [R-SESSION-022] totals include the children
#[test]
fn a_parent_reports_what_its_children_spent() {
    let f = Fixture::new();
    let parent = f.start(1, "triage");
    let child = f.id(2);
    f.sessions
        .start(&child, "review", "sub-task", Some(&parent))
        .unwrap();

    for (id, prompt) in [(&parent, 100u32), (&child, 40)] {
        f.sessions
            .append(
                id,
                EventKind::Usage(Usage {
                    prompt,
                    completion: 10,
                    cached: None,
                    cost_micros: Some(7),
                }),
            )
            .unwrap();
    }

    assert_eq!(f.sessions.usage(&child).unwrap().prompt, 40);
    let total = f.sessions.usage(&parent).unwrap();
    assert_eq!(total.prompt, 140, "the parent must include the child");
    assert_eq!(total.cost_micros, Some(14));
}

/// [R-SESSION-031] an ambiguous short identifier fails with the candidates
#[test]
fn an_ambiguous_identifier_lists_the_candidates_rather_than_guessing() {
    let f = Fixture::new();
    // Two sessions whose identifiers end the same way. Contrived here; in a
    // workspace it is a birthday collision on forty bits.
    let a = SessionId::from_text("0000000000000000000COLLIDE");
    let b = SessionId::from_text("1111111111111111111COLLIDE");
    f.sessions.start(&a, "review", "t", None).unwrap();
    f.sessions.start(&b, "review", "t", None).unwrap();

    match f.sessions.resolve("COLLIDE") {
        Err(SessionError::Ambiguous { candidates, .. }) => {
            assert_eq!(candidates.len(), 2);
        }
        other => panic!("expected Ambiguous, got {other:?}"),
    }
    // The full identifier is never ambiguous.
    assert_eq!(f.sessions.resolve(a.as_str()).unwrap().id, a);
}

/// [R-SESSION-032] the selectors resolve at the moment of use
#[test]
fn the_selectors_resolve_when_they_are_used() {
    let f = Fixture::new();
    let first = f.start(1, "review");
    let second = f.start(2, "triage");

    assert_eq!(f.sessions.resolve("@last").unwrap().id, second);
    assert_eq!(f.sessions.resolve("@last-1").unwrap().id, first);
    assert_eq!(f.sessions.resolve("@review").unwrap().id, first);

    // A newer run moves @last, which is the point of resolving late.
    let third = f.start(3, "review");
    assert_eq!(f.sessions.resolve("@last").unwrap().id, third);
    assert_eq!(f.sessions.resolve("@review").unwrap().id, third);
}

/// [R-SESSION-033] a name is unique and cannot shadow a selector
#[test]
fn a_name_resolves_and_cannot_begin_with_an_at_sign() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    f.sessions.name_session(&id, "nightly-triage").unwrap();
    assert_eq!(f.sessions.resolve("nightly-triage").unwrap().id, id);

    match f.sessions.name_session(&id, "@last") {
        Err(SessionError::InvalidName { reason, .. }) => assert!(reason.contains("shadow")),
        other => panic!("expected InvalidName, got {other:?}"),
    }

    let other = f.start(2, "review");
    assert!(
        f.sessions.name_session(&other, "nightly-triage").is_err(),
        "names must be unique"
    );
}

/// [R-SESSION-040] a session is running or in one of six stopped states
/// [R-SESSION-041] the state comes from the log
#[test]
fn the_state_is_whatever_the_log_says() {
    let f = Fixture::new();
    for (i, stop) in [
        StopReason::Finished,
        StopReason::Budget,
        StopReason::Cancelled,
        StopReason::Denied,
        StopReason::ToolAborted,
        StopReason::Failed,
    ]
    .into_iter()
    .enumerate()
    {
        let id = f.start(i as u8, "review");
        assert_eq!(f.sessions.state(&id).unwrap(), State::Running);
        f.sessions.finish(&id, stop, None).unwrap();
        assert_eq!(f.sessions.state(&id).unwrap(), State::Stopped(stop));
    }
}

/// [R-SESSION-041] a stale cached copy never wins over the log
#[test]
fn the_cached_state_never_overrides_the_log() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    f.sessions
        .finish(&id, StopReason::Budget, Some("tokens"))
        .unwrap();

    // Corrupt the denormalised copy, the way a crash between two writes could.
    f.sessions
        .store()
        .set_state_cache(id.as_str(), "finished")
        .unwrap();
    assert_eq!(
        f.sessions.state(&id).unwrap(),
        State::Stopped(StopReason::Budget),
        "the cached copy won over the log"
    );
}

/// [R-SESSION-042] a session whose writer died stops claiming to be running
/// [R-SESSION-043] liveness is a heartbeat
#[test]
fn a_session_whose_writer_died_is_closed_on_the_next_open() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    let now = 1_900_000_000;

    // Still beating: nothing happens.
    f.sessions.heartbeat(&id).unwrap();
    assert!(!f.sessions.reap_if_dead(&id, 0).unwrap());
    assert_eq!(f.sessions.state(&id).unwrap(), State::Running);

    // Long past three intervals: the session admits it failed.
    assert!(f.sessions.reap_if_dead(&id, now).unwrap());
    assert_eq!(
        f.sessions.state(&id).unwrap(),
        State::Stopped(StopReason::Failed)
    );

    // Reaping is not repeated once it has acted.
    assert!(!f.sessions.reap_if_dead(&id, now).unwrap());
}

/// [R-SESSION-060] a sub-agent records its parent, and the parent sees it
#[test]
fn a_parent_can_enumerate_its_children_in_order() {
    let f = Fixture::new();
    let parent = f.start(1, "triage");
    let first = f.id(2);
    let second = f.id(3);
    f.sessions
        .start(&first, "review", "a", Some(&parent))
        .unwrap();
    f.sessions
        .start(&second, "search", "b", Some(&parent))
        .unwrap();

    let kids = f.sessions.children(&parent).unwrap();
    assert_eq!(kids.len(), 2);
    assert_eq!(kids[0].id, first, "children come in creation order");
    assert_eq!(kids[1].id, second);
    assert_eq!(kids[0].parent.as_ref(), Some(&parent));
}

/// [R-SESSION-061] a session's parent must already exist
#[test]
fn a_session_cannot_name_a_parent_that_does_not_exist() {
    let f = Fixture::new();
    let orphan = f.id(1);
    let absent = SessionId::from_text("NOSUCHSESSION0000000000000");
    assert!(
        f.sessions
            .start(&orphan, "review", "t", Some(&absent))
            .is_err(),
        "a missing parent must be refused, which is what makes a cycle impossible"
    );
}

/// [R-SESSION-070] every tool invocation records a policy decision
/// [R-SESSION-071] denials are recorded too
#[test]
fn every_policy_decision_is_recorded_including_the_denials() {
    let f = Fixture::new();
    let id = f.start(1, "review");
    for (call, decision, rule) in [
        ("c1", "allow", Some("fs.read")),
        ("c2", "ask", Some("shell \"git push *\"")),
        ("c3", "deny", None),
    ] {
        f.sessions
            .append(
                &id,
                EventKind::Policy {
                    id: call.into(),
                    decision: decision.into(),
                    rule: rule.map(str::to_owned),
                    source: "rule".into(),
                },
            )
            .unwrap();
    }

    let decisions: Vec<String> = f
        .sessions
        .events(&id)
        .unwrap()
        .iter()
        .filter_map(|e| match &e.kind {
            EventKind::Policy { decision, .. } => Some(decision.clone()),
            _ => None,
        })
        .collect();
    assert_eq!(decisions, vec!["allow", "ask", "deny"]);
    assert!(
        decisions.contains(&"deny".to_owned()),
        "a denial must be in the log"
    );
}
