---
description: Review a pull request against its spec, correctness, and repository conventions
argument-hint: <pull request number, or nothing for the current branch>
---

Review: **$ARGUMENTS** - the current branch against `dev` if no target is given.

Load the `spec-driven` and `scm` skills first. Load `rust` if the diff touches
Rust code.

## Read before judging

Read the issue, the requirements it cites, and the whole diff. A review that
starts commenting before it has read the diff finds style problems and misses
defects.

## Check, in this order

1. **Spec conformance.** Does the code do what the cited requirements demand?
   Run `/verify` and use what it finds. This comes first because a correct
   implementation of the wrong thing is still wrong.

2. **Correctness.** For each change, ask what input makes it wrong. Report a
   defect only when you can name the input and the resulting behaviour. A
   defect you cannot demonstrate is a question, so ask it as one.

3. **Failure paths.** Every error branch the spec specifies: is it implemented,
   and does a test drive it? This is where `v0.2.x` accumulated its defects and
   it is where a review pays for itself.

4. **Test quality.** Would each test fail if its requirement were violated? A
   test that asserts the code does what the code does is worse than no test,
   because it reports coverage it does not provide.

5. **Boundaries.** `meow-core` depends on no workspace crate. `meow-agent` does
   not know Starlark exists. `meow-ui` does not know the engine exists. A
   change that crosses one of these is a finding regardless of how small it is,
   because these are the boundaries the whole redesign rests on.

6. **Scope.** Behaviour beyond the cited requirements does not belong here.

7. **Conventions.** Commit format, pull request body, and - always - no mention
   of Claude, Claude Code, or any AI tool anywhere in the commits or the pull
   request text.

## Output

Findings first, worst first, each with the file and line, what is wrong, and
the input that makes it wrong. Then questions, which are things you could not
resolve from the diff. Then a one-line verdict: ready, ready with the findings
fixed, or needs rework and why.

Say plainly when the pull request is clean. Do not manufacture findings to look
thorough.
