// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What arrives while a model is still answering.

use meow_core::Usage;

use crate::error::{LlmError, Result};
use crate::message::{Response, ToolCall};

/// One thing that happened during a streaming call.
///
/// `[R-LLM-020]` fixes the set at eight. A closed set is what lets a renderer
/// and a script handle the stream without either guessing what a ninth kind
/// might mean.
#[derive(Debug, Clone, PartialEq)]
pub enum StreamEvent {
    /// More of the answer.
    Text(String),
    /// More of the model's reasoning.
    Thinking(String),
    /// The model began asking for a tool.
    ToolCallStart {
        /// Ties this to its end and its result.
        id: String,
        /// Which tool.
        name: String,
    },
    /// More of a tool call's arguments.
    ToolCallDelta {
        /// Which call.
        id: String,
        /// More argument text.
        arguments: String,
    },
    /// A tool call is complete.
    ToolCallEnd {
        /// Which call.
        id: String,
    },
    /// What the call cost.
    Usage(Usage),
    /// Something went wrong mid-stream.
    Error(String),
    /// The model has finished.
    Done,
}

/// Collects a stream back into the response it describes.
///
/// `[R-LLM-021]` is stated over a recording rather than over two live calls,
/// which is what makes it checkable: the same events must always produce the
/// same value, and a test can supply the events.
#[derive(Debug, Default)]
pub struct Aggregator {
    text: String,
    thinking: String,
    calls: Vec<ToolCall>,
    usage: Option<Usage>,
    error: Option<String>,
    done: bool,
}

impl Aggregator {
    /// A fresh aggregator.
    pub fn new() -> Self {
        Self::default()
    }

    /// Fold one event in.
    pub fn push(&mut self, event: StreamEvent) {
        match event {
            StreamEvent::Text(t) => self.text.push_str(&t),
            StreamEvent::Thinking(t) => self.thinking.push_str(&t),
            StreamEvent::ToolCallStart { id, name } => {
                // Deduplicate here as well as at the provider boundary. A
                // provider that repeats a start with the same identifier must
                // not produce two calls, which is the v0.2.x defect that only
                // showed when sessions were off.
                if !self.calls.iter().any(|c| c.id == id) {
                    self.calls.push(ToolCall {
                        id,
                        name,
                        arguments: String::new(),
                    });
                }
            }
            StreamEvent::ToolCallDelta { id, arguments } => {
                if let Some(c) = self.calls.iter_mut().find(|c| c.id == id) {
                    c.arguments.push_str(&arguments);
                }
            }
            StreamEvent::ToolCallEnd { .. } => {}
            StreamEvent::Usage(u) => self.usage = Some(u),
            StreamEvent::Error(e) => self.error = Some(e),
            StreamEvent::Done => self.done = true,
        }
    }

    /// Whether the stream said it was finished.
    pub fn is_done(&self) -> bool {
        self.done
    }

    /// The response the events describe.
    pub fn finish(self, provider: &str) -> Result<Response> {
        if let Some(message) = self.error {
            return Err(LlmError::Transport {
                provider: provider.to_owned(),
                message,
            });
        }
        Ok(Response {
            text: self.text,
            thinking: if self.thinking.is_empty() {
                None
            } else {
                Some(self.thinking)
            },
            tool_calls: self.calls,
            usage: self.usage,
            value: None,
        })
    }
}

/// Where a streaming call sends what it receives.
///
/// An error from the sink aborts the request and reaches the caller unchanged,
/// per `[R-LLM-023]`: a consumer that cannot keep up, or that has decided to
/// stop, is not something to paper over.
pub trait Sink: Send {
    /// Take one event.
    ///
    /// # Errors
    ///
    /// Whatever the consumer decides. The error is not interpreted.
    fn event(&mut self, event: StreamEvent) -> Result<()>;
}

impl<F> Sink for F
where
    F: FnMut(StreamEvent) -> Result<()> + Send,
{
    fn event(&mut self, event: StreamEvent) -> Result<()> {
        self(event)
    }
}

/// A sink that keeps everything, for a caller that only wants the total.
#[derive(Debug, Default)]
pub struct Collect(pub Vec<StreamEvent>);

impl Sink for Collect {
    fn event(&mut self, event: StreamEvent) -> Result<()> {
        self.0.push(event);
        Ok(())
    }
}
