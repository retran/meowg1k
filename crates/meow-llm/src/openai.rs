// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! One provider for every API that speaks OpenAI's shape.
//!
//! OpenAI, OpenRouter, Together, `llama.cpp`'s server, LM Studio, and most
//! things that call themselves compatible all take `/v1/chat/completions` with
//! the same body. Writing one implementation with a base URL and a name
//! instead of five near-identical files is not a shortcut: five copies of a
//! streaming parser is five places for a tool-call index to be handled
//! differently.
//!
//! Where a vendor genuinely differs - Gemini's shape, Anthropic's blocks -
//! it gets its own file. This one is for the ones that do not.

use std::collections::BTreeMap;

use serde_json::{Value, json};
use tokio_util::sync::CancellationToken;

use crate::error::{LlmError, Result};
use crate::message::{Message, Request, Response, Role, ToolCall};
use crate::provider::{Capabilities, Provider, Structured, check_supported};
use crate::schema::{SCHEMA_ATTEMPTS, parse_and_validate};
use crate::stream::{Aggregator, Sink, StreamEvent};
use crate::transport::{HttpResponse, Transport};
use crate::usage_of;

/// Talks to anything that speaks OpenAI's shape.
pub struct OpenAi<T: Transport> {
    transport: T,
    api_key: String,
    base_url: String,
    name: String,
    structured: Structured,
}

impl<T: Transport> std::fmt::Debug for OpenAi<T> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        // The key never reaches a log or a panic message.
        f.debug_struct("OpenAi")
            .field("name", &self.name)
            .field("base_url", &self.base_url)
            .finish_non_exhaustive()
    }
}

impl<T: Transport> OpenAi<T> {
    /// Build a provider for the official endpoint.
    pub fn new(transport: T, api_key: impl Into<String>) -> Self {
        Self {
            transport,
            api_key: api_key.into(),
            base_url: "https://api.openai.com".to_owned(),
            name: "openai".to_owned(),
            structured: Structured::Native,
        }
    }

    /// Borrow the transport.
    pub fn transport(&self) -> &T {
        &self.transport
    }

    /// Point it somewhere else: another vendor, a proxy, or a local server.
    #[must_use]
    pub fn with_base_url(mut self, url: impl Into<String>) -> Self {
        self.base_url = url.into();
        self
    }

    /// Call it something else, so an error names the vendor a user declared.
    #[must_use]
    pub fn with_name(mut self, name: impl Into<String>) -> Self {
        self.name = name.into();
        self
    }

    /// Say this one cannot satisfy a schema itself.
    ///
    /// `response_format` is the part compatibility most often stops at: a
    /// local server takes the body and ignores the field. Declaring emulation
    /// makes `[R-LLM-050]` ask in the prompt and check the answer here, which
    /// works either way.
    #[must_use]
    pub fn with_emulated_schema(mut self) -> Self {
        self.structured = Structured::Emulated;
        self
    }

    fn headers(&self) -> Vec<(String, String)> {
        vec![
            (
                "authorization".to_owned(),
                format!("Bearer {}", self.api_key),
            ),
            ("content-type".to_owned(), "application/json".to_owned()),
        ]
    }

