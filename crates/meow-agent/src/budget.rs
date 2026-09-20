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
    /// What had been spent when this ledger was made.
    ///
    /// A child's budget is what its caller had left, which is a relative
    /// amount, and the counter it is checked against is cumulative and shared.
    /// Subtracting this is what puts the two in the same units. Without it a
    /// branch created after two steps had run compared its allowance of one
    /// against a counter already reading two and refused itself, so a fan-out
    /// spent less than its caller allowed and how much less depended on how
    /// the tasks happened to interleave.
    base: Base,
}

/// The counter's reading when a ledger was made.
#[derive(Debug, Clone, Copy, Default)]
struct Base {
    tokens: u32,
    steps: u32,
    cost_micros: u64,
    elapsed: std::time::Duration,
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
            base: Base::default(),
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
        // One lock, not two. The child's allowance and the point it measures
        // from must describe the same instant: taken separately, a step
        // charged in between makes the allowance larger than the base admits
        // and the child may take one step more than the caller had. That is
        // how six branches against a caller with three steps finished four.
        let Ok(s) = self.spend.lock() else {
            return Self {
                budget: asked,
                spend: Arc::clone(&self.spend),
                base: Base::default(),
            };
        };

        let base = Base {
            tokens: s.tokens,
            steps: s.steps,
            cost_micros: s.cost_micros,
            elapsed: s.started.elapsed(),
        };
        let left = self.left_from(&base);
        drop(s);

        Self {
            budget: asked.narrowed_to(left),
            spend: Arc::clone(&self.spend),
            base,
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
        // Each axis is measured from what the counter read when this ledger
        // was made, because that is the moment its budget was worked out.
        if let Some(limit) = self.budget.duration
            && s.started.elapsed().saturating_sub(self.base.elapsed) >= limit
        {
            return Err(Axis::Duration);
        }
        if let Some(limit) = self.budget.tokens
            && s.tokens.saturating_sub(self.base.tokens) >= limit
        {
            return Err(Axis::Tokens);
        }
        if let Some(limit) = self.budget.cost_micros
            && s.cost_micros.saturating_sub(self.base.cost_micros) >= limit
        {
            return Err(Axis::Cost);
        }
        if let Some(limit) = self.budget.steps
            && s.steps.saturating_sub(self.base.steps) >= limit
        {
            return Err(Axis::Steps);
        }
        s.steps += 1;
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
        // Measured from this ledger's own starting point, for the same reason
        // `reserve_step` is: the budget is relative and the counter is not.
        if let Some(limit) = self.budget.duration
            && s.started.elapsed().saturating_sub(self.base.elapsed) >= limit
        {
            return Some(Axis::Duration);
        }
        if let Some(limit) = self.budget.tokens
            && s.tokens.saturating_sub(self.base.tokens) >= limit
        {
            return Some(Axis::Tokens);
        }
        if let Some(limit) = self.budget.steps
            && s.steps.saturating_sub(self.base.steps) >= limit
        {
            return Some(Axis::Steps);
        }
        if let Some(limit) = self.budget.cost_micros
            && s.cost_micros.saturating_sub(self.base.cost_micros) >= limit
        {
            return Some(Axis::Cost);
        }
        None
    }

    /// How many steps this ledger has taken.
    ///
    /// Its own, not the shared total: a sub-agent numbering its steps from its
    /// caller's count would report a first step as step four.
    pub fn steps_taken(&self) -> u32 {
        self.spend
            .lock()
            .map(|s| s.steps.saturating_sub(self.base.steps))
            .unwrap_or(0)
    }

    /// What is left, as a budget a child can be narrowed against.
    /// What is left of this ledger's budget, given a reading of the counter.
    ///
    /// Takes the reading rather than taking the lock, so that a caller
    /// needing both this and the reading itself can get them from one lock
    /// and have them agree.
    fn left_from(&self, now: &Base) -> Budget {
        Budget {
            tokens: self
                .budget
                .tokens
                .map(|l| l.saturating_sub(now.tokens.saturating_sub(self.base.tokens))),
            steps: self
                .budget
                .steps
                .map(|l| l.saturating_sub(now.steps.saturating_sub(self.base.steps))),
            duration: self
                .budget
                .duration
                .map(|l| l.saturating_sub(now.elapsed.saturating_sub(self.base.elapsed))),
            cost_micros: self
                .budget
                .cost_micros
                .map(|l| l.saturating_sub(now.cost_micros.saturating_sub(self.base.cost_micros))),
        }
    }
}
