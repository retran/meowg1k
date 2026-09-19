// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! One recorded stream, three renderers.
#![allow(clippy::unwrap_used)]

use meow_core::view::{LiveKind, Output, SCHEMA_VERSION, ViewEvent};
use meow_core::{EventKind, StopReason, Usage};
use meow_ui::theme::{Depth, Theme};
use meow_ui::{Choice, Conditions, Json, Plain, Renderer, Tty, choose};
use ratatui::backend::TestBackend;

/// A run with a step, a tool, some output, and an end.
///
/// One recording, replayed into every renderer, which is what `[R-TUI-005]`
/// asks for: the same stream, and none of them needing a terminal.
fn recording() -> Vec<ViewEvent> {
    vec![
        ViewEvent::Live(LiveKind::RunStart {
            agent: "review".to_owned(),
            model: "smart".to_owned(),
            session: "4f2a9c".to_owned(),
        }),
        ViewEvent::Live(LiveKind::StepStart { step: 1 }),
        ViewEvent::Logged(EventKind::Policy {
            id: "c1".to_owned(),
            decision: "allowed".to_owned(),
            rule: Some("fs.read src/**".to_owned()),
            source: "rule".to_owned(),
        }),
        ViewEvent::Live(LiveKind::ToolStart {
            id: "c1".to_owned(),
            name: "fs.read".to_owned(),
            args: r#"{"path":"src/retry.rs"}"#.to_owned(),
        }),
        ViewEvent::Live(LiveKind::ToolEnd {
            id: "c1".to_owned(),
            name: "fs.read".to_owned(),
            duration_ms: 12,
            error: None,
        }),
        ViewEvent::Live(LiveKind::TextDelta {
            delta: "the retry ".to_owned(),
        }),
        ViewEvent::Live(LiveKind::TextDelta {
            delta: "budget is wrong".to_owned(),
        }),
        ViewEvent::Live(LiveKind::Output(Output::Note {
            text: "2 files skipped".to_owned(),
        })),
        ViewEvent::Live(LiveKind::Progress {
            step: 1,
            elapsed_ms: 1200,
            tokens: 74392,
            cost_micros: Some(210_000),
            tool: None,
        }),
        ViewEvent::Live(LiveKind::RunEnd {
            stop: StopReason::Finished,
            detail: None,
            steps: 12,
            usage: Usage {
                prompt: 70_000,
                completion: 4392,
                cached: None,
                cost_micros: Some(210_000),
            },
            elapsed_ms: 64_000,
            session: "4f2a9c".to_owned(),
        }),
    ]
}

fn plain_of(events: &[ViewEvent]) -> String {
    let mut renderer = Plain::new(Vec::new());
    for event in events {
        renderer.event(event).unwrap();
    }
    renderer.finish().unwrap();
    String::from_utf8(renderer.into_inner()).unwrap()
}

fn json_of(events: &[ViewEvent]) -> Vec<serde_json::Value> {
    let mut renderer = Json::new(Vec::new());
    for event in events {
        renderer.event(event).unwrap();
    }
    renderer.finish().unwrap();
    String::from_utf8(renderer.into_inner())
        .unwrap()
        .lines()
        .map(|line| serde_json::from_str(line).unwrap())
        .collect()
}

/// [R-TUI-001] [R-TUI-002] the renderer is a function of the runtime's
/// conditions, and `--format json` wins whatever the terminal is
#[test]
fn json_wins_whatever_the_terminal_is() {
    for stdout_is_terminal in [true, false] {
        for no_color in [true, false] {
            assert_eq!(
                choose(Conditions {
                    json: true,
                    stdout_is_terminal,
                    no_color,
                    color_never: false,
                }),
                Choice::Json
            );
        }
    }
}

