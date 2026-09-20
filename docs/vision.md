# What meowg1k is

Status: describes `dev` as it stands. Not a plan.

meowg1k is a runtime for AI commands you write yourself. The binary supplies
the parts that are hard to build and boring to rebuild: model providers, a
session log, budgets, a permission layer, retrieval over your code, and a
terminal that behaves. You supply the commands. They go in Starlark, in a
`.meow/` directory beside the code they work on.

Read [philosophy.md](philosophy.md) for the principles behind the design, and
[spec/](spec/README.md) for what the binary is required to do.

## The ratio is the point

This repository is its own best example. About 38,000 lines of Rust across ten
crates provide the runtime. About 360 lines of Starlark and markdown in
`.meow/` give this project `meow review`, `meow commit`, `meow ask`, and
`meow status`.

Those 360 lines are the whole of what you have to read to know what this
project's AI tooling does. They live in the repository, they go through code
review, and changing one does not mean rebuilding a binary.

The engine is large so that the workspace can be small. When the two conflict,
the workspace wins.

## Who this is for

Someone who wants their AI workflows to be code.

If you are happy operating a tool through its own interface, meowg1k asks more
of you than you need. It asks you to write a handler and declare it. In return
your workflow is a file you can diff, review, and hand to a colleague, and a
command you can put in front of `&&`.

The person this suits already thinks in scripts. They want `meow review` to
gate a commit, and `meow ask` to answer from the code rather than from a
model's memory of code like it. They want both to behave the same on a laptop
and in CI.

## What a workspace looks like

Three declarations carry the surface:

- `meow.tool(name, about, run, args)` is a typed callable. A model can call
  it, and `meow.command` can put it on the command line - one object, two
  roles.
- `meow.agent(...)` is a model that can call tools, with a budget, a policy,
  and a schema for what it returns. An agent can also be a markdown file with
  front matter, which is the same agent by a shorter route.
- `meow.command(tool)` is what shows up in `meow --help`.

A handler reaches the world through seventeen `@std//` modules. The file
system and the shell are confined to the workspace. The rest are `git`,
semantic and literal search, the index, HTTP, a durable key-value store, and
encoders for JSON, YAML, TOML, CSV, and XML.

## What the binary does that a script cannot

### Sessions

Every run appends to a log - each turn, each tool call, each
model response, each policy decision. You can show one, export one, fork one at
the step where it went wrong, and rerun from there with a different model.

### Budgets

An agent runs under limits on steps, tokens, and wall time. A
fan-out shares its caller's ledger, so six branches against a caller with three
steps get three between them rather than eighteen.

### Policy

Rules decide whether a model may make a given call. Deny beats ask
beats allow, and nothing matching means deny, so a workspace that declares no
policy asks about everything rather than permitting it. `--dry-run` plans the
calls and makes none.

### Retrieval

`meow index build` walks your workspace under its ignore files,
chunks it, embeds it, and builds a vector graph on disk. A handler asks by
meaning with `search.code`, or by text with `search.text`, which needs no index
at all.

### Trust

A `.meow/` in a repository you just cloned is code with tool access.
The first run shows what it declares and asks once, and asks again when those
declarations change.

## What it is not

### Not a chatbot

A run is a task with an end and a stop reason. Continuing is
explicit: `--continue` resumes the most recent session of the command you are
invoking, and fails when that command has none to resume.

### Not a plugin host

A package is Starlark under the same rules as code you
wrote. Nothing loads native code.

### Not tied to a vendor

Seven provider kinds ship, and a local `llama.cpp`
server is one of them. Every vendor that copied OpenAI's shape is served by one
implementation that differs only in its address.

### Not a service

The index, the sessions, the blobs, and the cache are one
SQLite file in your workspace. Nothing phones home. The only thing that needs a
network is the model, and that can be a process on your own machine.

## Where it came from

`v0.2.1` was the last Go release and is tagged. `v0.3.0` is a rewrite rather
than a port: ten crates written against 315 requirements in `docs/spec/`, with
`docs/design/` recording the decisions that produced them.

The rewrite happened because the Go implementation had accumulated behaviour
nobody decided on. Its workflows were 15,000 lines of Go that users could not
change without rebuilding; the same work is now Starlark they can read. Its
guides described modules that did not exist. It had no permission layer at all,
so an agent talked into running a command ran it.

[CHANGELOG.md](../CHANGELOG.md) has the difference in full.
