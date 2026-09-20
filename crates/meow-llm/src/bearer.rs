// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What a provider sends to authenticate, asked for rather than held.
//!
//! `[R-LLM-004]`. Every provider here but one uses a key that does not change,
//! and building the token in was right for them. It is wrong for one that
//! exchanges a long-lived grant for a short-lived token: the provider would
//! have to be rebuilt on a schedule nobody owns.

use std::sync::{Arc, Mutex};

use async_trait::async_trait;
use tokio_util::sync::CancellationToken;

use crate::error::{LlmError, Result};

/// What goes in the `authorization` header.
#[async_trait]
pub trait Bearer: Send + Sync + std::fmt::Debug {
    /// The token to send with this request.
    ///
    /// # Errors
    ///
    /// [`LlmError`] when a token cannot be obtained, which the caller reports
    /// rather than retrying: a credential that will not renew does not renew
    /// on the second attempt either.
    async fn token(&self, cancel: &CancellationToken) -> Result<String>;
}

/// A key that does not change, which is most of them.
#[derive(Debug, Clone)]
pub struct Fixed(String);

impl Fixed {
    /// Send this, always.
    pub fn new(key: impl Into<String>) -> Self {
        Self(key.into())
    }
}

#[async_trait]
impl Bearer for Fixed {
    async fn token(&self, _cancel: &CancellationToken) -> Result<String> {
        Ok(self.0.clone())
    }
}

/// A token obtained by exchanging something longer-lived, and re-obtained
/// when it expires.
///
/// `[R-AUTH-022]`: the person is not asked again. What they approved is the
/// grant; this is what the grant is for.
pub struct Exchanged {
    /// Which provider, for the error when it will not renew.
    provider: String,
    /// What the grant is exchanged at.
    endpoint: String,
    /// The long-lived grant.
    grant: String,
    /// Extra headers the exchange needs.
    headers: Vec<(String, String)>,
    /// The short-lived token and when it stops working.
    held: Mutex<Option<Held>>,
    /// How the exchange is made.
    client: reqwest::Client,
}

/// A token and its expiry.
#[derive(Debug, Clone)]
struct Held {
    token: String,
    /// Seconds since the epoch.
    expires: i64,
}

impl std::fmt::Debug for Exchanged {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Exchanged")
            .field("provider", &self.provider)
            .field("endpoint", &self.endpoint)
            .finish_non_exhaustive()
    }
}

/// How long before an expiry a token is treated as expired.
///
/// A token that expires during the request it was fetched for is a token that
/// failed, so the window is wide enough for a slow call to finish.
const EARLY: i64 = 60;

impl Exchanged {
    /// Exchange this grant at this endpoint, for this provider.
    pub fn new(
        provider: impl Into<String>,
        endpoint: impl Into<String>,
        grant: impl Into<String>,
        headers: Vec<(String, String)>,
    ) -> Result<Self> {
        let provider = provider.into();
        let client = reqwest::Client::builder()
            .user_agent(concat!("meowg1k/", env!("CARGO_PKG_VERSION")))
            .build()
            .map_err(|e| LlmError::Transport {
                provider: provider.clone(),
                message: e.to_string(),
            })?;

        Ok(Self {
            provider,
            endpoint: endpoint.into(),
            grant: grant.into(),
            headers,
            held: Mutex::new(None),
            client,
        })
    }

    /// What is held, if it is still good for a while.
    fn usable(&self) -> Option<String> {
        let held = self.held.lock().ok()?;
        let held = held.as_ref()?;
        (held.expires - EARLY > now_secs()).then(|| held.token.clone())
    }
}

#[async_trait]
impl Bearer for Exchanged {
    async fn token(&self, cancel: &CancellationToken) -> Result<String> {
        if let Some(token) = self.usable() {
            return Ok(token);
        }

        let mut request = self
            .client
            .get(&self.endpoint)
            .header("authorization", format!("token {}", self.grant));
        for (name, value) in &self.headers {
            request = request.header(name.as_str(), value.as_str());
        }

        let response = tokio::select! {
            () = cancel.cancelled() => return Err(LlmError::Cancelled),
            response = request.send() => response.map_err(|e| LlmError::Transport {
                provider: self.provider.clone(),
                message: format!("renewing the credential: {e}"),
            })?,
        };

        let status = response.status();
        let body = response.text().await.unwrap_or_default();

        if !status.is_success() {
            // `[R-AUTH-022]`: name the command, because the usual cause is a
            // grant that was revoked and the fix is to authenticate again.
            return Err(LlmError::Auth {
                provider: self.provider.clone(),
                message: format!(
                    "the credential would not renew ({status}); run `meow auth login {}`",
                    self.provider
                ),
            });
        }

        let parsed: Renewed = serde_json::from_str(&body).map_err(|e| LlmError::Transport {
            provider: self.provider.clone(),
            message: format!("renewing the credential: {e}"),
        })?;

        if let Ok(mut held) = self.held.lock() {
            *held = Some(Held {
                token: parsed.token.clone(),
                expires: parsed.expires_at,
            });
        }

        Ok(parsed.token)
    }
}

/// What an exchange answers with.
#[derive(Debug, serde::Deserialize)]
struct Renewed {
    token: String,
    #[serde(default)]
    expires_at: i64,
}

/// An `Arc` of a fixed key, which is what most callers want.
pub fn fixed(key: impl Into<String>) -> Arc<dyn Bearer> {
    Arc::new(Fixed::new(key))
}

fn now_secs() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_or(0, |d| i64::try_from(d.as_secs()).unwrap_or(0))
}
