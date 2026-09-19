// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Every requirement in `docs/spec/agent.md` that M4 covers.
//!
//! Compaction, sub-agents, and concurrency are M6; policy is M5.

// `allow-unwrap-in-tests` in clippy.toml covers `#[test]` functions, not the
// helpers beside them.
#![allow(clippy::unwrap_used)]

use std::sync::Arc;
use std::sync::atomic::{AtomicU32, Ordering};
use std::time::Duration;

use meow_agent::{
    AgentEvent, AgentSpec, Axis, Budget, Checked, Collect, Discard, Engine, Ledger, Tool,
    ToolError, ToolErrorPolicy, ToolSet, check,
};
use meow_core::{StopReason, Usage};
use meow_llm::{Capabilities, LlmError, Provider, Request, Response, Structured, ToolCall};
use serde_json::{Value, json};
use tokio_util::sync::CancellationToken;

/// A provider that answers from a script, so a test decides what the model
/// does without a network or an account.
struct Scripted {
    replies: std::sync::Mutex<Vec<Response>>,
    asked: AtomicU32,
}

impl Scripted {
    fn new(replies: Vec<Response>) -> Arc<Self> {
        Arc::new(Self {
            replies: std::sync::Mutex::new(replies.into_iter().rev().collect()),
            asked: AtomicU32::new(0),
        })
    }
    fn asked(&self) -> u32 {
        self.asked.load(Ordering::SeqCst)
    }
}

#[async_trait::async_trait]
impl Provider for Scripted {
    fn name(&self) -> &str {
        "scripted"
    }
    fn capabilities(&self) -> Capabilities {
        Capabilities {
            streaming: false,
            tools: true,
            structured: Structured::Emulated,
            embeddings: false,
        }
    }
    async fn generate(
        &self,
        _: &Request,
        cancel: &CancellationToken,
    ) -> meow_llm::Result<Response> {
        if cancel.is_cancelled() {
            return Err(LlmError::Cancelled);
        }
        self.asked.fetch_add(1, Ordering::SeqCst);
        self.replies
            .lock()
            .ok()
            .and_then(|mut r| r.pop())
            .ok_or(LlmError::Transport {
                provider: "scripted".into(),
                message: "the script ran out".into(),
            })
    }
}

fn says(text: &str) -> Response {
    Response {
        text: text.to_owned(),
        thinking: None,
        tool_calls: Vec::new(),
        usage: Some(Usage {
            prompt: 10,
            completion: 5,
            cached: None,
            cost_micros: Some(100),
        }),
        value: None,
    }
}

fn calls(name: &str, args: &str) -> Response {
    Response {
        text: String::new(),
        thinking: None,
        tool_calls: vec![ToolCall {
            id: format!("c-{name}"),
            name: name.to_owned(),
            arguments: args.to_owned(),
        }],
        usage: Some(Usage {
            prompt: 10,
            completion: 5,
            cached: None,
            cost_micros: Some(100),
        }),
        value: None,
    }
}

/// A tool a test can make succeed, fail, or count.
struct Probe {
    name: &'static str,
    schema: Value,
    fail: bool,
    ran: Arc<AtomicU32>,
}

impl Probe {
    fn new(name: &'static str) -> Self {
        Self {
            name,
            schema: json!({"type": "object", "properties": {}, "required": []}),
            fail: false,
            ran: Arc::new(AtomicU32::new(0)),
        }
    }
    fn requiring(mut self, field: &str, kind: &str) -> Self {
        self.schema = json!({
            "type": "object",
            "properties": { field: {"type": kind} },
            "required": [field]
        });
        self
    }
    fn failing(mut self) -> Self {
        self.fail = true;
        self
    }
}

#[async_trait::async_trait]
impl Tool for Probe {
    fn name(&self) -> &str {
        self.name
    }
    fn description(&self) -> &str {
        "a probe"
    }
    fn schema(&self) -> Value {
        self.schema.clone()
    }
    async fn call(&self, args: &Value, _: &CancellationToken) -> Result<String, ToolError> {
        self.ran.fetch_add(1, Ordering::SeqCst);
        if self.fail {
            return Err(ToolError("the probe was told to fail".into()));
        }
        Ok(args.to_string())
    }
}

async fn run(spec: &AgentSpec, provider: Arc<dyn Provider>) -> meow_agent::Outcome {
    let ledger = Ledger::new(spec.budget);
    Engine::new(provider)
        .run(
            spec,
            "a task",
            &ledger,
            &mut Discard,
            &CancellationToken::new(),
        )
        .await
}

/// [R-AGENT-001] every run returns an outcome, never an error
/// [R-AGENT-003] the text survives whatever the stop reason
#[tokio::test]
async fn a_budget_stop_keeps_the_text_instead_of_discarding_it() {
    let provider = Scripted::new(vec![
        calls("probe", "{}"),
        calls("probe", "{}"),
        says("the answer so far"),
    ]);
    let spec = AgentSpec::new("a", "m")
        .with_tools(ToolSet::new().with(Probe::new("probe")))
        .with_budget(Budget {
            steps: Some(2),
            ..Budget::unbounded()
        });

    let outcome = run(&spec, provider).await;
    assert_eq!(outcome.stop, StopReason::Budget);
    assert_eq!(outcome.detail.as_deref(), Some("steps"));
    assert_eq!(outcome.steps.len(), 2, "the transcript survives the stop");
}

