# CLAUDE.md

<role>
Root policy for Claude Code working in the meowg1k repository. This file is
canonical. Anything under `.claude/` adds routing and workflow detail and must
not override this policy. Files under `docs/guides/` are reference material, not
policy - where they disagree with this file, this file wins and the guide gets
fixed.
</role>

<project>
meowg1k is a script-friendly AI companion CLI. Users define their own commands in
Starlark; the Go binary supplies the runtime, the LLM gateways, the session
store, and the terminal UI.

The repository is mid-transition. `v0.2.1` is the final Go implementation and is
tagged. `v0.3.0` will be a ground-up redesign in canonical Rust - not a
transliteration of the Go code. The design is in `docs/design/`:
`0.3.0-architecture.md`, `0.3.0-starlark-api.md`, `0.3.0-sessions.md`, and
`0.3.0-tui.md`. Until the Rust tree exists, everything below describes the Go
codebase.
</project>

<principles>

<principle name="always_technical_english">
Load the `technical-english` skill at the start of every conversation, before
writing anything, and keep it loaded. It governs all prose you produce here -
design documents, specifications, code comments, commit messages, pull request
bodies, issue text, and your own replies in chat. This is not conditional on
the task looking like a writing task; a commit message is prose and a chat
reply is prose.
</principle>

<principle name="spec_first">
No behaviour ships that a specification does not describe. Before writing code
whose behaviour no requirement in `docs/spec/` covers, stop and write the
requirement. When an implementation contradicts a requirement, stop and run
`/amend-spec`; never write code the spec forbids and reword the spec
afterwards. The `spec-driven` skill has the method.
</principle>

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
section - never leave a third variant behind.
</principle>

<principle name="one_context_builder">
The handler context (`ctx.fs`, `ctx.llm`, `ctx.git`, ...) is assembled in two
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
`dev` is the default branch and the only long-lived one. Work on a feature
branch off `dev`, open a pull request, and squash merge it. Never commit to
`dev` directly, including for a one-line fix. Conventional commit subjects. The
`scm` skill has the branch names, the pull request body, and the merge rules.
</principle>

<principle name="no_ai_attribution">
Never mention Claude, Claude Code, or any AI tool in a commit message, a pull
request title or body, a review comment, an issue, a tag annotation, or release
notes. No `Co-Authored-By` trailer naming an AI, no "Generated with" footer, no
`noreply@anthropic.com`, and no paraphrase. This overrides the default harness
guidance that asks for such a trailer. The word "Claude" is allowed only when
it names a model the code talks to, such as a model id in a config file.

Check your own message before committing. Match the attribution patterns, not
the bare word, so a path such as `.claude/commands/` does not trip it:

```bash
git log -1 --format=%B |
  grep -iE 'co-authored-by.*(claude|anthropic|copilot)|generated with|noreply@anthropic'
```
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
`internal/adapters/gateway` directly for the embeddings factory - treat that as
a known defect, not a precedent.

</architecture>

<starlark_runtime>

A user command is a Starlark file under `.meow/` that calls `meow.tool(...)` to
declare typed arguments and a handler, then `meow.command(...)` to put it on the
command line. `.meow/meow.star` is the entry point: it declares providers,
models, the index, the policy, and the commands. An agent is a markdown file
under `.meow/agents/` whose frontmatter carries its settings and whose body is
its system prompt; shared prompt fragments live in `.meow/lib/*.md`.

This repository uses its own workspace on itself, which is the acceptance test
for the design. `meow review` reviews what is staged, `meow commit` writes a
message for it, and `meow ask` answers a question about the code.

Runtime modules are registered in `internal/core/starlark/module_*.go` for the
Go tree and in `crates/meow-star/src/modules.rs` for the Rust one. In Rust there
is one table and every consumer takes a module from it, which is what
`[R-STAR-010]` asks for; the Go tree assembles a context by hand in two places
and they have already drifted. To add a module to the Rust tree:

1. Write the `#[starlark_module]` function, taking `eval` and calling
   `running(eval, "<module>.<call>")` first so it is refused during
   declaration, per `[R-STAR-084]`.
2. Register it in `Modules::build` and add its name to `NAMES`.
3. Test each builtin in `crates/meow-star/tests/running.rs`, including the
   argument errors.
4. Document it in `docs/design/0.3.0-starlark-api.md` section 10.

The agentic loop is `meow-agent`'s `Engine`. It returns an `Outcome` carrying
the stop reason, the usage, and the parsed value, so a caller can tell a
finished answer from a budget stop; v0.2.x returned a bare string and could not.

</starlark_runtime>

<build>

The Go tree builds with Task; the toolchain comes from mise.

```bash
mise install            # golangci-lint; Go itself is already on the machine
task check:all          # lint, test, security - these run in parallel
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

`v0.3.0` replaces Task with mise tasks outright - do not invest in the Taskfile.

</build>

<conventions>

- Apache 2.0 header on every Go file; `LICENSE_HEADER.txt` is the template.
- Wrap errors with `fmt.Errorf("...: %w", err)` and enough context to locate the
  call site without a stack trace.
- Table-driven tests with testify. `require` for preconditions, `assert` for
  the assertions under test.
- Coverage gate is 65%. `internal/domain/`, `internal/ports/`, and
  `internal/templates/` have no tests at all; new domain logic needs them.
- `golangci-lint` passes on the Go tree. `goconst` is set to six occurrences
  and skips tests, because at the default of three a map key used in three
  places counted as a magic string; `internal/core/starlark/` is excluded from
  it entirely, since the repetition there is builtin names rather than
  literals.

</conventions>

<workflow>

Work is specification-driven. A change moves through five steps, each with a
command:

| Command | Does | Stops at |
| --- | --- | --- |
| `/spec` | Writes or extends a component spec in `docs/spec/` | Human approval |
| `/plan` | Decomposes an approved spec into issues with dependencies | Human approval, before creating anything on GitHub |
| `/implement` | Builds one issue on a feature branch | A pull request, never a merge |
| `/verify` | Checks code against spec in both directions | A report, fixes nothing |
| `/review` | Reviews a pull request against its spec and conventions | A verdict |
| `/amend-spec` | Proposes a spec change after implementation contradicted it | Human approval |

The skills in `.claude/skills/`:

- `technical-english` - all prose. Always loaded, see the principle above.
- `spec-driven` - requirement identifiers, traceability, the divergence
  protocol.
- `scm` - branches, commits, pull requests, squash merges, the attribution ban.
- `rust` - crate boundaries, errors, the async bridge, tests. For v0.3.0 code.

</workflow>

<maintenance>

Keep this file and `docs/` synchronized with the code. When you change:

- **the Starlark API** - update `docs/api/API_REFERENCE.md` and
  `docs/guides/starlark-system.md`
- **the agentic loop or session model** - update
  `docs/guides/agentic-system.md`
- **architecture or wiring** - update `docs/guides/architecture.md` and the
  table above
- **build or lint configuration** - update the `<build>` section above and
  `CONTRIBUTING.md`

This file is the only instruction file in the repository. If a tool wants its
own, point it here instead of adding a second source of truth.

</maintenance>
