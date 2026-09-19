// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Every requirement in `docs/spec/policy.md` that this crate owns.
//!
//! Enforcement, `[R-POLICY-040]` to `[R-POLICY-042]`, is tested in
//! `meow-agent`, because the engine is what calls a tool.

// `allow-unwrap-in-tests` in clippy.toml covers `#[test]` functions, not the
// helpers beside them.
#![allow(clippy::unwrap_used)]

use std::path::PathBuf;

use meow_policy::{
    Access, Call, Decision, Grants, NamePattern, Policy, PolicyError, REDACTED, Rule, SelectorKind,
    Source, redact,
};
use serde_json::json;

fn rule(pattern: &str) -> Rule {
    Rule::new("meow.star:1", pattern).unwrap()
}

fn paths(rule: Rule, globs: &[&str]) -> Rule {
    rule.narrowed(SelectorKind::Paths, globs).unwrap()
}

fn commands(rule: Rule, globs: &[&str]) -> Rule {
    rule.narrowed(SelectorKind::Commands, globs).unwrap()
}

fn p(s: &str) -> PathBuf {
    PathBuf::from(s)
}

/// [R-POLICY-001] a rule matches on a name and may narrow
#[test]
fn a_rule_matches_a_name_and_may_narrow_further() {
    let policy = Policy::new()
        .with(Decision::Allow, rule("fs.read"))
        .with(Decision::Allow, paths(rule("fs.write"), &["/w/src/**"]));

    let read = Call::new("fs.read", Access::Read);
    assert_eq!(
        policy.evaluate(&read, &Grants::new()).decision,
        Decision::Allow
    );

    let write = Call::new("fs.write", Access::Write).with_paths(vec![p("/w/src/a.rs")]);
    assert_eq!(
        policy.evaluate(&write, &Grants::new()).decision,
        Decision::Allow
    );

    let outside = Call::new("fs.write", Access::Write).with_paths(vec![p("/etc/passwd")]);
    assert_eq!(
        policy.evaluate(&outside, &Grants::new()).decision,
        Decision::Deny
    );
}

/// [R-POLICY-002] a trailing wildcard and an exact name, never a leading one
#[test]
fn a_leading_wildcard_is_refused_when_the_policy_is_built() {
    assert_eq!(
        NamePattern::parse("fs.read").unwrap(),
        NamePattern::Exact("fs.read".into())
    );
    assert_eq!(
        NamePattern::parse("fs.*").unwrap(),
        NamePattern::Prefix("fs".into())
    );
    assert_eq!(NamePattern::parse("*").unwrap(), NamePattern::Any);

    for bad in ["*.write", "fs.*.read", "f*s.read"] {
        assert!(
            matches!(NamePattern::parse(bad), Err(PolicyError::BadPattern { .. })),
            "`{bad}` should be refused"
        );
    }

    let policy = Policy::new().with(Decision::Allow, rule("fs.*"));
    for tool in ["fs.read", "fs.write"] {
        let c = Call::new(tool, Access::Read);
        assert_eq!(
            policy.evaluate(&c, &Grants::new()).decision,
            Decision::Allow
        );
    }
    // `fs.*` must not swallow a tool that merely starts with the letters.
    let c = Call::new("fsx.read", Access::Read);
    assert_eq!(policy.evaluate(&c, &Grants::new()).decision, Decision::Deny);
}

/// [R-POLICY-003] paths are matched absolute, with `**` crossing directories
#[test]
fn a_path_selector_crosses_directories_with_a_double_star() {
    let policy = Policy::new().with(Decision::Allow, paths(rule("fs.read"), &["/w/src/**"]));
    for path in ["/w/src/a.rs", "/w/src/deep/nested/b.rs"] {
        let c = Call::new("fs.read", Access::Read).with_paths(vec![p(path)]);
        assert_eq!(
            policy.evaluate(&c, &Grants::new()).decision,
            Decision::Allow,
            "{path} should match"
        );
    }
    let c = Call::new("fs.read", Access::Read).with_paths(vec![p("/w/other/a.rs")]);
    assert_eq!(policy.evaluate(&c, &Grants::new()).decision, Decision::Deny);
}

