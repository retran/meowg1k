// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Domain types shared across the meowg1k workspace.
//!
//! This crate is the bottom of the dependency graph. It depends on no other
//! workspace crate and performs no input or output, which is what lets every
//! other crate name a `Session`, an `Event`, or a `Budget` without pulling in
//! the engine, the store, or the terminal.
//!
//! The types themselves arrive with the specifications that describe them, one
//! component at a time. See `docs/spec/README.md` for the method and
//! `docs/design/0.3.0-architecture.md` for the layering this crate sits at the
//! bottom of.

#[cfg(test)]
mod tests {
    /// The workspace lints, the pinned toolchain, and the test runner all have
    /// to agree before any real code lands on top of them. This test is what
    /// proves the scaffold runs at all.
    #[test]
    fn the_test_harness_runs() {
        assert_eq!(2 + 2, 4);
    }
}