/// [R-AGENT-002] the six stop reasons
#[test]
fn there_are_exactly_six_stop_reasons() {
    let all = [
        StopReason::Finished,
        StopReason::Budget,
        StopReason::Cancelled,
        StopReason::Denied,
        StopReason::ToolAborted,
        StopReason::Failed,
    ];
    let names: std::collections::BTreeSet<&str> = all.iter().map(|s| s.as_str()).collect();
    assert_eq!(names.len(), 6);
    for s in all {
        assert_eq!(
            StopReason::parse(s.as_str()),
            Some(s),
            "{s} must round-trip"
        );
    }
}

/// [R-AGENT-004] the outcome names what bound the run
#[tokio::test]
async fn the_outcome_says_which_axis_bound_it() {
    for (budget, want) in [
        (
            Budget {
                steps: Some(1),
                ..Budget::unbounded()
            },
            "steps",
        ),
        (
            Budget {
                tokens: Some(1),
                ..Budget::unbounded()
            },
            "tokens",
        ),
        (
            Budget {
                cost_micros: Some(1),
                ..Budget::unbounded()
            },
            "cost",
        ),
    ] {
        let provider = Scripted::new(vec![calls("probe", "{}"), calls("probe", "{}")]);
        let spec = AgentSpec::new("a", "m")
            .with_tools(ToolSet::new().with(Probe::new("probe")))
            .with_budget(budget);
        let outcome = run(&spec, provider).await;
        assert_eq!(outcome.stop, StopReason::Budget);
        assert_eq!(outcome.detail.as_deref(), Some(want));
    }
}

/// [R-AGENT-005] a response with no tool calls finishes, even with empty text
#[tokio::test]
async fn an_empty_answer_finishes_and_says_it_was_empty() {
    let outcome = run(&AgentSpec::new("a", "m"), Scripted::new(vec![says("")])).await;
    assert_eq!(
        outcome.stop,
        StopReason::Finished,
        "empty text is not a failure"
    );
    assert_eq!(
        outcome.detail.as_deref(),
        Some("the model returned no text"),
        "but a caller must not be handed a silent success"
    );
}

/// [R-AGENT-010] four axes
/// [R-AGENT-012] an unset axis is unbounded, and no budget takes the default
#[test]
fn an_unset_axis_is_unbounded_and_the_default_is_bounded() {
    let none = Budget::unbounded();
    assert!(none.tokens.is_none() && none.steps.is_none());
    assert!(Ledger::new(none).exceeded().is_none());

    let d = Budget::default();
    assert!(d.tokens.is_some() && d.steps.is_some() && d.duration.is_some());
    assert_eq!(
        AgentSpec::new("a", "m").budget,
        d,
        "a spec with no budget takes the default"
    );
}

/// [R-AGENT-011] the run stops as soon as an axis is reached
#[test]
fn a_reached_axis_refuses_the_next_step() {
    let l = Ledger::new(Budget {
        steps: Some(2),
        ..Budget::unbounded()
    });
    assert!(l.reserve_step().is_ok());
    assert!(l.reserve_step().is_ok());
    assert_eq!(l.reserve_step(), Err(Axis::Steps));
}

/// [R-AGENT-013] a child's spend reaches its caller
/// [R-AGENT-014] a child cannot be given more than its caller has left
#[test]
fn a_child_spends_its_callers_budget_and_cannot_exceed_it() {
    let parent = Ledger::new(Budget {
        tokens: Some(100),
        steps: Some(10),
        ..Budget::unbounded()
    });
    parent.reserve_step().unwrap();
    parent.charge(&Usage {
        prompt: 30,
        completion: 0,
        cached: None,
        cost_micros: None,
    });

    // Asking for more than the caller has left is narrowed to what is left.
    let child = parent.child(Budget {
        tokens: Some(1_000),
        steps: Some(50),
        ..Budget::unbounded()
    });
    child.reserve_step().unwrap();
    child.charge(&Usage {
        prompt: 50,
        completion: 0,
        cached: None,
        cost_micros: None,
    });

    // The parent sees what the child spent, which is what makes the cap real.
    assert_eq!(parent.exceeded(), None);
    child.charge(&Usage {
        prompt: 30,
        completion: 0,
        cached: None,
        cost_micros: None,
    });
    assert_eq!(
        parent.exceeded(),
        Some(Axis::Tokens),
        "the child's spend must reach the parent"
    );
}

