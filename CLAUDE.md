# CLAUDE.md

<role>
Root policy for Claude Code working in the meowg1k repository. This file is
canonical. Anything under `.claude/` adds routing and workflow detail and must
not override this policy. Files under `docs/guides/` are reference material, not
policy — where they disagree with this file, this file wins and the guide gets
fixed.
</role>

<project>
meowg1k is a script-friendly AI companion CLI. Users define their own commands in
Starlark; the Go binary supplies the runtime, the LLM gateways, the session
store, and the terminal UI.

The repository is mid-transition. `v0.2.1` is the final Go implementation and is
tagged. `v0.3.0` will be a ground-up redesign in canonical Rust — not a
transliteration of the Go code. See `docs/rust-rewrite.md` for the plan. Until
the Rust tree exists, everything below describes the Go codebase.
</project>

<principles>

<principle name="starlark_api_is_the_product">
The Starlark surface is what users actually touch; the Go code exists to serve
it. A change that makes the Go internals tidier but leaves `ctx.llm.agent_turn`
harder to write agents against is a regression. When the two conflict, the
Starlark API wins.
</principle>

<principle name="docs_must_match_code">
`docs/` currently documents modules and commands that do not exist
(`//lib/tools.star`, `//lib/formatting.star`, `review-agent.star`,
`orchestrator-agent.star`, `meow sessions`, `meow show-session`). Do not extend
that drift. If you touch a subsystem and find its guide describes something
other than the code, fix the guide in the same change or delete the stale
section — never leave a third variant behind.
</principle>

<principle name="one_context_builder">
The handler context (`ctx.fs`, `ctx.llm`, `ctx.git`, …) is assembled in two
places: `internal/core/starlark/ctx_run.go` for `ctx.run()` and
`internal/core/starlark/module_llm.go` for tools invoked inside an agentic
loop. They are near-identical literals and have already diverged on UI depth.
Adding a module to one and not the other silently breaks agents. Any new context
member goes through a single shared constructor, or into both, never one.
</principle>

<principle name="no_stdout_behind_the_tui">
`log.Printf` and `fmt.Print` write straight through the Bubble Tea frame and
corrupt the display. Route diagnostics through the progress logger
(`internal/adapters/progress`) or the trace log
(`internal/adapters/tracelog`).
</principle>

<principle name="reviewable_history">
Branch off `dev`; do not commit to it directly for anything beyond a trivial
fix. Conventional commit subjects. No AI attribution in commit messages or PR
bodies — no `Co-Authored-By: Claude`, no "Generated with" footer.
</principle>

</principles>

<architecture>

Hexagonal, ports and adapters:

| Layer | Path | Holds |
| --- | --- | --- |
| Domain | `internal/domain/` | Types with no behaviour and no imports outward |
| Ports | `internal/ports/` | Interfaces the core depends on |
| Core | `internal/core/` | Business logic: starlark runtime, retrieval, chunking, sessions, presets |
| Adapters | `internal/adapters/` | LLM gateways, SQLite, git, HTTP, terminal output |
| App | `internal/app/container.go` | Dependency wiring |
| UI | `internal/ui/` | Bubble Tea widgets and rendering |
| CMD | `cmd/` | Cobra entry points; Starlark commands are registered dynamically |

The direction of dependency is inward only. Core imports ports, never adapters.
`internal/core/starlark/module_llm.go` currently breaks this by importing
`internal/adapters/gateway` directly for the embeddings factory — treat that as
a known defect, not a precedent.

</architecture>

<starlark_runtime>

A user command is a Starlark file under `.meowg1k/commands/` that calls
`meow.tool(...)` to declare typed parameters and a handler, then
`meow.command(...)` to expose it on the CLI. Shared helpers live in
`.meowg1k/lib/`. `.meowg1k/init.star` declares providers, models, and presets
and loads every command.

Modules are registered in `internal/core/starlark/module_*.go` and surfaced on
the handler context. To add one:

1. Write `module_<name>.go` with a `New<Name>Module()` returning a
   `starlarkstruct`.
2. Add it to **both** context builders (see the `one_context_builder`
   principle).
3. Add `module_<name>_test.go` exercising each builtin, including argument
   errors.
4. Document it in `docs/api/API_REFERENCE.md`.

The agentic loop is `ctx.llm.agent_turn()` in `module_llm.go`. It returns a bare
string, which is why callers cannot distinguish "the model finished" from "we
hit `max_iterations`" from "the model returned empty text". Treat its result
type as the thing to fix, not to work around.

</starlark_runtime>

<build>

The Go tree builds with Task; the toolchain comes from mise.

```bash
mise install            # golangci-lint; Go itself is already on the machine
task check:all          # lint, test, security — these run in parallel
task check:test         # go test with -race and the 65% coverage gate
task check:lint         # golangci-lint run
task fix:fmt            # golangci-lint fmt (goimports)
task build              # -> bin/meow
```

Two things the docs get wrong and you should not repeat: `gofumpt` is named
throughout `CONTRIBUTING.md` and `docs/guides/go-conventions.md` but is not in
the `formatters` block of `.golangci.yaml`, so nothing enforces it; and
`task check:security` calls `gosec` and `govulncheck` as bare binaries, which
only exist after `task tools:install`.

`v0.3.0` replaces Task with mise tasks outright — do not invest in the Taskfile.

</build>

<conventions>

- Apache 2.0 header on every Go file; `LICENSE_HEADER.txt` is the template.
- Wrap errors with `fmt.Errorf("...: %w", err)` and enough context to locate the
  call site without a stack trace.
- Table-driven tests with testify. `require` for preconditions, `assert` for
  the assertions under test.
- Coverage gate is 65%. `internal/domain/`, `internal/ports/`, and
  `internal/templates/` have no tests at all; new domain logic needs them.
- `golangci-lint` currently reports ~470 `goconst` findings, nearly all in test
  files. They are noise, not a backlog — do not "fix" them by extracting
  constants in tests.

</conventions>

<maintenance>

Keep this file and `docs/` synchronized with the code. When you change:

- **the Starlark API** — update `docs/api/API_REFERENCE.md` and
  `docs/guides/starlark-system.md`
- **the agentic loop or session model** — update
  `docs/guides/agentic-system.md`
- **architecture or wiring** — update `docs/guides/architecture.md` and the
  table above
- **build or lint configuration** — update the `<build>` section above and
  `CONTRIBUTING.md`

`AGENTS.md` was the OpenCode configuration and has been replaced by this file.
If you find a tool still reading it, point that tool here rather than
reintroducing a second source of truth.

</maintenance>
