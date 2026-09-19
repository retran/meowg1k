# Session

Status: approved 2026-09-19
Elaborates: docs/design/0.3.0-sessions.md, all sections

## Scope

`meow-session` owns the durable record of an agent run: the append-only event
log, the identifiers people type, the lifecycle, and the operations that
continue or branch a run. It sits on `meow-store` and knows nothing about the
engine that produces the events.

It does not decide when to compact, what a budget is, or how a transcript
looks. It records what happened and replays it two ways: for a model, and for
a person.

## Boundary

The Rust API on `meow-session`, the rows it writes through `meow-store`, and
the identifiers and JSON that reach a user through `meow session` commands.

## Requirements

### The log

**[R-SESSION-001]** The event log MUST be append-only. The session layer MUST
NOT update or delete an event that has been written.

**[R-SESSION-002]** Each event MUST carry a sequence number that is monotonic
and gapless within its session, starting at 1.

**[R-SESSION-003]** The event kinds MUST be exactly: `Started`, `UserMessage`,
`Assistant`, `ToolCall`, `ToolResult`, `Policy`, `Usage`, `Compaction`, `Note`,
and `Finished`.

**[R-SESSION-004]** Every event MUST carry the timestamp at which it was
recorded, in UTC.

**[R-SESSION-005]** A session MUST hold one or more runs. Each run MUST begin
with a `Started` event and, once it has ended, MUST be closed by exactly one
`Finished` event before the next `Started` event.

**[R-SESSION-006]** Resuming a session MUST append a new `Started` event, so
that a session that has already ended can be continued without rewriting the
event that ended it.

### Compaction

**[R-SESSION-010]** A `Compaction` event MUST name the inclusive sequence
range it supersedes and MUST NOT delete the events in that range.

**[R-SESSION-011]** Rebuilding the message list for a model call MUST skip
every superseded range and substitute that range's summary.

**[R-SESSION-012]** Rebuilding the log for display or export MUST return the
original events and MUST NOT substitute a summary.

**[R-SESSION-013]** A `Compaction` event MUST NOT supersede a range that an
earlier `Compaction` event already supersedes.

### Usage and cost

**[R-SESSION-020]** A `Usage` event MUST carry prompt tokens, completion
tokens, cached prompt tokens, and cost as separate typed fields.

**[R-SESSION-021]** Cost MUST be computed when the event is written, from the
price table in effect at that moment, and MUST NOT be recomputed on read. A
model with no price in the table MUST record the cost as absent, never as zero,
so an unpriced model does not read as a free one.

**[R-SESSION-022]** A session MUST report its totals as the sum of its own
`Usage` events plus the totals of its child sessions.

### Identity

**[R-SESSION-030]** Every session MUST have a full identifier that sorts by
creation time, and a short identifier that is a fixed eight-character prefix of
it. The length is fixed rather than "shortest unique" so that an identifier
written into a commit message or an issue keeps resolving as sessions
accumulate.

**[R-SESSION-031]** Resolving a short identifier that matches more than one
session MUST fail with an error listing the candidates, and MUST NOT pick one.

**[R-SESSION-032]** The selectors `@last`, `@last-N`, and `@<agent-name>` MUST
resolve at the moment of use: `@last` to the most recent session in the
workspace, `@last-N` to the Nth most recent, and `@<agent-name>` to the most
recent session of that agent.

**[R-SESSION-033]** A session MAY carry a name. A name MUST be unique within
the workspace, MUST NOT begin with `@`, and MUST resolve like an identifier, so
that a name can never shadow a selector.

### Lifecycle

**[R-SESSION-040]** A session MUST be in exactly one state: `running`, or one
of the terminal states `finished`, `budget`, `cancelled`, `denied`,
`tool_aborted`, or `failed`.

**[R-SESSION-041]** The state MUST be defined by the log: `running` when the
last event is not `Finished`, and otherwise the stop reason that last
`Finished` event carries. A denormalised copy MAY be kept so that listing a
thousand sessions does not read a thousand events, provided it is rebuildable
from the log and the log wins on any disagreement.

**[R-SESSION-043]** While a run is in flight its writer MUST update a
heartbeat timestamp on the session row at a fixed interval. The heartbeat MUST
NOT be an event, because it is mutable and the log is not. A session whose last
event is not `Finished` and whose heartbeat is older than three intervals MUST
be treated as dead.

