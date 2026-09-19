// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The Starlark surface: what a user writes, and how it is loaded.
//!
//! This crate owns the `meow` global, the load scheme, and the declarations a
//! workspace accumulates. It depends on `meow-agent` and never the other way
//! round, which is what lets the engine be tested without a script.

pub mod args;
pub mod declare;
pub mod error;
pub mod loader;
pub mod registry;
pub mod schema;
pub mod workspace;

pub use args::{Arg, Args};
pub use declare::{Declaring, Phase};
pub use error::{Result, StarError};
pub use loader::{Loaded, load};
pub use registry::{AgentDecl, Model, Origin, Provider, Registry, ToolDecl};
pub use schema::Violation;
pub use workspace::Workspace;
