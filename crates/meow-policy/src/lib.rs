// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The permission boundary.
//!
//! Every tool call passes through here before it runs. A rule matches on a
//! tool name and, where it narrows, on the paths, the command line, or the
//! host; the answer is allow, ask, or deny, and a call that matches nothing is
//! denied.
//!
//! This lives in the runtime rather than in a Starlark library because a
//! library cannot be trusted by the thing it constrains. It is also the
//! largest single addition over v0.2.x, which has no permission layer at all:
//! `shell_exec` is an ordinary tool there, so an agent that is talked into
//! running a command runs it.
//!
//! `docs/spec/policy.md` is normative.

mod error;
mod policy;
mod prompt;
mod rule;

pub use crate::error::{PolicyError, Result};
pub use crate::policy::{
    Access, Call, Decision, Grants, Narrowed, Policy, REDACTED, Source, Verdict, redact, resolve,
};
pub use crate::prompt::{Explanation, Prompt, Timeout, explain};
pub use crate::rule::{NamePattern, Rule, Selector, SelectorKind};
