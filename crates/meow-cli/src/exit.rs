// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What the process exits with.
//!
//! v0.2.x had success and failure and nothing else, so an agent could not act
//! as a gate in a shell script. Here every way a run can end has its own code,
//! and no code means two things.

use meow_core::StopReason;

/// How a run ended, as far as the process is concerned.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Ending {
    /// A run finished and the handler returned true or nothing.
    Passed,
    /// A run finished and the handler returned false.
    ///
    /// This is the gate: `meow review --strict && git push` turns on it.
    Failed,
    /// A run stopped for a reason of its own.
    Stopped(StopReason),
    /// The command line was wrong: an unknown command, a bad flag, a bad
    /// argument.
    Usage,
    /// `.meow/` would not load.
    Config,
    /// A provider or a credential failed.
    Provider,
}

/// The exit code for an ending.
///
/// Satisfies `[R-TUI-080]` by following its table exactly, and `[R-TUI-081]`
/// because the match is total and no arm repeats a number. A test walks all
/// six stop reasons and checks the codes are distinct, which is what a shell
/// branching on `$?` depends on.
pub fn code(ending: Ending) -> u8 {
    match ending {
        Ending::Passed => 0,
        Ending::Failed => 1,
        Ending::Usage => 2,
        Ending::Stopped(StopReason::Budget) => 3,
        Ending::Stopped(StopReason::Cancelled) => 4,
        Ending::Stopped(StopReason::Denied) => 5,
        Ending::Provider => 6,
        Ending::Config => 7,
        Ending::Stopped(StopReason::ToolAborted) => 8,
        Ending::Stopped(StopReason::Failed) => 9,
        // A finished run is `Passed` or `Failed`, decided by what the handler
        // returned. Reaching here means nobody asked the handler, so the
        // honest answer is that the run finished.
        Ending::Stopped(StopReason::Finished) => 0,
    }
}

/// How a run that finished ended, given what its handler returned.
///
/// A handler that returns nothing has not failed. `[R-TUI-080]` distinguishes
/// only `false` from everything else, because a handler returning a string is
/// reporting a result rather than a verdict.
pub fn finished(returned: Option<bool>) -> Ending {
    match returned {
        Some(false) => Ending::Failed,
        _ => Ending::Passed,
    }
}

/// Turn an outcome into an ending.
pub fn of(outcome: &meow_agent::Outcome, returned: Option<bool>) -> Ending {
    match outcome.stop {
        StopReason::Finished => finished(returned),
        other => Ending::Stopped(other),
    }
}

#[cfg(test)]
mod tests {
    use super::{Ending, code};
    use meow_core::StopReason;

    /// Every stop reason there is, so adding a seventh breaks this test rather
    /// than silently sharing a code with an existing one.
    const REASONS: [StopReason; 6] = [
        StopReason::Finished,
        StopReason::Budget,
        StopReason::Cancelled,
        StopReason::Denied,
        StopReason::ToolAborted,
        StopReason::Failed,
    ];

    /// [R-TUI-081] no code means two things, so a shell can branch on one
    #[test]
    fn every_stop_reason_has_a_code_of_its_own() {
        let mut seen: Vec<(u8, StopReason)> = Vec::new();

        for reason in REASONS {
            let this = code(Ending::Stopped(reason));
            // `finished` is the one reason that shares its code with an
            // ending: a run that finished and a handler that passed are the
            // same success, which is what makes `meow review && git push`
            // read the way it does.
            if reason == StopReason::Finished {
                assert_eq!(this, code(Ending::Passed));
                continue;
            }
            if let Some((_, other)) = seen.iter().find(|(c, _)| *c == this) {
                panic!("{reason} and {other} both exit {this}");
            }
            seen.push((this, reason));
        }
    }

    /// [R-TUI-080] the table, code by code
    #[test]
    fn the_codes_are_the_ones_the_table_names() {
        assert_eq!(code(Ending::Passed), 0);
        assert_eq!(code(Ending::Failed), 1);
        assert_eq!(code(Ending::Usage), 2);
        assert_eq!(code(Ending::Stopped(StopReason::Budget)), 3);
        assert_eq!(code(Ending::Stopped(StopReason::Cancelled)), 4);
        assert_eq!(code(Ending::Stopped(StopReason::Denied)), 5);
        assert_eq!(code(Ending::Provider), 6);
        assert_eq!(code(Ending::Config), 7);
        assert_eq!(code(Ending::Stopped(StopReason::ToolAborted)), 8);
        assert_eq!(code(Ending::Stopped(StopReason::Failed)), 9);
    }

    /// [R-TUI-080] only an explicit false is a failure
    #[test]
    fn a_handler_that_returns_nothing_has_not_failed() {
        assert_eq!(super::finished(None), Ending::Passed);
        assert_eq!(super::finished(Some(true)), Ending::Passed);
        assert_eq!(super::finished(Some(false)), Ending::Failed);
    }
}