    /// Turn a request into the body this API wants.
    fn body(&self, request: &Request, stream: bool) -> Value {
        let messages: Vec<Value> = request.messages.iter().map(message_of).collect();

        let mut body = json!({
            "model": request.model,
            "messages": messages,
            "max_completion_tokens": request.max_output_tokens,
        });

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
                            "type": "function",
                            "function": {
                                "name": t.name,
                                "description": t.description,
                                "parameters": t.parameters,
                            },
                        })
                    })
                    .collect(),
            );
        }
        if let (Some(schema), Structured::Native) = (&request.output_schema, self.structured) {
            body["response_format"] = json!({
                "type": "json_schema",
                "json_schema": {
                    "name": "answer",
                    "strict": false,
                    "schema": schema,
                },
            });
        }
        if stream {
            body["stream"] = json!(true);
            // Without this the stream carries no usage at all, and
            // [R-LLM-041] would have to report nothing rather than a cost.
            body["stream_options"] = json!({ "include_usage": true });
        }
        body
    }

    /// Turn a status and a body into a classified error.
    ///
    /// The quota signal is the API's own `error.code`, never text matched out
    /// of a message. `[R-LLM-037]`.
    fn error_from(&self, response: &HttpResponse) -> LlmError {
        let parsed: Option<Value> = serde_json::from_str(&response.body).ok();
        let code = parsed
            .as_ref()
            .and_then(|v| v.pointer("/error/code"))
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
            provider: self.name.clone(),
            status: response.status,
            message,
            retry_after: response.retry_after,
            quota_exhausted: code == "insufficient_quota" || code == "billing_hard_limit_reached",
        }
    }

    /// Read a non-streaming answer.
    fn parse(&self, body: &str) -> Result<Response> {
        let v: Value = serde_json::from_str(body).map_err(|e| LlmError::Malformed {
            provider: self.name.clone(),
            message: e.to_string(),
        })?;

        let choice = v
            .get("choices")
            .and_then(Value::as_array)
            .and_then(|c| c.first())
            .ok_or_else(|| LlmError::Malformed {
                provider: self.name.clone(),
                message: "the answer carries no choices".to_owned(),
            })?;

        let message = &choice["message"];
        let text = message
            .get("content")
            .and_then(Value::as_str)
            .unwrap_or_default()
            .to_owned();

        // Some compatible servers put the model's reasoning here; the official
        // API does not send it at all. Absent is absent, per [R-LLM-024].
        let thinking = message
            .get("reasoning_content")
            .or_else(|| message.get("reasoning"))
            .and_then(Value::as_str)
            .filter(|t| !t.is_empty())
            .map(str::to_owned);

        let mut calls: Vec<ToolCall> = Vec::new();
        for call in message
            .get("tool_calls")
            .and_then(Value::as_array)
            .into_iter()
            .flatten()
        {
            let id = call["id"].as_str().unwrap_or_default().to_owned();
            // [R-LLM-014]: a repeated identifier is one call, not two.
            if calls.iter().any(|c| c.id == id) {
                continue;
            }
            calls.push(ToolCall {
                id,
                name: call
                    .pointer("/function/name")
                    .and_then(Value::as_str)
                    .unwrap_or_default()
                    .to_owned(),
                arguments: call
                    .pointer("/function/arguments")
                    .and_then(Value::as_str)
                    .unwrap_or("{}")
                    .to_owned(),
            });
        }

        Ok(Response {
            text,
            thinking,
            tool_calls: calls,
            usage: usage_of(v.get("usage")),
            value: None,
        })
    }
}

/// Turn one message into the shape this API wants.
fn message_of(m: &Message) -> Value {
    match m.role {
        Role::System => json!({ "role": "system", "content": m.content }),
        Role::User => json!({ "role": "user", "content": m.content }),
        // A tool result is its own role here, tied to the call by identifier.
        Role::Tool => json!({
            "role": "tool",
            "tool_call_id": m.tool_call_id.clone().unwrap_or_default(),
            "content": m.content,
        }),
        Role::Assistant => {
            let mut out = json!({ "role": "assistant", "content": m.content });
            if !m.tool_calls.is_empty() {
                out["tool_calls"] = Value::Array(
                    m.tool_calls
                        .iter()
                        .map(|c| {
                            json!({
                                "id": c.id,
                                "type": "function",
                                "function": { "name": c.name, "arguments": c.arguments },
                            })
                        })
                        .collect(),
                );
            }
            out
        }
    }
}

#[async_trait::async_trait]
impl<T: Transport> Provider for OpenAi<T> {
    fn name(&self) -> &str {
        &self.name
    }

    fn capabilities(&self) -> Capabilities {
        Capabilities {
            streaming: true,
            tools: true,
            structured: self.structured,
            embeddings: true,
        }
    }

    async fn generate(&self, request: &Request, cancel: &CancellationToken) -> Result<Response> {
        check_supported(self, request, false)?;
        if cancel.is_cancelled() {
            return Err(LlmError::Cancelled);
        }

        let mut request = request.clone();
        let mut last_error = String::new();
        let attempts = if request.output_schema.is_some() && self.structured == Structured::Emulated
        {
            SCHEMA_ATTEMPTS
        } else {
            1
        };

        // [R-LLM-051]: ask again with the validation error in the follow-up,
        // so the model is told what was wrong rather than guessing. A native
        // schema needs one attempt, because the API enforced it.
        for attempt in 1..=attempts {
            if attempt > 1 {
                request.messages.push(Message::new(
                    Role::User,
                    format!(
                        "That response did not satisfy the schema: {last_error}. Answer again with JSON only."
                    ),
                ));
            }

            let url = format!("{}/v1/chat/completions", self.base_url);
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
            attempts,
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

        let url = format!("{}/v1/chat/completions", self.base_url);
        let body = self.body(request, true).to_string();
        let headers = self.headers();
        let mut lines = self.transport.post_lines(&url, &headers, body).await?;

        let mut agg = Aggregator::new();
        // A streamed tool call arrives by position, and only the first chunk
        // carries the identifier. The rest are deltas against a slot, so the
        // slot has to be remembered.
        let mut slots: BTreeMap<u64, String> = BTreeMap::new();
        let mut finished = false;

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
            if data.trim() == "[DONE]" {
                finished = true;
                break;
            }
            let Ok(v) = serde_json::from_str::<Value>(data) else {
                continue;
            };

            for event in events_from(&v, &mut slots) {
                // [R-LLM-023]: the consumer's error aborts the request and
                // reaches the caller unchanged.
                sink.event(event.clone())?;
                agg.push(event);
            }
        }

        if finished {
            for id in slots.values() {
                let event = StreamEvent::ToolCallEnd { id: id.clone() };
                sink.event(event.clone())?;
                agg.push(event);
            }
            let event = StreamEvent::Done;
            sink.event(event.clone())?;
            agg.push(event);
        }

        let mut response = agg.finish(&self.name)?;
        if let Some(schema) = &request.output_schema {
            response.value = Some(parse_and_validate(&response.text, schema, 1)?);
        }
        Ok(response)
    }

