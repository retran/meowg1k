// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What goes to a model and what comes back.
//!
//! No type here names a vendor. That is `[R-LLM-003]`, and it is what lets the
//! engine be written once instead of once per provider.

use meow_core::Usage;

/// Who a message is from.
///
/// `[R-LLM-010]` fixes the set at four, and a message carries exactly one.
#[derive(Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Role {
    /// Instructions that frame the whole conversation.
    System,
    /// The user, or the script standing in for one.
    User,
    /// The model.
    Assistant,
    /// A tool answering a call.
    Tool,
}

/// A request from the model to run a tool.
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct ToolCall {
    /// Ties the call to the result that answers it.
    pub id: String,
    /// Which tool.
    pub name: String,
    /// The arguments, as JSON text.
    pub arguments: String,
}

/// One message.
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct Message {
    /// Who it is from.
    pub role: Role,
    /// What it says.
    pub content: String,
    /// The tools the model asked to run.
    ///
    /// `[R-LLM-011]`: an assistant message carries text and tool calls in the
    /// same message, because a model that explains itself before calling a
    /// tool produces both at once and splitting them loses the order.
    pub tool_calls: Vec<ToolCall>,
    /// Which call this message answers.
    ///
    /// `[R-LLM-012]` requires it on a tool result. Without it a provider
    /// cannot match an answer to its question when several ran at once.
    pub tool_call_id: Option<String>,
    /// The model's own reasoning, when it produced any.
    ///
    /// `[R-LLM-024]` keeps this on the message rather than discarding it after
    /// the stream, because a provider can require it back on a later turn that
    /// continues a tool call.
    pub thinking: Option<String>,
    /// A hint that the prefix up to here is worth caching.
    ///
    /// `[R-LLM-015]`: a provider with explicit cache breakpoints turns this
    /// into one, a provider that caches on its own ignores it, and either way
    /// the content of the message is unchanged.
    pub cache_hint: bool,
}

impl Message {
    /// A message with only text.
    pub fn new(role: Role, content: impl Into<String>) -> Self {
        Self {
            role,
            content: content.into(),
            tool_calls: Vec::new(),
            tool_call_id: None,
            thinking: None,
            cache_hint: false,
        }
    }

    /// A tool result, answering one call.
    pub fn tool_result(call_id: impl Into<String>, content: impl Into<String>) -> Self {
        Self {
            role: Role::Tool,
            content: content.into(),
            tool_calls: Vec::new(),
            tool_call_id: Some(call_id.into()),
            thinking: None,
            cache_hint: false,
        }
    }

    /// Mark the prefix up to this message as worth caching.
    #[must_use]
    pub fn with_cache_hint(mut self) -> Self {
        self.cache_hint = true;
        self
    }

    /// Attach the model's reasoning.
    #[must_use]
    pub fn with_thinking(mut self, thinking: impl Into<String>) -> Self {
        self.thinking = Some(thinking.into());
        self
    }
}

/// A tool offered to the model.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ToolDefinition {
    /// What the model calls it.
    pub name: String,
    /// What it does, in the model's terms.
    pub description: String,
    /// Its arguments, as JSON Schema.
    pub parameters: serde_json::Value,
}

/// What to send.
#[derive(Debug, Clone)]
pub struct Request {
    /// Which model.
    pub model: String,
    /// The conversation so far.
    pub messages: Vec<Message>,
    /// The tools the model may call.
    pub tools: Vec<ToolDefinition>,
    /// The most tokens to produce.
    pub max_output_tokens: u32,
    /// How much to explore, when the provider takes one.
    pub temperature: Option<f32>,
    /// A schema the answer must satisfy, per `[R-LLM-050]`.
    pub output_schema: Option<serde_json::Value>,
}

impl Request {
    /// A request with the required fields and nothing else.
    pub fn new(model: impl Into<String>, messages: Vec<Message>, max_output_tokens: u32) -> Self {
        Self {
            model: model.into(),
            messages,
            tools: Vec::new(),
            max_output_tokens,
            temperature: None,
            output_schema: None,
        }
    }
}

/// What came back.
#[derive(Debug, Clone, PartialEq)]
pub struct Response {
    /// The text the model produced.
    pub text: String,
    /// Its reasoning, when it produced any.
    pub thinking: Option<String>,
    /// The tools it asked to run.
    pub tool_calls: Vec<ToolCall>,
    /// What the call cost.
    ///
    /// `None` when the provider reported nothing at all, which `[R-LLM-041]`
    /// keeps distinct from reporting zeroes.
    pub usage: Option<Usage>,
    /// The parsed answer, when the request carried a schema.
    ///
    /// `[R-LLM-052]`: parsed, not a string the caller has to parse again.
    pub value: Option<serde_json::Value>,
}
