// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Every requirement in `docs/spec/llm.md`, one test or more each.
//!
//! Nothing here reaches the network. Every provider test runs against a
//! recorded exchange, which is also why `[R-LLM-021]` is stated over a
//! recording: two live calls to a model cannot be compared, because the model
//! is free to answer differently.

// `allow-unwrap-in-tests` in clippy.toml covers `#[test]` functions, not the
// helpers beside them.
#![allow(clippy::unwrap_used)]

use std::sync::Arc;
use std::sync::atomic::{AtomicU32, Ordering};
use std::time::Duration;

use meow_core::Usage;
use meow_llm::{
    Anthropic, Capabilities, Class, Collect, HttpResponse, LlmError, Message, Provider, Recorded,
    Request, Response, Retry, Role, StreamEvent, Structured, ToolDefinition, check_supported,
    parse_and_validate, with_retry,
};
use tokio_util::sync::CancellationToken;

fn ok(body: &str) -> HttpResponse {
    HttpResponse {
        status: 200,
        body: body.to_owned(),
        retry_after: None,
    }
}

fn err(status: u16, body: &str) -> HttpResponse {
    HttpResponse {
        status,
        body: body.to_owned(),
        retry_after: None,
    }
}

fn text_reply(text: &str) -> String {
    format!(
        r#"{{"content":[{{"type":"text","text":"{text}"}}],
            "usage":{{"input_tokens":10,"output_tokens":3}}}}"#
    )
}

fn ask(model: &str) -> Request {
    Request::new(model, vec![Message::new(Role::User, "hello")], 100)
}

/// A provider that declares nothing, to exercise the refusals.
struct Bare;

#[async_trait::async_trait]
impl Provider for Bare {
    fn name(&self) -> &str {
        "bare"
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
        panic!("the request should have been refused before it was sent")
    }
}

/// [R-LLM-001] a provider declares what it can do
#[test]
fn a_provider_declares_its_capabilities() {
    let a = Anthropic::new(Recorded::default(), "k");
    let caps = a.capabilities();
    assert!(caps.streaming && caps.tools);
    assert_eq!(caps.structured, Structured::Emulated);
    assert!(!caps.embeddings, "anthropic has no embedding endpoint");
}

/// [R-LLM-002] an undeclared capability is refused before a request is sent
#[tokio::test]
async fn an_undeclared_capability_is_refused_without_sending_anything() {
    let bare = Bare;
    let mut with_tools = ask("m");
    with_tools.tools.push(ToolDefinition {
        name: "fs.read".into(),
        description: "read".into(),
        parameters: serde_json::json!({}),
    });

    match check_supported(&bare, &with_tools, false) {
        Err(LlmError::Unsupported {
            provider,
            capability,
        }) => {
            assert_eq!(provider, "bare");
            assert_eq!(capability, "tool calling");
        }
        other => panic!("expected Unsupported, got {other:?}"),
    }
    assert!(matches!(
        check_supported(&bare, &ask("m"), true),
        Err(LlmError::Unsupported {
            capability: "streaming",
            ..
        })
    ));
}

/// [R-LLM-003] no vendor type appears in a trait signature
#[test]
fn the_trait_names_no_vendor_type() {
    // A compile-time claim: this file uses the provider through its trait and
    // names only this crate's types. If a vendor type appeared in a signature,
    // this test file could not be written without importing it.
    fn _uses_only_our_types(p: &dyn Provider) -> (&str, Capabilities) {
        (p.name(), p.capabilities())
    }
    let a = Anthropic::new(Recorded::default(), "k");
    assert_eq!(_uses_only_our_types(&a).0, "anthropic");
}