/// [R-AGENT-015] the budget is checked before a call, not only after
#[tokio::test]
async fn the_budget_is_checked_before_the_call_not_after_it() {
    let provider = Scripted::new(vec![says("never asked")]);
    let spec = AgentSpec::new("a", "m").with_budget(Budget {
        steps: Some(0),
        ..Budget::unbounded()
    });
    let asked = Arc::clone(&provider) as Arc<dyn Provider>;
    let counted = Arc::clone(&provider);

    let outcome = run(&spec, asked).await;
    assert_eq!(outcome.stop, StopReason::Budget);
    assert_eq!(
        counted.asked(),
        0,
        "a spent budget must not spend one more call to find out"
    );
}

/// [R-AGENT-016] the default budget
#[test]
fn the_default_budget_is_the_one_the_specification_names() {
    let d = Budget::default();
    assert_eq!(d.tokens, Some(200_000));
    assert_eq!(d.steps, Some(40));
    assert_eq!(d.duration, Some(Duration::from_secs(1800)));
    assert_eq!(
        d.cost_micros, None,
        "cost needs a price before the call, which nothing has"
    );
}

/// [R-AGENT-017] budget is reserved, so concurrent runs cannot overspend it
#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn concurrent_runs_cannot_each_spend_the_same_remaining_budget() {
    let ledger = Ledger::new(Budget {
        steps: Some(5),
        ..Budget::unbounded()
    });
    let mut tasks = Vec::new();
    for _ in 0..8 {
        let l = ledger.clone();
        tasks.push(tokio::spawn(async move { l.reserve_step().is_ok() }));
    }
    let mut granted = 0;
    for t in tasks {
        if t.await.unwrap() {
            granted += 1;
        }
    }
    assert_eq!(granted, 5, "exactly the budget, whatever the interleaving");
}

/// [R-AGENT-020] arguments are checked against the schema first
/// [R-AGENT-021] a missing required argument is a correction, never a zero
#[test]
fn a_missing_required_argument_is_corrected_rather_than_invented() {
    let schema = json!({
        "type": "object",
        "properties": { "path": {"type": "string"}, "count": {"type": "integer"} },
        "required": ["path", "count"]
    });
    match check(&json!({"path": "a.rs"}), &schema) {
        Checked::Correction(m) => {
            assert!(m.contains("count"), "the argument must be named: {m}");
            assert!(m.contains("integer"), "and its type: {m}");
        }
        other => panic!("expected a correction, got {other:?}"),
    }
    // v0.2.x filled this with 0 and ran the tool anyway.
    assert!(matches!(
        check(&json!({"path": "a", "count": 3}), &schema),
        Checked::Ready(_)
    ));
}

/// [R-AGENT-021] a correction never aborts, whatever the error policy
#[tokio::test]
async fn an_argument_correction_does_not_abort_an_aborting_agent() {
    let provider = Scripted::new(vec![calls("probe", "{}"), says("fixed it")]);
    let spec = AgentSpec::new("a", "m")
        .with_tools(ToolSet::new().with(Probe::new("probe").requiring("path", "string")))
        .on_tool_error(ToolErrorPolicy::Abort);

    let outcome = run(&spec, provider).await;
    assert_eq!(
        outcome.stop,
        StopReason::Finished,
        "a typo must not kill the run"
    );
    assert_eq!(outcome.text, "fixed it");
}

/// [R-AGENT-022] an absent optional argument stays absent, or takes its default
#[test]
fn an_absent_optional_argument_is_absent_rather_than_zero() {
    let schema = json!({
        "type": "object",
        "properties": {
            "limit": {"type": "integer", "default": 20},
            "strict": {"type": "boolean"}
        },
        "required": []
    });
    let Checked::Ready(args) = check(&json!({}), &schema) else {
        panic!("nothing was required")
    };
    assert_eq!(args["limit"], 20, "a declared default is taken");
    assert!(
        args.get("strict").is_none(),
        "one with no default stays absent; false would be a different fact"
    );
}

/// [R-AGENT-023] a tool the agent was not given stays unreachable
#[tokio::test]
async fn a_tool_the_agent_was_not_given_is_not_found_anywhere() {
    let ran = Arc::new(AtomicU32::new(0));
    let probe = Probe::new("granted");
    let counter = Arc::clone(&probe.ran);
    let provider = Scripted::new(vec![calls("forbidden", "{}"), says("gave up")]);
    let spec = AgentSpec::new("a", "m").with_tools(ToolSet::new().with(probe));

    let outcome = run(&spec, provider).await;
    assert_eq!(outcome.stop, StopReason::Finished);
    assert_eq!(counter.load(Ordering::SeqCst), 0);
    assert_eq!(ran.load(Ordering::SeqCst), 0);
}

/// [R-AGENT-024] report continues, abort stops
#[tokio::test]
async fn a_failing_tool_reports_or_aborts_as_declared() {
    let provider = Scripted::new(vec![calls("probe", "{}"), says("recovered")]);
    let reporting = AgentSpec::new("a", "m")
        .with_tools(ToolSet::new().with(Probe::new("probe").failing()))
        .on_tool_error(ToolErrorPolicy::Report);
    let outcome = run(&reporting, provider).await;
    assert_eq!(outcome.stop, StopReason::Finished);
    assert_eq!(
        outcome.text, "recovered",
        "the model saw the error and carried on"
    );

    let provider = Scripted::new(vec![calls("probe", "{}"), says("never reached")]);
    let aborting = AgentSpec::new("a", "m")
        .with_tools(ToolSet::new().with(Probe::new("probe").failing()))
        .on_tool_error(ToolErrorPolicy::Abort);
    let outcome = run(&aborting, provider).await;
    assert_eq!(outcome.stop, StopReason::ToolAborted);
    assert_eq!(outcome.detail.as_deref(), Some("probe"));
}