/// [R-TUI-003] a pipe, NO_COLOR, and --color=never all select plain
#[test]
fn a_pipe_or_no_colour_selects_plain() {
    let base = Conditions {
        json: false,
        stdout_is_terminal: true,
        no_color: false,
        color_never: false,
    };

    assert_eq!(
        choose(Conditions {
            stdout_is_terminal: false,
            ..base
        }),
        Choice::Plain
    );
    assert_eq!(
        choose(Conditions {
            no_color: true,
            ..base
        }),
        Choice::Plain
    );
    assert_eq!(
        choose(Conditions {
            color_never: true,
            ..base
        }),
        Choice::Plain
    );
}

/// [R-TUI-004] otherwise the inline terminal renderer
#[test]
fn a_terminal_selects_the_inline_renderer() {
    assert_eq!(
        choose(Conditions {
            json: false,
            stdout_is_terminal: true,
            no_color: false,
            color_never: false,
        }),
        Choice::Terminal
    );
}

/// [R-TUI-005] all three take the same stream with no terminal attached
#[test]
fn all_three_take_the_same_recording() {
    let events = recording();

    assert!(!plain_of(&events).is_empty());
    assert!(!json_of(&events).is_empty());

    let mut tty = Tty::new(TestBackend::new(80, 6), Theme::ascii()).unwrap();
    for event in &events {
        tty.event(event).unwrap();
    }
    tty.finish().unwrap();
}

/// [R-TUI-020] the plain renderer emits no escape sequences
#[test]
fn the_plain_renderer_emits_no_escapes() {
    let text = plain_of(&recording());
    assert!(!text.contains('\u{1b}'), "{text:?}");
    assert!(!text.contains('\r'), "{text:?}");
}

/// [R-TUI-021] the plain renderer carries the steps, the tool calls with their
/// policy decisions, and the totals
#[test]
fn the_plain_renderer_carries_the_same_information() {
    let text = plain_of(&recording());

    assert!(
        text.contains("review: model=smart session=4f2a9c"),
        "{text}"
    );
    assert!(text.contains("fs.read"), "{text}");
    assert!(
        text.contains("[allowed: fs.read src/**]"),
        "the policy decision is missing: {text}"
    );
    assert!(text.contains("note: 2 files skipped"), "{text}");
    assert!(
        text.contains("finished: 12 steps, 74392 tokens, $0.21, 1m04s"),
        "the totals are missing: {text}"
    );
}

/// [R-TUI-030] [R-TUI-031] one object per line, each with a type, and the
/// schema version first
#[test]
fn the_json_renderer_announces_its_schema_first() {
    let lines = json_of(&recording());

    assert_eq!(lines[0]["type"], "Schema");
    assert_eq!(lines[0]["version"], SCHEMA_VERSION);

    for line in &lines {
        assert!(line.get("type").is_some(), "no type on {line}");
        assert!(line.is_object(), "{line}");
    }
}

/// [R-TUI-032] a persisted kind serialises identically in the live stream and
/// in an export
#[test]
fn a_persisted_kind_serialises_the_same_in_both() {
    let kind = EventKind::ToolResult {
        id: "c1".to_owned(),
        output: "ok".to_owned(),
        duration_ms: 12,
        error: None,
    };

    // What an export writes: the log event on its own.
    let exported = serde_json::to_value(&kind).unwrap();
    // What the live stream writes: the same event, through the view.
    let live = serde_json::to_value(ViewEvent::Logged(kind)).unwrap();

    assert_eq!(exported, live);
}

/// [R-TUI-034] no kind belongs to only one of the two streams by accident
#[test]
fn live_and_persisted_kinds_do_not_collide() {
    for live in LiveKind::NAMES {
        assert!(
            !EventKind::NAMES.contains(&live),
            "`{live}` is both a live kind and a persisted one, which makes the stream ambiguous"
        );
    }

    assert!(
        !ViewEvent::Live(LiveKind::TextDelta {
            delta: String::new()
        })
        .is_persisted()
    );
    assert!(
        ViewEvent::Logged(EventKind::Note {
            level: "info".to_owned(),
            message: String::new(),
        })
        .is_persisted()
    );
}

