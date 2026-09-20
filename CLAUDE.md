# CLAUDE.md

<role>
Root policy for Claude Code working in the meowg1k repository. This file is
canonical. Anything under `.claude/` adds routing and workflow detail and must
not override this policy. `docs/spec/` is normative about what the binary does;
this file is normative about how to work on it, and the two do not overlap.
</role>

<project>
meowg1k is a script-friendly AI companion CLI. Users define their own commands
in Starlark and their own agents in markdown; the Rust binary supplies the
runtime, the model gateways, the session store, the index, and the terminal.

The tree is Rust. `v0.2.1` was the last Go implementation and is tagged; the
Go code is gone from this branch and the history has it. `docs/spec/` fixes
what the binary does, in ten areas of numbered requirements, and
`docs/design/` records the decisions that produced them.
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
The Starlark surface is what users actually touch; the Rust code exists to
serve it. A change that makes a crate tidier but leaves `agent.run` harder to
write agents against is a regression. When the two conflict, the Starlark API
wins.

`.meow/` in this repository is the test of it. If a change makes the workspace
meowg1k uses on itself worse to read, the change is wrong however good the
internals look.
</principle>

<principle name="docs_must_match_code">
The v0.2.x guides described modules and commands that did not exist, and they
are deleted rather than corrected. Do not start a third account of the system:
`docs/spec/` says what the binary does, `docs/design/` says why, and the code
says how. If you touch a subsystem and find a document describing something
else, fix it in the same change or delete the stale section.
</principle>

<principle name="one_context_builder">
v0.2.x assembled the handler context in two places, `ctx_run.go` and
`module_llm.go`, and they had already diverged on UI nesting depth: a module
added to one was silently missing from tools running inside an agent loop.

`crates/meow-star/src/modules.rs` is the one table now, and
`crates/meow-star/src/context.rs` builds the one context. A second place for
either is the defect, whatever it buys.
</principle>

<principle name="no_stdout_behind_the_tui">
`println!` and `eprintln!` write straight through the live frame and corrupt
it, which is how v0.2.x lost its display to ten `log.Printf` calls in the agent
loop. The workspace lint denies both outside `meow-cli`. Diagnostics go through
the event stream, which every renderer handles and which puts them in
scrollback in order.
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

Nine crates, and the dependency direction is one way:

| Crate | Holds | Depends on |
| --- | --- | --- |
| `meow-core` | Types, no behaviour, no input or output | nothing |
| `meow-store` | One SQLite database: blobs, sessions, cache, index rows | `meow-core` |
| `meow-session` | The append-only log, forks, retention, export | `meow-store` |
| `meow-llm` | What a provider is, and five that are | `meow-core` |
| `meow-policy` | What an agent may do, and who decides | `meow-core` |
| `meow-agent` | The loop: budgets, tools, compaction, sub-agents | `meow-llm`, `meow-policy` |
| `meow-index` | Walking, chunking, embedding, retrieval | `meow-store` |
| `meow-star` | The Starlark surface and the thread a handler runs on | `meow-agent` |
| `meow-ui` | Three renderers over one event stream | `meow-core` |
| `meow-cli` | The binary: the command line, the wiring, the process | everything |

Three boundaries carry the design, and crossing one is a defect however small
it looks:

- `meow-core` depends on no workspace crate and performs no input or output.
- `meow-agent` does not know Starlark exists. `meow-star` depends on it, never
  the reverse, which is what lets the engine be tested without a script.
- `meow-ui` knows about the event types and nothing else. It does not know the
  engine exists, which is what lets a renderer be driven from a recorded log.

What `meow-star` needs from crates that depend on it - the terminal, the
session log, the index - arrives as a trait. That is also what lets a test
drive a handler with no terminal and no account.

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

Runtime modules are registered in `crates/meow-star/src/modules.rs`. There is
one table and every consumer takes a module from it, which is what
`[R-STAR-010]` asks for. To add one:

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

mise owns the toolchain and the tasks; there is no Taskfile.

```bash
mise install            # the toolchain and every tool a task invokes
mise run all            # everything CI runs, in parallel
mise run check          # clippy, with -D warnings
mise run test           # nextest across the workspace
mise run fmt            # rustfmt
mise run deny           # advisories, licences, bans, sources
mise run build          # -> target/debug/meow
```

`mise run all` is the gate. It runs the same seven things CI does, so a green
run here means a green run there; if they ever disagree, that is a defect in
one of them and not something to route around.

Two tool choices worth knowing. `cargo-nextest` comes from the prebuilt
`github:nextest-rs/nextest` backend rather than `cargo:`, because building it
from source compiles `aws-lc-sys` and needs a C toolchain and cmake. And
`cargo-deny` needs `allow-wildcard-paths` together with `publish = false` on
every library crate, or it reads an intra-workspace path dependency as an
unpinned one.

`deny.toml` carries five ignored advisories, each with its reason. All are
"unmaintained" rather than vulnerable, all arrive through a pinned dependency
with no safe upgrade, and they are listed one by one so a new advisory against
a direct dependency still fails the check.

</build>

<conventions>

- Apache 2.0 header on every Rust file; `LICENSE_HEADER.txt` is the template.
- One `thiserror` enum per crate. Variants named for what went wrong rather
  than for where, and each carries enough to act on.
- Never `unwrap` or `expect` outside tests. When an invariant truly cannot
  fail, restructure so the compiler sees it; where that is impossible, the
  message says which invariant and why.
- `clippy.toml` exempts `#[test]` functions from the `unwrap` denial and not
  the helpers beside them, so an integration test file with helpers needs a
  file-level `#![allow(clippy::unwrap_used)]`.
- Every test that checks a requirement names it in a doc comment:
  `/// [R-SESSION-014] compaction supersedes without deleting`.
- Test the failure paths as carefully as the success path. Missing arguments,
  cancellation mid-call, exhausted budgets, malformed model output, and storage
  failures are where the defects are.
- Do not test private internals. A test that reaches past a public API makes
  the crate hard to change and proves nothing a user could observe.
- An `#[allow]` needs a comment saying why. A derive that emits an `unsafe
  impl` needs the allow at module scope, because an attribute on the struct
  does not cover a sibling item.

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

Keep this file, `docs/spec/`, and `docs/design/` synchronized with the code.
When you change:

- **behaviour** - find the requirement in `docs/spec/` first. If there is none,
  write one; if the code would contradict one, amend it in place with the date
  and the reason, and say so in the pull request.
- **the Starlark API** - update `docs/design/0.3.0-starlark-api.md`, and check
  whether `.meow/` in this repository still reads well against it.
- **architecture or wiring** - update `docs/design/0.3.0-architecture.md` and
  the table above.
- **build or lint configuration** - update the `<build>` section above and
  `CONTRIBUTING.md`.

This file is the only instruction file in the repository. If a tool wants its
own, point it here instead of adding a second source of truth.

</maintenance>