/// [R-AGENT-025] tool calls run in the order the model returned them
#[tokio::test]
async fn tool_calls_run_in_the_order_they_arrived() {
    let mut reply = calls("first", "{}");
    reply.tool_calls.push(ToolCall {
        id: "c2".into(),
        name: "second".into(),
        arguments: "{}".into(),
    });
    let provider = Scripted::new(vec![reply, says("done")]);
    let spec = AgentSpec::new("a", "m").with_tools(
        ToolSet::new()
            .with(Probe::new("first"))
            .with(Probe::new("second")),
    );

    let ledger = Ledger::new(spec.budget);
    let mut sink = Collect::with_deltas();
    Engine::new(provider)
        .run(&spec, "t", &ledger, &mut sink, &CancellationToken::new())
        .await;

    let order: Vec<&str> = sink
        .events
        .iter()
        .filter_map(|e| match e {
            AgentEvent::ToolStart { name, .. } => Some(name.as_str()),
            _ => None,
        })
        .collect();
    assert_eq!(order, vec!["first", "second"]);
}

/// [R-AGENT-030] cancellation is checked before a call
/// [R-AGENT-031] a cancelled run stops, says so, and keeps its transcript
#[tokio::test]
async fn a_cancelled_run_stops_and_keeps_what_it_had() {
    let provider = Scripted::new(vec![calls("probe", "{}"), says("never reached")]);
    let spec = AgentSpec::new("a", "m").with_tools(ToolSet::new().with(Probe::new("probe")));
    let ledger = Ledger::new(spec.budget);
    let cancel = CancellationToken::new();
    let counted = Arc::clone(&provider);

    // Cancelled before the second step, after one has been recorded.
    let engine = Engine::new(provider as Arc<dyn Provider>);
    let token = cancel.clone();
    let mut sink = CancelAfter {
        token,
        seen: 0,
        events: Vec::new(),
    };
    let outcome = engine.run(&spec, "t", &ledger, &mut sink, &cancel).await;

    assert_eq!(outcome.stop, StopReason::Cancelled);
    assert_eq!(counted.asked(), 1, "no call after the cancellation");
    assert_eq!(outcome.steps.len(), 1, "the transcript survives");
}

struct CancelAfter {
    token: CancellationToken,
    seen: u32,
    events: Vec<AgentEvent>,
}

impl meow_agent::Sink for CancelAfter {
    fn event(&mut self, event: AgentEvent) -> Result<(), String> {
        if matches!(event, AgentEvent::StepEnd { .. }) {
            self.seen += 1;
            if self.seen == 1 {
                self.token.cancel();
            }
        }
        self.events.push(event);
        Ok(())
    }
    fn wants_deltas(&self) -> bool {
        false
    }
}

/// [R-AGENT-032] cancelling a parent cancels what is running inside it
#[test]
fn cancelling_a_parent_cancels_its_children() {
    let parent = CancellationToken::new();
    let child = parent.child_token();
    assert!(!child.is_cancelled());
    parent.cancel();
    assert!(
        child.is_cancelled(),
        "a sub-agent must not outlive its caller's cancellation"
    );
}

/// [R-AGENT-070] every transition reaches the sink
#[tokio::test]
async fn the_sink_sees_every_transition() {
    let provider = Scripted::new(vec![calls("probe", "{}"), says("done")]);
    let spec = AgentSpec::new("a", "m").with_tools(ToolSet::new().with(Probe::new("probe")));
    let ledger = Ledger::new(spec.budget);
    let mut sink = Collect::with_deltas();
    Engine::new(provider)
        .run(&spec, "t", &ledger, &mut sink, &CancellationToken::new())
        .await;

    let names: Vec<&str> = sink
        .events
        .iter()
        .map(|e| match e {
            AgentEvent::RunStart { .. } => "run start",
            AgentEvent::StepStart { .. } => "step start",
            AgentEvent::ToolStart { .. } => "tool start",
            AgentEvent::ToolEnd { .. } => "tool end",
            AgentEvent::Usage(_) => "usage",
            AgentEvent::StepEnd { .. } => "step end",
            AgentEvent::RunEnd { .. } => "run end",
            _ => "other",
        })
        .collect();
    for want in [
        "run start",
        "step start",
        "tool start",
        "tool end",
        "usage",
        "step end",
        "run end",
    ] {
        assert!(
            names.contains(&want),
            "{want} never reached the sink: {names:?}"
        );
    }
}

