# Principles

Status: the standard this implementation is held to, and where it currently
falls short.

These are the principles the project has held since it began. They did not
change when the implementation was rewritten in Rust; what changed is how well
the code meets them. Where the two disagree, the principle is right and the
code is the defect - every such gap below names the issue tracking it.

`docs/spec/` says what the binary does and is binding. This says what it is
for. When a principle here needs enforcing, it becomes a requirement there, and
the last section says which.

## 1. A composable engine, not an application

meowg1k is a component other tools call, not a program a person operates.

Every run exits with a code a shell can branch on, reads from stdin, and can
emit its events as one JSON object per line with nothing else on stdout.
`meow review && git commit` is the intended shape of use.

The rewrite strengthened this. A command a user declares is a top-level
subcommand with its own `--help` and its own typed flags, so a workspace's own
verbs compose the same way the built-in ones do.

## 2. Task execution, not a conversation

A run is a task with a beginning, an end, and a reason it stopped. It is not a
chatbot.

There is an agent loop now, and it does not contradict this. The loop runs
against a task under a budget and returns an outcome carrying its stop reason;
it does not wait for a person. Continuing is explicit and narrow: `--continue`
resumes the most recent session _of the command being invoked_, and fails when
there is none rather than starting a fresh run.

## 3. Native performance, zero dependencies

One static binary, no runtime to install. The tool starts instantly and runs
anywhere from a laptop to a stripped container.

## 4. Radical independence, no lock-in

Switching providers is a configuration change, never a code change. Six kinds
ship - Anthropic, OpenAI, OpenRouter, Gemini, Voyage, llama, and Copilot - and
every OpenAI-shaped vendor is served by one implementation differing only in
its address. A local `llama.cpp` server is a first-class provider, not a
fallback.

## 5. Local-first

The index, the session log, the blobs, and the response cache are one SQLite
file in the workspace. A semantic query walks a vector graph on disk and reads
only the rows that won.

The single thing needing a network is the model, and a local server counts. A
workspace that has been indexed searches offline; a workspace that has fetched
its packages loads offline.

## 6. Configuration is code

Not a data format that grew expressions - a program. `.meow/meow.star` is
Starlark, version-controlled, reviewable, and shared with the repository it
belongs to.

**Loading is exclusive, not layered.** A project configuration means the global
one is not read at all. The earlier statement of this principle described a
hierarchy, and that was changed deliberately: a global setting bleeding into a
project that did not restate it is a workspace that behaves differently on two
machines.

## 7. Predictable and auditable

The tool's own logic is deterministic. Randomness comes from the model's
sampling and from nowhere else, and the user sets it.

The rewrite went further than the principle asked. Every turn, tool call,
policy decision, and token lands in an append-only log. Compaction supersedes
rather than deletes, so an export after compaction still shows what happened.
`--dry-run` decides policy and plans the calls without making any.

## 8. Intelligent context, not raw input

The value is in what reaches the model, not in relaying what was typed. This
was aspiration when it was written; it is implemented now. The workspace is
walked under `.gitignore` and `.meowignore`, chunked on line boundaries,
embedded, and indexed into an HNSW graph, and `search.code` asks it by meaning.

## 9. Security by design

Secrets are held in memory for a request and not persisted by this tool.
Releases are signed so a user can verify that the binary is the one the build
produced.

**Not met.** Credentials live in a plaintext `~/.meow/auth.json`, created
`0600` and refused if wider, written atomically, never printed - and still a
file on disk with secrets in it, which is what this forbids. OAuth requires
persisting a refresh token, so "hold nothing" has to mean _meowg1k_ holds
nothing and the operating system's secret store does. Tracked by **#166**.

What is met: releases carry a Sigstore attestation and an SPDX bill of
materials, no runtime module can reach the credential store, and no command
prints a credential.

A second protection the principle did not anticipate: a `.meow/` directory in
a repository you just cloned is code with tool access, so the first run shows
what it declares and asks once, and asks again when those declarations change.

## 10. Predictable cost

Spend is a parameter of the workflow, not a surprise. The user chooses the
model, caps the tokens, and sets a hard ceiling on usage.

**Partly met.**

- _Model choice_ holds: a free or local model is one declaration away.
- _Token caps_ hold: a budget bounds steps, tokens, and wall time, and a
  fan-out shares its caller's ledger so branches cannot each spend the whole.
- _Rate limiting does not exist._ `requestsPerMinute` and `requestsPerDay`
  were a shared, database-backed ceiling in the previous implementation, and
  the rewrite dropped them. Tracked by **#167**.
- _The cost axis cannot fire._ `Budget` carries `cost_micros` and no provider
  reports a cost, so a declared cost cap silently does nothing. Tracked by
  **#168**.

## 11. Radically open

Apache 2.0, developed in the open, with the specifications and the reasoning
behind them in the repository rather than in somebody's head.

## What makes a principle binding

A principle is an intention until a requirement enforces it.

| Principle | Enforced by |
| --- | --- |
| 1. A composable engine | `R-TUI-030` to `R-TUI-034`, `R-TUI-080`, `R-TUI-081` |
| 2. Task execution | `R-AGENT-002`, `R-TUI-074` |
| 4. No lock-in | `docs/spec/llm.md`, `R-LLM-001` to `R-LLM-004` |
| 5. Local-first | `docs/spec/store.md`, `docs/spec/index.md`, `R-PKG-020` |
| 6. Configuration is code | `R-STAR-001`, and `docs/spec/starlark.md` throughout |
| 7. Auditable | `docs/spec/session.md`, `R-TUI-072` |
| 8. Intelligent context | `docs/spec/index.md` |
| 9. Security | `R-AUTH-001` to `R-AUTH-014`, `R-AUTH-030` to `R-AUTH-034`, `R-STAR-084` |
| 10. Predictable cost | `R-AGENT-010` to `R-AGENT-017` |

Three rows above are partly unenforced, and those are #166, #167, and #168. A
principle with no requirement behind it is a preference; a principle whose
requirement the code fails is a bug.
