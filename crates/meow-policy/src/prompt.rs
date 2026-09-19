// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What an approval must say, and what `policy explain` answers.
//!
//! The drawing is `meow-ui`'s, in M8. What has to be shown is decided here,
//! because a prompt that paraphrases what it is approving is worse than no
//! prompt, and that is a property of the decision rather than of the frame
//! around it.

use crate::policy::{Call, Decision, Grants, Policy, Verdict, redact};

/// How long an approval waits.
///
/// `[R-POLICY-024]`: indefinitely by default. A prompt that expires while you
/// are reading the command it is asking about turns a security decision into a
/// reflex. A timeout stays configurable for an unattended terminal that is
/// nevertheless a terminal.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct Timeout(pub Option<std::time::Duration>);

impl Timeout {
    /// What a prompt with no answer resolves to.
    ///
    /// Only reachable when a timeout was configured and expired; without one
    /// there is nothing to resolve, because the prompt is still waiting.
    pub fn on_expiry(self) -> Option<Decision> {
        self.0.map(|_| Decision::Deny)
    }
}

/// Everything an approval prompt must show.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Prompt {
    /// Which tool.
    pub tool: String,
    /// The arguments, verbatim except for what was marked sensitive.
    ///
    /// `[R-POLICY-021]`: verbatim, never a summary. An approval prompt that
    /// paraphrases what it is approving is worse than no prompt, because it
    /// asks for consent to something the reader did not see.
    pub arguments: String,
    /// The rule that caused the question.
    ///
    /// `[R-POLICY-022]`: so the user learns why they are being asked and can
    /// narrow the policy afterwards, instead of answering the same question
    /// every run.
    pub rule: String,
    /// Where that rule was written.
    pub origin: Option<String>,
    /// Which agent asked, and at which step.
    pub agent: String,
    /// The step number.
    pub step: u32,
}

impl Prompt {
    /// Build the prompt for a call the policy wants asked about.
    pub fn new(
        call: &Call,
        args: &serde_json::Value,
        verdict: &Verdict,
        sensitive: &[String],
        agent: &str,
        step: u32,
    ) -> Self {
        Self {
            tool: call.tool.clone(),
            arguments: redact(args, sensitive).to_string(),
            rule: verdict
                .rule
                .clone()
                .unwrap_or_else(|| "no rule matched".to_owned()),
            origin: verdict.origin.clone(),
            agent: agent.to_owned(),
            step,
        }
    }
}

/// What `meow policy explain` answers.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Explanation {
    /// What a real call would be told by the rules.
    pub decision: Decision,
    /// Which rule decides.
    pub rule: Option<String>,
    /// Where it was written.
    pub origin: Option<String>,
    /// How many higher-precedence rules were checked without matching.
    pub checked: usize,
}

/// Predict what the rules say about a call, without making it.
///
/// Satisfies `[R-POLICY-050]`: the *rule* decision, not the final outcome. It
/// does not claim to predict how an `ask` would be answered, because that
/// depends on a person and on grants made during a run that has not happened.
/// A permission system whose decisions cannot be queried without triggering
/// them is one people turn off.
pub fn explain(policy: &Policy, call: &Call) -> Explanation {
    // Deliberately with no grants: a grant is a fact about a run in progress,
    // and this is a question asked before one.
    let v = policy.evaluate(call, &Grants::new());
    Explanation {
        decision: v.decision,
        rule: v.rule,
        origin: v.origin,
        checked: v.checked,
    }
}
