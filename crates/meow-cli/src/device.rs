// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The OAuth device-code flow, for providers that do not issue API keys.
//!
//! `[R-AUTH-020]`: the command shows a code and a URL, waits for the person to
//! approve, and stores the result. `[R-AUTH-021]`: it honours the interval the
//! server asks for, stops when the server says the code expired, and can be
//! interrupted without leaving anything half-written.

use std::time::Duration;

use serde::Deserialize;
use tokio_util::sync::CancellationToken;

/// How long to wait before giving up entirely, whatever the server says.
///
/// The server sends its own expiry and this respects it; this is the ceiling
/// for a server that sends an absurd one or none at all.
const CEILING: Duration = Duration::from_secs(15 * 60);

/// The shortest poll interval this will use.
///
/// GitHub asks for five seconds and answers `slow_down` when a client is too
/// eager. Honouring the ask is `[R-AUTH-021]`; this floor is what stops a
/// server that sends `0` from becoming a busy loop.
const FLOOR: Duration = Duration::from_secs(1);

/// Where a flow talks to.
#[derive(Debug, Clone)]
pub struct Endpoints {
    /// Where a device code is requested.
    pub code: String,
    /// Where it is exchanged for a token.
    pub token: String,
    /// Which application is asking.
    pub client_id: String,
    /// What it is asking for.
    pub scope: String,
}

impl Endpoints {
    /// GitHub, as Copilot uses it.
    ///
    /// The client id is the one the editor plugins use. Copilot has no
    /// registration for third-party clients, so there is no other id to use
    /// and nothing here is a secret - it identifies the application, and the
    /// person still has to approve the request in a browser.
    pub fn github() -> Self {
        Self {
            code: "https://github.com/login/device/code".to_owned(),
            token: "https://github.com/login/oauth/access_token".to_owned(),
            client_id: "Iv1.b507a08c87ecfe98".to_owned(),
            scope: "read:user".to_owned(),
        }
    }
}

/// What went wrong.
#[derive(Debug, thiserror::Error)]
pub enum DeviceError {
    /// The server could not be reached, or answered something unreadable.
    #[error("{doing}: {message}")]
    Server {
        /// Which step.
        doing: &'static str,
        /// What went wrong.
        message: String,
    },

    /// The person did not approve in time.
    #[error("the code expired before it was approved; run the command again")]
    Expired,

    /// The server refused outright.
    #[error("{0}")]
    Refused(String),

    /// Ctrl-C.
    #[error("cancelled")]
    Cancelled,
}

/// What the server says to show a person.
#[derive(Debug, Deserialize)]
struct DeviceCode {
    device_code: String,
    user_code: String,
    verification_uri: String,
    #[serde(default)]
    interval: u64,
    #[serde(default)]
    expires_in: u64,
}

/// What comes back once it is approved, or why it has not been.
#[derive(Debug, Deserialize)]
struct TokenAnswer {
    #[serde(default)]
    access_token: Option<String>,
    #[serde(default)]
    refresh_token: Option<String>,
    #[serde(default)]
    expires_in: Option<i64>,
    #[serde(default)]
    error: Option<String>,
    #[serde(default)]
    error_description: Option<String>,
    #[serde(default)]
    interval: Option<u64>,
}

/// What a completed flow produced.
#[derive(Debug)]
pub struct Granted {
    /// What is sent until it expires.
    pub access: String,
    /// What obtains the next one, when the server issues one.
    pub refresh: String,
    /// When the access token stops working, or zero when it does not expire.
    pub expires: i64,
}

/// Run the flow, telling the person what to do through `show`.
///
/// # Errors
///
/// [`DeviceError`] when the server cannot be reached, refuses, or the code
/// expires before it is approved, and on cancellation.
pub async fn run(
    endpoints: &Endpoints,
    cancel: &CancellationToken,
    show: impl Fn(&str, &str),
) -> Result<Granted, DeviceError> {
    // Before the select in the loop below, for the reason given there.
    if cancel.is_cancelled() {
        return Err(DeviceError::Cancelled);
    }

    let client = reqwest::Client::builder()
        .user_agent(concat!("meowg1k/", env!("CARGO_PKG_VERSION")))
        .build()
        .map_err(|e| DeviceError::Server {
            doing: "building an HTTP client",
            message: e.to_string(),
        })?;

    let started: DeviceCode = client
        .post(&endpoints.code)
        .header("accept", "application/json")
        .json(&serde_json::json!({
            "client_id": endpoints.client_id,
            "scope": endpoints.scope,
        }))
        .send()
        .await
        .map_err(|e| DeviceError::Server {
            doing: "requesting a device code",
            message: e.to_string(),
        })?
        .json()
        .await
        .map_err(|e| DeviceError::Server {
            doing: "reading the device code",
            message: e.to_string(),
        })?;

    show(&started.user_code, &started.verification_uri);

    // The server's expiry, bounded. A server that sends nothing gets the
    // ceiling rather than an immediate failure.
    let deadline = Duration::from_secs(started.expires_in).clamp(FLOOR, CEILING);
    let mut wait = Duration::from_secs(started.interval).max(FLOOR);
    let began = std::time::Instant::now();

    loop {
        tokio::select! {
            () = cancel.cancelled() => return Err(DeviceError::Cancelled),
            () = tokio::time::sleep(wait) => {}
        }

        if began.elapsed() >= deadline {
            return Err(DeviceError::Expired);
        }

        let answer: TokenAnswer = client
            .post(&endpoints.token)
            .header("accept", "application/json")
            .json(&serde_json::json!({
                "client_id": endpoints.client_id,
                "device_code": started.device_code,
                "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
            }))
            .send()
            .await
            .map_err(|e| DeviceError::Server {
                doing: "asking whether it was approved",
                message: e.to_string(),
            })?
            .json()
            .await
            .map_err(|e| DeviceError::Server {
                doing: "reading the answer",
                message: e.to_string(),
            })?;

        if let Some(access) = answer.access_token {
            return Ok(Granted {
                access,
                refresh: answer.refresh_token.unwrap_or_default(),
                expires: answer.expires_in.map_or(0, |seconds| {
                    i64::try_from(now_secs()).unwrap_or(0) + seconds
                }),
            });
        }

        match answer.error.as_deref() {
            // Nobody has approved it yet, which is the ordinary case.
            Some("authorization_pending") => {}
            // `[R-AUTH-021]`: the server is entitled to ask for more room.
            Some("slow_down") => {
                wait = answer
                    .interval
                    .map_or(wait + Duration::from_secs(5), Duration::from_secs)
                    .max(FLOOR);
            }
            Some("expired_token") => return Err(DeviceError::Expired),
            Some(other) => {
                return Err(DeviceError::Refused(
                    answer.error_description.unwrap_or_else(|| other.to_owned()),
                ));
            }
            // No token and no error is a server this cannot make sense of,
            // and looping forever on it would look like a hang.
            None => {
                return Err(DeviceError::Server {
                    doing: "asking whether it was approved",
                    message: "the server sent neither a token nor an error".to_owned(),
                });
            }
        }
    }
}

/// Now, in seconds.
fn now_secs() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_or(0, |d| d.as_secs())
}
