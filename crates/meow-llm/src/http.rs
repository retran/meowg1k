// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The transport that actually reaches a vendor.

use std::time::Duration;

use tokio::sync::mpsc::{Receiver, channel};

use crate::error::{LlmError, Result};
use crate::transport::{HttpResponse, Transport};

/// How many lines may be buffered before the reader has to catch up.
///
/// A streaming response arrives faster than a renderer draws, and an unbounded
/// channel would let a slow terminal turn into unbounded memory.
const LINE_BUFFER: usize = 256;

/// How long to wait for the first byte of a response.
///
/// Not a limit on the whole response: a model thinking for two minutes is
/// normal, and a connect timeout that fires on it would make the retry logic
/// hammer a provider that is working.
const CONNECT_TIMEOUT: Duration = Duration::from_secs(30);

/// Talks to a vendor over HTTPS.
#[derive(Debug, Clone)]
pub struct Http {
    client: reqwest::Client,
    provider: String,
}

impl Http {
    /// Build a transport for one provider.
    ///
    /// The name is carried only so an error can say which vendor failed;
    /// "connection refused" without one is unactionable when a workspace
    /// declares three.
    ///
    /// # Errors
    ///
    /// [`LlmError::Transport`] when the client cannot be built at all, which
    /// in practice means the TLS backend failed to initialise.
    pub fn new(provider: impl Into<String>) -> Result<Self> {
        let provider = provider.into();
        let client = reqwest::Client::builder()
            .connect_timeout(CONNECT_TIMEOUT)
            .user_agent(concat!("meowg1k/", env!("CARGO_PKG_VERSION")))
            .build()
            .map_err(|e| LlmError::Transport {
                provider: provider.clone(),
                message: e.to_string(),
            })?;
        Ok(Self { client, provider })
    }

    fn failed(&self, e: &reqwest::Error) -> LlmError {
        LlmError::Transport {
            provider: self.provider.clone(),
            message: e.to_string(),
        }
    }

    fn request(
        &self,
        url: &str,
        headers: &[(String, String)],
        body: String,
    ) -> reqwest::RequestBuilder {
        let mut request = self.client.post(url).body(body);
        for (name, value) in headers {
            request = request.header(name, value);
        }
        request
    }
}

/// What the server asked us to wait, when it said.
///
/// Only the seconds form is read. The HTTP-date form is legal and no vendor
/// here sends it, and guessing wrong about a date is worse than falling back
/// to the retry policy's own backoff.
fn retry_after(headers: &reqwest::header::HeaderMap) -> Option<Duration> {
    let value = headers.get(reqwest::header::RETRY_AFTER)?.to_str().ok()?;
    value.trim().parse::<u64>().ok().map(Duration::from_secs)
}

#[async_trait::async_trait]
impl Transport for Http {
    async fn post(
        &self,
        url: &str,
        headers: &[(String, String)],
        body: String,
    ) -> Result<HttpResponse> {
        let response = self
            .request(url, headers, body)
            .send()
            .await
            .map_err(|e| self.failed(&e))?;

        let status = response.status().as_u16();
        let retry_after = retry_after(response.headers());
        // A status is not an error here, by the trait's contract: the provider
        // decides what one means, because only it knows which of its codes are
        // transient.
        let body = response.text().await.map_err(|e| self.failed(&e))?;

        Ok(HttpResponse {
            status,
            body,
            retry_after,
        })
    }

    async fn post_lines(
        &self,
        url: &str,
        headers: &[(String, String)],
        body: String,
    ) -> Result<Receiver<Result<String>>> {
        let response = self
            .request(url, headers, body)
            .send()
            .await
            .map_err(|e| self.failed(&e))?;

        let status = response.status();
        if !status.is_success() {
            // A failed stream never produces a line, so reporting it through
            // the channel would make the caller wait for something that is not
            // coming. The body is the vendor's error object, and the provider
            // knows how to read it.
            let body = response.text().await.map_err(|e| self.failed(&e))?;
            return Err(LlmError::Http {
                provider: self.provider.clone(),
                status: status.as_u16(),
                message: body,
                retry_after: None,
                quota_exhausted: false,
            });
        }

        let (sender, receiver) = channel(LINE_BUFFER);
        let provider = self.provider.clone();

        tokio::spawn(async move {
            let mut stream = response;
            let mut pending = String::new();

            loop {
                match stream.chunk().await {
                    Ok(Some(bytes)) => {
                        pending.push_str(&String::from_utf8_lossy(&bytes));
                        // A chunk boundary falls anywhere, so a line is only
                        // complete once its newline has arrived.
                        while let Some(at) = pending.find('\n') {
                            let line: String = pending.drain(..=at).collect();
                            if sender
                                .send(Ok(line.trim_end_matches(['\r', '\n']).to_owned()))
                                .await
                                .is_err()
                            {
                                return;
                            }
                        }
                    }
                    Ok(None) => {
                        if !pending.is_empty() {
                            let _ = sender.send(Ok(pending)).await;
                        }
                        return;
                    }
                    Err(e) => {
                        let _ = sender
                            .send(Err(LlmError::Transport {
                                provider,
                                message: e.to_string(),
                            }))
                            .await;
                        return;
                    }
                }
            }
        });

        Ok(receiver)
    }
}