    async fn embed(&self, texts: &[String], cancel: &CancellationToken) -> Result<Vec<Vec<f32>>> {
        if cancel.is_cancelled() {
            return Err(LlmError::Cancelled);
        }

        let url = format!("{}/v1/embeddings", self.base_url);
        let body = json!({ "model": "text-embedding-3-small", "input": texts }).to_string();
        let headers = self.headers();

        let http = tokio::select! {
            () = cancel.cancelled() => return Err(LlmError::Cancelled),
            r = self.transport.post(&url, &headers, body) => r?,
        };
        if http.status >= 400 {
            return Err(self.error_from(&http));
        }

        vectors_from(&http.body, &self.name)
    }
}

/// Read an embeddings answer.
///
/// The vectors come back with an index each, and the order is not promised, so
/// they are put back in the order asked for rather than trusted.
pub(crate) fn vectors_from(body: &str, provider: &str) -> Result<Vec<Vec<f32>>> {
    let v: Value = serde_json::from_str(body).map_err(|e| LlmError::Malformed {
        provider: provider.to_owned(),
        message: e.to_string(),
    })?;

    let mut by_index: BTreeMap<u64, Vec<f32>> = BTreeMap::new();
    for item in v
        .get("data")
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
    {
        let index = item.get("index").and_then(Value::as_u64).unwrap_or(0);
        let vector = item
            .get("embedding")
            .and_then(Value::as_array)
            .map(|values| {
                values
                    .iter()
                    .filter_map(Value::as_f64)
                    .map(|f| f as f32)
                    .collect()
            })
            .unwrap_or_default();
        by_index.insert(index, vector);
    }

    if by_index.is_empty() {
        return Err(LlmError::Malformed {
            provider: provider.to_owned(),
            message: "the answer carries no embeddings".to_owned(),
        });
    }

    Ok(by_index.into_values().collect())
}

/// Turn one server-sent event into zero or more stream events.
fn events_from(v: &Value, slots: &mut BTreeMap<u64, String>) -> Vec<StreamEvent> {
    let Some(delta) = v.pointer("/choices/0/delta") else {
        // A chunk with no choices is the usage chunk that `include_usage` asks
        // for, which arrives after the last content.
        return usage_of(v.get("usage"))
            .map(StreamEvent::Usage)
            .into_iter()
            .collect();
    };

    let mut out = Vec::new();

    if let Some(text) = delta.get("content").and_then(Value::as_str)
        && !text.is_empty()
    {
        out.push(StreamEvent::Text(text.to_owned()));
    }
    if let Some(thinking) = delta
        .get("reasoning_content")
        .or_else(|| delta.get("reasoning"))
        .and_then(Value::as_str)
        && !thinking.is_empty()
    {
        out.push(StreamEvent::Thinking(thinking.to_owned()));
    }

    for call in delta
        .get("tool_calls")
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
    {
        let slot = call.get("index").and_then(Value::as_u64).unwrap_or(0);

        if let Some(id) = call.get("id").and_then(Value::as_str)
            && !id.is_empty()
        {
            slots.insert(slot, id.to_owned());
            out.push(StreamEvent::ToolCallStart {
                id: id.to_owned(),
                name: call
                    .pointer("/function/name")
                    .and_then(Value::as_str)
                    .unwrap_or_default()
                    .to_owned(),
            });
        }

        if let Some(arguments) = call.pointer("/function/arguments").and_then(Value::as_str)
            && !arguments.is_empty()
            && let Some(id) = slots.get(&slot)
        {
            out.push(StreamEvent::ToolCallDelta {
                id: id.clone(),
                arguments: arguments.to_owned(),
            });
        }
    }

    out.extend(usage_of(v.get("usage")).map(StreamEvent::Usage));
    out
}
