// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What an agent is.

use serde_json::Value;

use crate::budget::Budget;
use crate::tool::ToolSet;

/// What to do when a tool fails.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum ToolErrorPolicy {
    /// Tell the model and carry on, so it can try something else.
    #[default]
    Report,
    /// Stop the run.
    Abort,
}

/// An agent, declared.
#[derive(Debug)]
pub struct AgentSpec {
    /// What to call it.
    pub name: String,
    /// Which model.
    pub model: String,
    /// How it is framed.
    pub system: Option<String>,
    /// What it may call.
    pub tools: ToolSet,
    /// What bounds it.
    pub budget: Budget,
    /// A schema the answer must satisfy.
    pub output: Option<Value>,
    /// What to do when a tool fails.
    pub on_tool_error: ToolErrorPolicy,
    /// The most tokens one model call may produce.
    pub max_output_tokens: u32,
}

impl AgentSpec {
    /// An agent with the required fields and defaults for the rest.
    pub fn new(name: impl Into<String>, model: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            model: model.into(),
            system: None,
            tools: ToolSet::new(),
            budget: Budget::default(),
            output: None,
            on_tool_error: ToolErrorPolicy::default(),
            max_output_tokens: 4096,
        }
    }

    /// Frame it.
    #[must_use]
    pub fn with_system(mut self, system: impl Into<String>) -> Self {
        self.system = Some(system.into());
        self
    }

    /// Give it tools.
    #[must_use]
    pub fn with_tools(mut self, tools: ToolSet) -> Self {
        self.tools = tools;
        self
    }

    /// Bound it.
    #[must_use]
    pub fn with_budget(mut self, budget: Budget) -> Self {
        self.budget = budget;
        self
    }

    /// Require a shape.
    #[must_use]
    pub fn with_output(mut self, schema: Value) -> Self {
        self.output = Some(schema);
        self
    }

    /// Decide what a failing tool does.
    #[must_use]
    pub fn on_tool_error(mut self, policy: ToolErrorPolicy) -> Self {
        self.on_tool_error = policy;
        self
    }
}