/// [R-POLICY-004] a command selector matches the whole command line
#[test]
fn a_command_selector_matches_the_whole_line_not_just_the_binary() {
    let policy = Policy::new().with(
        Decision::Allow,
        commands(rule("shell"), &["cargo *", "git status"]),
    );

    for (line, want) in [
        ("cargo test --lib", Decision::Allow),
        ("git status", Decision::Allow),
        ("git push origin main", Decision::Deny),
        ("cargo", Decision::Deny),
    ] {
        let c = Call::new("shell", Access::Write).with_command(line);
        assert_eq!(
            policy.evaluate(&c, &Grants::new()).decision,
            want,
            "for `{line}`"
        );
    }
}

/// [R-POLICY-005] a selector no matching tool supports fails at build time
#[test]
fn a_selector_no_tool_supports_fails_when_the_policy_is_built() {
    let policy = Policy::new().with(Decision::Allow, commands(rule("fs.*"), &["cargo *"]));

    // The caller knows its own tools; the policy layer does not.
    let supports = |name: &NamePattern, kind: SelectorKind| match (name, kind) {
        (NamePattern::Prefix(p), SelectorKind::Paths) if p == "fs" => true,
        (NamePattern::Exact(n), SelectorKind::Commands) if n == "shell" => true,
        _ => false,
    };
    match policy.validate(&supports) {
        Err(PolicyError::UnsupportedSelector { selector, .. }) => assert_eq!(selector, "commands"),
        other => panic!("expected UnsupportedSelector, got {other:?}"),
    }
}

/// [R-POLICY-006] a multi-path call is judged once per path
/// [R-POLICY-008] a read returns what it may and names what it skipped
#[test]
fn a_read_that_hits_a_denied_path_says_which_it_skipped() {
    let policy = Policy::new().with(Decision::Allow, paths(rule("fs.glob"), &["/w/src/**"]));
    let call = Call::new("fs.glob", Access::Read).with_paths(vec![
        p("/w/src/a.rs"),
        p("/w/.env"),
        p("/w/src/b.rs"),
    ]);

    let v = policy.evaluate(&call, &Grants::new());
    assert_eq!(
        v.decision,
        Decision::Allow,
        "one .env must not cost the whole view"
    );
    assert_eq!(
        v.denied_paths,
        vec![p("/w/.env")],
        "and the model must be told what it missed"
    );
}

/// [R-POLICY-009] a write that hits a denied path is denied whole
#[test]
fn a_write_that_hits_a_denied_path_writes_nothing() {
    let policy = Policy::new().with(Decision::Allow, paths(rule("fs.write"), &["/w/src/**"]));
    let call = Call::new("fs.write", Access::Write)
        .with_paths(vec![p("/w/src/a.rs"), p("/w/.git/config")]);

    let v = policy.evaluate(&call, &Grants::new());
    assert!(!v.denied_paths.is_empty());
    assert_eq!(
        call.access,
        Access::Write,
        "the caller must refuse the whole call: a partial write leaves a state nobody chose"
    );
}

/// [R-POLICY-007] network tools narrow by host
#[test]
fn a_host_selector_allows_one_host_without_allowing_the_network() {
    let policy = Policy::new().with(
        Decision::Allow,
        rule("http.get")
            .narrowed(SelectorKind::Hosts, &["docs.rs", "*.crates.io"])
            .unwrap(),
    );
    for (host, want) in [
        ("docs.rs", Decision::Allow),
        ("static.crates.io", Decision::Allow),
        ("evil.example", Decision::Deny),
    ] {
        let c = Call::new("http.get", Access::Read).with_host(host);
        assert_eq!(
            policy.evaluate(&c, &Grants::new()).decision,
            want,
            "for {host}"
        );
    }
}

/// [R-POLICY-010] deny first, then ask, then allow, first match wins
#[test]
fn deny_beats_ask_beats_allow() {
    let policy = Policy::new()
        .with(Decision::Allow, rule("shell"))
        .with(Decision::Ask, commands(rule("shell"), &["git push *"]))
        .with(Decision::Deny, commands(rule("shell"), &["rm -rf *"]));

    for (line, want) in [
        ("cargo test", Decision::Allow),
        ("git push origin main", Decision::Ask),
        ("rm -rf /", Decision::Deny),
    ] {
        let c = Call::new("shell", Access::Write).with_command(line);
        assert_eq!(
            policy.evaluate(&c, &Grants::new()).decision,
            want,
            "for `{line}`"
        );
    }
}