/// [R-AGENT-071] a broken sink stops receiving, and the run carries on
#[tokio::test]
async fn a_broken_sink_does_not_destroy_the_run() {
    struct Breaks(u32);
    impl meow_agent::Sink for Breaks {
        fn event(&mut self, _e: AgentEvent) -> Result<(), String> {
            self.0 += 1;
            Err("the renderer fell over".into())
        }
        fn wants_deltas(&self) -> bool {
            false
        }
    }

    let provider = Scripted::new(vec![calls("probe", "{}"), says("finished anyway")]);
    let spec = AgentSpec::new("a", "m").with_tools(ToolSet::new().with(Probe::new("probe")));
    let ledger = Ledger::new(spec.budget);
    let mut sink = Breaks(0);
    let outcome = Engine::new(provider)
        .run(&spec, "t", &ledger, &mut sink, &CancellationToken::new())
        .await;

    assert_eq!(
        outcome.stop,
        StopReason::Finished,
        "rendering is not the work"
    );
    assert_eq!(outcome.text, "finished anyway");
    assert_eq!(sink.0, 1, "and a sink that failed stops being called");
}

/// [R-AGENT-072] the engine works with nothing attached
#[tokio::test]
async fn the_engine_runs_with_no_sink() {
    let outcome = run(
        &AgentSpec::new("a", "m"),
        Scripted::new(vec![says("alone")]),
    )
    .await;
    assert_eq!(outcome.text, "alone");
}

/// [R-AGENT-073] a sink may decline deltas and get the step's text once
#[tokio::test]
async fn a_sink_that_declines_deltas_gets_the_text_once_per_step() {
    let provider = Scripted::new(vec![says("a whole answer")]);
    let spec = AgentSpec::new("a", "m");
    let ledger = Ledger::new(spec.budget);
    let mut sink = Collect::without_deltas();
    Engine::new(provider)
        .run(&spec, "t", &ledger, &mut sink, &CancellationToken::new())
        .await;

    assert!(
        !sink.events.iter().any(|e| matches!(e, AgentEvent::Text(_))),
        "a sink that declined must not be paid a call per token"
    );
    assert!(
        sink.events
            .iter()
            .any(|e| matches!(e, AgentEvent::StepText { text, .. } if text == "a whole answer"))
    );
}

/// [R-AGENT-080] a parsed value is present when the run finished, absent otherwise
#[tokio::test]
async fn a_parsed_value_arrives_only_with_a_finished_run() {
    let schema =
        json!({"type": "object", "properties": {"n": {"type": "integer"}}, "required": ["n"]});

    let mut reply = says("{\"n\":1}");
    reply.value = Some(json!({"n": 1}));
    let spec = AgentSpec::new("a", "m").with_output(schema.clone());
    let outcome = run(&spec, Scripted::new(vec![reply])).await;
    assert_eq!(outcome.stop, StopReason::Finished);
    assert_eq!(outcome.value, Some(json!({"n": 1})));

    // A run that stopped early has no complete answer to hand over.
    let mut partial = calls("probe", "{}");
    partial.value = Some(json!({"n": 1}));
    let spec = AgentSpec::new("a", "m")
        .with_output(schema)
        .with_tools(ToolSet::new().with(Probe::new("probe")))
        .with_budget(Budget {
            steps: Some(1),
            ..Budget::unbounded()
        });
    let outcome = run(&spec, Scripted::new(vec![partial])).await;
    assert_eq!(outcome.stop, StopReason::Budget);
    assert_eq!(
        outcome.value, None,
        "a partial answer must not wear the shape of a complete one"
    );
}

/// [R-AGENT-081] a response the provider could not make fit fails the run
#[tokio::test]
async fn a_schema_the_provider_gave_up_on_fails_the_run() {
    struct AlwaysWrong;
    #[async_trait::async_trait]
    impl Provider for AlwaysWrong {
        fn name(&self) -> &str {
            "wrong"
        }
        fn capabilities(&self) -> Capabilities {
            Capabilities {
                streaming: false,
                tools: false,
                structured: Structured::Emulated,
                embeddings: false,
            }
        }
        async fn generate(&self, _: &Request, _: &CancellationToken) -> meow_llm::Result<Response> {
            // The provider has already retried per R-LLM-051 and given up.
            Err(LlmError::Schema {
                attempts: 3,
                message: "missing `n`".into(),
            })
        }
    }

    let spec = AgentSpec::new("a", "m").with_output(json!({"type": "object"}));
    let outcome = run(&spec, Arc::new(AlwaysWrong)).await;
    assert_eq!(outcome.stop, StopReason::Failed);
    assert!(outcome.detail.unwrap_or_default().contains("schema"));
    assert_eq!(outcome.value, None);
}

