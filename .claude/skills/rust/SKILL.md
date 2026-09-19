---
name: rust
description: Rust conventions for the meowg1k v0.3.0 workspace - crate boundaries, error handling with thiserror and miette, async and the Starlark threading bridge, tracing instead of println, testing with nextest and insta, and the lints this repository enforces. Load before writing or reviewing Rust code here.
---

# Rust in this workspace

`docs/design/0.3.0-architecture.md` decides the crate layout and the execution
model. This skill covers how to write code inside those decisions. When the two
disagree, the design document wins.

## Crate boundaries

Three boundaries carry the whole design, and a change that crosses one is a
defect however small it looks:

- `meow-core` depends on no other workspace crate and performs no input or
  output. It holds types.
- `meow-agent` does not know Starlark exists. `meow-star` depends on
  `meow-agent`, never the reverse. This is what lets the engine be tested
  without a script, and what would let a second frontend drive it.
- `meow-ui` depends on `meow-core` for event types and on nothing else in the
  workspace. It does not know the engine exists.

`v0.2.x` broke the equivalent boundary: `internal/core/starlark` imports
`internal/adapters/gateway` directly. Do not reproduce it.

Keep the layering checkable. `cargo deny` and a workspace lint enforce it, so a
violation fails CI instead of being found in review.

## Errors

Libraries return `thiserror` enums. One enum per crate, variants named for what
went wrong rather than for where it happened, and every variant carries enough
to act on:

```rust
#[derive(Debug, thiserror::Error)]
pub enum EngineError {
    #[error("budget exhausted after {steps} steps ({tokens} tokens)")]
    BudgetExhausted { steps: u32, tokens: u32 },

    #[error("tool {name} is not permitted by policy rule {rule}")]
    PolicyDenied { name: String, rule: RuleId },
}
```

`meow-cli` converts to `miette` at the boundary and renders with source spans.
This is how a Starlark mistake becomes a diagnostic that points at the line
instead of a string that describes it.

Never use `unwrap` or `expect` outside tests and `build.rs`. When an invariant
truly cannot fail, write the reason:

```rust
// The registry inserted this id one line above, so the lookup cannot miss.
let tool = registry.get(&id).expect("id was just inserted");
```

Never swallow an error. `v0.2.x` logs ten session write failures with
`log.Printf` and continues, which produces a session log that is silently
wrong. If an operation can fail and the caller needs to know, return the error.

## Async and the Starlark bridge

`tokio` for everything asynchronous. The engine is `Send` and async; the
Starlark evaluator is neither.

Each script invocation runs on its own blocking thread with its own
`Evaluator`. Builtins that perform input or output are thin synchronous
wrappers that call the async engine through a `tokio::runtime::Handle`. Never
hold a `starlark::Value` across an `.await`; it cannot cross the boundary and
the compiler will tell you so in an error worth reading carefully.

Cancellation is a `CancellationToken` threaded through every engine call. Any
operation that can run longer than a few milliseconds takes one and checks it.
An agent that cannot be stopped with Ctrl-C is a defect, not a limitation.

## Diagnostics

`tracing` everywhere. `println!` and `eprintln!` are denied by lint outside
`meow-cli`, because they write straight through the terminal frame and corrupt
it.

Instrument at the boundary, not inside loops. A span per agent step and per
tool call is useful; a span per iteration of an inner loop is noise that costs
more than it tells you.

## Tests

`cargo nextest run` is the runner. Unit tests live beside the code in
`mod tests`; integration tests live in `tests/`.

Every test that checks a specification requirement names it:

```rust
/// [R-SESSION-014] compaction supersedes without deleting
#[test]
fn compaction_preserves_superseded_events_for_export() { ... }
```

Use `insta` for anything rendered: transcripts, diagnostics, help text, and the
JSON event stream. These change often and reviewing a snapshot diff is faster
and more honest than maintaining assertions by hand.

Test the failure paths as carefully as the success path. Missing tool
arguments, cancellation mid-call, exhausted budgets, malformed model output,
and storage failures are where the defects live.

Do not test private internals. A test that reaches past a public API makes the
crate hard to change and proves nothing a user could observe.

## Style

Keep the default `rustfmt`; there is no project style beyond it. Clippy runs
with `-D warnings`, and an `#[allow]` needs a comment saying why.

Derive `Debug` on every public type. Prefer `&str` over `String` in arguments,
`impl Trait` in argument position for simple bounds, and a named type in return
position when the caller has to name it.

Name things for what they are in the design documents. If the design calls it a
`Budget`, the struct is `Budget`, not `Limits`. One term, one meaning, across
prose and code alike.

## Dependencies

Adding one is a decision. Prefer the standard library, then something already
in the tree, then a well-maintained crate with a licence `cargo deny` accepts.
Say in the pull request why the dependency earns its place.

`cargo machete` catches dependencies nobody uses. Run it before opening a pull
request that changes `Cargo.toml`.