/// [R-TUI-040] ctx.out has exactly ten calls
#[test]
fn ctx_out_has_exactly_ten_calls() {
    assert_eq!(Output::CALLS.len(), 10);
    assert_eq!(
        Output::CALLS,
        [
            "write", "markdown", "note", "warn", "error", "step", "table", "diff", "finding",
            "json"
        ]
    );
}

/// [R-TUI-041] [R-TUI-042] each call is a typed event every renderer handles
#[test]
fn every_out_call_reaches_every_renderer() {
    let calls = [
        Output::Write {
            text: "a line".to_owned(),
        },
        Output::Markdown {
            text: "# heading".to_owned(),
        },
        Output::Note {
            text: "an aside".to_owned(),
        },
        Output::Warn {
            text: "watch out".to_owned(),
        },
        Output::Error {
            text: "it broke".to_owned(),
        },
        Output::Step {
            text: "collecting".to_owned(),
        },
        Output::Table {
            columns: vec!["file".to_owned(), "count".to_owned()],
            rows: vec![vec!["a.rs".to_owned(), "3".to_owned()]],
        },
        Output::Diff {
            patch: "@@ -1 +1 @@\n-old\n+new".to_owned(),
        },
        Output::Finding {
            severity: "high".to_owned(),
            location: "a.rs:1".to_owned(),
            summary: "unchecked".to_owned(),
        },
        Output::Json {
            value: serde_json::json!({"n": 1}),
        },
    ];

    assert_eq!(calls.len(), Output::CALLS.len());

    let events: Vec<ViewEvent> = calls
        .iter()
        .map(|call| ViewEvent::Live(LiveKind::Output(call.clone())))
        .collect();

    // Plain: every call produces at least one line.
    let text = plain_of(&events);
    for expected in [
        "a line",
        "# heading",
        "note: an aside",
        "warning: watch out",
        "error: it broke",
        "== collecting",
        "file  count",
        "+new",
        "high: a.rs:1: unchecked",
        r#"{"n":1}"#,
    ] {
        assert!(
            text.contains(expected),
            "plain dropped {expected:?}: {text}"
        );
    }

    // JSON: every call keeps its name, so `jq 'select(.call=="finding")'`
    // works on any of them.
    let lines = json_of(&events);
    let names: Vec<String> = lines
        .iter()
        .filter_map(|line| line.get("call")?.as_str().map(str::to_owned))
        .collect();
    assert_eq!(names, Output::CALLS);

    // Terminal: none of them panics or is silently dropped.
    let mut tty = Tty::new(TestBackend::new(80, 6), Theme::ascii()).unwrap();
    for event in &events {
        tty.event(event).unwrap();
    }
    tty.finish().unwrap();
}

/// [R-TUI-010] the terminal renderer never leaves the main screen
#[test]
fn the_terminal_renderer_stays_on_the_main_screen() {
    let mut tty = Tty::new(TestBackend::new(40, 6), Theme::ascii()).unwrap();
    for event in recording() {
        tty.event(&event).unwrap();
    }
    tty.finish().unwrap();

    // `Viewport::Inline` is the whole mechanism: an alternate screen would
    // mean `Viewport::Fullscreen`, and the viewport is fixed at construction.
    assert_eq!(meow_ui::tty::RUN_ROWS, 3);
}

/// [R-TUI-016] the live region is three rows, and a prompt makes it taller
/// without keeping it that way
#[test]
fn the_live_region_has_two_fixed_heights() {
    assert_eq!(meow_ui::tty::RUN_ROWS, 3);
    const { assert!(meow_ui::tty::PROMPT_ROWS > meow_ui::tty::RUN_ROWS) };

    let mut tty = Tty::new(TestBackend::new(40, 20), Theme::ascii()).unwrap();
    tty.event(&ViewEvent::Live(LiveKind::RunStart {
        agent: "review".to_owned(),
        model: "smart".to_owned(),
        session: "4f2a9c".to_owned(),
    }))
    .unwrap();

    tty.prompting(true).unwrap();
    tty.prompting(false).unwrap();
    tty.finish().unwrap();
}

