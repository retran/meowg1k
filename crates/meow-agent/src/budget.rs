// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What bounds a run.

use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use meow_core::Usage;

/// The four axes a run is bounded on.
///
/// `[R-AGENT-010]`. An axis left unset is unbounded, per `[R-AGENT-012]`, and
/// a spec with no budget at all takes [`Budget::default`] rather than running
/// free.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Budget {
    /// Prompt plus completion tokens across the run.
    pub tokens: Option<u32>,
    /// How many times the model may be asked.
    pub steps: Option<u32>,
    /// Wall clock.
    pub duration: Option<Duration>,
    /// Estimated spend, in millionths.
    pub cost_micros: Option<u64>,
}

impl Default for Budget {
    /// `[R-AGENT-016]`: 200,000 tokens, 40 steps, 30 minutes, cost unbounded.
    ///
    /// Tokens and steps are what actually bound spend. Wall clock bounds
    /// patience, and it is the axis that misfires: a slow provider or a long
    /// tool makes it stop a run that is working. Thirty minutes still catches
    /// a wedged run without punishing a slow one.
    ///
    /// Cost stays unbounded because capping it means estimating the price of a
    /// call before making it, and the token cap is the same guard with fewer
    /// moving parts.
    fn default() -> Self {
        Self {
            tokens: Some(200_000),
            steps: Some(40),
            duration: Some(Duration::from_secs(30 * 60)),
            cost_micros: None,
        }
    }
}

impl Budget {
    /// A budget with nothing set.
    pub fn unbounded() -> Self {
        Self {
            tokens: None,
            steps: None,
            duration: None,
            cost_micros: None,
        }
    }

    /// The tighter of two bounds on every axis.
    fn narrowed_to(self, cap: Self) -> Self {
        fn tighter<T: Ord>(a: Option<T>, b: Option<T>) -> Option<T> {
            match (a, b) {
                (Some(a), Some(b)) => Some(a.min(b)),
                (Some(a), None) => Some(a),
                (None, b) => b,
            }
        }
        Self {
            tokens: tighter(self.tokens, cap.tokens),
            steps: tighter(self.steps, cap.steps),
            duration: tighter(self.duration, cap.duration),
            cost_micros: tighter(self.cost_micros, cap.cost_micros),
        }
    }
}

/// Which axis stopped a run.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Axis {
    /// Tokens.
    Tokens,
    /// Steps.
    Steps,
    /// Wall clock.
    Duration,
    /// Estimated cost.
    Cost,
}

impl Axis {
    /// How it reads in an outcome's detail.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Tokens => "tokens",
            Self::Steps => "steps",
            Self::Duration => "duration",
            Self::Cost => "cost",
        }
    }
}

#[derive(Debug)]
struct Spend {
    tokens: u32,
    steps: u32,
    cost_micros: u64,
    started: Instant,
}

/// What a run and its descendants have spent, shared.
///
/// Satisfies `[R-AGENT-013]`: a child's spend reaches its caller because both
/// hold the same ledger. Satisfies `[R-AGENT-017]`: a step is reserved before
/// the call and reconciled after, so three concurrent invocations cannot each
/// observe the same remaining amount and each spend it.
#[derive(Debug, Clone)]
pub struct Ledger {
    budget: Budget,
    spend: Arc<Mutex<Spend>>,
}

impl Ledger {
    /// A ledger for a top-level run.
    pub fn new(budget: Budget) -> Self {
        Self {
            budget,
            spend: Arc::new(Mutex::new(Spend {
                tokens: 0,
                steps: 0,
                cost_micros: 0,
                started: Instant::now(),
            })),
        }
    }

    /// A ledger for a run inside this one.
    ///
    /// Satisfies `[R-AGENT-014]`: the child's budget is the tighter of what it
    /// asked for and what the caller has left, so a fan-out cannot exceed the
    /// top-level cap however generous each child's own declaration is. The
    /// spend is the same allocation, which is what makes `[R-AGENT-013]`
    /// transitive rather than one level deep.
    pub fn child(&self, asked: Budget) -> Self {
        Self {
            budget: asked.narrowed_to(self.remaining_as_budget()),
            spend: Arc::clone(&self.spend),
        }
    }

    /// Take one step, if there is one to take.
    ///
    /// Reserves before the call rather than checking before and charging
    /// after. Returns the axis that refused, when one did.
    pub fn reserve_step(&self) -> Result<(), Axis> {
        let Ok(mut s) = self.spend.lock() else {
            return Err(Axis::Steps);
        };
        if let Some(limit) = self.budget.duration
            && s.started.elapsed() >= limit
        {
            return Err(Axis::Duration);
        }
        if let Some(limit) = self.budget.tokens
            && s.tokens >= limit
        {
            return Err(Axis::Tokens);
        }
        if let Some(limit) = self.budget.cost_micros
            && s.cost_micros >= limit
        {
            return Err(Axis::Cost);
        }
        if let Some(limit) = self.budget.steps {
            if s.steps >= limit {
                return Err(Axis::Steps);
            }
            s.steps += 1;
        } else {
            s.steps += 1;
        }
        Ok(())
    }

    /// Record what a call actually cost.
    pub fn charge(&self, usage: &Usage) {
        if let Ok(mut s) = self.spend.lock() {
            s.tokens = s.tokens.saturating_add(usage.total());
            s.cost_micros = s.cost_micros.saturating_add(usage.cost_micros.unwrap_or(0));
        }
    }

    /// Whether any axis is now spent, and which.
    pub fn exceeded(&self) -> Option<Axis> {
        let s = self.spend.lock().ok()?;
        if let Some(limit) = self.budget.duration
            && s.started.elapsed() >= limit
        {
            return Some(Axis::Duration);
        }
        if let Some(limit) = self.budget.tokens
            && s.tokens >= limit
        {
            return Some(Axis::Tokens);
        }
        if let Some(limit) = self.budget.steps
            && s.steps >= limit
        {
            return Some(Axis::Steps);
        }
        if let Some(limit) = self.budget.cost_micros
            && s.cost_micros >= limit
        {
            return Some(Axis::Cost);
        }
        None
    }

    /// How many steps have been taken.
    pub fn steps_taken(&self) -> u32 {
        self.spend.lock().map(|s| s.steps).unwrap_or(0)
    }

    /// What is left, as a budget a child can be narrowed against.
    fn remaining_as_budget(&self) -> Budget {
        let Ok(s) = self.spend.lock() else {
            return Budget::unbounded();
        };
        Budget {
            tokens: self.budget.tokens.map(|l| l.saturating_sub(s.tokens)),
            steps: self.budget.steps.map(|l| l.saturating_sub(s.steps)),
            duration: self
                .budget
                .duration
                .map(|l| l.saturating_sub(s.started.elapsed())),
            cost_micros: self
                .budget
                .cost_micros
                .map(|l| l.saturating_sub(s.cost_micros)),
        }
    }
}
