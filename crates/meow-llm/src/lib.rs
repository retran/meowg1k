// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What a model provider is, and one that talks to Anthropic.
//!
//! This crate defines the request and response types, the streaming events,
//! the error classification, and the retry policy. Each provider implements
//! [`Provider`] over one vendor's API.
//!
//! No vendor type crosses the trait. That is `[R-LLM-003]`, and it is what
//! lets the engine be written once instead of once per provider.
//!
//! `docs/spec/llm.md` is normative.

mod anthropic;
mod bearer;
pub mod copilot;
mod error;
mod gemini;
mod http;
mod message;
mod openai;
mod provider;
mod retry;
mod schema;
mod stream;
mod transport;
mod voyage;

pub use crate::anthropic::Anthropic;
pub use crate::bearer::{Bearer, Exchanged, Fixed};
pub use crate::error::{Class, LlmError, Result};
pub use crate::gemini::Gemini;
pub use crate::http::Http;
pub use crate::message::{Message, Request, Response, Role, ToolCall, ToolDefinition};
pub use crate::openai::OpenAi;
pub use crate::provider::{Capabilities, Provider, Structured, check_supported};
pub use crate::retry::{Retry, with_retry};
pub use crate::schema::{SCHEMA_ATTEMPTS, parse_and_validate, validate};
pub use crate::stream::{Aggregator, Collect, Sink, StreamEvent};
pub use crate::transport::{HttpResponse, Recorded, Transport};
pub use crate::voyage::Voyage;

/// Read a usage block written the way most APIs write one.
///
/// `[R-LLM-040]`: cached tokens stay absent when the provider does not report
/// them, never zero. `[R-LLM-041]`: no usage block at all is `None`, which is
/// a different fact from a block of zeroes.
pub(crate) fn usage_of(v: Option<&serde_json::Value>) -> Option<meow_core::Usage> {
    use serde_json::Value;

    let u = v?;
    if !u.is_object() {
        return None;
    }
    Some(meow_core::Usage {
        prompt: u.get("prompt_tokens").and_then(Value::as_u64).unwrap_or(0) as u32,
        completion: u
            .get("completion_tokens")
            .and_then(Value::as_u64)
            .unwrap_or(0) as u32,
        cached: u
            .pointer("/prompt_tokens_details/cached_tokens")
            .and_then(Value::as_u64)
            .map(|n| n as u32),
        cost_micros: None,
    })
}
