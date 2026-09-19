---
description: Write or extend a component specification, then stop for review
argument-hint: <component or feature, e.g. "session fork" or "meow-policy rule matching">
---

Write or extend the specification for: **$ARGUMENTS**

Load the `spec-driven` skill before doing anything else. It defines where specs
live, how requirement IDs are allocated, and what makes a statement normative.

## Do this

1. **Locate the authority.** Find what `docs/design/` already decides about
   this component. A component spec elaborates those decisions; it never
   contradicts them. Quote the relevant design sections in your working notes.

2. **Locate the existing spec.** If `docs/spec/<component>.md` exists, you are
   extending it - read all of it, and reuse its area prefix and allocation
   counter. If it does not, you are creating it from the template in
   `docs/spec/README.md`.

3. **Find the boundary.** Write down what is observable from outside this
   component: its API, the files it writes, the events it emits, the exit codes
   and errors it produces. Requirements constrain that boundary and nothing
   behind it.

4. **Check the prior art.** For anything that replaces a `v0.2.x` behaviour,
   read the Go implementation and say what it did. A spec that silently changes
   behaviour is how migrations go wrong. Name the change explicitly.

5. **Write the requirements.** Testable, observable, singular, RFC 2119
   keywords. Specify the failure paths as precisely as the success path -
   missing arguments, cancellation mid-call, exhausted budgets, storage
   failures, malformed model output.

6. **Write the open questions.** Anything you could not decide goes in an
   `## Open questions` section with the options and your recommendation. Do not
   resolve a genuine product decision by picking quietly.

## Then stop

**Do not implement. Do not create a branch. Do not open an issue.** This
command produces a document for a human to read and approve, and that approval
is the step the whole method exists to create.

Report:

- the file you wrote or extended
- the requirement IDs you allocated, as a range
- which `docs/design/` sections they elaborate
- any `v0.2.x` behaviour the spec deliberately changes
- the open questions, if any, with your recommendation on each