/// [R-POLICY-040] the policy decides before the tool runs
/// [R-POLICY-041] a denied call tells the model which tool and why
/// [R-POLICY-042] every evaluation produces an event
/// [R-AGENT-006] `denied` is the stop reason when policy refused
#[tokio::test]
async fn policy_stops_a_tool_before_it_runs_and_says_so() {
    use meow_policy::{Access, Call, Decision, Policy, Rule};

    let probe = Probe::new("shell");
    let ran = Arc::clone(&probe.ran);
    let policy = Policy::new().with(
        Decision::Allow,
        Rule::new("meow.star:3", "fs.read").unwrap(),
    );
    let describe: meow_agent::DescribeCall =
        Arc::new(|name: &str, _args: &Value| Call::new(name, Access::Write));

    let provider = Scripted::new(vec![calls("shell", "{}"), says("tried something else")]);
    let spec = AgentSpec::new("a", "m")
        .with_tools(ToolSet::new().with(probe))
        .with_policy(policy, describe);

    let ledger = Ledger::new(spec.budget);
    let mut sink = Collect::without_deltas();
    let outcome = Engine::new(provider)
        .run(&spec, "t", &ledger, &mut sink, &CancellationToken::new())
        .await;

    assert_eq!(ran.load(Ordering::SeqCst), 0, "the tool must not have run");
    assert_eq!(
        outcome.stop,
        StopReason::Finished,
        "report continues by default"
    );
    assert!(
        sink.events
            .iter()
            .any(|e| matches!(e, AgentEvent::Policy { decision, .. } if decision == "deny")),
        "the decision must be recorded"
    );
}

/// [R-AGENT-006] denied and tool_aborted are told apart
#[tokio::test]
async fn a_denial_stops_with_denied_and_a_failure_with_tool_aborted() {
    use meow_policy::{Access, Call, Decision, Policy, Rule};

    let describe: meow_agent::DescribeCall =
        Arc::new(|name: &str, _args: &Value| Call::new(name, Access::Write));

    // Policy refuses: the boundary held.
    let provider = Scripted::new(vec![calls("shell", "{}")]);
    let spec = AgentSpec::new("a", "m")
        .with_tools(ToolSet::new().with(Probe::new("shell")))
        .with_policy(Policy::new(), describe.clone())
        .on_tool_error(ToolErrorPolicy::Abort);
    let outcome = run(&spec, provider).await;
    assert_eq!(outcome.stop, StopReason::Denied);

    // The tool runs and fails: something broke.
    let provider = Scripted::new(vec![calls("shell", "{}")]);
    let allowed = Policy::new().with(Decision::Allow, Rule::new("meow.star:1", "shell").unwrap());
    let spec = AgentSpec::new("a", "m")
        .with_tools(ToolSet::new().with(Probe::new("shell").failing()))
        .with_policy(allowed, describe)
        .on_tool_error(ToolErrorPolicy::Abort);
    let outcome = run(&spec, provider).await;
    assert_eq!(outcome.stop, StopReason::ToolAborted);
}

// ---------------------------------------------------------------------------
// M6: compaction, sub-agents, concurrency
// ---------------------------------------------------------------------------

fn long_message(n: usize) -> meow_llm::Message {
    meow_llm::Message::new(meow_llm::Role::User, "x".repeat(n))
}

/// [R-AGENT-040] compaction happens before the call that would not fit
/// [R-AGENT-041] the most recent messages stay verbatim
#[test]
fn compaction_triggers_on_the_threshold_and_keeps_the_recent_ones() {
    use meow_agent::{Compaction, estimate_tokens, range_to_compact};

    let policy = Compaction {
        at: 0.8,
        keep_recent: 2,
        model: None,
    };
    let mut messages = vec![meow_llm::Message::new(meow_llm::Role::System, "system")];
    messages.extend((0..10).map(|_| long_message(400)));

    let window = estimate_tokens(&messages) * 2;
    assert_eq!(
        range_to_compact(&messages, &policy, window),
        None,
        "under the threshold"
    );

    let window = (estimate_tokens(&messages) as f32 / 0.9) as u32;
    let range = range_to_compact(&messages, &policy, window).expect("over the threshold");
    assert_eq!(range.start, 1, "the system message stays");
    assert_eq!(range.end, messages.len() - 2, "the two most recent stay");
}

/// [R-AGENT-042] compaction records the range and deletes nothing
/// [R-AGENT-044] it summarises rather than dropping, and says what it saved
#[tokio::test]
async fn compaction_summarises_and_records_what_it_replaced() {
    use meow_agent::Compaction;

    // Step one asks for a tool, which grows the conversation; step two is
    // where compaction has something to do.
    // A bulky first turn, so the summary genuinely replaces more than it
    // costs. On a two-message conversation a summary is longer than what it
    // replaces, and reporting a saving of zero there is truthful rather than
    // a defect.
    let mut bulky = calls("probe", "{}");
    bulky.text = "a long explanation of what the agent is about to do. ".repeat(40);
    let provider = Scripted::new(vec![bulky, says("a summary"), says("done")]);

    let spec = AgentSpec::new("a", "m")
        .with_tools(ToolSet::new().with(Probe::new("probe")))
        .with_context_window(10)
        .with_compaction(Compaction {
            at: 0.1,
            keep_recent: 1,
            model: None,
        })
        .with_system("a system prompt long enough to cross the threshold on its own");

    let ledger = Ledger::new(spec.budget);
    let mut sink = Collect::without_deltas();
    Engine::new(provider)
        .run(
            &spec,
            "and a task",
            &ledger,
            &mut sink,
            &CancellationToken::new(),
        )
        .await;

    let compacted = sink.events.iter().find_map(|e| match e {
        AgentEvent::Compacted {
            supersedes,
            summary,
            tokens_saved,
        } => Some((supersedes.clone(), summary.clone(), *tokens_saved)),
        _ => None,
    });
    let (range, summary, saved) = compacted.expect("a compaction should have been recorded");
    assert!(!range.is_empty(), "the range it replaced must be named");
    assert_eq!(summary, "a summary", "and the summary that stands in");
    assert!(saved > 0, "and what it saved, so the cost is visible");
}

