// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The Anthropic provider.
//!
//! First because it exercises everything: tool calling, streaming, structured
//! output, thinking that has to come back on a later turn, and explicit cache
//! breakpoints. A second provider is then filling in a shape rather than
//! discovering one.

use meow_core::Usage;
use serde_json::{Value, json};
use tokio_util::sync::CancellationToken;

use crate::error::{LlmError, Result};
use crate::message::{Message, Request, Response, Role, ToolCall};
use crate::provider::{Capabilities, Provider, Structured, check_supported};
use crate::schema::{SCHEMA_ATTEMPTS, parse_and_validate};
use crate::stream::{Aggregator, Sink, StreamEvent};
use crate::transport::{HttpResponse, Transport};

const API_VERSION: &str = "2023-06-01";

/// Talks to Anthropic over whatever transport it is given.
pub struct Anthropic<T: Transport> {
    transport: T,
    api_key: String,
    base_url: String,
}

impl<T: Transport> std::fmt::Debug for Anthropic<T> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        // The key never reaches a log or a panic message.
        f.debug_struct("Anthropic")
            .field("base_url", &self.base_url)
            .finish_non_exhaustive()
    }
}

impl<T: Transport> Anthropic<T> {
    /// Build a provider.
    pub fn new(transport: T, api_key: impl Into<String>) -> Self {
        Self {
            transport,
            api_key: api_key.into(),
            base_url: "https://api.anthropic.com".to_owned(),
        }
    }

    /// Borrow the transport.
    ///
    /// Public because [`crate::Recorded`] is: a caller that supplied a
    /// recording wants to read back what was sent, and a test that cannot do
    /// that can only check what came out, never what went in.
    pub fn transport(&self) -> &T {
        &self.transport
    }

    /// Point it somewhere else, for a proxy or a recording.
    #[must_use]
    pub fn with_base_url(mut self, url: impl Into<String>) -> Self {
        self.base_url = url.into();
        self
    }

    fn headers(&self) -> Vec<(String, String)> {
        vec![
            ("x-api-key".to_owned(), self.api_key.clone()),
            ("anthropic-version".to_owned(), API_VERSION.to_owned()),
            ("content-type".to_owned(), "application/json".to_owned()),
        ]
    }

    /// Turn a request into the body this API wants.
    fn body(&self, request: &Request, stream: bool) -> Value {
        let mut system = Vec::new();
        let mut messages = Vec::new();

        for m in &request.messages {
            match m.role {
                Role::System => {
                    // A cache hint becomes an explicit breakpoint, which is
                    // what [R-LLM-015] means by translating it. The content is
                    // untouched either way.
                    let mut block = json!({ "type": "text", "text": m.content });
                    if m.cache_hint {
                        block["cache_control"] = json!({ "type": "ephemeral" });
                    }
                    system.push(block);
                }
                Role::Tool => messages.push(json!({
                    "role": "user",
                    "content": [{
                        "type": "tool_result",
                        "tool_use_id": m.tool_call_id.clone().unwrap_or_default(),
                        "content": m.content,
                    }],
                })),
                Role::User => messages.push(json!({ "role": "user", "content": m.content })),
                Role::Assistant => {
                    let mut blocks = Vec::new();
                    // Thinking goes back first, in the order the API expects.
                    // [R-LLM-024] keeps it on the message for exactly this.
                    if let Some(t) = &m.thinking {
                        blocks.push(json!({ "type": "thinking", "thinking": t }));
                    }
                    if !m.content.is_empty() {
                        blocks.push(json!({ "type": "text", "text": m.content }));
                    }
                    for c in &m.tool_calls {
                        blocks.push(json!({
                            "type": "tool_use",
                            "id": c.id,
                            "name": c.name,
                            "input": serde_json::from_str::<Value>(&c.arguments)
                                .unwrap_or_else(|_| json!({})),
                        }));
                    }
                    messages.push(json!({ "role": "assistant", "content": blocks }));
                }
            }
        }

        let mut body = json!({
            "model": request.model,
            "max_tokens": request.max_output_tokens,
            "messages": messages,
        });
        if !system.is_empty() {
            body["system"] = Value::Array(system);
        }
        if let Some(t) = request.temperature {
            body["temperature"] = json!(t);
        }
        if !request.tools.is_empty() {
            body["tools"] = Value::Array(
                request
                    .tools
                    .iter()
                    .map(|t| {
                        json!({
                            "name": t.name,
                            "description": t.description,
                            "input_schema": t.parameters,
                        })
                    })
                    .collect(),
            );
        }
        if stream {
            body["stream"] = json!(true);
        }
        body
    }

