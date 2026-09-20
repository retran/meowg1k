# Contributing

Thank you for wanting to help. This document says how work moves through the
repository, what a change has to satisfy before it lands, and where to look
when something is unclear.

## Code of conduct

By taking part you agree to uphold the [Code of Conduct](CODE_OF_CONDUCT.md).

## Getting set up

You need [mise](https://mise.jdx.dev/) and `git`. Everything else, including
the Rust toolchain and every tool a task invokes, comes from `mise.toml`.

```bash
git clone https://github.com/retran/meowg1k.git
cd meowg1k
mise install
mise run build      # -> target/debug/meow
```

To use the binary on this repository:

```bash
export ANTHROPIC_API_KEY=...   # and VOYAGE_API_KEY for the index
./target/debug/meow check      # does .meow/ load?
./target/debug/meow status     # what is changed here
```

## Specifications come first

No behaviour ships that a specification does not describe.

`docs/spec/` holds ten areas of requirements, each with an identifier like
`R-AGENT-013`. Before writing code whose behaviour no
requirement covers, write the requirement. When an implementation would
contradict one, amend the requirement in place, with the date and the reason,
and say so in the pull request - never write the code first and reword the
requirement afterwards.

Every test that checks a requirement names it:

```rust
/// [R-SESSION-014] compaction supersedes without deleting
#[test]
fn compaction_preserves_superseded_events_for_export() { ... }
```

That is what makes the traceability real rather than aspirational.

## The gate

```bash
mise run all
```

Seven things, in parallel: `fmt-check`, `check` (clippy with `-D warnings`),
`test` (nextest across the workspace), `doc`, `deny`, `unused-deps`, and
`lint-md`. CI runs the same seven, so a green run here means a green run
there. If they ever disagree, that is a defect in one of them.

Run it before opening a pull request, and again after any change you make in
review.

## Branches and commits

Work on a branch off `dev`. Never commit to `dev` directly, including for a
one-line fix: a change that skipped review is invisible to everyone who reads
the pull request log to learn what happened.

Name the branch `<type>/<short-slug>`, using the same types as commit
subjects: `feat/session-fork`, `fix/retry-classifies-auth-errors`,
`docs/starlark-examples`, `chore/renovate-cargo-manager`.

Commit subjects are conventional: `type(scope): subject`, imperative, no
trailing period, under 72 characters.

```text
feat(agent): return a structured outcome from every run
fix(retry): classify auth failures as fatal
```

The body explains why, not what. A reader can see the diff; they cannot see
the constraint that ruled out the shorter fix. Wrap at 72 columns.

Link the issue a commit closes with `Closes #123` on its own line.

**Never mention an AI tool in a commit message, a pull request, an issue, a
review comment, or release notes.** No co-author trailer, no "generated with"
footer, no paraphrase. The word "Claude" is allowed only when it names a model
the code talks to, such as a model id in a configuration file.

## Pull requests

Open it as soon as the branch has one meaningful commit, as a draft if it is
not ready. The body has four parts:

1. **What changed and why.** Prose, two to five sentences. Not a bullet list
   of the diff.
2. **Requirement identifiers.** Every `R-*` the change closes. If it closes
   none, say why not.
3. **How it was verified.** The commands you ran and what they reported.
   "Tests pass" is not verification; `mise run all` with the count is.
4. **What is not covered.** Known gaps, deferred work, and anything a reviewer
   should look at with more care than usual.

Squash merge, always. The branch's history is working material - the order in
which you happened to discover things - and it is noise in `dev`. Check the
squash message before confirming: GitHub builds it from the branch commits, so
anything you left in an intermediate one reappears there.

Delete the branch afterwards.

## Code standards

- Apache 2.0 header on every Rust file; `LICENSE_HEADER.txt` is the template.
- One `thiserror` enum per crate. Variants named for what went wrong rather
  than for where, and each carrying enough to act on.
- Never `unwrap` or `expect` outside tests. When an invariant truly cannot
  fail, restructure so the compiler sees it.
- `println!` and `eprintln!` are denied outside `meow-cli`. They write through
  the live terminal frame and corrupt it.
- Default `rustfmt`; there is no project style beyond it. An `#[allow]` needs
  a comment saying why.
- Adding a dependency is a decision. Prefer the standard library, then
  something already in the tree, then a well-maintained crate with a licence
  `cargo deny` accepts. Say in the pull request why it earns its place.

Test the failure paths as carefully as the success path. Missing arguments,
cancellation mid-call, exhausted budgets, malformed model output, and storage
failures are where the defects live. Do not test private internals: a test
that reaches past a public API makes the crate hard to change and proves
nothing a user could observe.

## Releases

A release is a tag, and the tag does everything. Push `v0.3.1` and
`.github/workflows/release.yaml` builds four targets, archives each one,
writes `SHA256SUMS` and an SPDX bill of materials, attests all of it through
Sigstore, and opens the GitHub release. There is nothing to run by hand and
nothing to upload afterwards.

Three things have to be true before the tag exists:

1. `CHANGELOG.md` has a section for the version, with the date.
2. `version` in the workspace `Cargo.toml` matches the tag without its `v`.
   The workflow checks this and refuses the release if they disagree.
3. The tagged commit is on `dev`. The workflow checks this too, because a tag
   can point at any object in the repository and a release built from an
   unreviewed commit is the one failure nothing else catches.

Run `mise run release-check` first. It is `mise run all` plus the release
build, which is where a profile-specific failure shows up - finding one in the
workflow instead costs twenty minutes and a deleted tag.

Tags are annotated and say in two or three sentences what the release is. The
changelog is in `CHANGELOG.md`; do not paste it into the annotation.

## Where to look

- `docs/spec/` - what the binary does, normatively
- `docs/design/` - why, with the reasoning that produced each decision
- `.meow/` - meowg1k configured to work on itself, which is the worked example
- `CHANGELOG.md` - what changed between releases
- `CLAUDE.md` - the working policy for this repository, in more detail than
  this document

## Getting help

Open a [discussion](https://github.com/retran/meowg1k/discussions) for a
question, an [issue](https://github.com/retran/meowg1k/issues) for a defect or
a proposal.

## License

By contributing you agree that your contributions are licensed under the
Apache License 2.0.
