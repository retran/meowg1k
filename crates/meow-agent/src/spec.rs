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
    /// What this agent may do.
    ///
    /// `None` permits everything, which is only right for a test. A real
    /// agent is given one, and `[R-POLICY-011]` makes an empty policy deny
    /// rather than allow, so forgetting to grant something fails closed.
    pub policy: Option<meow_policy::Policy>,
    /// Grants a person made during this process.
    pub grants: meow_policy::Grants,
    /// How a tool call turns into something the policy can judge.
    ///
    /// The engine does not know which arguments are paths or which is a
    /// command line; the tool layer does. `[R-POLICY-003]` also requires paths
    /// to be resolved before evaluation, and resolving is the caller's job.
    pub describe_call: Option<DescribeCall>,
}

/// Turns a tool name and its arguments into a resolved call.
pub type DescribeCall = std::sync::Arc<dyn Fn(&str, &Value) -> meow_policy::Call + Send + Sync>;

impl std::fmt::Debug for AgentSpec {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("AgentSpec")
            .field("name", &self.name)
            .field("model", &self.model)
            .field("tools", &self.tools)
            .field("budget", &self.budget)
            .field("on_tool_error", &self.on_tool_error)
            .field("policy", &self.policy)
            .finish_non_exhaustive()
    }
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
            policy: None,
            grants: meow_policy::Grants::new(),
            describe_call: None,
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

    /// Constrain it.
    #[must_use]
    pub fn with_policy(mut self, policy: meow_policy::Policy, describe: DescribeCall) -> Self {
        self.policy = Some(policy);
        self.describe_call = Some(describe);
        self
    }

    /// Decide what a failing tool does.
    #[must_use]
    pub fn on_tool_error(mut self, policy: ToolErrorPolicy) -> Self {
        self.on_tool_error = policy;
        self
    }
}
