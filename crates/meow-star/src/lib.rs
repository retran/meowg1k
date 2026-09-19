// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The Starlark surface: what a user writes, and how it is loaded.
//!
//! This crate owns the `meow` global, the load scheme, the declarations a
//! workspace accumulates, and the thread a handler runs on. It depends on
//! `meow-agent` and never the other way round, which is what lets the engine
//! be tested without a script.

pub mod agent;
pub mod args;
pub mod context;
pub mod declare;
pub mod error;
pub mod loader;
pub mod markdown;
pub mod modules;
pub mod port;
pub mod registry;
pub mod run;
pub mod schema;
pub mod state;
pub mod value;
pub mod workspace;

pub use agent::{AgentDecl, Fields, Source};
pub use args::{Arg, Args};
pub use error::{Result, StarError};
pub use loader::{Loaded, load};
pub use port::{Ask, Events, Found, Search, Session, Stdin};
pub use registry::{Model, Origin, Provider, Registry, ToolDecl};
pub use run::{Handler, Ports, Runtime};
pub use schema::Violation;
pub use state::{Declaring, Phase};
pub use workspace::Workspace;
