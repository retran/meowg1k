# Policy

Status: approved 2026-09-19
Elaborates: docs/design/0.3.0-architecture.md section 5.3;
docs/design/0.3.0-starlark-api.md section 9; docs/design/0.3.0-tui.md section 7

## Scope

`meow-policy` decides whether a tool call runs. It matches a call against
declared rules, yields allow, ask, or deny, and records why.

It does not execute tools, prompt the user, or write to the session. It
answers one question and explains the answer; the engine acts on it and the
renderer asks the human.

## Scope of trust

This is the one part of the system a script must not be able to weaken, which
is why it lives in the runtime and not in a Starlark library. A library cannot
be trusted by the thing it constrains.

## Boundary

The `Policy` type built from a declaration, the `evaluate` call that returns a
decision, and the text `meow policy explain` prints.

## Requirements

### Rules

**[R-POLICY-001]** A rule MUST match on a tool name pattern, and MAY narrow
further with a selector belonging to that tool: `paths` for file tools,
`commands` for shell tools.

**[R-POLICY-002]** A tool name pattern MUST support a trailing wildcard
(`fs.*`) and an exact name (`fs.read`), and MUST NOT support a leading
wildcard.

**[R-POLICY-003]** A `paths` selector MUST match against an absolute path
with symlinks already resolved, using glob semantics where `**` crosses
directory boundaries. Resolving the path is the caller's job and MUST happen
before evaluation, so that evaluation itself touches no filesystem.

**[R-POLICY-014]** A tool MUST act on the exact path the policy evaluated, and
MUST NOT resolve the path a second time. Re-resolving reopens the window in
which a path allowed as a file becomes a symlink to somewhere denied.

**[R-POLICY-004]** A `commands` selector MUST match against the full command
line as a single string, using glob semantics.

**[R-POLICY-005]** A selector that no tool matching the rule's name pattern
supports MUST fail when the policy is built, not when a call is evaluated.

**[R-POLICY-006]** A call that touches several paths MUST be evaluated once per
resolved path.

**[R-POLICY-008]** A read that hits a denied path MUST return the allowed
paths and MUST name every path it skipped, so the model knows its view is
partial.

**[R-POLICY-009]** A write that hits a denied path MUST be denied as a whole
and MUST NOT write any path, so the workspace is never left half-applied.

**[R-POLICY-007]** Network tools MUST support a `hosts` selector matching the
host of the request, so a policy can allow one host without allowing the
network.

### Decisions

**[R-POLICY-010]** Evaluation MUST check deny rules first, then ask rules,
then allow rules, and MUST return the first match.

**[R-POLICY-011]** A call that matches no rule MUST be denied.

**[R-POLICY-012]** A decision MUST name the rule that produced it, or record
that no rule matched.

**[R-POLICY-013]** Evaluation MUST touch no filesystem, network, or clock,
and MUST return the same decision for the same resolved call, the same policy,
and the same set of session grants. Grants are an input to evaluation, not a
side effect of it.

### Ask

**[R-POLICY-020]** An `ask` decision MUST resolve to `deny` when standard
input is not a terminal, or when the invocation set `--yes`.

**[R-POLICY-021]** An approval prompt MUST show the tool name and the exact
arguments the call would use, verbatim, and MUST NOT show a summary or a
paraphrase.

**[R-POLICY-022]** An approval prompt MUST name the rule that caused it.

**[R-POLICY-023]** An approval granted as "always" MUST apply for the current
process only. The policy layer MUST NOT write a grant back to any file.

**[R-POLICY-024]** An approval prompt MUST wait indefinitely by default. A
timeout MAY be configured, and when one is configured and expires the prompt
MUST resolve to `deny`.

### Narrowing

**[R-POLICY-030]** An agent MAY declare a policy of its own. For every call,
the effective decision MUST be the more restrictive of what the two policies
say, ordering `deny` above `ask` above `allow`.

**[R-POLICY-031]** An agent policy MUST NOT allow a call the workspace policy
denies, and MUST NOT turn an `ask` into an `allow`.

**[R-POLICY-032]** A sub-agent MUST inherit its caller's effective policy and
MAY narrow it further, by the same rule.

### Enforcement

**[R-POLICY-040]** The engine MUST evaluate the policy before the tool
executes, and MUST NOT execute a tool whose decision is `deny`.

**[R-POLICY-041]** A denied call MUST return a message to the model naming the
tool and stating that policy denied it, so the model can choose another
approach.

**[R-POLICY-042]** Every evaluation MUST produce a `Policy` session event,
whatever the decision.

### Sensitive values

**[R-POLICY-060]** A rule MAY mark an argument or a result field sensitive. A
value so marked MUST be redacted in the approval prompt, in the transcript, and
in every export, and MUST NOT be written to the session log in the clear.

**[R-POLICY-061]** Redaction MUST replace the value with a fixed placeholder
and MUST NOT reveal its length.

### Explanation

**[R-POLICY-050]** `meow policy explain <tool> <argument>` MUST return the
rule decision a real call with those arguments would receive: `allow`, `ask`,
or `deny`. It MUST NOT claim to predict how an `ask` would be answered, because
that depends on a person and on grants made during a run that has not
happened.

**[R-POLICY-051]** The explanation MUST name the matching rule and the file
and line it was declared on, and MUST report how many higher-precedence rules
were checked without matching.

## Changes from v0.2.x

There is no policy layer in v0.2.x. `shell_exec` is an ordinary tool, so an
agent that is talked into running a command runs it. This whole specification
is new, and it is the largest single addition of the rewrite.

## Decisions

**A multi-path read is partial and says so; a multi-path write is all or
nothing**, by [R-POLICY-008] and [R-POLICY-009]. The first answer denied both
alike, to avoid a result that looks complete and is not. That is the right fear
and the wrong fix: naming the skipped paths removes the danger, and denying the
whole read costs an agent its view of a repository because of one `.env` file
it never wanted. A write is different, because a partial one leaves the
workspace in a state nobody chose.

**Network tools get a `hosts` selector now**, by [R-POLICY-007], rather than
waiting for an agent that needs one. An all-or-nothing network rule forces the
choice between no network and unrestricted egress, and egress is exactly where
a prompt-injected agent does the most damage.

**An approval prompt waits**, by [R-POLICY-024]. A prompt that expires while
you are reading the command it is asking about turns a security decision into
a reflex. A timeout stays configurable for an unattended terminal that is
nevertheless a terminal.
