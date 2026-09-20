# Authentication and trust

Status: draft
Elaborates: docs/design/0.3.0-starlark-api.md section 4.1

## Scope

Where a credential comes from, where it is kept, how a provider that uses
OAuth rather than an API key obtains and refreshes one, and whether this
machine has agreed to run the scripts in a given workspace.

These are one area because they are one question asked twice: what is this
process allowed to use, and who said so. A credential answers it for a
provider and trust answers it for a workspace, and both are facts about the
machine rather than about the repository.

## Boundary

Two files under `~/.meow/`, both owned by the user and written only by
`meow auth` and `meow trust`. Nothing in `.meow/` can read or write either,
and no Starlark builtin exposes them: a workspace that could read the
credential store would be a workspace that could exfiltrate it.

## Requirements

### Where a credential comes from

**[R-AUTH-001]** A credential MUST be resolved in this order: the `api_key`
the declaration gives, then the global store, then the provider's environment
variable. The first that is present and not empty wins.

**[R-AUTH-002]** A provider with no credential from any of the three MUST fail
naming all three places, with the environment variable spelled out, so the
reader can act without consulting a document.

**[R-AUTH-003]** Resolution MUST NOT happen while `.meow/` is being evaluated
beyond what `[R-STAR-084]` already allows: a declaration may call
`env.get`, and the store is read when a provider is built rather than when it
is declared.

**[R-AUTH-004]** No runtime module MUST expose the credential store. A handler
that wants a secret reads it from the environment, which is a decision the
person running the command made.

### The store

**[R-AUTH-010]** Credentials MUST live in one file at `~/.meow/auth.json`,
outside every workspace, so that a repository cannot carry one and a
contributor cannot commit one.

**[R-AUTH-011]** The file MUST be created with owner-only permissions, and
`meow auth` MUST refuse to read one that is readable by anyone else, naming
the file and the permission it expects.

**[R-AUTH-012]** Writing the store MUST be atomic: a write MUST NOT leave a
truncated file if the process dies, because a truncated credential store locks
the user out of every provider at once.

**[R-AUTH-013]** `meow auth login <provider>` MUST store a credential,
`meow auth logout <provider>` MUST remove one, and `meow auth list` MUST show
which providers have one without showing any of them.

**[R-AUTH-014]** No command MUST print a credential, in full or in part. A
listing says that a credential is present and when it was stored.

### OAuth providers

**[R-AUTH-020]** A provider kind that authenticates by OAuth rather than an
API key MUST obtain its credential through a device-code flow: the command
shows a code and a URL, waits for the person to approve, and stores the result.

**[R-AUTH-021]** The flow MUST honour the interval the authorisation server
asks for, MUST stop when the server says the code expired, and MUST be
interruptible, so that Ctrl-C leaves nothing half-written.

**[R-AUTH-022]** A refresh token MUST be kept in the same store as an API key,
and an access token that has expired MUST be refreshed without asking the
person again. A refresh that fails MUST say so and MUST say which command
re-authenticates.

**[R-AUTH-023]** `meow auth login` for an OAuth provider MUST be refused when
there is no terminal, because there is nobody to read the code.

### Trusting a workspace

**[R-AUTH-030]** A `.meow/` directory is executable code with tool access. The
first invocation in a workspace this machine has not agreed to MUST show what
the workspace declares - its agents, its tools, and the policy it asks for -
and MUST ask once.

**[R-AUTH-031]** A run in an untrusted workspace with no terminal MUST fail
rather than proceed or block, and MUST name the command that grants trust.

**[R-AUTH-032]** Trust MUST be recorded per workspace path in
`~/.meow/trust.json`, and MUST be withdrawable.

**[R-AUTH-033]** Loading `.meow/` to ask the question MUST NOT run any of it
beyond declaration, which `[R-STAR-084]` already guarantees: the prompt
describes what was declared, and declaring reaches nothing.

**[R-AUTH-034]** A workspace whose declarations changed MUST NOT silently keep
its trust. What is recorded is what was shown, so a `.meow/` that grows a new
tool or widens its policy asks again.

## Changes from v0.2.x

The Go implementation had `meow auth copilot` and nothing else: one provider's
device flow, with the token in a file beside it, and every other provider
reading an environment variable. There was no listing, no logout, and no
notion of trusting a workspace at all - cloning a repository and running
`meow` in it ran whatever its `.meow/` declared.

## Decisions

**Three places, in one order, always**, by [R-AUTH-001]. The alternative is to
let a declaration say where its key comes from, which moves the decision into
the repository - and the whole point of the store is that credentials belong
to the machine.

**A listing never shows a credential**, by [R-AUTH-014]. "Show me what I
stored" is a reasonable thing to want and a bad thing to build: it puts a
secret on a terminal, into scrollback, and into whatever recorded the session.
The store is a file the user owns; `cat` is available to them.

**Trust is per workspace and re-asked when declarations change**, by
[R-AUTH-034]. Trusting a path once and forever makes the question a formality,
because the repository at that path is not the repository the question was
answered about. Re-asking on change is the smallest thing that keeps the answer
meaningful.
