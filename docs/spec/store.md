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
enforcement on every connection, and MUST set `synchronous` to `NORMAL`.

**[R-STORE-007]** The store MUST NOT encrypt the database, and `meow doctor`
MUST report that it is unencrypted, together with its path, so that a user
deciding what a workspace may hold is told rather than left to assume.

**[R-STORE-008]** The database and its side files MUST be created readable and
writable by their owner only, on every platform that has file permissions.

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

**[R-STORE-010]** Every event payload MUST be addressed by the BLAKE3 hash of
its content. The store MAY keep a payload under 512 bytes inline rather than in
the `blobs` table, and that choice MUST be invisible to a reader: the same hash
MUST return the same bytes either way.

**[R-STORE-011]** Writing a blob whose hash is already present MUST NOT store
a second copy and MUST NOT fail.

**[R-STORE-012]** The store MUST maintain a reference count per blob and MUST
delete a blob only when its count reaches zero.

**[R-STORE-013]** Reading a blob whose hash has no row MUST fail with an error
naming the hash, and MUST NOT return empty content.

### Writing

**[R-STORE-020]** Each event MUST be committed on its own. A turn MUST NOT be
written in one transaction held open across tool execution: the log is
append-only, so a half-written turn is a true record of how far the run got,
and holding the single write lock for the length of a tool would block every
other session and lose that tool's result on a crash.

**[R-STORE-021]** A failed write MUST propagate as an error to the caller. The
store MUST NOT log a failure and continue.

**[R-STORE-022]** The store MUST allow one writer at a time and MUST allow
readers concurrently with that writer.

**[R-STORE-024]** A bulk write such as an index build MUST commit in bounded
batches and MUST NOT hold the write lock for the length of the operation, so
that indexing cannot block an agent run.

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

### Cache

**[R-STORE-045]** The store MUST provide a cache keyed by a hash of the
request that produced the entry. Embedding responses MUST be cached by
default. Generation responses MUST NOT be cached unless the caller asks,
because an agent that retries wants a fresh attempt and a cache would hand it
the answer that already failed.

**[R-STORE-046]** A cache entry MUST record the model that produced it, and a
lookup MUST miss when the model differs, so that changing a model cannot return
another model's answer.

**[R-STORE-047]** Cache eviction MUST be by total size and age, and evicting an
entry MUST NOT affect any session that quoted it.

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

## Decisions

**The database is plaintext, with owner-only permissions**, by [R-STORE-007]
and [R-STORE-008]. Encrypting it would need SQLCipher, a C dependency the
workspace otherwise avoids, and a key management story nobody has designed.

The first answer stopped at "plaintext, and we say so", which accepted the risk
without doing the cheap part. File permissions cost one flag and stop every
other account on the machine from reading a transcript. They are not
encryption, and the honest place to keep a secret out of the log is still the
policy.

**`synchronous` is `NORMAL`**, by [R-STORE-002]. With write-ahead logging that
loses at most the last transaction on a power failure, which costs a rerun of
one turn. `FULL` would buy that back at a cost paid on the agent's critical
path, on every write.
