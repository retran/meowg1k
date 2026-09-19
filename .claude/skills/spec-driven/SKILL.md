---
name: spec-driven
description: How specification-driven development works in this repository - where specs live, how requirements are identified and traced to tests, when a spec must be written or amended, and what to do when code and spec disagree. Load before writing or changing a spec, before implementing against one, and whenever an implementation reveals the spec is wrong.
---

# Specification-driven development

## The rule

**No behaviour ships that a specification does not describe.** If you are about
to write code whose behaviour is not covered by a normative statement in
`docs/`, stop and write the statement first.

This is not process for its own sake. meowg1k v0.3.0 is a ground-up redesign
whose whole justification is that v0.2.x accumulated behaviour nobody decided
on - a context object assembled twice, an agent loop returning a bare string, a
retry path that backs off on authentication failures. Every one of those is
behaviour that exists because someone wrote code, not because someone decided.
The spec is how that stops happening again.

## The two trees

| Tree | Holds | Changes when |
| --- | --- | --- |
| `docs/design/` | Architecture-level decisions for v0.3.0 - the execution model, crate boundaries, the Starlark API shape, the session model, the terminal surface | A decision changes. Rare, and always a deliberate act |
| `docs/spec/` | Component-level normative behaviour, one file per crate or feature | Continuously, as components are specified ahead of being built |

`docs/design/` answers *why the system is shaped this way*. `docs/spec/`
answers *what this component must do*. When they disagree, `docs/design/` wins
and the component spec is wrong.

`docs/guides/` is neither. It is Go-era reference material for `v0.2.1`, it is
not normative, and it is being deleted as the Rust tree replaces it. Never cite
it as authority.

## Requirements

A spec is prose with **normative statements** embedded in it. Each one carries
a stable identifier:

```markdown
**[R-SESSION-014]** A `Compaction` event MUST NOT delete the events it
supersedes. Rebuilding context for a model call MUST skip the superseded range
and substitute the summary; rebuilding it for display or export MUST show the
original events.
```

- `R-<AREA>-<n>` where `<AREA>` matches the spec file (`SESSION`, `AGENT`,
  `POLICY`, `STAR`, `TUI`, `LLM`, `STORE`, `INDEX`).
- Numbers are **allocated, never reused**. A deleted requirement leaves a
  tombstone (`**[R-SESSION-009]** *Withdrawn in #142 - superseded by
  [R-SESSION-014].*`) so an old PR or test referencing it still resolves.
- Numbers are not ordered by importance and do not renumber when the document
  is reorganised.

### Writing one well

A normative statement is testable, observable, and singular.

- **Testable.** Someone must be able to write a test that fails if it is
  violated. "The engine SHOULD be efficient" is not a requirement.
- **Observable.** It constrains behaviour visible at a boundary - an API
  return, a file on disk, an exit code, a rendered line. It does not constrain
  how a crate is laid out internally. `meow-agent` may be restructured freely
  as long as every `R-AGENT-*` still holds.
- **Singular.** One obligation per statement. Two obligations means two
  requirements, because they will be tested separately and one may be withdrawn
  without the other.

Use MUST / MUST NOT / SHOULD / SHOULD NOT / MAY in the RFC 2119 sense. `SHOULD`
means a deliberate, documented exception is permitted; if you cannot imagine
the exception, write `MUST`.

Specify the failure as precisely as the success. Most of the defects in
`v0.2.x` are underspecified failure paths: what happens when a required tool
argument is missing, when the model returns empty text, when a session write
fails, when the budget is exhausted mid-tool-call. A spec that only describes
the happy path has not done its job.

## Traceability

Every requirement has at least one test, and the test names the requirement:

```rust
/// [R-SESSION-014] compaction supersedes without deleting
#[test]
fn compaction_preserves_superseded_events_for_export() { ... }
```

This is what makes `/verify` mechanical rather than a matter of opinion: the
requirement IDs present in `docs/spec/` and the requirement IDs referenced in
`crates/*/src/**` and `tests/**` are two sets, and the interesting cases are
the differences.

- **In the spec, not in any test** - unimplemented or untested. Both are
  findings.
- **In a test, not in the spec** - the requirement was withdrawn and the test
  was not updated, or the ID is a typo.

## The loop

```text
  /spec --> review & approve --> /plan --> /implement --> /verify --> PR
              ^                               |
              +------- /amend-spec <----------+
                      (divergence found)
```

1. **`/spec`** - write or extend a component spec. Produces requirements and
   stops. No implementation, no branch, no code.
2. **Approval** - a human reads it. This is the step the whole method exists to
   create, and skipping it makes the rest ceremony.
3. **`/plan`** - decompose approved requirements into issues with explicit
   dependencies. Each issue cites the requirement IDs it closes.
4. **`/implement`** - one issue, one branch, one PR. The spec is the
   acceptance criteria; the PR body lists the requirement IDs it satisfies.
5. **`/verify`** - check the implementation against the spec in both
   directions before asking for review.

## Divergence

Implementation reveals that specs are wrong. That is expected and is not a
failure of the method - it is the method working, because the discovery happens
against a written claim instead of against a vague memory.

**When implementation contradicts the spec, stop.** Do not write code that the
spec forbids and fix the wording afterwards; the spec stops being trustworthy
the first time that is allowed, and an untrustworthy spec is worse than none
because people still cite it.

The protocol:

1. Stop implementing. Leave the work in place, uncommitted or on its branch.
2. State the contradiction precisely: the requirement ID, what it demands, what
   the implementation found, and why the requirement cannot hold.
3. Run `/amend-spec`. It proposes the change, the blast radius (which other
   requirements and which existing tests depend on it), and the migration for
   anything already built.
4. Get the amendment approved as its own reviewable change.
5. Resume.

The step people skip is the third, and the failure mode is subtle: a
requirement is quietly reworded to match what was built, the reasoning behind
the original is lost, and six months later the same mistake is made again
because the record of why it was a mistake is gone. An amendment says what
changed and why, in the commit that changes it.

## What a spec is not

- **Not a design document.** `docs/design/` holds the reasoning and the
  rejected alternatives. A component spec states obligations and can be read by
  someone who does not care why.
- **Not documentation.** User-facing docs explain how to accomplish a task and
  may omit, simplify, and recommend. A spec is exhaustive about a boundary and
  omits nothing normative.
- **Not a test plan.** The spec says what must be true; the tests say how it is
  checked. One requirement may need six tests, and six requirements may share
  one.
- **Not a backlog.** Requirements describe the system as it is specified to be,
  in present tense. Sequencing lives in issues.

## Scope discipline

Specify what is being built now, to the depth needed to build it. A speculative
spec for a component nobody is implementing is worse than no spec: it will be
wrong by the time it matters, and it will be cited as authority in the
meantime.

Equally, do not write a requirement you are unwilling to test. If a statement
cannot be checked, it is a design note - put it in `docs/design/` and leave it
out of the normative tree.
