# Starlark

Status: approved 2026-09-19
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

**[R-STAR-012]** The table MUST contain `re` and `time`, and each MUST behave
identically in a handler invoked from the command line and in one invoked
inside an agent loop.

**[R-STAR-013]** `re.match`, `re.find_all`, `re.replace`, and `re.split` MUST
accept a pattern and a subject, and MUST fail at the call with the pattern and
the reason when the pattern does not compile. A pattern that compiles but
matches nothing MUST NOT be an error: `match` MUST return `None`, and
`find_all` and `split` MUST return a list.

**[R-STAR-014]** `time.now`, `time.parse`, `time.format`, and `time.since` MUST
work in a single scale - a UTC instant counted in seconds - and MUST NOT accept
or return a local time. A handler that reports a duration MUST get the same
number whatever the machine's zone.

**[R-STAR-015]** The table MUST contain `yaml`, `toml`, `csv`, and `xml`, and
each MUST expose `parse` and `encode`. Text that the format rejects MUST fail
at the call with the reason, and MUST NOT be returned as a partial value.

**[R-STAR-016]** `yaml.parse`, `toml.parse`, and `json.parse` MUST produce the
same Starlark value for documents that describe the same data, and each
`encode` MUST accept any value the other two produce. A handler MUST be able to
read one format and write another without knowing which it read.

**[R-STAR-017]** `csv.parse` MUST return a list of dictionaries when the first
record names the columns and a list of lists when it does not, and MUST fail
naming the record number when a record's length disagrees with the header.
`csv.encode` MUST accept either shape.

**[R-STAR-018]** `xml.parse` MUST return a tree in which every element carries
its `tag`, its `attrs`, its `children`, and its `text`, and MUST NOT flatten an
element into a dictionary. `xml.encode` MUST accept that tree, and equally a
tree of dictionaries carrying the same four keys, and MUST escape text and
attribute values.

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
not exist MUST fail at load time, not at first use. References MUST be resolved
after every declaration file has been evaluated, so that declaration order
inside and between files does not matter.

**[R-STAR-033]** A command whose name collides with a built-in MUST fail at
load time naming the collision, and MUST NOT shadow the built-in.

**[R-STAR-034]** `meow.model` MUST accept a `kind` of `chat` or `embedding`,
defaulting to `chat`. An agent naming an embedding model, or an index naming a
chat model, MUST fail at load time naming both the model and the kind it is.

> Added 2026-09-20. The index needs an embedding model and nothing said how a
> workspace names one, so [R-INDEX-051] could record which model built an index
> that no declaration could choose.

**[R-STAR-035]** `meow.index` MUST declare which model embeds the workspace and
MAY set the chunk size, the overlap, and the file-size limit that [R-INDEX-003]
and [R-INDEX-012] call configured. It MUST be declarable at most once, and an
index command in a workspace that declares none MUST fail saying so rather than
choosing a model.

> Added 2026-09-20 alongside [R-STAR-034]. Choosing a model for somebody is
> how an index gets built by one model and queried by another, which
> [R-INDEX-051] exists to catch after the fact and this prevents.

### Agents

**[R-STAR-040]** `meow.agent` MUST require `name`, `model`, and `system`, and
MUST accept `about`, `tools`, `budget`, `compaction`, `output`,
`on_tool_error`, and `policy`.

**[R-STAR-041]** An agent value MUST be usable in another agent's `tools`
list, and MUST present the same schema there as a tool declared with
`meow.tool`.

**[R-STAR-042]** `agent.run(task, ...)` MUST return a value carrying `text`,
`value`, `stop`, `detail`, `ok`, `usage`, `steps`, and `session`, with `stop`
taking one of the six values in [R-AGENT-002] and `detail` carrying the
explanation required by [R-AGENT-004].

**[R-STAR-043]** `agent.call(...)` MUST build an invocation without running
it, and `meow.parallel([...])` MUST accept a list of such invocations.

**[R-STAR-044]** `meow.parallel` MUST reject anything that is not an
invocation, including a Starlark function, with an error explaining that
Starlark values cannot cross a thread boundary.

### Markdown agents

**[R-STAR-050]** A `.md` file under `.meow/agents/` MUST declare an agent
whose frontmatter accepts every keyword argument of `meow.agent` except
`system`, plus `include`. The body supplies `system`, so frontmatter carrying
it MUST fail.

**[R-STAR-051]** A markdown agent and a Starlark agent MUST produce the same
value, and MUST be indistinguishable to a caller.

**[R-STAR-052]** Frontmatter that is not valid YAML, or that carries a key
outside the set [R-STAR-050] allows, MUST fail at load time with the file and
line.

**[R-STAR-053]** A `.md` file under `.meow/lib/` MUST be loadable as a string.

**[R-STAR-054]** Frontmatter MAY carry an `include` key naming `.meow/lib/*.md`
files. Their contents MUST be prepended to the system prompt in the order
given, separated by a blank line. `include` MUST be the only composition a
markdown agent has: there MUST be no substitution, conditional, or loop.

### Tools and arguments

**[R-STAR-060]** `meow.arg` MUST support the types `string`, `int`, `float`,
`bool`, `enum`, and `list`, each with an optional default and an optional
`about`. `string` MUST accept `max_len` and `pattern`; `int` and `float` MUST
accept `min` and `max`; `enum` MUST require its list of values; `list` MUST
require an element type.

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

**[R-STAR-083]** A declaration file MUST NOT write files, run commands, or
make network requests.

**[R-STAR-084]** While `.meow/` is being evaluated, only `load` and `@std//env`
MUST be callable. Every other runtime module MUST fail with an error saying it
is unavailable during declaration. `@std//env` is carved out because
credentials are resolved there.

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

## Decisions

**Declaration files may read the environment and nothing else**, by
[R-STAR-084]. Credentials are resolved there, so forbidding it outright would
make the normal configuration impossible. Every other module stays unavailable,
so loading `.meow/` cannot have consequences.

**A handler may not declare a tool**, by [R-STAR-030]. Generated tools would be
useful and would make the tool set unknowable before a run, which breaks
`meow policy explain` and with it the promise that a permission decision can be
predicted without triggering it.

**`re` and `time` return one scale each**, by [R-STAR-013] and [R-STAR-014].
A regular expression module that sometimes returns a string and sometimes a
list forces every caller to test the type first, and a time module that knows
about zones turns every comparison into a question about where the machine is.
So a match is a list of groups or `None`, and an instant is seconds in UTC.
Formatting for a human is what `time.format` is for, and it is the only place a
zone could ever enter.

**XML is a tree and not a dictionary**, by [R-STAR-018]. Every library that
maps XML onto the shape JSON has must decide what to do when an element has
both attributes and children, or two children with one tag, and every such
decision is wrong for some document. So `xml.parse` returns what XML actually
is - a tag, attributes, ordered children, text - and a handler that wants a
dictionary writes the three lines that build one, knowing its own document.

**A CSV with a header is a list of dictionaries**, by [R-STAR-017]. The
alternative is to return rows and a separate header list and make every caller
zip them, which is the same work done once per handler instead of once here. A
file whose first record is data rather than names is the case `header = False`
exists for.

**Markdown agents compose by inclusion only**, by [R-STAR-054]. A shared prompt
is the real need; substitution and conditionals are how a configuration format
turns into a bad programming language. An agent that needs logic is a Starlark
agent.
