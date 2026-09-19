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

/// Who answers an approval prompt.
///
/// `[R-POLICY-024]`: it waits. A prompt that expires while you are reading the
/// command it is asking about turns a security decision into a reflex, so
/// there is no timeout here and the implementation decides whether to have
/// one.
///
/// The engine calls this and does nothing else with the answer: an "always"
/// is remembered by whoever implements this, which is what keeps
/// `[R-POLICY-023]` true without a grant travelling back into a file.
pub trait Approver: Send + Sync + std::fmt::Debug {
    /// Ask, and wait.
    fn ask(&self, prompt: &meow_policy::Prompt) -> meow_policy::Answer;
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
    /// When and how to summarise a long run.
    pub compaction: crate::compaction::Compaction,
    /// How many tokens the model can hold at once.
    pub context_window: u32,
    /// How deep a chain of sub-agents may go.
    ///
    /// `[R-AGENT-052]`.
    pub max_depth: u32,
    /// What this agent may do.
    ///
    /// `None` permits everything, which is only right for a test. A real
    /// agent is given one, and `[R-POLICY-011]` makes an empty policy deny
    /// rather than allow, so forgetting to grant something fails closed.
    pub policy: Option<meow_policy::Policy>,
    /// Grants a person made during this process.
    pub grants: meow_policy::Grants,
    /// Who answers when the policy says to ask.
    ///
    /// `None` resolves every `ask` to `deny`, which is `[R-POLICY-020]`: an
    /// unattended run must not be able to approve itself.
    pub approve: Option<std::sync::Arc<dyn Approver>>,
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
            compaction: crate::compaction::Compaction::default(),
            context_window: 200_000,
            max_depth: 4,
            policy: None,
            grants: meow_policy::Grants::new(),
            approve: None,
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

    /// Say when and how to summarise.
    #[must_use]
    pub fn with_compaction(mut self, compaction: crate::compaction::Compaction) -> Self {
        self.compaction = compaction;
        self
    }

    /// Say how much the model can hold.
    #[must_use]
    pub fn with_context_window(mut self, tokens: u32) -> Self {
        self.context_window = tokens;
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
