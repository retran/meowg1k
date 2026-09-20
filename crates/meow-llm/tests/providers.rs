// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The providers, against recorded exchanges.
//!
//! `[R-LLM-021]` is stated over a recording rather than a live call, because
//! two live calls to a model cannot be compared: the model is free to answer
//! differently. These check the translation in both directions, which is the
//! part that can be wrong in a way nobody notices until a tool runs twice.
#![allow(clippy::unwrap_used)]

use meow_core::Usage;
use meow_llm::{
    Collect, Gemini, HttpResponse, LlmError, Message, OpenAi, Provider, Recorded, Request, Role,
    StreamEvent, ToolDefinition, Voyage,
};
use serde_json::{Value, json};
use tokio_util::sync::CancellationToken;

fn ok(body: Value) -> HttpResponse {
    HttpResponse {
        status: 200,
        body: body.to_string(),
        retry_after: None,
    }
}

fn failed(status: u16, body: Value) -> HttpResponse {
    HttpResponse {
        status,
        body: body.to_string(),
        retry_after: None,
    }
}

fn asking() -> Request {
    Request {
        model: "m".to_owned(),
        messages: vec![
            Message::new(Role::System, "be brief"),
            Message::new(Role::User, "what changed?"),
        ],
        tools: vec![ToolDefinition {
            name: "fs.read".to_owned(),
            description: "read a file".to_owned(),
            parameters: json!({"type": "object", "properties": {"path": {"type": "string"}}}),
        }],
        max_output_tokens: 1024,
        temperature: Some(0.2),
        output_schema: None,
    }
}

fn sent(transport: &Recorded) -> Value {
    serde_json::from_str(&transport.bodies()[0]).unwrap()
}

/// [R-LLM-003] no vendor type crosses the trait: one request becomes each
/// vendor's shape
#[tokio::test]
async fn one_request_becomes_each_vendors_shape() {
    let openai = OpenAi::new(
        Recorded::with_responses(vec![ok(json!({
            "choices": [{"message": {"role": "assistant", "content": "a lot"}}],
            "usage": {"prompt_tokens": 10, "completion_tokens": 3},
        }))]),
        "k",
    );
    openai
        .generate(&asking(), &CancellationToken::new())
        .await
        .unwrap();

    let body = sent(openai.transport());
    assert_eq!(body["messages"][0]["role"], "system");
    assert_eq!(body["messages"][0]["content"], "be brief");
    assert_eq!(body["tools"][0]["type"], "function");
    assert_eq!(body["tools"][0]["function"]["name"], "fs.read");
    assert_eq!(body["max_completion_tokens"], 1024);

    let gemini = Gemini::new(
        Recorded::with_responses(vec![ok(json!({
            "candidates": [{"content": {"parts": [{"text": "a lot"}]}}],
        }))]),
        "k",
    );
    gemini
        .generate(&asking(), &CancellationToken::new())
        .await
        .unwrap();

    let body = sent(gemini.transport());
    // The system prompt has its own field, and a tool is a function
    // declaration rather than a tool.
    assert_eq!(body["systemInstruction"]["parts"][0]["text"], "be brief");
    assert_eq!(body["contents"][0]["role"], "user");
    assert_eq!(
        body["tools"][0]["functionDeclarations"][0]["name"],
        "fs.read"
    );
    assert_eq!(body["generationConfig"]["maxOutputTokens"], 1024);
}

/// [R-LLM-014] a repeated tool-call identifier is one call, not two
#[tokio::test]
async fn a_repeated_tool_call_identifier_is_one_call() {
    let provider = OpenAi::new(
        Recorded::with_responses(vec![ok(json!({
            "choices": [{"message": {
                "role": "assistant",
                "content": "",
                "tool_calls": [
                    {"id": "c1", "type": "function",
                     "function": {"name": "fs.read", "arguments": "{\"path\":\"a\"}"}},
                    {"id": "c1", "type": "function",
                     "function": {"name": "fs.read", "arguments": "{\"path\":\"a\"}"}},
                ],
            }}],
        }))]),
        "k",
    );

    let answer = provider
        .generate(&asking(), &CancellationToken::new())
        .await
        .unwrap();

    assert_eq!(answer.tool_calls.len(), 1, "{:?}", answer.tool_calls);
    assert_eq!(answer.tool_calls[0].id, "c1");
}

