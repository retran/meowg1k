# meowg1k

A CLI you write yourself. `meow` supplies the runtime - model providers,
sessions, budgets, a policy layer, semantic search, and a terminal UI - and you
supply the commands, in Starlark, in a `.meow/` directory beside your code.

```starlark
def _review(ctx):
    patch = diff(staged = True)
    result = reviewer.run("Review this staged diff.\n\n" + patch)
    for finding in result.value["findings"]:
        ctx.out.finding(finding["severity"], finding["location"], finding["summary"])
    return "false" if result.value["verdict"] == "hold" else "true"

meow.command(meow.tool(name = "review", about = "review the staged changes", run = _review))
```

That becomes `meow review`, with `--help`, an exit code you can put in front of
`&&`, and a session log you can read afterwards.

## Install

Download an archive for your platform from the
[releases page](https://github.com/retran/meowg1k/releases), verify it against
`SHA256SUMS`, and put `meow` on your `PATH`. Or build it:

```bash
cargo install --git https://github.com/retran/meowg1k meow-cli
```

## Start

```bash
meow init      # write a .meow/ with a provider and a model declared
meow check     # load it and say what is wrong
meow doctor    # check the toolchain, the workspace, and the credentials
```

`meow init` leaves a `.meow/meow.star` with a provider and a model already
declared, and nothing else. Add a command by writing a handler, wrapping it in
`meow.tool`, and passing that to `meow.command`; `meow check` tells you what it
thinks of the result.

## What you write against

Three declarations carry the surface:

- `meow.tool(name, about, run, args)` - a typed callable. An agent can call it,
  and `meow.command` can expose it on the CLI.
- `meow.agent(name, model, prompt, tools, ...)` - a model that can call tools,
  with a budget and a schema for what it returns. An agent can also be a
  markdown file under `.meow/agents/`, which is the same agent by a shorter
  route.
- `meow.command(tool)` - what shows up when you run `meow --help`.

Eight `@std//` modules are available to a handler: `env`, `fs`, `git`, `json`,
`path`, `search`, `shell`, and `text`. `fs` and `shell` stay inside the
workspace; `search` asks the index.

## What the binary does for you

**Sessions.** Every run appends to a log: each turn, each tool call, each model
response. `meow session list`, `meow session show`, `meow session export`. A
fork branches a session at one of its events, so you can retry a turn without
losing what came before.

**Budgets.** An agent runs under limits on steps, tokens, wall time, and cost.
A fan-out shares its caller's ledger, so a branch cannot spend what the caller
no longer has.

**Policy.** Rules decide whether a model may make a given call - allow, deny,
or ask. `--dry-run` plans the calls and makes none of them.

**Search.** `meow index build` walks the workspace, chunks it, and embeds it
into an HNSW graph on disk. `search.code(question)` from a handler, or
`meow index query` from the shell.

**Output.** The UI is inline: it scrolls with your shell rather than taking the
screen. `ctx.out.step`, `.note`, `.warn`, `.markdown`, `.finding`, and the rest
render to a terminal, to plain text, or to a JSON event stream, and the live
stream and the export say the same thing.

## Documentation

[`docs/`](docs/README.md) has three kinds of document: the specifications in
`docs/spec/` are normative and every behaviour traces to one, `docs/design/`
records the decisions and the reasoning, and `.meow/` in this repository is
meowg1k configured to work on itself - the worked example that has to keep
working.

## Contributing

[`CONTRIBUTING.md`](CONTRIBUTING.md) has the workflow. In short: `mise install`,
then `mise run all` before you open a pull request, and a requirement in
`docs/spec/` before you write behaviour that none of them describes.

## Versions

`0.3.0` is a rewrite in Rust. `0.2.1` was the final Go release; it remains
tagged and gets no further changes. [`CHANGELOG.md`](CHANGELOG.md) has the
difference.

## Licence

Apache 2.0. See [LICENSE](LICENSE).