    /// Turn a status and a body into a classified error.
    ///
    /// The quota signal is the API's own `error.type`, never text matched out
    /// of a message. `[R-LLM-037]`: matching text is what v0.2.x did, and it
    /// broke whenever a provider reworded an error.
    fn error_from(&self, response: &HttpResponse) -> LlmError {
        let parsed: Option<Value> = serde_json::from_str(&response.body).ok();
        let kind = parsed
            .as_ref()
            .and_then(|v| v.pointer("/error/type"))
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_owned();
        let message = parsed
            .as_ref()
            .and_then(|v| v.pointer("/error/message"))
            .and_then(Value::as_str)
            .unwrap_or(&response.body)
            .to_owned();
        LlmError::Http {
            provider: self.name().to_owned(),
            status: response.status,
            message,
            retry_after: response.retry_after,
            quota_exhausted: kind == "billing_error" || kind == "credit_balance_too_low",
        }
    }

    /// Read a non-streaming answer.
    fn parse(&self, body: &str) -> Result<Response> {
        let v: Value = serde_json::from_str(body).map_err(|e| LlmError::Malformed {
            provider: self.name().to_owned(),
            message: e.to_string(),
        })?;

        let mut text = String::new();
        let mut thinking = String::new();
        let mut calls: Vec<ToolCall> = Vec::new();

        for block in v
            .get("content")
            .and_then(Value::as_array)
            .into_iter()
            .flatten()
        {
            match block.get("type").and_then(Value::as_str) {
                Some("text") => text.push_str(block["text"].as_str().unwrap_or_default()),
                Some("thinking") => {
                    thinking.push_str(block["thinking"].as_str().unwrap_or_default());
                }
                Some("tool_use") => {
                    let id = block["id"].as_str().unwrap_or_default().to_owned();
                    // [R-LLM-014]: a repeated identifier is one call, not two.
                    // v0.2.x deduplicated only when sessions were on, so with
                    // them off the same tool ran twice.
                    if calls.iter().any(|c| c.id == id) {
                        continue;
                    }
                    calls.push(ToolCall {
                        id,
                        name: block["name"].as_str().unwrap_or_default().to_owned(),
                        arguments: block
                            .get("input")
                            .map(ToString::to_string)
                            .unwrap_or_default(),
                    });
                }
                _ => {}
            }
        }

        Ok(Response {
            text,
            thinking: if thinking.is_empty() {
                None
            } else {
                Some(thinking)
            },
            tool_calls: calls,
            usage: usage_from(v.get("usage")),
            value: None,
        })
    }
}

/// Read the usage block.
///
/// `[R-LLM-040]`: cached tokens stay absent when the provider does not report
/// them, never zero. `[R-LLM-041]`: no usage block at all is `None`, which is
/// a different fact from a block of zeroes.
fn usage_from(v: Option<&Value>) -> Option<Usage> {
    let u = v?;
    Some(Usage {
        prompt: u.get("input_tokens").and_then(Value::as_u64).unwrap_or(0) as u32,
        completion: u.get("output_tokens").and_then(Value::as_u64).unwrap_or(0) as u32,
        cached: u
            .get("cache_read_input_tokens")
            .and_then(Value::as_u64)
            .map(|n| n as u32),
        cost_micros: None,
    })
}

#[async_trait::async_trait]
impl<T: Transport> Provider for Anthropic<T> {
    fn name(&self) -> &str {
        "anthropic"
    }

    fn capabilities(&self) -> Capabilities {
        Capabilities {
            streaming: true,
            tools: true,
            // Anthropic has no schema parameter, so the schema is asked for in
            // the prompt and checked here. [R-LLM-050] makes that a way of
            // satisfying the request, not a reason to refuse it.
            structured: Structured::Emulated,
            embeddings: false,
        }
    }

    async fn generate(&self, request: &Request, cancel: &CancellationToken) -> Result<Response> {
        check_supported(self, request, false)?;
        if cancel.is_cancelled() {
            return Err(LlmError::Cancelled);
        }

        let mut request = request.clone();
        let mut last_error = String::new();

        // [R-LLM-051]: ask again with the validation error in the follow-up,
        // so the model is told what was wrong rather than guessing.
        for attempt in 1..=schema_attempts(&request) {
            if attempt > 1 {
                request.messages.push(Message::new(
                    Role::User,
                    format!("That response did not satisfy the schema: {last_error}. Answer again with JSON only."),
                ));
            }

            let url = format!("{}/v1/messages", self.base_url);
            let body = self.body(&request, false).to_string();
            let headers = self.headers();
            let http = tokio::select! {
                () = cancel.cancelled() => return Err(LlmError::Cancelled),
                r = self.transport.post(&url, &headers, body) => r?,
            };
            if http.status >= 400 {
                return Err(self.error_from(&http));
            }

            let mut response = self.parse(&http.body)?;
            let Some(schema) = request.output_schema.clone() else {
                return Ok(response);
            };
            match parse_and_validate(&response.text, &schema, attempt) {
                Ok(value) => {
                    response.value = Some(value);
                    return Ok(response);
                }
                Err(LlmError::Schema { message, .. }) => last_error = message,
                Err(other) => return Err(other),
            }
        }

        Err(LlmError::Schema {
            attempts: SCHEMA_ATTEMPTS,
            message: last_error,
        })
    }