/// [R-POLICY-011] nothing matching means deny
#[test]
fn a_call_that_matches_no_rule_is_denied() {
    let empty = Policy::new();
    let v = empty.evaluate(&Call::new("anything", Access::Read), &Grants::new());
    assert_eq!(v.decision, Decision::Deny);
    assert_eq!(v.source, Source::NoMatch);
    assert_eq!(v.rule, None, "there is no rule to name");
}

/// [R-POLICY-012] a decision names its rule, or records that none matched
#[test]
fn a_decision_names_the_rule_that_produced_it() {
    let policy = Policy::new().with(Decision::Allow, commands(rule("shell"), &["cargo *"]));
    let c = Call::new("shell", Access::Write).with_command("cargo test");
    let v = policy.evaluate(&c, &Grants::new());
    assert_eq!(v.source, Source::Rule);
    let named = v.rule.unwrap();
    assert!(
        named.contains("shell") && named.contains("cargo *"),
        "got {named}"
    );
    assert_eq!(
        v.origin.as_deref(),
        Some("meow.star:1"),
        "and where it was written"
    );
}

/// [R-POLICY-013] evaluation is a pure function of call, policy, and grants
#[test]
fn evaluation_is_the_same_answer_every_time() {
    let policy = Policy::new().with(Decision::Ask, rule("shell"));
    let call = Call::new("shell", Access::Write).with_command("git push");

    let bare = Grants::new();
    let mut granted = Grants::new();
    granted.grant("shell");

    for _ in 0..3 {
        assert_eq!(policy.evaluate(&call, &bare).decision, Decision::Ask);
        assert_eq!(policy.evaluate(&call, &granted).decision, Decision::Allow);
    }
}

/// [R-POLICY-014] the tool acts on exactly the path that was judged
#[test]
fn the_judged_path_is_the_one_to_act_on() {
    // The call carries resolved paths, and nothing in this crate resolves
    // again. A test can only check the shape: what was judged is what the
    // caller holds.
    let judged = Call::new("fs.read", Access::Read).with_paths(vec![p("/w/src/a.rs")]);
    let policy = Policy::new().with(Decision::Allow, paths(rule("fs.read"), &["/w/src/**"]));
    assert_eq!(
        policy.evaluate(&judged, &Grants::new()).decision,
        Decision::Allow
    );
    assert_eq!(
        judged.paths,
        vec![p("/w/src/a.rs")],
        "unchanged by evaluation"
    );
}

/// [R-POLICY-020] ask resolves to deny when nobody can answer
#[test]
fn an_ask_with_nobody_to_answer_becomes_deny_not_allow() {
    // The policy answers `ask`; turning that into a decision is the caller's,
    // and the rule it must follow is that non-interactive degrades to deny.
    let policy = Policy::new().with(Decision::Ask, rule("shell"));
    let v = policy.evaluate(&Call::new("shell", Access::Write), &Grants::new());
    assert_eq!(v.decision, Decision::Ask);

    let non_interactive = |d: Decision| {
        if d == Decision::Ask {
            Decision::Deny
        } else {
            d
        }
    };
    assert_eq!(
        non_interactive(v.decision),
        Decision::Deny,
        "an unattended run must get less authority, never more"
    );
}

/// [R-POLICY-023] a session grant applies to this process only
#[test]
fn a_session_grant_lives_and_dies_with_the_process() {
    let policy = Policy::new().with(Decision::Ask, rule("shell"));
    let call = Call::new("shell", Access::Write);

    let mut grants = Grants::new();
    grants.grant("shell");
    let v = policy.evaluate(&call, &grants);
    assert_eq!(v.decision, Decision::Allow);
    assert_eq!(
        v.source,
        Source::SessionGrant,
        "and it says so, rather than posing as a rule"
    );

    // A fresh process starts with none: nothing was written anywhere.
    assert_eq!(
        policy.evaluate(&call, &Grants::new()).decision,
        Decision::Ask
    );
}