/// [R-LLM-010] a message carries exactly one role
/// [R-LLM-011] an assistant message carries text and tool calls together
/// [R-LLM-012] a tool result names the call it answers
#[test]
fn a_message_carries_one_role_and_a_result_names_its_call() {
    let m = Message::new(Role::User, "hi");
    assert_eq!(m.role, Role::User);
    assert!(m.tool_calls.is_empty() && m.tool_call_id.is_none());

    let mut assistant = Message::new(Role::Assistant, "let me look");
    assistant.tool_calls.push(meow_llm::ToolCall {
        id: "c1".into(),
        name: "fs.read".into(),
        arguments: "{}".into(),
    });
    assert!(!assistant.content.is_empty() && !assistant.tool_calls.is_empty());

    assert_eq!(
        Message::tool_result("c1", "contents")
            .tool_call_id
            .as_deref(),
        Some("c1")
    );
}

/// [R-LLM-013] tool call identifiers are unique within a response
/// [R-LLM-014] a repeated identifier is one call, whatever the session setting
#[tokio::test]
async fn a_repeated_tool_call_identifier_produces_one_call() {
    let body = r#"{"content":[
        {"type":"tool_use","id":"c1","name":"fs.read","input":{"path":"a"}},
        {"type":"tool_use","id":"c1","name":"fs.read","input":{"path":"a"}},
        {"type":"tool_use","id":"c2","name":"fs.write","input":{"path":"b"}}
    ]}"#;
    let a = Anthropic::new(Recorded::with_responses(vec![ok(body)]), "k");
    let r = a
        .generate(&ask("m"), &CancellationToken::new())
        .await
        .unwrap();

    assert_eq!(
        r.tool_calls.len(),
        2,
        "the duplicate should have been dropped"
    );
    let ids: Vec<&str> = r.tool_calls.iter().map(|c| c.id.as_str()).collect();
    assert_eq!(ids, vec!["c1", "c2"]);
}

/// [R-LLM-015] a cache hint becomes a breakpoint and leaves content alone
#[tokio::test]
async fn a_cache_hint_becomes_a_breakpoint_without_changing_the_message() {
    let transport = Recorded::with_responses(vec![ok(&text_reply("hi"))]);
    let a = Anthropic::new(transport, "k");
    let request = Request::new(
        "m",
        vec![
            Message::new(Role::System, "a long preamble").with_cache_hint(),
            Message::new(Role::User, "hello"),
        ],
        100,
    );
    a.generate(&request, &CancellationToken::new())
        .await
        .unwrap();

    // The transport is moved into the provider, so read what was sent back out
    // of the parsed body.
    let sent: serde_json::Value =
        serde_json::from_str(&a_sent_body(&a)).expect("the body should be JSON");
    assert_eq!(
        sent["system"][0]["text"], "a long preamble",
        "content must be untouched"
    );
    assert_eq!(sent["system"][0]["cache_control"]["type"], "ephemeral");
}

fn a_sent_body(a: &Anthropic<Recorded>) -> String {
    a.transport().bodies().first().cloned().unwrap_or_default()
}

