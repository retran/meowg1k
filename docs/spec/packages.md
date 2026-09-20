# Packages

Status: draft
Elaborates: docs/design/0.3.0-starlark-api.md sections 3, 10

## Scope

Fetching Starlark a workspace did not write, pinning it so that the same
workspace loads the same bytes tomorrow, verifying it before it runs, and
keeping it where a second checkout need not fetch it again.

This is the third `load` scheme. `@std//` is Rust in this binary and `//` is a
file the workspace wrote; `@<pkg>//` is neither, and the difference that
matters is that it arrives over a network from somebody else.

## Boundary

A declaration in `.meow/meow.star`, a lockfile at `.meow/meow.lock` that is
committed, and a cache under `.meow/.data/pkg/` that is not. Everything else
is the loader resolving a scheme.

## Requirements

### Declaring

**[R-PKG-001]** A workspace MUST declare every package it loads, with a name,
a source, and a version. `load("@<pkg>//<path>", ...)` naming an undeclared
package MUST fail saying so and MUST NOT fetch anything.

**[R-PKG-002]** A declaration MUST be refused at load time when two packages
claim the same name, naming both.

**[R-PKG-003]** A package name MUST NOT be `std`. The scheme that reaches the
runtime modules cannot be shadowed by something fetched.

### Pinning

**[R-PKG-010]** Every package MUST be pinned in `.meow/meow.lock` by its
resolved version and the SHA-256 of its contents. The lockfile MUST be
deterministic: the same declarations and the same upstream produce the same
bytes, so a diff shows a dependency change and nothing else.

**[R-PKG-011]** Loading MUST verify the hash of what it is about to run
against the lockfile, and MUST fail without evaluating anything when they
differ, naming the package, the expected hash, and the one found.

**[R-PKG-012]** Loading MUST fail when a declared package is absent from the
lockfile, naming the command that writes one. It MUST NOT fetch silently:
a load that reaches the network without being asked is a load that can change
behaviour between two runs of the same commit.

**[R-PKG-013]** `meow pkg update` MUST re-resolve versions and rewrite the
lockfile. `meow pkg fetch` MUST download what the lockfile already pins and
MUST NOT change it.

### Fetching and caching

**[R-PKG-020]** A package whose contents are already in the cache and match
the lockfile MUST load without any network access. A workspace that has
fetched once MUST work offline.

**[R-PKG-021]** The cache MUST be keyed by hash, so two workspaces pinning the
same package share one copy and a changed pin cannot be served the old bytes.

**[R-PKG-022]** A fetch MUST be atomic: an interrupted download MUST NOT leave
something the next run mistakes for a complete package.

**[R-PKG-023]** A fetch MUST carry a deadline and MUST be interruptible.

### What a package may do

**[R-PKG-030]** A file in a package MUST be able to `load("@std//...")` and to
load another file in the same package by a relative path.

**[R-PKG-031]** A file in a package MUST NOT be able to load `//<path>`, which
is the host workspace's own tree. A dependency reaching into the workspace that
depends on it inverts the direction and makes the package's behaviour depend on
who loaded it.

**[R-PKG-032]** A package MUST NOT be able to load another package the host
workspace did not declare. Dependencies are the workspace's to state, so that
`meow.lock` is the whole list.

**[R-PKG-033]** Code from a package MUST run under the same rules as code the
workspace wrote: the declaration phase reaches nothing, and what a model
decides still passes through policy. A package is Starlark, not a plugin.

## Changes from v0.2.x

There were no packages. Item 7 of the 0.2.x TODO reserved the idea and nothing
was built, so there is nothing to migrate and no compatibility to keep.

## Decisions

**A load never fetches**, by [R-PKG-012]. Resolving on demand is what every
package manager did first and stopped doing: it makes a build depend on the
network and on when it ran. Fetching is a command a person runs, and the
lockfile is what a load consults.

**The cache is keyed by hash and not by name and version**, by [R-PKG-021].
A version is a label upstream controls and can move; a hash is the bytes. Two
workspaces that pinned the same hash are provably running the same code, which
is the property the lockfile exists to give.

**A package cannot see the workspace**, by [R-PKG-031]. The alternative reads
well in a small example - a package that loads `//lib/style.md` from whoever
uses it - and makes the package untestable on its own and its behaviour a
function of its caller.