/// A tool result goes back in the shape each vendor matches on.
#[tokio::test]
async fn a_tool_result_goes_back_the_way_each_vendor_wants_it() {
    let mut request = asking();
    request
        .messages
        .push(Message::tool_result("c1", "fn main() {}"));

    let openai = OpenAi::new(
        Recorded::with_responses(vec![ok(json!({
            "choices": [{"message": {"role": "assistant", "content": "seen"}}],
        }))]),
        "k",
    );
    openai
        .generate(&request, &CancellationToken::new())
        .await
        .unwrap();

    let body = sent(openai.transport());
    let last = body["messages"].as_array().unwrap().last().unwrap();
    assert_eq!(last["role"], "tool");
    assert_eq!(last["tool_call_id"], "c1");

    let gemini = Gemini::new(
        Recorded::with_responses(vec![ok(json!({
            "candidates": [{"content": {"parts": [{"text": "seen"}]}}],
        }))]),
        "k",
    );
    gemini
        .generate(&request, &CancellationToken::new())
        .await
        .unwrap();

    // Gemini matches on the name it sent, because it sent no identifier.
    let body = sent(gemini.transport());
    let last = body["contents"].as_array().unwrap().last().unwrap();
    assert_eq!(last["parts"][0]["functionResponse"]["name"], "c1");
}

/// Gemini gives a call no identifier, so one is made that stays unique within
/// a turn.
#[tokio::test]
async fn gemini_calls_get_identifiers_that_do_not_collide() {
    let provider = Gemini::new(
        Recorded::with_responses(vec![ok(json!({
            "candidates": [{"content": {"parts": [
                {"functionCall": {"name": "fs.read", "args": {"path": "a"}}},
                {"functionCall": {"name": "fs.read", "args": {"path": "b"}}},
            ]}}],
        }))]),
        "k",
    );

    let answer = provider
        .generate(&asking(), &CancellationToken::new())
        .await
        .unwrap();

    assert_eq!(answer.tool_calls.len(), 2);
    assert_ne!(
        answer.tool_calls[0].id, answer.tool_calls[1].id,
        "two calls to the same tool share an identifier, so a result cannot be tied back"
    );
    assert_eq!(answer.tool_calls[0].name, "fs.read");
    assert_eq!(answer.tool_calls[1].name, "fs.read");
}

/// [R-LLM-037] a spent quota is the API's own code, never matched out of a
/// message
#[tokio::test]
async fn a_spent_quota_comes_from_the_api_rather_than_its_wording() {
    let provider = OpenAi::new(
        Recorded::with_responses(vec![failed(
            429,
            json!({"error": {"code": "insufficient_quota", "message": "you have run out"}}),
        )]),
        "k",
    );

    match provider
        .generate(&asking(), &CancellationToken::new())
        .await
    {
        Err(LlmError::Http {
            quota_exhausted, ..
        }) => assert!(quota_exhausted),
        other => panic!("expected an HTTP error, got {other:?}"),
    }

    // The same status without the code is a rate limit, not a spent quota.
    let limited = OpenAi::new(
        Recorded::with_responses(vec![failed(
            429,
            json!({"error": {"code": "rate_limit_exceeded", "message": "slow down"}}),
        )]),
        "k",
    );
    match limited.generate(&asking(), &CancellationToken::new()).await {
        Err(LlmError::Http {
            quota_exhausted, ..
        }) => assert!(!quota_exhausted),
        other => panic!("expected an HTTP error, got {other:?}"),
    }
}