/// [R-LLM-020] the stream event kinds are exactly eight
/// [R-LLM-021] aggregating a recorded stream gives the non-streaming value
#[tokio::test]
async fn aggregating_a_recorded_stream_matches_the_whole_answer() {
    let lines: Vec<String> = [
        r#"data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hel"}}"#,
        r#"data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"lo"}}"#,
        r#"data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"c1","name":"fs.read"}}"#,
        r#"data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\""}}"#,
        r#"data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":":\"a\"}"}}"#,
        r#"data: {"type":"content_block_stop","index":1}"#,
        r#"data: {"type":"message_delta","usage":{"input_tokens":10,"output_tokens":3}}"#,
        r#"data: {"type":"message_stop"}"#,
    ]
    .iter()
    .map(ToString::to_string)
    .collect();

    let a = Anthropic::new(Recorded::with_stream(lines), "k");
    let mut sink = Collect::default();
    let streamed = a
        .generate_stream(&ask("m"), &mut sink, &CancellationToken::new())
        .await
        .unwrap();

    assert_eq!(streamed.text, "Hello");
    assert_eq!(streamed.tool_calls.len(), 1);
    assert_eq!(streamed.tool_calls[0].arguments, r#"{"path":"a"}"#);
    assert_eq!(
        streamed.usage,
        Some(Usage {
            prompt: 10,
            completion: 3,
            cached: None,
            cost_micros: None
        })
    );
    assert!(sink.0.contains(&StreamEvent::Done));
    assert!(
        sink.0
            .iter()
            .any(|e| matches!(e, StreamEvent::ToolCallEnd { .. }))
    );
}

/// [R-LLM-022] a provider without native streaming declares it unsupported
#[tokio::test]
async fn a_provider_without_streaming_refuses_rather_than_synthesising_it() {
    let mut sink = Collect::default();
    let err = Bare
        .generate_stream(&ask("m"), &mut sink, &CancellationToken::new())
        .await
        .unwrap_err();
    assert!(matches!(
        err,
        LlmError::Unsupported {
            capability: "streaming",
            ..
        }
    ));
    assert!(
        sink.0.is_empty(),
        "no event may be invented for a stream that never ran"
    );
}

/// [R-LLM-023] a sink error aborts the request and reaches the caller
#[tokio::test]
async fn a_sink_error_stops_the_stream_and_is_not_interpreted() {
    let lines: Vec<String> = (0..5)
        .map(|i| {
            format!(
                r#"data: {{"type":"content_block_delta","index":0,"delta":{{"type":"text_delta","text":"{i}"}}}}"#
            )
        })
        .collect();
    let a = Anthropic::new(Recorded::with_stream(lines), "k");

    let seen = Arc::new(AtomicU32::new(0));
    let counter = Arc::clone(&seen);
    let mut sink = move |_e: StreamEvent| -> meow_llm::Result<()> {
        if counter.fetch_add(1, Ordering::SeqCst) >= 1 {
            return Err(LlmError::Cancelled);
        }
        Ok(())
    };

    let err = a
        .generate_stream(&ask("m"), &mut sink, &CancellationToken::new())
        .await
        .unwrap_err();
    assert!(
        matches!(err, LlmError::Cancelled),
        "the sink's error must arrive unchanged"
    );
    assert_eq!(
        seen.load(Ordering::SeqCst),
        2,
        "the stream should have stopped at once"
    );
}

/// [R-LLM-024] thinking survives onto the message and goes back next turn
#[tokio::test]
async fn thinking_is_kept_and_sent_back_on_the_next_turn() {
    let body = r#"{"content":[
        {"type":"thinking","thinking":"the user wants a file"},
        {"type":"text","text":"reading it"}
    ]}"#;
    let a = Anthropic::new(Recorded::with_responses(vec![ok(body)]), "k");
    let first = a
        .generate(&ask("m"), &CancellationToken::new())
        .await
        .unwrap();
    assert_eq!(first.thinking.as_deref(), Some("the user wants a file"));

    // A second turn carrying it back, which Anthropic requires when a tool
    // call continues.
    let b = Anthropic::new(Recorded::with_responses(vec![ok(&text_reply("done"))]), "k");
    let request = Request::new(
        "m",
        vec![
            Message::new(Role::User, "read a"),
            Message::new(Role::Assistant, "reading it")
                .with_thinking(first.thinking.clone().unwrap()),
        ],
        100,
    );
    b.generate(&request, &CancellationToken::new())
        .await
        .unwrap();
    let sent: serde_json::Value = serde_json::from_str(&a_sent_body(&b)).unwrap();
    assert_eq!(sent["messages"][1]["content"][0]["type"], "thinking");
}

