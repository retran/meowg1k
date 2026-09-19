# Store

Status: draft
Elaborates: docs/design/0.3.0-architecture.md sections 4, 6; docs/design/0.3.0-sessions.md sections 3, 4, 8, 11

## Scope

`meow-store` owns the one SQLite database a workspace keeps. It persists the
session event log, the content-addressed blobs those events point at, the
workspace key-value store, the response cache, and the retrieval index.

It does not interpret what it stores. Deciding that a `Compaction` event
supersedes a range is `meow-session`'s job; the store only guarantees that the
event is written, ordered, and still there later.

## Boundary

A single file at `.meow/.data/meow.db`, plus the WAL and shared-memory files
SQLite manages beside it. Everything else is the Rust API: open, migrate,
append, query, and a transaction handle.

## Requirements

### Database and schema

**[R-STORE-001]** The store MUST keep all data for one workspace in a single
SQLite database at `.meow/.data/meow.db`, resolved from the workspace root.

**[R-STORE-002]** The store MUST enable write-ahead logging and foreign key
enforcement on every connection.

**[R-STORE-003]** The store MUST record the schema version it was written
under in a `meta` table.

**[R-STORE-004]** On open, the store MUST apply every migration whose version
is greater than the recorded one, in ascending order, each inside its own
transaction. A migration that fails MUST leave the database at the version it
had before that migration.

**[R-STORE-005]** Migrations MUST be forward-only. The store MUST NOT contain
a downgrade path.

**[R-STORE-006]** On opening a database whose recorded schema version is
greater than the binary supports, the store MUST fail with an error naming both
versions and MUST NOT modify the file.

### Blobs

**[R-STORE-010]** Event payloads larger than 512 bytes MUST be stored in a
`blobs` table keyed by the BLAKE3 hash of their content, and referenced from
the event by that hash.

**[R-STORE-011]** Writing a blob whose hash is already present MUST NOT store
a second copy and MUST NOT fail.

**[R-STORE-012]** The store MUST maintain a reference count per blob and MUST
delete a blob only when its count reaches zero.

**[R-STORE-013]** Reading a blob whose hash has no row MUST fail with an error
naming the hash, and MUST NOT return empty content.

### Writing

**[R-STORE-020]** All writes belonging to one agent turn MUST be committed in
a single transaction.

**[R-STORE-021]** A failed write MUST propagate as an error to the caller. The
store MUST NOT log a failure and continue.

**[R-STORE-022]** The store MUST allow one writer at a time and MUST allow
readers concurrently with that writer.

**[R-STORE-023]** A write that blocks on another writer MUST wait up to a
configured busy timeout of at least 5 seconds before failing.

### Key-value store

**[R-STORE-030]** The store MUST provide a durable key-value table, scoped to
the workspace, with `get`, `put`, `delete`, and `keys` operations.

**[R-STORE-031]** Workspace key-value data MUST be stored separately from
session state, so that deleting a session leaves it intact.

### Retention

**[R-STORE-040]** The store MUST delete a session by removing its rows and
decrementing the reference count of every blob those rows referenced.

**[R-STORE-041]** The store MUST NOT delete part of a session. A deletion
either removes the whole session or fails.

**[R-STORE-042]** The store MUST provide the total on-disk size of the
database so retention can act on it.

## Changes from v0.2.x

The Go implementation spreads state across several SQLite files and repository
packages under `internal/adapters/sqlite/`, with separate databases for the
index, cache, sessions, and metadata. One database per workspace replaces them,
because the blob table is shared and a cross-file reference count is not
something SQLite can enforce.

Session writes in v0.2.x are best-effort: `module_llm.go` logs ten distinct
failures through `log.Printf` and continues. [R-STORE-021] reverses that. A log
whose entire job is to be trusted later is worse than useless when it can be
silently incomplete.

Content addressing is new. v0.2.x stores each tool result inline, so an agent
that reads the same file at three steps stores it three times.

## Open questions

- **Encryption at rest.** Tool output can contain secrets a policy did not
  catch. SQLCipher would cover it at the cost of a C dependency and a key
  management story. Recommendation: leave it out of v0.3.0, document that the
  database is plaintext, and revisit when someone asks.
- **`synchronous` level.** `NORMAL` with WAL loses at most the last
  transaction on power failure and is much faster than `FULL`. Recommendation:
  `NORMAL`, because a lost final turn is recoverable by rerunning and the write
  path is on the agent's critical path.