/// [R-LLM-022] a streamed answer arrives as events and adds up to the same
/// response
#[tokio::test]
async fn a_streamed_answer_adds_up() {
    let provider = OpenAi::new(
        Recorded::with_stream(vec![
            r#"data: {"choices":[{"delta":{"content":"the "}}]}"#.to_owned(),
            r#"data: {"choices":[{"delta":{"content":"retry"}}]}"#.to_owned(),
            r#"data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"fs.read","arguments":""}}]}}]}"#.to_owned(),
            r#"data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\""}}]}}]}"#.to_owned(),
            r#"data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"a\"}"}}]}}]}"#.to_owned(),
            r#"data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":4}}"#.to_owned(),
            "data: [DONE]".to_owned(),
        ]),
        "k",
    );

    let mut sink = Collect::default();
    let answer = provider
        .generate_stream(&asking(), &mut sink, &CancellationToken::new())
        .await
        .unwrap();

    assert_eq!(answer.text, "the retry");
    assert_eq!(answer.tool_calls.len(), 1);
    assert_eq!(answer.tool_calls[0].id, "c1");
    assert_eq!(answer.tool_calls[0].name, "fs.read");
    assert_eq!(answer.tool_calls[0].arguments, r#"{"path":"a"}"#);
    assert_eq!(
        answer.usage,
        Some(Usage {
            prompt: 10,
            completion: 4,
            cached: None,
            cost_micros: None,
        })
    );

    // And the consumer saw the pieces, not just the total.
    assert!(
        sink.0
            .iter()
            .any(|e| matches!(e, StreamEvent::Text(t) if t == "the "))
    );
    assert!(
        sink.0
            .iter()
            .any(|e| matches!(e, StreamEvent::ToolCallEnd { id } if id == "c1"))
    );
    assert!(sink.0.iter().any(|e| matches!(e, StreamEvent::Done)));
}

/// [R-LLM-050] a native schema goes in the request rather than the prompt
#[tokio::test]
async fn a_native_schema_goes_in_the_request() {
    let mut request = asking();
    request.output_schema = Some(json!({
        "type": "object",
        "properties": {"verdict": {"type": "string"}},
        "additionalProperties": false,
    }));

    let provider = OpenAi::new(
        Recorded::with_responses(vec![ok(json!({
            "choices": [{"message": {"role": "assistant", "content": "{\"verdict\":\"ship\"}"}}],
        }))]),
        "k",
    );

    let answer = provider
        .generate(&request, &CancellationToken::new())
        .await
        .unwrap();

    let body = sent(provider.transport());
    assert_eq!(body["response_format"]["type"], "json_schema");
    assert_eq!(answer.value.unwrap()["verdict"], "ship");
}

/// Gemini refuses schema keywords the others accept, so they are dropped
/// rather than sent.
#[tokio::test]
async fn gemini_gets_a_schema_it_can_read() {
    let mut request = asking();
    request.output_schema = Some(json!({
        "type": "object",
        "properties": {"verdict": {"type": "string", "default": "hold"}},
        "additionalProperties": false,
        "x-meow": {"required": true},
    }));

    let provider = Gemini::new(
        Recorded::with_responses(vec![ok(json!({
            "candidates": [{"content": {"parts": [{"text": "{\"verdict\":\"ship\"}"}]}}],
        }))]),
        "k",
    );
    provider
        .generate(&request, &CancellationToken::new())
        .await
        .unwrap();

    let schema = &sent(provider.transport())["generationConfig"]["responseSchema"];
    assert!(schema.get("additionalProperties").is_none(), "{schema}");
    assert!(schema.get("x-meow").is_none(), "{schema}");
    assert!(
        schema["properties"]["verdict"].get("default").is_none(),
        "{schema}"
    );
    // And what it can read is still there.
    assert_eq!(schema["properties"]["verdict"]["type"], "string");
}

/// [R-LLM-040] [R-LLM-041] usage is read where each vendor puts it, and a
/// missing block is absent rather than zero
#[tokio::test]
async fn usage_is_read_or_absent() {
    let with = OpenAi::new(
        Recorded::with_responses(vec![ok(json!({
            "choices": [{"message": {"role": "assistant", "content": "x"}}],
            "usage": {
                "prompt_tokens": 100,
                "completion_tokens": 20,
                "prompt_tokens_details": {"cached_tokens": 40},
            },
        }))]),
        "k",
    );
    let answer = with
        .generate(&asking(), &CancellationToken::new())
        .await
        .unwrap();
    assert_eq!(
        answer.usage,
        Some(Usage {
            prompt: 100,
            completion: 20,
            cached: Some(40),
            cost_micros: None,
        })
    );

    let without = OpenAi::new(
        Recorded::with_responses(vec![ok(json!({
            "choices": [{"message": {"role": "assistant", "content": "x"}}],
        }))]),
        "k",
    );
    let answer = without
        .generate(&asking(), &CancellationToken::new())
        .await
        .unwrap();
    assert_eq!(
        answer.usage, None,
        "no usage block should be absent, not zero"
    );
}

/// Embeddings come back in the order they were asked for.
#[tokio::test]
async fn embeddings_keep_the_order_they_were_asked_in() {
    let provider = OpenAi::new(
        Recorded::with_responses(vec![ok(json!({
            "data": [
                {"index": 1, "embedding": [0.0, 1.0]},
                {"index": 0, "embedding": [1.0, 0.0]},
            ],
        }))]),
        "k",
    );

    let vectors = provider
        .embed(
            &["first".to_owned(), "second".to_owned()],
            &CancellationToken::new(),
        )
        .await
        .unwrap();

    assert_eq!(vectors, vec![vec![1.0, 0.0], vec![0.0, 1.0]]);
}

/// [R-LLM-002] a provider that only embeds refuses generation before sending
/// anything
#[tokio::test]
async fn an_embedding_only_provider_refuses_generation() {
    let provider = Voyage::new(Recorded::default(), "k", "voyage-3");

    match provider
        .generate(&asking(), &CancellationToken::new())
        .await
    {
        Err(LlmError::Unsupported { capability, .. }) => assert_eq!(capability, "generation"),
        other => panic!("expected a refusal, got {other:?}"),
    }
    assert!(
        provider.transport().bodies().is_empty(),
        "it sent a request it could not satisfy"
    );
}

/// Voyage says what the vectors are for, because a document and a query embed
/// differently.
#[tokio::test]
async fn voyage_says_what_the_vectors_are_for() {
    let provider = Voyage::new(
        Recorded::with_responses(vec![ok(json!({
            "data": [{"index": 0, "embedding": [1.0, 0.0]}],
        }))]),
        "k",
        "voyage-3",
    );

    provider
        .embed(&["a chunk".to_owned()], &CancellationToken::new())
        .await
        .unwrap();

    let body = sent(provider.transport());
    assert_eq!(body["model"], "voyage-3");
    assert_eq!(body["input_type"], "document");
}

/// A compatible server that ignores `response_format` still satisfies a
/// schema, by being asked and checked here.
#[tokio::test]
async fn an_emulated_schema_is_asked_again_when_it_is_wrong() {
    let mut request = asking();
    request.output_schema = Some(json!({
        "type": "object",
        "properties": {"verdict": {"type": "string"}},
        "required": ["verdict"],
    }));

    let provider = OpenAi::new(
        Recorded::with_responses(vec![
            ok(json!({"choices": [{"message": {"role": "assistant", "content": "not json"}}]})),
            ok(json!({
                "choices": [{"message": {"role": "assistant", "content": "{\"verdict\":\"ship\"}"}}],
            })),
        ]),
        "k",
    )
    .with_emulated_schema()
    .with_name("local");

    let answer = provider
        .generate(&request, &CancellationToken::new())
        .await
        .unwrap();
    assert_eq!(answer.value.unwrap()["verdict"], "ship");

    // The second request carried the reason the first was rejected.
    let bodies = provider.transport().bodies();
    assert_eq!(bodies.len(), 2);
    let second: Value = serde_json::from_str(&bodies[1]).unwrap();
    let last = second["messages"].as_array().unwrap().last().unwrap();
    assert!(
        last["content"]
            .as_str()
            .unwrap()
            .contains("did not satisfy"),
        "{last}"
    );
    // And no `response_format`, because this one was told it cannot read it.
    assert!(second.get("response_format").is_none(), "{second}");
}