/// [R-AGENT-043] a compaction that fails stops the run
#[tokio::test]
async fn a_failed_compaction_fails_the_run_rather_than_sending_an_over_long_context() {
    use meow_agent::Compaction;

    // One reply grows the conversation; the script then runs out, so the
    // summariser's call fails.
    let provider = Scripted::new(vec![calls("probe", "{}")]);
    let spec = AgentSpec::new("a", "m")
        .with_tools(ToolSet::new().with(Probe::new("probe")))
        .with_context_window(10)
        .with_compaction(Compaction {
            at: 0.1,
            keep_recent: 1,
            model: None,
        })
        .with_system("a system prompt long enough to cross the threshold on its own");

    let outcome = run(&spec, provider).await;
    assert_eq!(outcome.stop, StopReason::Failed);
    assert!(
        outcome.detail.unwrap_or_default().contains("compaction"),
        "the run must say what failed"
    );
}

/// [R-AGENT-045] compaction may name its own model
#[tokio::test]
async fn compaction_uses_its_own_model_when_given_one() {
    use meow_agent::Compaction;

    // A provider that records which model each request named.
    struct Watching(std::sync::Mutex<Vec<String>>);
    #[async_trait::async_trait]
    impl Provider for Watching {
        fn name(&self) -> &str {
            "watching"
        }
        fn capabilities(&self) -> Capabilities {
            Capabilities {
                streaming: false,
                tools: false,
                structured: Structured::Emulated,
                embeddings: false,
            }
        }
        async fn generate(&self, r: &Request, _: &CancellationToken) -> meow_llm::Result<Response> {
            let first = self.0.lock().map(|s| s.is_empty()).unwrap_or(false);
            if let Ok(mut seen) = self.0.lock() {
                seen.push(r.model.clone());
            }
            // Grow the conversation once, so there is something to summarise.
            Ok(if first {
                calls("probe", "{}")
            } else {
                says("ok")
            })
        }
    }

    let provider = Arc::new(Watching(std::sync::Mutex::new(Vec::new())));
    let spec = AgentSpec::new("a", "expensive")
        .with_tools(ToolSet::new().with(Probe::new("probe")))
        .with_context_window(10)
        .with_compaction(Compaction {
            at: 0.1,
            keep_recent: 1,
            model: Some("cheap".into()),
        })
        .with_system("a system prompt long enough to cross the threshold on its own");

    let ledger = Ledger::new(spec.budget);
    Engine::new(Arc::clone(&provider) as Arc<dyn Provider>)
        .run(&spec, "t", &ledger, &mut Discard, &CancellationToken::new())
        .await;

    let seen = provider.0.lock().unwrap().clone();
    assert!(
        seen.contains(&"cheap".to_owned()),
        "the summariser is cheap: {seen:?}"
    );
    assert_eq!(
        seen.first().map(String::as_str),
        Some("expensive"),
        "the agent itself is not"
    );
}

/// [R-AGENT-050] a sub-agent runs on its own, under the caller's budget
/// [R-AGENT-051] its outcome comes back with text, stop reason, and value
#[tokio::test]
async fn a_sub_agent_returns_its_text_and_its_stop_reason() {
    use meow_agent::SubAgent;

    let provider = Scripted::new(vec![says("the sub-agent's conclusion")]);
    let engine = Arc::new(Engine::new(provider));
    let child = Arc::new(AgentSpec::new("reviewer", "m"));
    let parent_ledger = Ledger::new(Budget::default());

    let tool = SubAgent::new(child, Arc::clone(&engine), &parent_ledger, 0);
    let out = tool
        .call(&json!({"task": "review this"}), &CancellationToken::new())
        .await
        .unwrap();

    let parsed: Value = serde_json::from_str(&out).unwrap();
    assert_eq!(parsed["text"], "the sub-agent's conclusion");
    assert_eq!(
        parsed["stop"], "finished",
        "a caller must be able to tell partial from complete"
    );
}

/// [R-AGENT-052] nesting too deep is a tool error, not a panic
#[tokio::test]
async fn nesting_past_the_limit_is_reported_rather_than_fatal() {
    use meow_agent::SubAgent;

    let engine = Arc::new(Engine::new(Scripted::new(vec![says("never reached")])));
    let mut child = AgentSpec::new("deep", "m");
    child.max_depth = 2;
    let ledger = Ledger::new(Budget::default());

    let tool = SubAgent::new(Arc::new(child), engine, &ledger, 2);
    let err = tool
        .call(&json!({"task": "t"}), &CancellationToken::new())
        .await
        .unwrap_err();
    assert!(err.to_string().contains("limit"), "got {err}");
}

