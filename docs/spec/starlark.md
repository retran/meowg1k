# Starlark

Status: draft
Elaborates: docs/design/0.3.0-starlark-api.md, all sections; docs/design/0.3.0-architecture.md section 3

## Scope

`meow-star` is the surface users write against. It loads `.meow/`, evaluates
the declarations it finds, and turns them into the specs and tools the engine
runs. It also implements the standard library modules a handler imports.

It depends on `meow-agent` and never the other way round.

## Boundary

The `meow` global, the `@std//` modules, the handler context, the loading
scheme, and the diagnostics a user sees when any of it is wrong.

## Requirements

### Discovery and loading

**[R-STAR-001]** The workspace root MUST be the nearest ancestor directory of
the working directory containing `.meow/meow.star`. Discovery MUST stop at the
first match and MUST NOT merge with a global configuration.

**[R-STAR-002]** Running outside any workspace MUST fail with an error naming
the directories searched, and MUST suggest `meow init`.

**[R-STAR-003]** `load("@std//<name>", ...)` MUST resolve to a runtime module,
and a name with no module MUST fail listing the available modules.

**[R-STAR-004]** `load("//<path>", ...)` MUST resolve relative to `.meow/`,
and MUST fail for a path that escapes it.

**[R-STAR-005]** `load("@<pkg>//<path>", ...)` MUST fail with an error stating
that packages are not implemented, and MUST NOT be interpreted as a local
path.

**[R-STAR-006]** Loading MUST detect an import cycle and fail with the cycle
listed in order.

**[R-STAR-007]** A file MUST be evaluated at most once per invocation,
whatever the number of `load` statements naming it.

### The module table

**[R-STAR-010]** Runtime modules MUST be registered in one table, and every
consumer MUST obtain a module from that table. No code path may construct a
context or module set of its own.

**[R-STAR-011]** A tool handler invoked inside an agent loop MUST see exactly
the same modules, with the same behaviour, as a handler invoked from the
command line.

### The handler context

**[R-STAR-020]** The handler context MUST expose exactly six members: `args`,
`session`, `out`, `ask`, `stdin`, and `workspace`, plus `cancelled()`.

**[R-STAR-021]** Runtime capabilities MUST NOT be members of the context. A
handler reaches them by `load`.

### Declarations

**[R-STAR-030]** `meow.provider`, `meow.model`, `meow.agent`, `meow.tool`,
`meow.command`, and `meow.policy` MUST be callable only while `.meow/` is
being evaluated, and MUST fail inside a handler.

**[R-STAR-031]** Declaring two providers, models, agents, or tools with the
same name MUST fail naming both declaration sites.

**[R-STAR-032]** A declaration that names a provider, model, or tool that does
not exist MUST fail at load time, not at first use.

**[R-STAR-033]** A command whose name collides with a built-in MUST fail at
load time naming the collision, and MUST NOT shadow the built-in.

### Agents

**[R-STAR-040]** `meow.agent` MUST require `name`, `model`, and `system`, and
MUST accept `about`, `tools`, `budget`, `compaction`, `output`,
`on_tool_error`, and `policy`.

**[R-STAR-041]** An agent value MUST be usable in another agent's `tools`
list, and MUST present the same schema there as a tool declared with
`meow.tool`.

**[R-STAR-042]** `agent.run(task, ...)` MUST return a value carrying `text`,
`value`, `stop`, `ok`, `usage`, `steps`, and `session`, with `stop` taking one
of the six values in [R-AGENT-002].

**[R-STAR-043]** `agent.call(...)` MUST build an invocation without running
it, and `meow.parallel([...])` MUST accept a list of such invocations.

**[R-STAR-044]** `meow.parallel` MUST reject anything that is not an
invocation, including a Starlark function, with an error explaining that
Starlark values cannot cross a thread boundary.

### Markdown agents

**[R-STAR-050]** A `.md` file under `.meow/agents/` MUST declare an agent
whose frontmatter keys are exactly the keyword arguments of `meow.agent` and
whose body is the system prompt.

**[R-STAR-051]** A markdown agent and a Starlark agent MUST produce the same
value, and MUST be indistinguishable to a caller.

**[R-STAR-052]** Frontmatter that is not valid YAML, or that carries a key
`meow.agent` does not accept, MUST fail at load time with the file and line.

**[R-STAR-053]** A `.md` file under `.meow/lib/` MUST be loadable as a string.

### Tools and arguments

**[R-STAR-060]** `meow.arg` MUST support the types `string`, `int`, `float`,
`bool`, `enum`, and `list`, each with an optional default, an optional
`about`, and type-appropriate constraints.

**[R-STAR-061]** One argument declaration MUST produce the command-line flag,
the help text, and the JSON Schema sent to the model, so the three cannot
drift.

**[R-STAR-062]** An argument marked `positional` MUST take an index, and
indices within one tool MUST be unique and contiguous from zero.

**[R-STAR-063]** A declared constraint MUST be enforced on the command line
and on a model-supplied argument alike.

### Schemas

**[R-STAR-070]** `meow.schema` MUST build object, list, string, int, float,
bool, and enum schemas, and MUST emit valid JSON Schema.

**[R-STAR-071]** A schema naming a required field it does not declare MUST
fail when the schema is built.

### Evaluation model

**[R-STAR-080]** Each invocation MUST own one evaluator, and the runtime MUST
NOT move an evaluator or a Starlark value between threads.

**[R-STAR-081]** A builtin that performs input or output MUST block the
calling script thread and MUST NOT expose a future or a callback to Starlark.

**[R-STAR-082]** `print` MUST fail with an error directing the caller to
`ctx.out`.

**[R-STAR-083]** Evaluation of `.meow/` MUST be free of side effects outside
the declaration registry: a declaration file MUST NOT read files, run
commands, or make requests.

### Diagnostics

**[R-STAR-090]** Every load-time and run-time error MUST carry the file, line,
and column of the Starlark expression that caused it.

**[R-STAR-091]** An error naming an unknown parameter, module, or type MUST
suggest the closest declared name when one is within a small edit distance.

## Changes from v0.2.x

The handler context carries 26 members and is assembled by hand in two places,
`ctx_run.go` and `module_llm.go`. They have already diverged on UI nesting
depth, and a module added to one is silently missing from tools running inside
an agent loop. [R-STAR-010], [R-STAR-011], and [R-STAR-020] replace both with
one table and a six-member context.

Presets are gone: a named model is what a preset was.

`meow.param` becomes `meow.arg` with real types, and the same declaration now
drives the flag, the help, and the model schema.

Markdown agents are new. In v0.2.x every agent is a Starlark file, and the
shipped `lib/agent.star` exists only to hide the boilerplate that makes one
work.

## Open questions

- **Whether declaration files may read the environment.** `env.require` is
  needed for credentials, and it is a side effect. Recommendation: allow
  `env`, forbid the rest, and note that [R-STAR-083] carves out exactly this.
- **Whether a handler may declare a tool at run time.** It would allow
  generated tools; it also makes the tool set unknowable before a run and
  breaks `meow policy explain`. Recommendation: forbid it.
- **`.md` agents and `load`.** A markdown agent cannot import a shared prompt
  the way a Starlark one can. Recommendation: allow a frontmatter key that
  names `.meow/lib/*.md` files to prepend, rather than inventing templating.
