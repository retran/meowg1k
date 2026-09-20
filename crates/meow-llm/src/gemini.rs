// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Gemini, whose API is shaped differently enough to need its own file.
//!
//! Contents rather than messages, parts rather than blocks, a system prompt in
//! its own field, and a tool result that goes back as a function response with
//! no identifier. The last of those is the one that matters: the engine ties a
//! result to a call by identifier, so this keeps the names it saw and hands
//! them back, because that is all Gemini gives it to match on.

use std::collections::BTreeMap;

use serde_json::{Value, json};
use tokio_util::sync::CancellationToken;

use crate::error::{LlmError, Result};
use crate::message::{Request, Response, Role, ToolCall};
use crate::provider::{Capabilities, Provider, Structured, check_supported};
use crate::schema::parse_and_validate;
use crate::stream::{Aggregator, Sink, StreamEvent};
use crate::transport::{HttpResponse, Transport};

/// Talks to Gemini.
pub struct Gemini<T: Transport> {
    transport: T,
    api_key: String,
    base_url: String,
}

impl<T: Transport> std::fmt::Debug for Gemini<T> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Gemini")
            .field("base_url", &self.base_url)
            .finish_non_exhaustive()
    }
}

impl<T: Transport> Gemini<T> {
    /// Build a provider.
    pub fn new(transport: T, api_key: impl Into<String>) -> Self {
        Self {
            transport,
            api_key: api_key.into(),
            base_url: "https://generativelanguage.googleapis.com".to_owned(),
        }
    }

    /// Borrow the transport.
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
            ("x-goog-api-key".to_owned(), self.api_key.clone()),
            ("content-type".to_owned(), "application/json".to_owned()),
        ]
    }

    /// Turn a request into the body this API wants.
    fn body(&self, request: &Request) -> Value {
        let mut system = String::new();
        let mut contents: Vec<Value> = Vec::new();

        for m in &request.messages {
            match m.role {
                Role::System => {
                    if !system.is_empty() {
                        system.push_str("\n\n");
                    }
                    system.push_str(&m.content);
                }
                Role::User => contents.push(json!({
                    "role": "user",
                    "parts": [{ "text": m.content }],
                })),
                // A result goes back as a function response named after the
                // tool, because Gemini matches on the name and not on an
                // identifier it never sent.
                Role::Tool => contents.push(json!({
                    "role": "user",
                    "parts": [{
                        "functionResponse": {
                            "name": m.tool_call_id.clone().unwrap_or_default(),
                            "response": { "result": m.content },
                        },
                    }],
                })),
                Role::Assistant => {
                    let mut parts = Vec::new();
                    if !m.content.is_empty() {
                        parts.push(json!({ "text": m.content }));
                    }
                    for c in &m.tool_calls {
                        parts.push(json!({
                            "functionCall": {
                                "name": c.name,
                                "args": serde_json::from_str::<Value>(&c.arguments)
                                    .unwrap_or_else(|_| json!({})),
                            },
                        }));
                    }
                    contents.push(json!({ "role": "model", "parts": parts }));
                }
            }
        }

        let mut config = json!({ "maxOutputTokens": request.max_output_tokens });
        if let Some(t) = request.temperature {
            config["temperature"] = json!(t);
        }
        if let Some(schema) = &request.output_schema {
            config["responseMimeType"] = json!("application/json");
            config["responseSchema"] = clean_schema(schema);
        }

        let mut body = json!({
            "contents": contents,
            "generationConfig": config,
        });
        if !system.is_empty() {
            body["systemInstruction"] = json!({ "parts": [{ "text": system }] });
        }
        if !request.tools.is_empty() {
            body["tools"] = json!([{
                "functionDeclarations": request
                    .tools
                    .iter()
                    .map(|t| json!({
                        "name": t.name,
                        "description": t.description,
                        "parameters": clean_schema(&t.parameters),
                    }))
                    .collect::<Vec<_>>(),
            }]);
        }
        body
    }

    /// Turn a status and a body into a classified error.
    fn error_from(&self, response: &HttpResponse) -> LlmError {
        let parsed: Option<Value> = serde_json::from_str(&response.body).ok();
        let status = parsed
            .as_ref()
            .and_then(|v| v.pointer("/error/status"))
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
            // The API's own status, never text matched out of a message, per
            // [R-LLM-037]. `RESOURCE_EXHAUSTED` covers both a spent quota and
            // a rate limit; the retry policy tells them apart by the header.
            quota_exhausted: status == "RESOURCE_EXHAUSTED" && response.retry_after.is_none(),
        }
    }

    /// Read a non-streaming answer.
    fn parse(&self, body: &str) -> Result<Response> {
        let v: Value = serde_json::from_str(body).map_err(|e| LlmError::Malformed {
            provider: self.name().to_owned(),
            message: e.to_string(),
        })?;

        let (text, calls) = parts_of(&v, &mut BTreeMap::new());

        Ok(Response {
            text,
            // Gemini returns its reasoning only when asked, and asking is a
            // model setting rather than a request field. Absent is absent.
            thinking: None,
            tool_calls: calls,
            usage: usage_of(v.get("usageMetadata")),
            value: None,
        })
    }
}

