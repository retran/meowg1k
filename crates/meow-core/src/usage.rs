// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What a model call cost.

/// Tokens and money for one model call.
///
/// `[R-SESSION-020]` requires these as separate typed fields rather than the
/// metadata strings v0.2.x wrote, which could not be summed without parsing a
/// key and then a value.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, serde::Serialize, serde::Deserialize)]
pub struct Usage {
    /// Tokens sent.
    pub prompt: u32,
    /// Tokens received.
    pub completion: u32,
    /// Prompt tokens the provider served from its cache.
    ///
    /// `None` when the provider does not report caching, which
    /// `[R-LLM-040]` keeps distinct from a reported zero: "no cache hits" and
    /// "this provider does not say" are different facts, and prompt caching is
    /// the largest cost lever a long agent run has.
    pub cached: Option<u32>,
    /// What the call cost, in millionths of a unit of currency.
    ///
    /// `None` when the model has no price in the table. `[R-SESSION-021]`
    /// forbids recording that as zero, because an unpriced model would then
    /// read as a free one. Integer micros rather than a float, so that summing
    /// a thousand events does not drift.
    pub cost_micros: Option<u64>,
}

impl Usage {
    /// Tokens sent plus tokens received.
    pub fn total(&self) -> u32 {
        self.prompt.saturating_add(self.completion)
    }

    /// Add another call's usage to this one.
    ///
    /// A cost is known only when both sides are: adding a priced call to an
    /// unpriced one gives an unknown total, because the alternative is
    /// reporting a number that silently omits part of what was spent.
    pub fn add(&self, other: &Self) -> Self {
        Self {
            prompt: self.prompt.saturating_add(other.prompt),
            completion: self.completion.saturating_add(other.completion),
            cached: match (self.cached, other.cached) {
                (Some(a), Some(b)) => Some(a.saturating_add(b)),
                (Some(a), None) => Some(a),
                (None, Some(b)) => Some(b),
                (None, None) => None,
            },
            cost_micros: match (self.cost_micros, other.cost_micros) {
                (Some(a), Some(b)) => Some(a.saturating_add(b)),
                _ => None,
            },
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn an_unpriced_call_makes_the_total_unknown() {
        let priced = Usage {
            prompt: 10,
            completion: 5,
            cached: Some(2),
            cost_micros: Some(100),
        };
        let unpriced = Usage {
            prompt: 1,
            completion: 1,
            cached: None,
            cost_micros: None,
        };
        let sum = priced.add(&unpriced);
        assert_eq!(sum.prompt, 11);
        assert_eq!(sum.cached, Some(2));
        assert_eq!(
            sum.cost_micros, None,
            "a total that omits a cost must not look complete"
        );
    }
}
