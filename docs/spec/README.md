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

## Decisions

Each question once it is settled: what was decided, and why the alternative
lost. Keep the reason. In six months the decision is obvious and the reason is
the only thing that stops it being reopened.
```

## Status

All eight component specifications are drafted, with 262 requirements
between them, and every open question is closed. None is approved yet, and no
implementation exists.

| Area | File | Requirements |
| --- | --- | --- |
| `AGENT` | `agent.md` | 42 |
| `STAR` | `starlark.md` | 38 |
| `POLICY` | `policy.md` | 29 |
| `SESSION` | `session.md` | 37 |
| `LLM` | `llm.md` | 29 |
| `STORE` | `store.md` | 25 |
| `INDEX` | `index.md` | 24 |
| `TUI` | `tui.md` | 38 |

Drafting them together rather than one at a time was a deliberate trade. It
caught four contradictions between the design documents that a sequential pass
would have met one at a time, months apart: three different lists of stop
reasons, an exit code table missing an outcome, a context member documented in
one place and not the other, and a count that disagreed with the list beneath
it. All four were fixed in the design documents, since a specification that
contradicts its source is the wrong thing to correct.

The cost is that these drafts are ahead of the code. The `starlark-rust` spike
in `docs/design/0.3.0-architecture.md` section 12 has not run, and if the
`!Send` value model fights the module design, `starlark.md` and `agent.md` will
need amendments. Use `/amend-spec` when that happens rather than rewording a
requirement in place.
