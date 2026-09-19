---
description: Propose a specification change after implementation contradicted it
argument-hint: <requirement IDs and what the implementation found>
---

Propose a specification amendment: **$ARGUMENTS**

Load the `spec-driven` and `technical-english` skills first.

You are here because implementing against the spec found that the spec is
wrong. That is the method working, not failing - the discovery happened against
a written claim instead of a vague memory. What matters now is that the change
is recorded as a decision rather than absorbed quietly.

## Do this

1. **State the contradiction precisely.** The requirement ID, what it demands,
   what the implementation found, and why the requirement cannot hold as
   written. "It was awkward" is not a contradiction. "It cannot hold because
   `starlark-rust` values are `!Send` and the requirement assumes the evaluator
   moves between threads" is.

2. **Check `docs/design/` first.** If the requirement follows from an
   architecture decision, the amendment is an architecture change and needs a
   bigger conversation than this command. Say so and stop.

3. **Measure the blast radius.** List:
   - other requirements that depend on this one
   - tests that reference it
   - code already merged that relies on it
   - issues in flight whose acceptance criteria cite it

4. **Draft the replacement.** Withdraw the old ID and allocate a new one; never
   silently reword in place. A withdrawn requirement leaves a tombstone naming
   what replaced it, so an old pull request or test still resolves.

   ```markdown
   **[R-AGENT-007]** *Withdrawn - superseded by [R-AGENT-023].*

   **[R-AGENT-023]** The engine MUST run each parallel agent on its own
   blocking thread with its own evaluator, because `starlark-rust` values
   cannot cross a thread boundary.
   ```

5. **Write the migration.** For anything already built against the old
   requirement: what changes, who does it, and whether it blocks the amendment
   or follows it.

## Then stop

Present the amendment for approval as its own reviewable change. Do not apply
it to `docs/spec/` and continue implementing in the same breath; the approval is
the point.

Report the contradiction, the withdrawn and new IDs, the blast radius, the
migration, and what is blocked until the amendment lands.
