// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Voyage, which only embeds.
//!
//! A provider that declares no chat is not a lesser provider: `[R-LLM-002]`
//! refuses a request for something a provider did not declare before it is
//! sent, so a workspace that names this for an agent finds out at declaration
//! time rather than from a confusing answer.

use serde_json::json;
use tokio_util::sync::CancellationToken;

use crate::error::{LlmError, Result};
use crate::message::{Request, Response};
use crate::openai::vectors_from;
use crate::provider::{Capabilities, Provider, Structured};
use crate::stream::Sink;
use crate::transport::Transport;

/// Talks to Voyage.
pub struct Voyage<T: Transport> {
    transport: T,
    api_key: String,
    base_url: String,
    model: String,
}

impl<T: Transport> std::fmt::Debug for Voyage<T> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Voyage")
            .field("base_url", &self.base_url)
            .field("model", &self.model)
            .finish_non_exhaustive()
    }
}

impl<T: Transport> Voyage<T> {
    /// Build a provider for one embedding model.
    ///
    /// The model is fixed at construction rather than taken per call, because
    /// `[R-INDEX-051]` makes an index answerable for which model built it and
    /// a provider that could silently use another would undo that.
    pub fn new(transport: T, api_key: impl Into<String>, model: impl Into<String>) -> Self {
        Self {
            transport,
            api_key: api_key.into(),
            base_url: "https://api.voyageai.com".to_owned(),
            model: model.into(),
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
}

#[async_trait::async_trait]
impl<T: Transport> Provider for Voyage<T> {
    fn name(&self) -> &str {
        "voyage"
    }

    fn capabilities(&self) -> Capabilities {
        Capabilities {
            streaming: false,
            tools: false,
            structured: Structured::None,
            embeddings: true,
        }
    }

    async fn generate(&self, request: &Request, _cancel: &CancellationToken) -> Result<Response> {
        let _ = request;
        Err(LlmError::Unsupported {
            provider: self.name().to_owned(),
            capability: "generation",
        })
    }

    async fn generate_stream(
        &self,
        request: &Request,
        sink: &mut dyn Sink,
        _cancel: &CancellationToken,
    ) -> Result<Response> {
        let _ = (request, sink);
        Err(LlmError::Unsupported {
            provider: self.name().to_owned(),
            capability: "generation",
        })
    }

    async fn embed(&self, texts: &[String], cancel: &CancellationToken) -> Result<Vec<Vec<f32>>> {
        if cancel.is_cancelled() {
            return Err(LlmError::Cancelled);
        }

        let url = format!("{}/v1/embeddings", self.base_url);
        let headers = vec![
            (
                "authorization".to_owned(),
                format!("Bearer {}", self.api_key),
            ),
            ("content-type".to_owned(), "application/json".to_owned()),
        ];
        let body = json!({
            "model": self.model,
            "input": texts,
            // What the vectors are for. Voyage embeds a document and a query
            // differently, and using the wrong one costs recall quietly.
            "input_type": "document",
        })
        .to_string();

        let http = tokio::select! {
            () = cancel.cancelled() => return Err(LlmError::Cancelled),
            r = self.transport.post(&url, &headers, body) => r?,
        };

        if http.status >= 400 {
            let parsed: Option<serde_json::Value> = serde_json::from_str(&http.body).ok();
            let message = parsed
                .as_ref()
                .and_then(|v| v.get("detail"))
                .and_then(serde_json::Value::as_str)
                .unwrap_or(&http.body)
                .to_owned();
            return Err(LlmError::Http {
                provider: self.name().to_owned(),
                status: http.status,
                message,
                retry_after: http.retry_after,
                // Voyage reports a spent quota as a plain 402, with no code to
                // read. The status is the signal, which is still the API's own
                // rather than its wording.
                quota_exhausted: http.status == 402,
            });
        }

        vectors_from(&http.body, self.name())
    }
}
