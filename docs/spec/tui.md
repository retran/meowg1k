# Terminal

Status: draft
Elaborates: docs/design/0.3.0-tui.md, all sections

## Scope

`meow-ui` turns the engine's event stream into something a person or a program
can read, and `meow-cli` turns a command line into a run. Together they are
everything a user touches.

`meow-ui` depends on `meow-core` for the event types and on nothing else in
the workspace. It does not know the engine exists.

## Boundary

What appears on stdout and stderr, what the exit code is, what `--format json`
emits, and what the approval prompt shows.

## Requirements

### Renderer selection

**[R-TUI-001]** The renderer MUST be chosen by the runtime, never by a script.

**[R-TUI-002]** `--format json` MUST select the JSON renderer whatever the
terminal is.

**[R-TUI-003]** A stdout that is not a terminal, or `NO_COLOR` in the
environment, or `--color=never`, MUST select the plain renderer.

**[R-TUI-004]** Otherwise the inline terminal renderer MUST be selected.

**[R-TUI-005]** All three renderers MUST accept the same event stream, and
each MUST be drivable from a recorded log with no terminal attached.

### Inline rendering

**[R-TUI-010]** The terminal renderer MUST use an inline viewport and MUST NOT
switch to the alternate screen buffer.

**[R-TUI-011]** A finalized transcript line MUST be written into scrollback
and MUST NOT be redrawn afterwards.

**[R-TUI-012]** Only the live region MUST be redrawn, and it MUST show the
current tool, the elapsed time, the step count, and the budget consumed.

**[R-TUI-013]** On any exit, including an interrupt, the transcript already in
scrollback MUST remain valid and the live region MUST be replaced by a final
line naming the stop reason.

**[R-TUI-014]** A terminal resize MUST reflow only the live region.

**[R-TUI-015]** Diagnostics from the logging layer MUST be inserted into
scrollback in order, and MUST NOT be drawn over the live region.

### Plain rendering

**[R-TUI-020]** The plain renderer MUST emit no ANSI escape sequences and MUST
NOT move the cursor.

**[R-TUI-021]** The plain renderer MUST carry the same information as the
terminal renderer: every step, every tool call with its policy decision, and
the totals.

### JSON rendering

**[R-TUI-030]** The JSON renderer MUST emit one JSON object per line, each
with a `type` field.

**[R-TUI-031]** The stream MUST begin with an event carrying the schema
version.

**[R-TUI-032]** The event schema MUST be identical to the one
`meow session export --format json` emits, per [R-SESSION-090].

**[R-TUI-033]** With `--format json`, stdout MUST carry only the event stream.
Diagnostics MUST go to stderr.

### Script output

**[R-TUI-040]** `ctx.out` MUST expose exactly `write`, `markdown`, `note`,
`warn`, `error`, `step`, `table`, `diff`, `finding`, and `json`.

**[R-TUI-041]** `ctx.out` MUST NOT expose any call that positions the cursor,
draws a frame, or paginates.

**[R-TUI-042]** Each `ctx.out` call MUST produce a typed event that all three
renderers handle.

### Interaction

**[R-TUI-050]** `ctx.ask` MUST expose exactly `text`, `confirm`, and `select`.

**[R-TUI-051]** Any `ctx.ask` call MUST fail with an error when stdin is not a
terminal or `--yes` was given, and MUST NOT block.

### Approval

**[R-TUI-060]** An approval prompt MUST occupy the live region only, and MUST
NOT overwrite the transcript above it.

**[R-TUI-061]** The prompt MUST show the tool name, the exact arguments, the
matching rule, and the agent and step, and MUST offer once, always, deny, and
stop.

**[R-TUI-062]** Choosing "always" MUST apply for the current process only, per
[R-POLICY-023].

### Command surface

**[R-TUI-070]** A user's agents and tools MUST be reachable at the top level
of the command line, without a prefix.

**[R-TUI-071]** Built-in commands MUST be grouped under `session`, `auth`,
`index`, and `policy`, except `init`, `run`, `check`, `models`, `providers`,
`doctor`, `trust`, `completions`, and `version`.

**[R-TUI-072]** `--dry-run` MUST evaluate policy and plan tool calls without
executing any, and the transcript MUST record what would have run and how
policy would have decided.

**[R-TUI-073]** `--yes` MUST make every `ask` decision resolve to `deny`, per
[R-POLICY-020], and MUST NOT make any decision more permissive.

**[R-TUI-074]** `--continue` MUST resume the most recent session of the
command being invoked, in this workspace, and MUST fail if there is none
rather than starting a fresh run.

### Exit codes

**[R-TUI-080]** The process exit code MUST be derived from the stop reason and
the handler's return value:

| Code | Meaning |
| --- | --- |
| 0 | `finished`, and the handler returned true or nothing |
| 1 | `finished`, and the handler returned false |
| 2 | Usage error: unknown command, bad flag, bad argument |
| 3 | `budget` |
| 4 | `cancelled` |
| 5 | `denied` |
| 6 | Provider or credential failure |
| 7 | Configuration error: `.meow/` failed to load |
| 8 | `tool_aborted` |

**[R-TUI-081]** An exit code MUST NOT be reused for a different stop reason,
so that a shell can branch on it.

### Theme and accessibility

**[R-TUI-090]** `NO_COLOR` MUST disable colour unconditionally, whatever the
theme declares.

**[R-TUI-091]** Colour depth MUST be detected and the palette quantised to it,
rather than colour being dropped.

**[R-TUI-092]** Colour MUST NOT be the only carrier of meaning: every severity
and status MUST also carry a word or a sigil.

**[R-TUI-093]** A terminal that cannot be shown to support the box-drawing and
spinner characters MUST get the ASCII fallback.

## Changes from v0.2.x

Three rendering stacks run at once in v0.2.x: a uilive progress logger, an
807-line Bubble Tea program, and a set of lipgloss widgets. Each owns the
cursor and which one wins is timing-dependent. One event stream and three
renderers replace them.

`ctx.ui` exposes 22 layout builtins, so presentation is decided in userland and
cannot be fixed centrally. [R-TUI-040] and [R-TUI-041] cut it to ten semantic
calls.

There is no machine-readable mode, so `meow review | jq` is impossible, and no
exit code beyond success and failure, so an agent cannot act as a gate.

Ten `log.Printf` calls in `module_llm.go` write straight through the live
frame. [R-TUI-015] routes them into scrollback instead.

## Open questions

- **The live region's height.** A fixed three lines is predictable; growing it
  for an approval prompt costs a reflow. Recommendation: fixed at three, and
  let the approval prompt take a larger fixed height while it is open.
- **Whether `--format json` should stream or buffer.** Streaming lets a
  consumer react mid-run; buffering lets the output be a single JSON document.
  Recommendation: stream as JSONL, because the buffered form is one `jq -s`
  away and the streaming form is not recoverable from a document.
