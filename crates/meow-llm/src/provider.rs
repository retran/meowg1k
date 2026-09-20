// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What a model provider is.

use tokio_util::sync::CancellationToken;

use crate::error::{LlmError, Result};
use crate::message::{Request, Response};
use crate::stream::Sink;

/// How a provider satisfies a request for structured output.
///
/// `[R-LLM-001]` asks a provider to declare this rather than simply whether it
/// supports schemas, because `[R-LLM-050]` says schema support is never
/// refused - only satisfied two different ways.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Structured {
    /// The API takes the schema and enforces it.
    Native,
    /// The provider asks for JSON and checks the answer here.
    Emulated,
    /// It produces no text, so there is nothing for a schema to describe.
    ///
    /// An embedding-only provider. Saying `Emulated` instead would claim it
    /// asks for JSON in a prompt it never sends, and `[R-LLM-002]` would then
    /// accept a schema it cannot satisfy.
    None,
}

/// What a provider can do.
///
/// `[R-LLM-001]`. Declared up front so that `[R-LLM-002]` can refuse before a
/// request is sent, rather than after a caller has been billed for finding out.
#[derive(Debug, Clone, Copy)]
pub struct Capabilities {
    /// Whether it has a native streaming endpoint.
    pub streaming: bool,
    /// Whether it can call tools.
    pub tools: bool,
    /// How it satisfies a schema.
    pub structured: Structured,
    /// Whether it can embed text.
    pub embeddings: bool,
}

/// One model provider.
#[async_trait::async_trait]
pub trait Provider: Send + Sync {
    /// What to call it in an error.
    fn name(&self) -> &str;

    /// What it can do.
    fn capabilities(&self) -> Capabilities;

    /// Ask for an answer.
    ///
    /// # Errors
    ///
    /// Any [`LlmError`]. Classified per `[R-LLM-030]`.
    async fn generate(&self, request: &Request, cancel: &CancellationToken) -> Result<Response>;

    /// Ask for an answer, reporting progress as it arrives.
    ///
    /// The default refuses, which is what `[R-LLM-022]` requires of a provider
    /// with no native streaming endpoint: declaring streaming unsupported is
    /// honest, and synthesising events from a completed response reports
    /// progress that never happened.
    ///
    /// # Errors
    ///
    /// [`LlmError::Unsupported`] unless the provider overrides this.
    async fn generate_stream(
        &self,
        request: &Request,
        sink: &mut dyn Sink,
        cancel: &CancellationToken,
    ) -> Result<Response> {
        let _ = (request, sink, cancel);
        Err(LlmError::Unsupported {
            provider: self.name().to_owned(),
            capability: "streaming",
        })
    }

    /// Turn text into vectors.
    ///
    /// # Errors
    ///
    /// [`LlmError::Unsupported`] unless the provider overrides this.
    async fn embed(&self, texts: &[String], cancel: &CancellationToken) -> Result<Vec<Vec<f32>>> {
        let _ = (texts, cancel);
        Err(LlmError::Unsupported {
            provider: self.name().to_owned(),
            capability: "embeddings",
        })
    }
}

/// Refuse a request that asks for something the provider did not declare.
///
/// `[R-LLM-002]`: before any request is sent.
///
/// # Errors
///
/// [`LlmError::Unsupported`] naming the provider and the capability.
pub fn check_supported(provider: &dyn Provider, request: &Request, streaming: bool) -> Result<()> {
    let caps = provider.capabilities();
    if streaming && !caps.streaming {
        return Err(LlmError::Unsupported {
            provider: provider.name().to_owned(),
            capability: "streaming",
        });
    }
    if !request.tools.is_empty() && !caps.tools {
        return Err(LlmError::Unsupported {
            provider: provider.name().to_owned(),
            capability: "tool calling",
        });
    }
    if request.output_schema.is_some() && caps.structured == Structured::None {
        return Err(LlmError::Unsupported {
            provider: provider.name().to_owned(),
            capability: "structured output",
        });
    }
    Ok(())
}
