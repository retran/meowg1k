// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The one thing a provider needs from the network.
//!
//! Keeping HTTP behind a trait is what lets a provider be tested against a
//! recorded exchange instead of a live account. `[R-LLM-021]` is stated over a
//! recording for the same reason: two live calls to a model cannot be compared,
//! because the model is free to answer differently.

use tokio::sync::mpsc::Receiver;

use crate::error::Result;

/// What came back from one request.
#[derive(Debug, Clone)]
pub struct HttpResponse {
    /// The status.
    pub status: u16,
    /// The body.
    pub body: String,
    /// How long the server asked us to wait, when it said.
    pub retry_after: Option<std::time::Duration>,
}

/// Somewhere to send a request.
#[async_trait::async_trait]
pub trait Transport: Send + Sync {
    /// Send a request and wait for the whole answer.
    ///
    /// # Errors
    ///
    /// [`crate::LlmError::Transport`] when the connection fails. A status is
    /// not an error here: the provider decides what a status means.
    async fn post(
        &self,
        url: &str,
        headers: &[(String, String)],
        body: String,
    ) -> Result<HttpResponse>;

    /// Send a request and receive the answer as it arrives, line by line.
    ///
    /// # Errors
    ///
    /// [`crate::LlmError::Transport`] when the connection fails before the
    /// first line. A failure afterwards arrives through the channel.
    async fn post_lines(
        &self,
        url: &str,
        headers: &[(String, String)],
        body: String,
    ) -> Result<Receiver<Result<String>>>;
}

/// A transport that replays an exchange somebody recorded.
///
/// This is how every provider test in this crate runs: no network, no account,
/// and an answer that does not change between runs.
#[derive(Debug, Default)]
pub struct Recorded {
    /// Answered in order, one per call to [`Transport::post`].
    pub responses: std::sync::Mutex<Vec<HttpResponse>>,
    /// Answered in order, one per call to [`Transport::post_lines`].
    pub streams: std::sync::Mutex<Vec<Vec<String>>>,
    /// Every body that was sent, so a test can check what was asked.
    pub sent: std::sync::Mutex<Vec<String>>,
}

impl Recorded {
    /// A transport that answers these, in order.
    pub fn with_responses(responses: Vec<HttpResponse>) -> Self {
        Self {
            responses: std::sync::Mutex::new(responses.into_iter().rev().collect()),
            ..Self::default()
        }
    }

    /// A transport that streams these lines, in order.
    pub fn with_stream(lines: Vec<String>) -> Self {
        Self {
            streams: std::sync::Mutex::new(vec![lines]),
            ..Self::default()
        }
    }

    /// Everything that was sent, oldest first.
    pub fn bodies(&self) -> Vec<String> {
        self.sent.lock().map(|g| g.clone()).unwrap_or_default()
    }
}

#[async_trait::async_trait]
impl Transport for Recorded {
    async fn post(
        &self,
        _url: &str,
        _headers: &[(String, String)],
        body: String,
    ) -> Result<HttpResponse> {
        if let Ok(mut sent) = self.sent.lock() {
            sent.push(body);
        }
        let next = self.responses.lock().ok().and_then(|mut r| r.pop());
        next.ok_or_else(|| crate::LlmError::Transport {
            provider: "recorded".to_owned(),
            message: "the recording has no more responses".to_owned(),
        })
    }

    async fn post_lines(
        &self,
        _url: &str,
        _headers: &[(String, String)],
        body: String,
    ) -> Result<Receiver<Result<String>>> {
        if let Ok(mut sent) = self.sent.lock() {
            sent.push(body);
        }
        let lines = self.streams.lock().ok().and_then(|mut s| {
            if s.is_empty() {
                None
            } else {
                Some(s.remove(0))
            }
        });
        let lines = lines.ok_or_else(|| crate::LlmError::Transport {
            provider: "recorded".to_owned(),
            message: "the recording has no more streams".to_owned(),
        })?;

        let (tx, rx) = tokio::sync::mpsc::channel(lines.len().max(1));
        for line in lines {
            let _ = tx.send(Ok(line)).await;
        }
        Ok(rx)
    }
}
