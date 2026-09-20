# Changelog

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the version numbers follow [semantic versioning](https://semver.org/).

## [0.3.0] - 2026-09-20

`0.3.0` is a rewrite. The Go implementation is gone, and what replaces it is
Rust written against eight specifications rather than a transliteration of the
old code. Read this section as a description of a new program, not as a list of
changes to the old one: nothing in `0.2.x` survives except the idea that you
write your commands in Starlark and the binary supplies the runtime.

### Added

- A Starlark surface built around three declarations: `meow.tool` for a typed
  callable, `meow.agent` for a model that can call tools, and `meow.command`
  for what appears on the CLI. Modules are loaded with `@std//`, `//` for the
  workspace, and `@pkg//` for a dependency; the loader evaluates each module
  once and reports a cycle by naming the path around it.
- Agents written in markdown. A file under `.meow/agents/` with front matter is
  the same agent as one declared in Starlark, because both build the same
  struct through the same constructor.
- An append-only session log. Every turn, tool call, and model response is an
  event; compaction supersedes events without deleting them, so an export
  after compaction still shows what happened.
- A budget with four axes - steps, tokens, wall time, and cost - enforced by a
  ledger that a fan-out shares with its children, so a branch cannot spend what
  the caller no longer has.
- A policy layer that decides whether a model may make a given call, with rules
  that match on the tool, the arguments, and the agent that asked.
- Semantic search over the workspace: chunking, embeddings, and an HNSW graph
  persisted to disk and loaded through a memory map, so a query reads the
  vectors it needs rather than every vector in the corpus.
- An inline terminal UI that scrolls with the shell instead of taking the
  screen, and emits the same event vocabulary live that an export writes to a
  file.
- `meow init`, `meow check`, `meow run`, and the commands your own scripts
  declare.

### Removed

- The Go implementation, and with it the Taskfile, GoReleaser, and every
  module under `internal/`. `v0.2.1` remains tagged and installable; it gets no
  further changes.

## [0.2.1] - 2026-08-30

The final Go release. See the tag for its notes.