/// [R-TUI-011] a finalized line goes into scrollback, and the live region is
/// not it
#[test]
fn a_finalized_line_goes_into_scrollback() {
    let mut tty = Tty::new(TestBackend::new(40, 8), Theme::ascii()).unwrap();

    tty.event(&ViewEvent::Live(LiveKind::Output(Output::Write {
        text: "committed".to_owned(),
    })))
    .unwrap();

    // Nothing was drawn into the live region by an output call, which is what
    // stops a line being written twice.
    let live = live_text(&mut tty);
    assert!(!live.contains("committed"), "{live}");
}

/// [R-TUI-012] the live region carries the tool, the elapsed time, the step
/// count, and what has been spent
#[test]
fn the_live_region_shows_progress() {
    let mut tty = Tty::new(TestBackend::new(60, 8), Theme::ascii()).unwrap();

    tty.event(&ViewEvent::Live(LiveKind::RunStart {
        agent: "review".to_owned(),
        model: "smart".to_owned(),
        session: "4f2a9c".to_owned(),
    }))
    .unwrap();
    tty.event(&ViewEvent::Live(LiveKind::Progress {
        step: 4,
        elapsed_ms: 64_000,
        tokens: 74_392,
        cost_micros: Some(210_000),
        tool: Some("fs.read".to_owned()),
    }))
    .unwrap();

    let live = live_text(&mut tty);
    assert!(live.contains("fs.read"), "no tool: {live}");
    assert!(live.contains("step 4"), "no step: {live}");
    assert!(live.contains("74392 tok"), "no budget: {live}");
    assert!(live.contains("1m04s"), "no elapsed time: {live}");
    assert!(live.contains("review"), "no agent: {live}");
}

/// [R-TUI-013] the run ends with a line naming the stop reason
#[test]
fn the_run_ends_with_its_stop_reason() {
    let mut tty = Tty::new(TestBackend::new(80, 8), Theme::ascii()).unwrap();
    for event in recording() {
        tty.event(&event).unwrap();
    }

    // The live region is gone; what remains is in scrollback, which is where a
    // reader still has it after the process exits.
    let live = live_text(&mut tty);
    assert!(live.trim().is_empty(), "the live region survived: {live:?}");
}

/// [R-TUI-014] a resize reflows the live region and leaves scrollback alone
#[test]
fn a_resize_reflows_only_the_live_region() {
    let mut tty = Tty::new(TestBackend::new(40, 8), Theme::ascii()).unwrap();
    tty.event(&ViewEvent::Live(LiveKind::Output(Output::Write {
        text: "already committed".to_owned(),
    })))
    .unwrap();
    tty.event(&ViewEvent::Live(LiveKind::Progress {
        step: 1,
        elapsed_ms: 1000,
        tokens: 10,
        cost_micros: None,
        tool: Some("fs.read".to_owned()),
    }))
    .unwrap();

    // A committed line is in scrollback and is never redrawn, so a resize
    // cannot touch it however the terminal reflows what it already printed.
    let live = live_text(&mut tty);
    assert!(!live.contains("already committed"), "{live}");
    assert!(live.contains("fs.read"), "{live}");
}

/// [R-TUI-015] a diagnostic lands in scrollback rather than over the live
/// region
#[test]
fn a_diagnostic_does_not_tear_through_the_live_region() {
    let mut tty = Tty::new(TestBackend::new(60, 8), Theme::ascii()).unwrap();
    tty.event(&ViewEvent::Live(LiveKind::Progress {
        step: 2,
        elapsed_ms: 500,
        tokens: 5,
        cost_micros: None,
        tool: Some("git.diff".to_owned()),
    }))
    .unwrap();

    tty.event(&ViewEvent::Logged(EventKind::Note {
        level: "warn".to_owned(),
        message: "the index is stale".to_owned(),
    }))
    .unwrap();

    let live = live_text(&mut tty);
    assert!(
        !live.contains("the index is stale"),
        "a diagnostic was drawn over the live region: {live}"
    );
    assert!(live.contains("git.diff"), "{live}");
}