/// Every text part joined, and every function call as a tool call.
fn parts_of(v: &Value, seen: &mut BTreeMap<String, usize>) -> (String, Vec<ToolCall>) {
    let mut text = String::new();
    let mut calls = Vec::new();

    for part in v
        .pointer("/candidates/0/content/parts")
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
    {
        if let Some(piece) = part.get("text").and_then(Value::as_str) {
            text.push_str(piece);
        }
        if let Some(call) = part.get("functionCall") {
            let name = call
                .get("name")
                .and_then(Value::as_str)
                .unwrap_or_default()
                .to_owned();
            // Gemini gives a call no identifier, and the engine needs one to
            // tie a result back. The name plus how many times it has been
            // asked for is unique within a turn, which is as far as a result
            // ever has to travel.
            let nth = seen.entry(name.clone()).or_insert(0);
            let id = if *nth == 0 {
                name.clone()
            } else {
                format!("{name}-{nth}")
            };
            *nth += 1;

            calls.push(ToolCall {
                id,
                name,
                arguments: call
                    .get("args")
                    .map(ToString::to_string)
                    .unwrap_or_default(),
            });
        }
    }

    (text, calls)
}

/// Strip the JSON Schema keywords Gemini refuses.
///
/// It takes a subset and rejects the whole request for a keyword it does not
/// know, which turns a schema written for every other provider into a 400.
/// Dropping what it cannot read is what lets one declaration serve all of
/// them, per `[R-STAR-061]`.
fn clean_schema(schema: &Value) -> Value {
    const REFUSED: [&str; 4] = ["additionalProperties", "$schema", "x-meow", "default"];

    match schema {
        Value::Object(fields) => Value::Object(
            fields
                .iter()
                .filter(|(key, _)| !REFUSED.contains(&key.as_str()))
                .map(|(key, value)| (key.clone(), clean_schema(value)))
                .collect(),
        ),
        Value::Array(items) => Value::Array(items.iter().map(clean_schema).collect()),
        other => other.clone(),
    }
}

/// Read the usage block.
fn usage_of(v: Option<&Value>) -> Option<meow_core::Usage> {
    let u = v?;
    Some(meow_core::Usage {
        prompt: u
            .get("promptTokenCount")
            .and_then(Value::as_u64)
            .unwrap_or(0) as u32,
        completion: u
            .get("candidatesTokenCount")
            .and_then(Value::as_u64)
            .unwrap_or(0) as u32,
        cached: u
            .get("cachedContentTokenCount")
            .and_then(Value::as_u64)
            .map(|n| n as u32),
        cost_micros: None,
    })
}

#[async_trait::async_trait]
impl<T: Transport> Provider for Gemini<T> {
    fn name(&self) -> &str {
        "gemini"
    }

    fn capabilities(&self) -> Capabilities {
        Capabilities {
            streaming: true,
            tools: true,
            // `responseSchema` is a real parameter, so the API enforces it.
            structured: Structured::Native,
            embeddings: true,
        }
    }