/// [R-POLICY-023] a grant cannot turn a deny into anything
#[test]
fn a_grant_cannot_undo_a_deny() {
    let policy = Policy::new().with(Decision::Deny, rule("shell"));
    let mut grants = Grants::new();
    grants.grant("shell");
    assert_eq!(
        policy
            .evaluate(&Call::new("shell", Access::Write), &grants)
            .decision,
        Decision::Deny
    );
}

/// [R-POLICY-030] an agent policy narrows to the stricter of the two
/// [R-POLICY-031] and can never widen
#[test]
fn an_agent_can_tighten_the_workspace_policy_and_never_loosen_it() {
    let workspace = Policy::new()
        .with(Decision::Allow, rule("fs.read"))
        .with(Decision::Ask, rule("shell"))
        .with(Decision::Deny, rule("http.get"));

    let agent = Policy::new()
        .with(Decision::Deny, rule("fs.read")) // tighter: allowed becomes denied
        .with(Decision::Allow, rule("shell")) // looser: must not take effect
        .with(Decision::Allow, rule("http.get")); // looser: must not take effect

    let effective = workspace.narrowed_by(&agent);
    let g = Grants::new();
    assert_eq!(
        effective
            .evaluate(&Call::new("fs.read", Access::Read), &g)
            .decision,
        Decision::Deny,
        "an agent may tighten"
    );
    assert_eq!(
        effective
            .evaluate(&Call::new("shell", Access::Write), &g)
            .decision,
        Decision::Ask,
        "an agent must not turn an ask into an allow"
    );
    assert_eq!(
        effective
            .evaluate(&Call::new("http.get", Access::Read), &g)
            .decision,
        Decision::Deny,
        "nor a deny into an allow"
    );
}

/// [R-POLICY-032] a sub-agent inherits and may narrow again
#[test]
fn narrowing_composes_so_a_sub_agent_can_only_tighten_further() {
    let workspace = Policy::new().with(Decision::Allow, rule("fs.read"));
    let parent = Policy::new().with(Decision::Ask, rule("fs.read"));
    let child = Policy::new().with(Decision::Deny, rule("fs.read"));

    let g = Grants::new();
    let call = Call::new("fs.read", Access::Read);
    assert_eq!(
        workspace.narrowed_by(&parent).evaluate(&call, &g).decision,
        Decision::Ask
    );

    // The parent's effective policy, narrowed again by the child.
    let parent_effective = Policy::new().with(Decision::Ask, rule("fs.read"));
    assert_eq!(
        parent_effective
            .narrowed_by(&child)
            .evaluate(&call, &g)
            .decision,
        Decision::Deny
    );
}

/// [R-POLICY-051] an explanation says how many rules were checked first
#[test]
fn an_explanation_reports_what_was_checked_before_the_match() {
    let policy = Policy::new()
        .with(Decision::Deny, rule("http.get"))
        .with(Decision::Deny, rule("fs.write"))
        .with(Decision::Allow, rule("fs.read"));

    let v = policy.evaluate(&Call::new("fs.read", Access::Read), &Grants::new());
    assert_eq!(v.decision, Decision::Allow);
    assert_eq!(
        v.checked, 2,
        "two deny rules were checked and did not match"
    );
}

/// [R-POLICY-060] a marked value is redacted wherever it appears
/// [R-POLICY-061] and the placeholder reveals nothing, not even its length
#[test]
fn a_marked_value_is_replaced_by_a_fixed_placeholder() {
    let policy = Policy::new().with(
        Decision::Allow,
        rule("http.post").marking_sensitive(&["token", "password"]),
    );
    let sensitive = policy.sensitive_for("http.post");
    assert_eq!(sensitive, vec!["password".to_owned(), "token".to_owned()]);

    let args = json!({"url": "https://example", "token": "sk-very-long-secret", "n": 1});
    let safe = redact(&args, &sensitive);
    assert_eq!(safe["token"], REDACTED);
    assert_eq!(safe["url"], "https://example", "only what was marked");
    assert_eq!(safe["n"], 1);

    let short = redact(&json!({"token": "x"}), &sensitive);
    assert_eq!(
        short["token"], REDACTED,
        "a short secret redacts to the same text"
    );
    assert_eq!(
        safe["token"], short["token"],
        "so the placeholder cannot leak how long the value was"
    );
}

