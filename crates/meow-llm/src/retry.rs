// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Trying again, and knowing when not to.

use std::time::Duration;

use rand::Rng;
use tokio_util::sync::CancellationToken;

use crate::error::{Class, LlmError, Result};

/// How many times, and how long between.
#[derive(Debug, Clone, Copy)]
pub struct Retry {
    /// Including the first try.
    pub attempts: u32,
    /// How long to wait after the first failure.
    pub base: Duration,
    /// The longest any single wait may be.
    pub cap: Duration,
}

impl Default for Retry {
    fn default() -> Self {
        Self {
            attempts: 4,
            base: Duration::from_millis(500),
            cap: Duration::from_secs(30),
        }
    }
}

impl Retry {
    /// How long to wait before attempt `n`, counting the first as one.
    ///
    /// Exponential with full jitter: the wait is uniform over the whole
    /// interval rather than clustered at its end, so a hundred clients that
    /// failed together do not all return together.
    fn backoff(&self, attempt: u32, rng: &mut impl Rng) -> Duration {
        let exp = self
            .base
            .saturating_mul(1u32 << (attempt.saturating_sub(1)).min(16));
        let capped = exp.min(self.cap);
        Duration::from_millis(rng.random_range(0..=capped.as_millis().max(1) as u64))
    }
}

/// Run an operation, retrying only what is worth retrying.
///
/// Satisfies `[R-LLM-033]` by retrying `Transient` and nothing else: a `Fatal`
/// error surfaces on its first occurrence with no delay, and `[R-LLM-035]`
/// keeps a spent quota from being retried at all. Satisfies `[R-LLM-034]` by
/// honouring `Retry-After` when the provider sent one, since a server that
/// says how long to wait knows better than an exponent. Satisfies
/// `[R-LLM-036]` by checking cancellation before each sleep and before each
/// attempt, so stopping a run does not first wait out a backoff.
pub async fn with_retry<T, F, Fut>(
    provider: &str,
    policy: Retry,
    cancel: &CancellationToken,
    mut operation: F,
) -> Result<T>
where
    F: FnMut() -> Fut,
    Fut: Future<Output = Result<T>>,
{
    let mut last: Option<LlmError> = None;
    for attempt in 1..=policy.attempts {
        if cancel.is_cancelled() {
            return Err(LlmError::Cancelled);
        }
        if attempt > 1 {
            let wait = last
                .as_ref()
                .and_then(LlmError::retry_after)
                .unwrap_or_else(|| policy.backoff(attempt - 1, &mut rand::rng()));
            tokio::select! {
                () = cancel.cancelled() => return Err(LlmError::Cancelled),
                () = tokio::time::sleep(wait) => {}
            }
        }

        match operation().await {
            Ok(value) => return Ok(value),
            Err(e) if e.class() == Class::Transient => last = Some(e),
            Err(e) => return Err(e),
        }
    }

    Err(LlmError::Exhausted {
        provider: provider.to_owned(),
        attempts: policy.attempts,
        source: Box::new(last.unwrap_or(LlmError::Transport {
            provider: provider.to_owned(),
            message: "no attempt was made".to_owned(),
        })),
    })
}
