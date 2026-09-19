// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Making a long run fit.

use meow_llm::{Message, Role};

/// When to compact, how much to keep, and who summarises.
#[derive(Debug, Clone, PartialEq)]
pub struct Compaction {
    /// The fraction of the context window at which to act.
    pub at: f32,
    /// How many of the most recent messages stay verbatim.
    ///
    /// `[R-AGENT-041]`. The recent ones are what the model is reasoning about
    /// right now; summarising those is what makes a compacted agent lose the
    /// thread.
    pub keep_recent: usize,
    /// Which model summarises.
    ///
    /// `[R-AGENT-045]`. Summarising is a cheap task, and the first draft ran
    /// it on the agent's expensive model at the worst possible moment of every
    /// long run. `None` falls back to the agent's own.
    pub model: Option<String>,
}

impl Default for Compaction {
    fn default() -> Self {
        Self {
            at: 0.8,
            keep_recent: 10,
            model: None,
        }
    }
}

/// Roughly how many tokens a message list is.
///
/// Four characters to a token, which is close enough for a threshold and
/// wrong enough that nothing should bill from it. A real tokenizer is a
/// dependency this does not need to decide when to summarise.
pub fn estimate_tokens(messages: &[Message]) -> u32 {
    let chars: usize = messages
        .iter()
        .map(|m| m.content.len() + m.thinking.as_ref().map_or(0, String::len))
        .sum();
    (chars / 4) as u32
}

/// Which messages a compaction would supersede.
///
/// Returns `None` when there is nothing worth doing: the system message stays,
/// the most recent stay, and if that leaves fewer than two to summarise the
/// call would cost more than it saves.
pub fn range_to_compact(
    messages: &[Message],
    policy: &Compaction,
    context_window: u32,
) -> Option<std::ops::Range<usize>> {
    let limit = (context_window as f32 * policy.at) as u32;
    if estimate_tokens(messages) < limit {
        return None;
    }
    let first = usize::from(messages.first().is_some_and(|m| m.role == Role::System));
    let last = messages.len().saturating_sub(policy.keep_recent);
    (last > first + 1).then_some(first..last)
}