    async fn generate(&self, request: &Request, cancel: &CancellationToken) -> Result<Response> {
        check_supported(self, request, false)?;
        if cancel.is_cancelled() {
            return Err(LlmError::Cancelled);
        }

        let url = format!(
            "{}/v1beta/models/{}:generateContent",
            self.base_url, request.model
        );
        let body = self.body(request).to_string();
        let headers = self.headers();

        let http = tokio::select! {
            () = cancel.cancelled() => return Err(LlmError::Cancelled),
            r = self.transport.post(&url, &headers, body) => r?,
        };
        if http.status >= 400 {
            return Err(self.error_from(&http));
        }

        let mut response = self.parse(&http.body)?;
        if let Some(schema) = &request.output_schema {
            // One attempt: the API enforced the schema, so a failure here is
            // the answer being wrong rather than the model needing another go.
            response.value = Some(parse_and_validate(&response.text, schema, 1)?);
        }
        Ok(response)
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

        let url = format!(
            "{}/v1beta/models/{}:streamGenerateContent?alt=sse",
            self.base_url, request.model
        );
        let body = self.body(request).to_string();
        let headers = self.headers();
        let mut lines = self.transport.post_lines(&url, &headers, body).await?;

        let mut agg = Aggregator::new();
        let mut seen = BTreeMap::new();
        let mut started: Vec<String> = Vec::new();

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

            let (text, calls) = parts_of(&v, &mut seen);
            let mut events = Vec::new();

            if !text.is_empty() {
                events.push(StreamEvent::Text(text));
            }
            // Gemini sends a function call whole rather than in pieces, so the
            // three events that a partial call would produce are emitted
            // together and the aggregator sees the same shape either way.
            for call in calls {
                events.push(StreamEvent::ToolCallStart {
                    id: call.id.clone(),
                    name: call.name.clone(),
                });
                events.push(StreamEvent::ToolCallDelta {
                    id: call.id.clone(),
                    arguments: call.arguments,
                });
                events.push(StreamEvent::ToolCallEnd {
                    id: call.id.clone(),
                });
                started.push(call.id);
            }
            events.extend(usage_of(v.get("usageMetadata")).map(StreamEvent::Usage));

            for event in events {
                // [R-LLM-023]: the consumer's error aborts the request and
                // reaches the caller unchanged.
                sink.event(event.clone())?;
                agg.push(event);
            }
        }

        let done = StreamEvent::Done;
        sink.event(done.clone())?;
        agg.push(done);

        let mut response = agg.finish(self.name())?;
        if let Some(schema) = &request.output_schema {
            response.value = Some(parse_and_validate(&response.text, schema, 1)?);
        }
        Ok(response)
    }

    async fn embed(&self, texts: &[String], cancel: &CancellationToken) -> Result<Vec<Vec<f32>>> {
        if cancel.is_cancelled() {
            return Err(LlmError::Cancelled);
        }

        let model = "text-embedding-004";
        let url = format!("{}/v1beta/models/{model}:batchEmbedContents", self.base_url);
        let body = json!({
            "requests": texts
                .iter()
                .map(|text| json!({
                    "model": format!("models/{model}"),
                    "content": { "parts": [{ "text": text }] },
                }))
                .collect::<Vec<_>>(),
        })
        .to_string();

        let headers = self.headers();
        let http = tokio::select! {
            () = cancel.cancelled() => return Err(LlmError::Cancelled),
            r = self.transport.post(&url, &headers, body) => r?,
        };
        if http.status >= 400 {
            return Err(self.error_from(&http));
        }

        let v: Value = serde_json::from_str(&http.body).map_err(|e| LlmError::Malformed {
            provider: self.name().to_owned(),
            message: e.to_string(),
        })?;

        let vectors: Vec<Vec<f32>> = v
            .get("embeddings")
            .and_then(Value::as_array)
            .into_iter()
            .flatten()
            .map(|e| {
                e.pointer("/values")
                    .and_then(Value::as_array)
                    .map(|values| {
                        values
                            .iter()
                            .filter_map(Value::as_f64)
                            .map(|f| f as f32)
                            .collect()
                    })
                    .unwrap_or_default()
            })
            .collect();

        if vectors.is_empty() {
            return Err(LlmError::Malformed {
                provider: self.name().to_owned(),
                message: "the answer carries no embeddings".to_owned(),
            });
        }
        Ok(vectors)
    }
}
