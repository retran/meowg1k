// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What an evaluator carries with it.
//!
//! `Evaluator::extra` is the only channel from a builtin back to Rust, and it
//! hands out a shared reference, so the interior mutability here is the shape
//! the evaluator offers rather than a shortcut around a borrow.
//!
//! The module exists for its inner attribute. `ProvidesStaticType` is an
//! unsafe trait and its derive writes the `unsafe impl` as a sibling item, so
//! an `#[allow]` on a struct does not cover it. Scoping the exemption to one
//! module keeps the workspace-wide denial intact everywhere else, and there is
//! no unsafe block of ours in here to hide behind it.
#![allow(unsafe_code)]

use std::cell::{Cell, RefCell};
use std::sync::Arc;

use meow_agent::Ledger;
use serde_json::{Map, Value};
use starlark::any::ProvidesStaticType;

use crate::registry::{Origin, Registry};
use crate::run::Runtime;

/// Which half of a run is executing.
///
/// `[R-STAR-030]` and `[R-STAR-084]` both turn on this one distinction: a
/// declaration call is refused outside the declaration phase, and a runtime
/// module's builtins are refused inside it.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Phase {
    /// `.meow/` is being evaluated.
    Declaring,
    /// A handler is running.
    Running,
}

/// What the declaration files are building.
#[derive(Debug, ProvidesStaticType)]
pub struct Declaring {
    registry: RefCell<Registry>,
    file: RefCell<String>,
    phase: Cell<Phase>,
}

impl Default for Declaring {
    fn default() -> Self {
        Self::new()
    }
}

impl Declaring {
    /// Start with nothing declared.
    pub fn new() -> Self {
        Self {
            registry: RefCell::new(Registry::new()),
            file: RefCell::new(String::new()),
            phase: Cell::new(Phase::Declaring),
        }
    }

    /// Say which file is being evaluated, and get back the one it replaced.
    ///
    /// The caller restores the previous name, because a `load` evaluates a
    /// second file in the middle of the first and an origin recorded after
    /// that would otherwise name the wrong one.
    pub fn entering(&self, file: &str) -> String {
        self.file.replace(file.to_owned())
    }

    /// Which phase the evaluator is in.
    pub fn phase(&self) -> Phase {
        self.phase.get()
    }

    /// Take the registry out once loading is done.
    pub fn finish(self) -> Registry {
        self.registry.into_inner()
    }

    /// Borrow what has been declared so far.
    pub fn registry(&self) -> std::cell::Ref<'_, Registry> {
        self.registry.borrow()
    }

    /// Borrow it to add to it.
    pub(crate) fn registry_mut(&self) -> std::cell::RefMut<'_, Registry> {
        self.registry.borrow_mut()
    }

    /// Which file a declaration made now belongs to.
    pub(crate) fn origin(&self) -> Origin {
        Origin(self.file.borrow().clone())
    }
}

/// What a handler can reach while it runs.
#[derive(Debug, ProvidesStaticType)]
pub struct Running {
    /// The engine, the registry, and the ports.
    pub runtime: Arc<Runtime>,
    /// This invocation's arguments, already checked against their declaration.
    pub args: Map<String, Value>,
    /// What is left of the budget.
    pub ledger: Ledger,
    /// How deep in a chain of sub-agents this handler is.
    pub depth: u32,
}
