# Documentation

Three kinds of document, and they answer different questions.

## Start here

[vision.md](vision.md) is what meowg1k is, who it is for, and what it
deliberately is not. Read it first if you have not used the tool.

## Principles

[philosophy.md](philosophy.md) is what the project is for: eleven principles,
what each means for the implementation, and - where the code does not meet one -
the issue tracking the gap. It is the document to read before arguing about a
design decision, and the one to correct when a principle stops being true.

## Specifications

`spec/` is normative. Every behaviour the binary has traces to a requirement
there, and a change that no requirement describes needs a requirement first.
Ten files, one per area:

| File | Area | What it fixes |
| --- | --- | --- |
| [agent.md](spec/agent.md) | `R-AGENT-*` | the loop, budgets, tools, compaction |
| [auth.md](spec/auth.md) | `R-AUTH-*` | where a credential comes from, and trusting a workspace |
| [index.md](spec/index.md) | `R-INDEX-*` | walking, chunking, embedding, retrieval |
| [llm.md](spec/llm.md) | `R-LLM-*` | what a provider is and what it promises |
| [packages.md](spec/packages.md) | `R-PKG-*` | fetching, pinning, and verifying Starlark from elsewhere |
| [policy.md](spec/policy.md) | `R-POLICY-*` | what an agent may do, and who decides |
| [session.md](spec/session.md) | `R-SESSION-*` | the append-only log, forks, retention |
| [starlark.md](spec/starlark.md) | `R-STAR-*` | the surface users write against |
| [store.md](spec/store.md) | `R-STORE-*` | one database, content-addressed |
| [tui.md](spec/tui.md) | `R-TUI-*` | three renderers, one event stream |

A requirement that was withdrawn leaves a tombstone naming what replaced it.
An amended one records the date and the reason in place.

## Design

`design/` is where the decisions live, with the reasoning that produced them.
It is not normative: where it and a specification disagree, the specification
wins and the design document gets fixed.

- [0.3.0-architecture.md](design/0.3.0-architecture.md) - the crates, the
  execution model, and what each dependency is for
- [0.3.0-starlark-api.md](design/0.3.0-starlark-api.md) - the surface, with
  worked examples and a migration table from v0.2.x
- [0.3.0-sessions.md](design/0.3.0-sessions.md) - the log, forking, retention
- [0.3.0-tui.md](design/0.3.0-tui.md) - the inline viewport and the command
  surface
- [0.3.0-plan.md](design/0.3.0-plan.md) - twelve milestones, M0 to M11, and
  how the requirements were assigned across them; executed

## The workspace this repository uses

`.meow/` is meowg1k configured to work on itself: three markdown agents, two
shared prompt fragments, and the commands in `.meow/meow.star`. It is the
worked example that has to keep working, because it is what the maintainers
run.

## What is no longer here

The v0.2.x guides described the Go implementation, which v0.3.0 replaced. They
documented modules and commands that no longer exist, and keeping them would
have meant maintaining a third account of the system beside the specifications
and the code. The history has them if anybody needs one.
