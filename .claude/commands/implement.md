---
description: Implement one issue against its spec, on a feature branch, ending in a pull request
argument-hint: <issue number or requirement IDs, e.g. #42 or R-SESSION-014..018>
---

Implement: **$ARGUMENTS**

Load the `spec-driven`, `scm`, and `technical-english` skills first. Load `rust`
if the work touches Rust code.

## Before writing code

1. **Read the issue and every requirement it cites.** Quote the requirement
   text in your working notes. You are building to that text, not to your
   memory of a conversation.

2. **Refuse to start if the spec does not cover the work.** If the issue asks
   for behaviour that no requirement describes, stop and say so. Run `/spec`
   first. Implementing uncovered behaviour is the failure the whole method
   exists to prevent.

3. **Check the dependencies.** If the issue names blockers that have not
   closed, say so and stop.

4. **Branch off `dev`.** Name it for the change, not for the issue number.

## While implementing

Build to the requirements and nothing more. Behaviour the spec does not ask for
does not go in, however obvious it seems; open an issue instead.

Write the tests against the requirement IDs as you go, not afterwards. Each test
names the requirement it checks in a doc comment. A requirement with no test is
not implemented, whatever the code does.

Cover the failure paths the spec specifies. Most of what went wrong in `v0.2.x`
was an unspecified failure path that nobody tested: a missing tool argument, an
empty model response, a failed session write, a budget exhausted mid-call.

**When the implementation contradicts the spec, stop.** Do not write code the
spec forbids and reword the spec afterwards. Run `/amend-spec`, get the
amendment approved, then resume. This is the single rule that decides whether
the spec stays worth reading.

## Before opening the pull request

Run the full local gate and report what it actually said:

```bash
mise run fmt-check && mise run check && mise run test
```

Then run `/verify` against the requirement IDs. Fix what it finds.

## Output

Open the pull request as described in the `scm` skill, then report:

- the branch and the pull request number
- the requirement IDs closed, and the test that covers each
- the gate results, with real numbers
- anything deferred, and the issue you opened for it
- any spec amendment this work required

Do not merge. Merging needs explicit approval.
