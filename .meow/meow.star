# The workspace meowg1k uses on itself.
#
# Providers and models first, then the commands. Everything a command needs at
# run time is loaded here; `.meow/` is evaluated once and nothing in it reaches
# the world, so a load is a declaration rather than an action.

load("@std//env", "get")
load("@std//git", "diff", "status", "branch", "stage", "commit")
load("@std//fs", "read", "glob")
load("@std//search", "code")
load("@std//text", "truncate")

# -----------------------------------------------------------------------------
# Providers and models
# -----------------------------------------------------------------------------

meow.provider(
    name = "anthropic",
    kind = "anthropic",
    api_key = get("ANTHROPIC_API_KEY"),
)

meow.provider(
    name = "voyage",
    kind = "voyage",
    api_key = get("VOYAGE_API_KEY"),
)

# Two models, not three layers. A named model is what a preset was.
meow.model(
    name = "smart",
    provider = "anthropic",
    id = "claude-sonnet-4-5",
    context = 200000,
    max_output = 64000,
    temperature = 0.2,
)

meow.model(
    name = "fast",
    provider = "anthropic",
    id = "claude-haiku-4-5",
    context = 200000,
    max_output = 16000,
    temperature = 0.0,
)

meow.model(
    name = "embed",
    provider = "voyage",
    id = "voyage-3",
    context = 32000,
    max_output = 0,
    kind = "embedding",
)

meow.index(model = "embed", chunk_lines = 60, overlap = 10)

# -----------------------------------------------------------------------------
# What an agent may do
# -----------------------------------------------------------------------------

# Read anything in the repository, ask before running anything, and never touch
# the store. An empty policy would deny everything, which is the safe default
# and not a useful one.
meow.policy(rules = [
    {"tools": ["read_file", "find_code", "staged_diff"], "decision": "allow"},
    {"tools": ["*"], "decision": "ask"},
])

# -----------------------------------------------------------------------------
# Tools an agent may call
# -----------------------------------------------------------------------------

def _read_file(ctx):
    """Read one file from the repository."""
    return truncate(read(ctx.args.path), 40000)

read_file = meow.tool(
    name = "read_file",
    about = "read a file from the repository, by path relative to its root",
    run = _read_file,
    args = {"path": meow.arg.string(about = "the path to read")},
)

def _find_code(ctx):
    """Search the repository by meaning."""
    hits = code(ctx.args.question, limit = 6)
    if not hits:
        return "nothing matched; the index may not be built"

    lines = []
    for hit in hits:
        lines.append("%s:%d-%d (%s)\n%s" % (
            hit.path, hit.first_line, hit.last_line, hit.score, hit.text,
        ))
    return "\n\n".join(lines)

find_code = meow.tool(
    name = "find_code",
    about = "find code in this repository by what it does, not by its text",
    run = _find_code,
    args = {"question": meow.arg.string(about = "what to look for")},
)

def _staged_diff(ctx):
    """The staged diff, whole."""
    return diff(staged = True)

staged_diff = meow.tool(
    name = "staged_diff",
    about = "the diff of what is currently staged",
    run = _staged_diff,
)

# -----------------------------------------------------------------------------
# Agents
#
# Every one is a markdown file under `.meow/agents/`: a prompt and four
# settings is what most agents are, and this is what a prompt and four settings
# should look like. They are read after this file, so a handler names one
# rather than holding it; the name is still checked at load time.
# -----------------------------------------------------------------------------

reviewer = meow.agent_named("reviewer")
committer = meow.agent_named("committer")
assistant = meow.agent_named("assistant")

# -----------------------------------------------------------------------------
# Commands
# -----------------------------------------------------------------------------

def _review(ctx):
    """Review what is staged and report what should change."""
    patch = diff(staged = True)
    if not patch.strip():
        ctx.out.note("nothing is staged")
        return "true"

    ctx.out.step("reviewing %d lines on %s" % (len(patch.split("\n")), branch()))

    result = reviewer.run(
        "Review this staged diff.\n\n" + patch +
        ("\n\nWhat the author says they were doing: " + ctx.args.intent
         if ctx.args.intent else ""),
    )

    if not result.ok:
        ctx.out.warn("the review stopped early: %s" % result.stop)
        if result.detail:
            ctx.out.note(result.detail)

    answer = result.value
    if answer == None:
        ctx.out.markdown(result.text)
        return "true"

    for finding in answer["findings"]:
        ctx.out.finding(finding["severity"], finding["location"], finding["summary"])

    ctx.out.note("%s: %s" % (answer["verdict"], answer["summary"]))
    ctx.out.note("%d tokens" % (result.usage.prompt + result.usage.completion))

    # The exit code is the point: `meow review && git commit` gates on it.
    return "false" if answer["verdict"] == "hold" else "true"

meow.command(meow.tool(
    name = "review",
    about = "review the staged changes",
    run = _review,
    args = {
        "intent": meow.arg.string(
            about = "why you made these changes",
            required = False,
            default = "",
        ),
    },
))

def _commit(ctx):
    """Write a commit message for what is staged, and commit it."""
    patch = diff(staged = True)
    if not patch.strip():
        ctx.out.warn("nothing is staged")
        return "false"

    result = committer.run(
        "Write a commit message for this staged diff.\n\n" + patch +
        ("\n\nWhat the author says they were doing: " + ctx.args.intent
         if ctx.args.intent else ""),
    )

    message = result.text.strip()
    if not result.ok or not message:
        ctx.out.error("no message was produced: %s" % result.stop)
        return "false"

    ctx.out.markdown(message)

    if not ctx.args.write:
        ctx.out.note("pass --write to commit it")
        return "true"

    ctx.out.step("committed %s" % commit(message))
    return "true"

meow.command(meow.tool(
    name = "commit",
    about = "write a commit message for what is staged",
    run = _commit,
    args = {
        "intent": meow.arg.string(
            about = "why you made these changes",
            required = False,
            default = "",
        ),
        "write": meow.arg.bool(
            about = "commit it rather than only printing it",
            default = False,
        ),
    },
))

def _ask(ctx):
    """Answer a question about this repository."""
    ctx.out.write(assistant.run(ctx.args.question).text)
    return "true"

meow.command(meow.tool(
    name = "ask",
    about = "ask a question about this repository",
    run = _ask,
    args = {
        "question": meow.arg.string(about = "what to ask", positional = 0),
    },
))

def _status(ctx):
    """What is changed here, without asking a model anything."""
    lines = status().strip()
    if not lines:
        ctx.out.note("the tree is clean on %s" % branch())
        return "true"

    ctx.out.step(branch())
    ctx.out.write(lines)
    return "true"

meow.command(meow.tool(
    name = "status",
    about = "what is changed here",
    run = _status,
))