**[R-SESSION-042]** Opening a session whose last event is not `Finished` and
whose recording process is no longer alive MUST append
`Finished { stop: failed, reason: "process exited" }` and MUST NOT leave the
session reported as running.

### Continuing and branching

**[R-SESSION-050]** Resuming a session MUST append to it, continuing the
existing sequence, and MUST NOT create a new session.

**[R-SESSION-051]** Resuming MUST rebuild the message list per
[R-SESSION-011], so a resumed run sees compaction exactly as the original did.

**[R-SESSION-052]** Forking at sequence `n` MUST create a new session whose
first `n` events are copies referencing the same blobs, MUST increment the
reference count of every blob so referenced, MUST record the origin session and
sequence, and MUST NOT modify the origin. Without the increment, collecting the
origin would delete content the fork still points at.

**[R-SESSION-053]** Forking at a sequence that does not exist, or at a
sequence inside a superseded range, MUST fail with an error naming the valid
range.

**[R-SESSION-054]** A run that names no session MUST start a fresh one. The
session layer MUST NOT infer continuation from the workspace.

### Parentage

**[R-SESSION-060]** A session started by a sub-agent MUST record its parent,
and the parent MUST be able to enumerate its children in creation order.

**[R-SESSION-061]** A cycle in the parent relation MUST be impossible: a
session's parent MUST already exist when the session is created.

### Audit

**[R-SESSION-070]** Every tool invocation MUST produce a `Policy` event before
the tool runs, recording the decision, the rule that produced it, and whether
the decision came from a rule, an interactive answer, or a session grant.

**[R-SESSION-071]** A `Policy` event MUST be written for denied invocations as
well as allowed ones.

### Retention

**[R-SESSION-080]** Garbage collection MUST delete whole sessions only, and
MUST delete a parent only together with its descendants.

**[R-SESSION-081]** Garbage collection MUST NOT delete a named session unless
the caller asks for named sessions explicitly.

**[R-SESSION-082]** Retention MUST be configurable by age, by count, and by
total database size, and MUST apply the strictest of the configured limits.

### Export

**[R-SESSION-090]** JSON export MUST use the schema definition and version
number that the live `--format json` renderer uses, and every persisted event
kind MUST serialise identically in both, per [R-TUI-032].

**[R-SESSION-091]** Markdown export MUST include the transcript, the tool
calls with their policy decisions, and the usage totals.

**[R-SESSION-092]** Export MUST redact every value the policy marked
sensitive under [R-POLICY-060], in both formats.

**[R-SESSION-093]** Export MUST omit thinking content unless it is asked for
explicitly, because it is the part of a transcript least likely to be meant for
an audience.

## Changes from v0.2.x

`ctx.session.mark_obsolete(ids)` mutated the log, so a compacted run could no
longer be replayed in full. [R-SESSION-010] and [R-SESSION-012] replace it.

Usage was written as metadata strings such as
`llm_tokens_0_1762...` = `"prompt=91,completion=12,total=103"`, which cannot be
summed without parsing twice, and carried no cached-token count. [R-SESSION-020]
makes it typed.

Identifiers were UUIDs. Nobody types `meow show-session 9f8e7d6c-5b4a-...`.

`status` was a mutable column that could disagree with the events beneath it,
and a session whose process died stayed `running` forever.

Fork did not exist, so investigating a run that went wrong at step 9 of 40 cost
a full rerun.

## Decisions

**Sessions are workspace-local.** A run against a monorepo subdirectory writes
to that subdirectory's `.meow/`, which [R-SESSION-030] and [R-STAR-001] already
require between them. A global store would need a global identifier scheme and
a way to decide which workspace a session belongs to, and nobody has asked for
either.

**Liveness is a heartbeat**, by [R-SESSION-043]. The first answer was a
process identifier plus its start time, chosen for being portable. It is not:
reading another process's start time means procfs on Linux, sysctl on macOS,
and a Win32 call on Windows, which is the same per-platform work as the
advisory lock it was preferred over, for a weaker guarantee.

A heartbeat needs no platform code at all, and the window in which a dead
session still looks alive is bounded by the interval and tunable. The cost is
one small write per interval on a run that is already writing events.
