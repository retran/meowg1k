# Specifications

This directory holds the normative specifications for meowg1k v0.3.0. A
specification says what a component must do; the code either satisfies it or
has a defect.

Read `.claude/skills/spec-driven/SKILL.md` for the method: when a spec gets
written, how requirements are identified, how tests trace to them, and what to
do when an implementation contradicts one.

## How this relates to the other documentation

| Directory | Holds | Normative |
| --- | --- | --- |
| `docs/design/` | Architecture decisions for v0.3.0 and the reasoning behind them | Yes, at the architecture level |
| `docs/spec/` | Component behaviour, as numbered requirements | Yes |
| `docs/guides/` | Go-era reference material for v0.2.1 | No, and being deleted |

`docs/design/` answers why the system is shaped this way. A spec here
elaborates those decisions into obligations a test can check. When a spec
contradicts `docs/design/`, the design document wins and the spec is wrong.

## Requirement identifiers

Each normative statement carries an identifier, `R-<AREA>-<n>`:

```markdown
**[R-SESSION-014]** A `Compaction` event MUST NOT delete the events it
supersedes. Rebuilding context for a model call MUST skip the superseded range
and substitute the summary; rebuilding it for display or export MUST show the
original events.
```

The areas, one per file:

| Area | File | Covers |
| --- | --- | --- |
| `AGENT` | `agent.md` | The turn loop, budgets, outcomes, sub-agents |
| `STAR` | `starlark.md` | The Starlark surface, loading, tools, schemas |
| `POLICY` | `policy.md` | Rule matching, decisions, approval |
| `SESSION` | `session.md` | The event log, resume, fork, retention |
| `LLM` | `llm.md` | Provider traits, streaming, retry classification |
| `STORE` | `store.md` | SQLite schema, blobs, migrations |
| `INDEX` | `index.md` | Chunking, embeddings, retrieval |
| `TUI` | `tui.md` | Renderers, the command surface, exit codes |

Numbers are allocated and never reused. A withdrawn requirement leaves a
tombstone so an older pull request or test still resolves:

```markdown
**[R-AGENT-007]** *Withdrawn - superseded by [R-AGENT-023].*
```

## Template

A new spec file starts like this:

```markdown
# <Component>

Status: draft | approved
Elaborates: docs/design/0.3.0-<document>.md sections <n>, <m>

## Scope

What this component is responsible for, in two or three sentences, and what it
explicitly is not.

## Boundary

What is observable from outside: the API, the files it writes, the events it
emits, the errors and exit codes it produces. Requirements constrain this and
nothing behind it.

## Requirements

**[R-AREA-001]** ...

## Changes from v0.2.x

What the Go implementation did, and what this spec deliberately changes. A spec
that silently changes behaviour is how a migration goes wrong.

## Open questions

Anything undecided, with the options and a recommendation.
```

## Status

No specs are written yet. The v0.3.0 design documents are approved at the
architecture level; component specs get written ahead of the work that
implements them, one at a time, with `/spec`.