/// [R-LLM-030] every error is classified as exactly one of three
/// [R-LLM-031] the transient statuses
/// [R-LLM-032] the fatal statuses
#[test]
fn every_status_lands_in_exactly_one_class() {
    let http = |status, quota| LlmError::Http {
        provider: "p".into(),
        status,
        message: String::new(),
        retry_after: None,
        quota_exhausted: quota,
    };
    for s in [408, 429, 500, 502, 503, 504] {
        assert_eq!(
            http(s, false).class(),
            Class::Transient,
            "{s} should be transient"
        );
    }
    for s in [400, 401, 403, 404, 422] {
        assert_eq!(http(s, false).class(), Class::Fatal, "{s} should be fatal");
    }
    assert_eq!(
        LlmError::Transport {
            provider: "p".into(),
            message: "reset".into()
        }
        .class(),
        Class::Transient
    );
    assert_eq!(LlmError::Cancelled.class(), Class::Fatal);
}

/// [R-LLM-033] only transient errors are retried, and a fatal one does not wait
#[tokio::test(start_paused = true)]
async fn a_fatal_error_surfaces_at_once_and_is_not_retried() {
    let tries = Arc::new(AtomicU32::new(0));
    let n = Arc::clone(&tries);
    let started = tokio::time::Instant::now();

    let result: meow_llm::Result<()> =
        with_retry("p", Retry::default(), &CancellationToken::new(), || {
            let n = Arc::clone(&n);
            async move {
                n.fetch_add(1, Ordering::SeqCst);
                Err(LlmError::Http {
                    provider: "p".into(),
                    status: 401,
                    message: "bad key".into(),
                    retry_after: None,
                    quota_exhausted: false,
                })
            }
        })
        .await;

    assert!(matches!(result, Err(LlmError::Http { status: 401, .. })));
    assert_eq!(
        tries.load(Ordering::SeqCst),
        1,
        "a bad key must not be retried"
    );
    assert_eq!(started.elapsed(), Duration::ZERO, "and must not wait first");
}

/// [R-LLM-033] a transient error is retried until it succeeds
#[tokio::test(start_paused = true)]
async fn a_transient_error_is_retried() {
    let tries = Arc::new(AtomicU32::new(0));
    let n = Arc::clone(&tries);

    let result = with_retry("p", Retry::default(), &CancellationToken::new(), || {
        let n = Arc::clone(&n);
        async move {
            if n.fetch_add(1, Ordering::SeqCst) < 2 {
                Err(LlmError::Transport {
                    provider: "p".into(),
                    message: "reset".into(),
                })
            } else {
                Ok(7)
            }
        }
    })
    .await;

    assert_eq!(result.unwrap(), 7);
    assert_eq!(tries.load(Ordering::SeqCst), 3);
}

/// [R-LLM-034] Retry-After is honoured over the exponent
#[tokio::test(start_paused = true)]
async fn retry_after_is_honoured() {
    let tries = Arc::new(AtomicU32::new(0));
    let n = Arc::clone(&tries);
    let started = tokio::time::Instant::now();

    let result = with_retry("p", Retry::default(), &CancellationToken::new(), || {
        let n = Arc::clone(&n);
        async move {
            if n.fetch_add(1, Ordering::SeqCst) == 0 {
                Err(LlmError::Http {
                    provider: "p".into(),
                    status: 429,
                    message: "slow down".into(),
                    retry_after: Some(Duration::from_secs(12)),
                    quota_exhausted: false,
                })
            } else {
                Ok(())
            }
        }
    })
    .await;

    assert!(result.is_ok());
    assert_eq!(
        started.elapsed(),
        Duration::from_secs(12),
        "the server's number must win"
    );
}

