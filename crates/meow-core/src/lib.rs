// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Domain types shared across the meowg1k workspace.
//!
//! This crate is the bottom of the dependency graph. It depends on no other
//! workspace crate and performs no input or output, which is what lets every
//! other crate name a `Session`, an `Event`, or a `Usage` without pulling in
//! the engine, the store, or the terminal.
//!
//! Types arrive here when a second crate needs them, not before.

mod event;
mod id;
mod usage;
pub mod view;

pub use crate::event::{EventKind, StopReason};
pub use crate::id::SessionId;
pub use crate::usage::Usage;
pub use crate::view::{LiveKind, Output, SCHEMA_VERSION, ViewEvent};