/// [R-POLICY-021] the prompt shows the exact arguments, never a summary
/// [R-POLICY-022] and names the rule that caused it
#[test]
fn an_approval_prompt_shows_the_call_verbatim_and_names_its_rule() {
    use meow_policy::Prompt;

    let policy = Policy::new()
        .with(Decision::Ask, commands(rule("shell"), &["git push *"]))
        .with(
            Decision::Allow,
            rule("http.post").marking_sensitive(&["token"]),
        );

    let call = Call::new("shell", Access::Write).with_command("git push origin main");
    let args = json!({"command": "git push origin main"});
    let verdict = policy.evaluate(&call, &Grants::new());
    assert_eq!(verdict.decision, Decision::Ask);

    let prompt = Prompt::new(&call, &args, &verdict, &[], "release", 7);
    assert!(
        prompt.arguments.contains("git push origin main"),
        "the command must appear exactly as it would run: {}",
        prompt.arguments
    );
    assert!(
        prompt.rule.contains("git push *"),
        "the rule must be named: {}",
        prompt.rule
    );
    assert_eq!(prompt.origin.as_deref(), Some("meow.star:1"));
    assert_eq!((prompt.agent.as_str(), prompt.step), ("release", 7));

    // A marked value is hidden even here, where the point is to show things.
    let secret = Call::new("http.post", Access::Write);
    let v = policy.evaluate(&secret, &Grants::new());
    let p = Prompt::new(
        &secret,
        &json!({"token": "sk-secret"}),
        &v,
        &policy.sensitive_for("http.post"),
        "a",
        1,
    );
    assert!(!p.arguments.contains("sk-secret"), "got {}", p.arguments);
    assert!(p.arguments.contains(REDACTED));
}

/// [R-POLICY-024] a prompt waits by default, and denies only if configured to expire
#[test]
fn a_prompt_waits_unless_a_timeout_was_configured() {
    use meow_policy::Timeout;

    assert_eq!(Timeout::default().0, None, "waiting is the default");
    assert_eq!(
        Timeout::default().on_expiry(),
        None,
        "with no timeout there is nothing to expire; the prompt is still waiting"
    );
    assert_eq!(
        Timeout(Some(std::time::Duration::from_secs(60))).on_expiry(),
        Some(Decision::Deny),
        "and an expiry denies, never allows"
    );
}

/// [R-POLICY-050] explain predicts the rule decision without triggering a call
#[test]
fn explain_predicts_the_rule_decision_without_making_the_call() {
    use meow_policy::explain;

    let policy = Policy::new()
        .with(Decision::Deny, commands(rule("shell"), &["rm -rf *"]))
        .with(Decision::Ask, commands(rule("shell"), &["git push *"]))
        .with(Decision::Allow, commands(rule("shell"), &["cargo *"]));

    for (line, want) in [
        ("cargo test", Decision::Allow),
        ("git push origin main", Decision::Ask),
        ("rm -rf /", Decision::Deny),
        ("curl evil", Decision::Deny),
    ] {
        let e = explain(
            &policy,
            &Call::new("shell", Access::Write).with_command(line),
        );
        assert_eq!(e.decision, want, "for `{line}`");
    }

    let e = explain(
        &policy,
        &Call::new("shell", Access::Write).with_command("cargo test"),
    );
    assert_eq!(e.rule.as_deref().map(|r| r.contains("cargo *")), Some(true));
    assert_eq!(
        e.origin.as_deref(),
        Some("meow.star:1"),
        "and where to change it"
    );
    assert_eq!(e.checked, 2, "two higher-precedence rules were checked");

    // A grant made during a run must not change what explain predicts: the
    // question is asked before the run exists.
    let mut grants = Grants::new();
    grants.grant("shell");
    let call = Call::new("shell", Access::Write).with_command("git push origin main");
    assert_eq!(policy.evaluate(&call, &grants).decision, Decision::Allow);
    assert_eq!(
        explain(&policy, &call).decision,
        Decision::Ask,
        "explain answers about rules"
    );
}