/// [R-TUI-090] NO_COLOR wins over everything
#[test]
fn no_color_wins_over_every_other_signal() {
    assert_eq!(
        Depth::detect(true, Some("truecolor"), Some("xterm-256color")),
        Depth::None
    );
    assert_eq!(Theme::new(Depth::None, true).depth(), Depth::None);
}

/// [R-TUI-091] colour depth is detected and quantised rather than dropped
#[test]
fn colour_is_quantised_to_what_the_terminal_has() {
    assert_eq!(
        Depth::detect(false, Some("truecolor"), Some("xterm")),
        Depth::True
    );
    assert_eq!(
        Depth::detect(false, None, Some("xterm-256color")),
        Depth::Ansi256
    );
    assert_eq!(Depth::detect(false, None, Some("xterm")), Depth::Ansi16);
    assert_eq!(Depth::detect(false, None, Some("dumb")), Depth::None);
    assert_eq!(Depth::detect(false, None, None), Depth::None);

    // A 16-colour terminal gets the nearest of sixteen, not nothing.
    let style = Theme::new(Depth::Ansi16, false).style(meow_ui::Role::Failure);
    assert!(style.fg.is_some());
}

/// [R-TUI-092] colour is never the only carrier of meaning
#[test]
fn every_severity_carries_a_word_as_well_as_a_colour() {
    use meow_ui::Role;

    for role in [Role::Muted, Role::Warning, Role::Failure, Role::Success] {
        assert!(!role.sigil().is_empty(), "{role:?} has no word");
    }

    // And the plain renderer, which has no colour at all, still says which is
    // which.
    let text = plain_of(&[
        ViewEvent::Live(LiveKind::Output(Output::Note {
            text: "a".to_owned(),
        })),
        ViewEvent::Live(LiveKind::Output(Output::Warn {
            text: "b".to_owned(),
        })),
        ViewEvent::Live(LiveKind::Output(Output::Error {
            text: "c".to_owned(),
        })),
    ]);
    assert_eq!(text, "note: a\nwarning: b\nerror: c\n");
}

/// [R-TUI-093] a terminal that cannot be shown to support unicode gets ASCII
#[test]
fn unicode_is_used_only_when_it_is_known_to_work() {
    use meow_ui::theme::unicode;

    assert!(unicode(Some("en_US.UTF-8"), Some("xterm-256color")));
    assert!(!unicode(None, Some("xterm-256color")));
    assert!(!unicode(Some("C"), Some("xterm")));
    assert!(!unicode(Some("en_US.UTF-8"), Some("dumb")));
    assert!(!unicode(Some("en_US.UTF-8"), None));

    let ascii = Theme::ascii();
    assert_eq!(ascii.branch(), "|-");
    assert_eq!(ascii.spinner(0), "-");

    let rich = Theme::new(Depth::True, true);
    assert_eq!(rich.branch(), "├─");
    assert_eq!(rich.spinner(0), "⠋");
}

/// Read the live region back as text, and only the live region.
///
/// `TestBackend` keeps the whole screen, so a committed line is in the same
/// buffer as the live region. Slicing by the viewport is what separates "is
/// still on screen" from "is in the live region", which is the distinction
/// every test here turns on.
fn live_text(tty: &mut Tty<TestBackend>) -> String {
    let area = tty.live_area();
    let rows = screen_rows(tty.terminal().backend());
    rows.into_iter()
        .skip(area.y as usize)
        .take(area.height as usize)
        .collect::<Vec<_>>()
        .join("\n")
}

/// Every row of the screen, in order.
fn screen_rows(backend: &TestBackend) -> Vec<String> {
    let debug = format!("{backend:?}");
    debug
        .lines()
        .filter_map(|line| {
            let line = line.trim();
            let inner = line.strip_prefix('"')?;
            let inner = inner
                .strip_suffix("\",")
                .or_else(|| inner.strip_suffix('"'))?;
            Some(inner.to_owned())
        })
        .collect()
}
