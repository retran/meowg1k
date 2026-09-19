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
mod error;
mod http;
mod message;
mod provider;
mod retry;
mod schema;
mod stream;
mod transport;

pub use crate::anthropic::Anthropic;
pub use crate::error::{Class, LlmError, Result};
pub use crate::http::Http;
pub use crate::message::{Message, Request, Response, Role, ToolCall, ToolDefinition};
pub use crate::provider::{Capabilities, Provider, Structured, check_supported};
pub use crate::retry::{Retry, with_retry};
pub use crate::schema::{SCHEMA_ATTEMPTS, parse_and_validate, validate};
pub use crate::stream::{Aggregator, Collect, Sink, StreamEvent};
pub use crate::transport::{HttpResponse, Recorded, Transport};
