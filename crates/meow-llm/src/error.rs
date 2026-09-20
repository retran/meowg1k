// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! How a provider failure is classified, and what follows from it.

/// What kind of failure this is.
///
/// `[R-LLM-030]` requires exactly one of three for every error. The v0.2.x
/// gateway had no classification and retried everything that was not a hard
/// quota error, which is why an invalid API key cost the full backoff schedule
/// before anyone saw it.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Class {
    /// Worth trying again: a rate limit, a gateway hiccup, a dropped
    /// connection.
    Transient,
    /// Not worth trying again: a bad request, a bad key, a rejected schema.
    Fatal,
    /// The account is out of budget. Retrying cannot help.
    QuotaExhausted,
}

/// Everything a provider can fail with.
#[derive(Debug, thiserror::Error)]
pub enum LlmError {
    /// The provider returned a status.
    #[error("{provider} returned {status}: {message}")]
    Http {
        /// Which provider.
        provider: String,
        /// The HTTP status.
        status: u16,
        /// What it said.
        message: String,
        /// How long it asked us to wait, when it said.
        retry_after: Option<std::time::Duration>,
        /// Whether the provider's own signal says the quota is spent.
        quota_exhausted: bool,
    },

    /// The connection failed or timed out.
    #[error("{provider}: {message}")]
    Transport {
        /// Which provider.
        provider: String,
        /// What went wrong.
        message: String,
    },

    /// A credential could not be obtained or renewed.
    ///
    /// `[R-LLM-004]`. Its own variant rather than a `Transport`, because it
    /// is the one failure here a person can fix and the message says how.
    /// Retrying it is pointless: a grant that will not renew does not renew
    /// on the second attempt either.
    #[error("{provider}: {message}")]
    Auth {
        /// Which provider.
        provider: String,
        /// What went wrong, and what to run.
        message: String,
    },

    /// The provider does not do this.
    ///
    /// `[R-LLM-002]`: raised before a request is sent, so a caller is not
    /// billed for discovering it.
    #[error("{provider} does not support {capability}")]
    Unsupported {
        /// Which provider.
        provider: String,
        /// What was asked of it.
        capability: &'static str,
    },

    /// The answer did not satisfy the schema, after every allowed attempt.
    #[error("the response did not satisfy the schema after {attempts} attempts: {message}")]
    Schema {
        /// How many times it was asked.
        attempts: u32,
        /// What was wrong with the last one.
        message: String,
    },

    /// The response could not be read.
    #[error("{provider} sent a response this client cannot read: {message}")]
    Malformed {
        /// Which provider.
        provider: String,
        /// What was wrong with it.
        message: String,
    },

    /// The caller cancelled.
    ///
    /// `[R-LLM-061]` keeps this distinct from a timeout or a transport error,
    /// because a run that was stopped on purpose is not a run that broke.
    #[error("cancelled")]
    Cancelled,

    /// Retrying did not help.
    #[error("{provider} failed after {attempts} attempts: {source}")]
    Exhausted {
        /// Which provider.
        provider: String,
        /// How many times it was tried.
        attempts: u32,
        /// The last failure.
        #[source]
        source: Box<LlmError>,
    },
}

impl LlmError {
    /// Which kind of failure this is.
    ///
    /// Satisfies `[R-LLM-031]`, `[R-LLM-032]`, and `[R-LLM-037]`. A 429 is a
    /// rate limit unless the provider's own signal says the quota is spent:
    /// retrying a spent quota costs a delay, while refusing a rate limit costs
    /// the run, so the ambiguous case takes the cheaper mistake. The signal is
    /// a field the provider fills from its own documented response, never text
    /// matched out of a message, which is what v0.2.x did.
    pub fn class(&self) -> Class {
        match self {
            Self::Http {
                status,
                quota_exhausted,
                ..
            } => {
                if *quota_exhausted {
                    Class::QuotaExhausted
                } else {
                    match status {
                        408 | 429 | 500 | 502 | 503 | 504 => Class::Transient,
                        _ => Class::Fatal,
                    }
                }
            }
            Self::Transport { .. } => Class::Transient,
            // Fatal, deliberately: a grant that will not renew does not renew
            // on the second attempt, and retrying spends the backoff to reach
            // the same message a person has to read anyway.
            Self::Auth { .. }
            | Self::Unsupported { .. }
            | Self::Schema { .. }
            | Self::Malformed { .. }
            | Self::Cancelled
            | Self::Exhausted { .. } => Class::Fatal,
        }
    }

    /// How long the provider asked us to wait, when it did.
    pub fn retry_after(&self) -> Option<std::time::Duration> {
        match self {
            Self::Http { retry_after, .. } => *retry_after,
            _ => None,
        }
    }
}

/// The result type every fallible call in this crate returns.
pub type Result<T> = std::result::Result<T, LlmError>;