/// [R-LLM-035] a spent quota surfaces at once
/// [R-LLM-037] a spent quota is told apart from a rate limit by the provider's
///             own signal, not by matching text
#[tokio::test]
async fn a_spent_quota_is_told_apart_from_a_rate_limit() {
    let rate_limited = err(
        429,
        r#"{"error":{"type":"rate_limit_error","message":"slow down"}}"#,
    );
    let out_of_credit = err(
        429,
        r#"{"error":{"type":"credit_balance_too_low","message":"topped out"}}"#,
    );

    let a = Anthropic::new(Recorded::with_responses(vec![rate_limited]), "k");
    let e = a
        .generate(&ask("m"), &CancellationToken::new())
        .await
        .unwrap_err();
    assert_eq!(
        e.class(),
        Class::Transient,
        "a rate limit is worth retrying"
    );

    let b = Anthropic::new(Recorded::with_responses(vec![out_of_credit]), "k");
    let e = b
        .generate(&ask("m"), &CancellationToken::new())
        .await
        .unwrap_err();
    assert_eq!(e.class(), Class::QuotaExhausted, "a spent quota is not");
    assert!(
        e.to_string().contains("anthropic"),
        "the provider must be named"
    );
}

/// [R-LLM-036] cancellation is checked before sleeping and before each attempt
#[tokio::test(start_paused = true)]
async fn cancelling_during_a_backoff_does_not_wait_it_out() {
    let cancel = CancellationToken::new();
    let tries = Arc::new(AtomicU32::new(0));
    let n = Arc::clone(&tries);
    let token = cancel.clone();

    let result: meow_llm::Result<()> = with_retry("p", Retry::default(), &cancel, || {
        let n = Arc::clone(&n);
        let token = token.clone();
        async move {
            n.fetch_add(1, Ordering::SeqCst);
            token.cancel();
            Err(LlmError::Transport {
                provider: "p".into(),
                message: "reset".into(),
            })
        }
    })
    .await;

    assert!(matches!(result, Err(LlmError::Cancelled)));
    assert_eq!(
        tries.load(Ordering::SeqCst),
        1,
        "no attempt after the cancellation"
    );
}

/// [R-LLM-040] cached tokens stay absent when the provider does not report them
#[tokio::test]
async fn an_unreported_cache_count_stays_absent_rather_than_zero() {
    let without = r#"{"content":[],"usage":{"input_tokens":10,"output_tokens":3}}"#;
    let with = r#"{"content":[],"usage":{"input_tokens":10,"output_tokens":3,"cache_read_input_tokens":0}}"#;

    let a = Anthropic::new(Recorded::with_responses(vec![ok(without)]), "k");
    assert_eq!(
        a.generate(&ask("m"), &CancellationToken::new())
            .await
            .unwrap()
            .usage
            .unwrap()
            .cached,
        None
    );

    let b = Anthropic::new(Recorded::with_responses(vec![ok(with)]), "k");
    assert_eq!(
        b.generate(&ask("m"), &CancellationToken::new())
            .await
            .unwrap()
            .usage
            .unwrap()
            .cached,
        Some(0),
        "a reported zero is a different fact from silence"
    );
}

