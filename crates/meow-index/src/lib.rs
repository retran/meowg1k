// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Making a workspace searchable by meaning.
//!
//! This crate walks files, splits them into chunks, embeds the chunks, and
//! answers a query with ranked chunks. It does not decide what to do with a
//! result and does not call a generation model.
//!
//! `docs/spec/index.md` is normative.

pub mod chunk;
pub mod error;
pub mod walk;

pub use crate::chunk::{Chunk, Chunking};
pub use crate::error::{IndexError, Result};
pub use crate::walk::{Walk, Walked};