/// [R-AGENT-053] a sub-agent does not see its caller's messages
#[tokio::test]
async fn a_sub_agent_starts_from_its_task_and_nothing_else() {
    use meow_agent::SubAgent;

    // A provider that records the conversation it was given.
    struct Recording(std::sync::Mutex<Vec<Vec<String>>>);
    #[async_trait::async_trait]
    impl Provider for Recording {
        fn name(&self) -> &str {
            "recording"
        }
        fn capabilities(&self) -> Capabilities {
            Capabilities {
                streaming: false,
                tools: false,
                structured: Structured::Emulated,
                embeddings: false,
            }
        }
        async fn generate(&self, r: &Request, _: &CancellationToken) -> meow_llm::Result<Response> {
            if let Ok(mut seen) = self.0.lock() {
                seen.push(r.messages.iter().map(|m| m.content.clone()).collect());
            }
            Ok(says("ok"))
        }
    }

    let provider = Arc::new(Recording(std::sync::Mutex::new(Vec::new())));
    let engine = Arc::new(Engine::new(Arc::clone(&provider) as Arc<dyn Provider>));
    let ledger = Ledger::new(Budget::default());

    let tool = SubAgent::new(Arc::new(AgentSpec::new("child", "m")), engine, &ledger, 0);
    tool.call(&json!({"task": "the sub-task"}), &CancellationToken::new())
        .await
        .unwrap();

    let seen = provider.0.lock().unwrap().clone();
    let conversation = seen.first().expect("the child should have been asked");
    assert_eq!(conversation, &vec!["the sub-task".to_owned()]);
    assert!(
        !conversation.iter().any(|m| m.contains("caller")),
        "nothing of the caller's may cross"
    );
}

/// [R-AGENT-060] results come back in the order given
/// [R-AGENT-061] one failure does not abort the others
#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn a_fan_out_returns_in_order_and_survives_one_failure() {
    use meow_agent::{Invocation, run_parallel};

    // One provider that answers by task, so a test can make exactly one fail.
    struct ByTask;
    #[async_trait::async_trait]
    impl Provider for ByTask {
        fn name(&self) -> &str {
            "by-task"
        }
        fn capabilities(&self) -> Capabilities {
            Capabilities {
                streaming: false,
                tools: false,
                structured: Structured::Emulated,
                embeddings: false,
            }
        }
        async fn generate(&self, r: &Request, _: &CancellationToken) -> meow_llm::Result<Response> {
            let task = r
                .messages
                .last()
                .map(|m| m.content.clone())
                .unwrap_or_default();
            if task == "b" {
                return Err(LlmError::Http {
                    provider: "by-task".into(),
                    status: 400,
                    message: "no".into(),
                    retry_after: None,
                    quota_exhausted: false,
                });
            }
            Ok(says(&format!("answered {task}")))
        }
    }

    let engine = Arc::new(Engine::new(Arc::new(ByTask)));
    let ledger = Ledger::new(Budget::default());
    let invocations: Vec<Invocation> = ["a", "b", "c"]
        .into_iter()
        .map(|t| Invocation {
            spec: Arc::new(AgentSpec::new(t, "m")),
            task: t.to_owned(),
        })
        .collect();

    let out = run_parallel(engine, invocations, &ledger, &CancellationToken::new()).await;
    assert_eq!(out.len(), 3);
    assert_eq!(
        out[0].text, "answered a",
        "in the order given, not the order finished"
    );
    assert_eq!(
        out[1].stop,
        StopReason::Failed,
        "the failure is a result, not an abort"
    );
    assert_eq!(out[2].text, "answered c", "and the third still ran");
}

/// [R-AGENT-062] a fan-out shares the caller's budget
#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn a_fan_out_cannot_spend_more_than_the_caller_has() {
    use meow_agent::{Invocation, run_parallel};

    let engine = Arc::new(Engine::new(Scripted::new(
        (0..10).map(|_| says("ok")).collect(),
    )));
    // Three branches, each declaring plenty, against a caller with three steps.
    let ledger = Ledger::new(Budget {
        steps: Some(3),
        ..Budget::unbounded()
    });
    let invocations: Vec<Invocation> = (0..6)
        .map(|i| Invocation {
            spec: Arc::new(AgentSpec::new(format!("a{i}"), "m").with_budget(Budget {
                steps: Some(100),
                ..Budget::unbounded()
            })),
            task: format!("t{i}"),
        })
        .collect();

    let out = run_parallel(engine, invocations, &ledger, &CancellationToken::new()).await;
    let finished = out
        .iter()
        .filter(|o| o.stop == StopReason::Finished)
        .count();
    assert_eq!(
        finished, 3,
        "the caller's three steps, not six branches' hundred each"
    );
    assert_eq!(
        out.iter().filter(|o| o.stop == StopReason::Budget).count(),
        3,
        "and the rest are stopped, not silently dropped"
    );
}