/// [R-LLM-041] no usage block at all is distinguishable from zeroes
#[tokio::test]
async fn no_usage_at_all_is_distinguishable_from_zeroes() {
    let a = Anthropic::new(Recorded::with_responses(vec![ok(r#"{"content":[]}"#)]), "k");
    assert_eq!(
        a.generate(&ask("m"), &CancellationToken::new())
            .await
            .unwrap()
            .usage,
        None
    );

    let b = Anthropic::new(
        Recorded::with_responses(vec![ok(
            r#"{"content":[],"usage":{"input_tokens":0,"output_tokens":0}}"#,
        )]),
        "k",
    );
    assert_eq!(
        b.generate(&ask("m"), &CancellationToken::new())
            .await
            .unwrap()
            .usage,
        Some(Usage::default())
    );
}

/// [R-LLM-050] a provider without native schemas validates here instead
/// [R-LLM-052] a satisfied schema returns parsed, not a string
#[tokio::test]
async fn a_schema_is_satisfied_by_validation_and_returned_parsed() {
    let reply = r#"{"content":[{"type":"text","text":"{\"file\":\"a.rs\",\"line\":3}"}]}"#;
    let a = Anthropic::new(Recorded::with_responses(vec![ok(reply)]), "k");
    let mut request = ask("m");
    request.output_schema = Some(serde_json::json!({
        "type": "object",
        "properties": { "file": {"type": "string"}, "line": {"type": "integer", "minimum": 1} },
        "required": ["file", "line"]
    }));

    let r = a
        .generate(&request, &CancellationToken::new())
        .await
        .unwrap();
    let value = r
        .value
        .expect("a satisfied schema must return a parsed value");
    assert_eq!(value["file"], "a.rs");
    assert_eq!(value["line"], 3);
}

/// [R-LLM-051] a failing answer is asked again with the error, then gives up
#[tokio::test]
async fn a_failing_schema_is_asked_again_with_the_reason_and_then_gives_up() {
    let bad = r#"{"content":[{"type":"text","text":"{\"file\":\"a.rs\"}"}]}"#;
    let a = Anthropic::new(
        Recorded::with_responses(vec![ok(bad), ok(bad), ok(bad)]),
        "k",
    );
    let mut request = ask("m");
    request.output_schema = Some(serde_json::json!({
        "type": "object",
        "properties": { "file": {"type": "string"}, "line": {"type": "integer"} },
        "required": ["file", "line"]
    }));

    let e = a
        .generate(&request, &CancellationToken::new())
        .await
        .unwrap_err();
    match e {
        LlmError::Schema { message, .. } => assert!(
            message.contains("line"),
            "the model must be told which field was missing, got {message}"
        ),
        other => panic!("expected Schema, got {other:?}"),
    }

    let bodies = a.transport().bodies();
    assert_eq!(bodies.len(), 3, "it should have asked three times");
    assert!(
        bodies[1].contains("did not satisfy the schema"),
        "the follow-up must carry the reason"
    );
}

/// [R-LLM-052] validation catches every constraint the schema builder emits
#[test]
fn validation_covers_the_constraints_the_schema_builder_can_express() {
    let schema = serde_json::json!({
        "type": "object",
        "properties": {
            "severity": {"type": "string", "enum": ["low", "high"]},
            "line": {"type": "integer", "minimum": 1},
            "summary": {"type": "string", "maxLength": 5}
        },
        "required": ["severity"]
    });
    for (text, expect) in [
        (r#"{"severity":"low"}"#, None),
        (r#"{"line":1}"#, Some("severity")),
        (r#"{"severity":"medium"}"#, Some("not one of")),
        (r#"{"severity":"low","line":0}"#, Some("minimum")),
        (
            r#"{"severity":"low","summary":"far too long"}"#,
            Some("5 characters"),
        ),
        ("not json at all", Some("not JSON")),
    ] {
        match (parse_and_validate(text, &schema, 1), expect) {
            (Ok(_), None) => {}
            (Err(LlmError::Schema { message, .. }), Some(want)) => {
                assert!(
                    message.contains(want),
                    "for {text}: wanted {want:?}, got {message}"
                );
            }
            (got, want) => panic!("for {text}: wanted {want:?}, got {got:?}"),
        }
    }
}

/// [R-LLM-060] a cancelled request stops
/// [R-LLM-061] and says it was cancelled, not that it timed out
#[tokio::test]
async fn a_cancelled_request_says_so() {
    let cancel = CancellationToken::new();
    cancel.cancel();

    let a = Anthropic::new(Recorded::with_responses(vec![ok(&text_reply("hi"))]), "k");
    let e = a.generate(&ask("m"), &cancel).await.unwrap_err();
    assert!(matches!(e, LlmError::Cancelled), "got {e:?}");
    assert!(
        a.transport().bodies().is_empty(),
        "nothing should have been sent"
    );

    let mut sink = Collect::default();
    let e = a
        .generate_stream(&ask("m"), &mut sink, &cancel)
        .await
        .unwrap_err();
    assert!(matches!(e, LlmError::Cancelled));
}