    async fn generate_stream(
        &self,
        request: &Request,
        sink: &mut dyn Sink,
        cancel: &CancellationToken,
    ) -> Result<Response> {
        check_supported(self, request, true)?;
        if cancel.is_cancelled() {
            return Err(LlmError::Cancelled);
        }

        let url = format!("{}/v1/messages", self.base_url);
        let body = self.body(request, true).to_string();
        let headers = self.headers();
        let mut lines = self.transport.post_lines(&url, &headers, body).await?;

        let mut agg = Aggregator::new();
        let mut open: Vec<(usize, String)> = Vec::new();

        loop {
            let line = tokio::select! {
                () = cancel.cancelled() => return Err(LlmError::Cancelled),
                l = lines.recv() => match l {
                    Some(Ok(line)) => line,
                    Some(Err(e)) => return Err(e),
                    None => break,
                },
            };
            let Some(data) = line.strip_prefix("data: ") else {
                continue;
            };
            let Ok(v) = serde_json::from_str::<Value>(data) else {
                continue;
            };

            for event in events_from(&v, &mut open) {
                // [R-LLM-023]: the consumer's error aborts the request and
                // reaches the caller unchanged.
                sink.event(event.clone())?;
                agg.push(event);
            }
        }

        let mut response = agg.finish(self.name())?;
        if let Some(schema) = &request.output_schema {
            response.value = Some(parse_and_validate(&response.text, schema, 1)?);
        }
        Ok(response)
    }
}

fn schema_attempts(request: &Request) -> u32 {
    if request.output_schema.is_some() {
        SCHEMA_ATTEMPTS
    } else {
        1
    }
}

/// Turn one server-sent event into zero or more stream events.
fn events_from(v: &Value, open: &mut Vec<(usize, String)>) -> Vec<StreamEvent> {
    let kind = v.get("type").and_then(Value::as_str).unwrap_or_default();
    let index = v.get("index").and_then(Value::as_u64).unwrap_or(0) as usize;
    match kind {
        "content_block_start" => {
            let block = &v["content_block"];
            match block.get("type").and_then(Value::as_str) {
                Some("tool_use") => {
                    let id = block["id"].as_str().unwrap_or_default().to_owned();
                    open.push((index, id.clone()));
                    vec![StreamEvent::ToolCallStart {
                        id,
                        name: block["name"].as_str().unwrap_or_default().to_owned(),
                    }]
                }
                _ => Vec::new(),
            }
        }
        "content_block_delta" => {
            let delta = &v["delta"];
            match delta.get("type").and_then(Value::as_str) {
                Some("text_delta") => {
                    vec![StreamEvent::Text(
                        delta["text"].as_str().unwrap_or_default().to_owned(),
                    )]
                }
                Some("thinking_delta") => vec![StreamEvent::Thinking(
                    delta["thinking"].as_str().unwrap_or_default().to_owned(),
                )],
                Some("input_json_delta") => open
                    .iter()
                    .find(|(i, _)| *i == index)
                    .map(|(_, id)| StreamEvent::ToolCallDelta {
                        id: id.clone(),
                        arguments: delta["partial_json"]
                            .as_str()
                            .unwrap_or_default()
                            .to_owned(),
                    })
                    .into_iter()
                    .collect(),
                _ => Vec::new(),
            }
        }
        "content_block_stop" => open
            .iter()
            .find(|(i, _)| *i == index)
            .map(|(_, id)| StreamEvent::ToolCallEnd { id: id.clone() })
            .into_iter()
            .collect(),
        "message_delta" => usage_from(v.get("usage"))
            .map(StreamEvent::Usage)
            .into_iter()
            .collect(),
        "message_stop" => vec![StreamEvent::Done],
        "error" => vec![StreamEvent::Error(
            v.pointer("/error/message")
                .and_then(Value::as_str)
                .unwrap_or("stream error")
                .to_owned(),
        )],
        _ => Vec::new(),
    }
}
