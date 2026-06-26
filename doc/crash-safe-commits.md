# Crash-Safe Commits

This document describes the crash-safety architecture used by write transactions.
The implementation uses a redo write-ahead log (WAL), transaction-local page
allocation, and a short commit apply lock.

## Goals

- A committed write transaction must survive a process crash once its WAL is
  synced.
- A partially applied commit must be completed when the database is opened again.
- Rollback must not mutate committed page allocation state.
- Read transactions may run while a writer is open, but readers must not observe
  uncommitted writes.
- Locks must be released on commit errors.

## Files

For a database file named `data.db`, the WAL file is `data.db.wal`.

The WAL is a temporary redo log. It is created during commit, synced before any
database pages are applied, and removed after all pages are written and the
database file is synced.

`*.wal` files are ignored by Git because they are runtime recovery artifacts.

## Lock Model

`DB` uses two locks:

- `writeLock` allows only one write transaction at a time.
- `commitLock` keeps readers out of the short disk-apply phase.

A read transaction takes `commitLock.RLock()`. A write transaction takes
`writeLock` when it starts, but it does not block readers while it is building
dirty state. During `Commit`, the writer takes `commitLock.Lock()` before writing
pages to disk. This means readers can run while a writer is open, but a commit
waits for active readers before applying pages.

## Write Transaction State

A write transaction owns:

- `dirtyNodes`: nodes changed by the transaction.
- `pagesToDelete`: pages that should be released if the transaction commits.
- `freelist`: a cloned transaction-local freelist.
- `root`: the transaction's root collection page.
- `collections`: write-transaction collection handles whose metadata may need
  flushing.
- `deletedCollections`: collection names removed inside the transaction.

New pages are allocated from the transaction-local freelist. On rollback, the
transaction simply discards this state and releases `writeLock`; the committed
database freelist is unchanged.

## Commit Flow

`Tx.Commit` follows this sequence:

1. If the transaction is read-only, release the read lock and return.
2. Defer release of `writeLock`, so errors cannot leave the writer lock held.
3. Flush dirty collection metadata into the root collection. This persists root
   page changes from collection splits and counter changes from `Collection.ID`.
4. Take `commitLock.Lock()` to exclude readers from disk application.
5. Build the complete page set to commit:
   - serialized dirty B+ tree nodes
   - serialized freelist page
   - serialized meta page
6. Write and sync the WAL.
7. Write the WAL pages to the database file.
8. Sync the database file.
9. Remove the WAL.
10. Publish the new in-memory freelist and meta state.

The WAL includes all pages needed to finish the transaction. The meta page is in
the WAL too, so recovery replays the same root and freelist pointers that the
commit intended to publish.

## WAL Format

The WAL layout is:

```text
magic[8] | version[4] | page_size[4] | page_count[8] | crc32[4] | records...
```

Each record is:

```text
page_num[8] | page_data[page_size]
```

The WAL reader validates:

- magic bytes: `MYKVWAL1`
- WAL version
- page size compatibility with the opened database
- total file size
- CRC32 of the records section

Empty or truncated WAL files are treated as incomplete logs and removed. A WAL
with invalid magic, unsupported version, page-size mismatch, or checksum mismatch
returns an error on open.

## Recovery Flow

`Open` initializes the DAL, opens the database file, then calls WAL recovery
before reading normal meta and freelist state.

Recovery does this:

1. Read `data.db.wal`.
2. If there is no valid complete WAL, do nothing or remove the incomplete log.
3. Replay each logged page into the database file.
4. Sync the database file.
5. Remove the WAL.
6. Continue normal open by reading meta and freelist from the now-recovered file.

This handles crashes after the WAL was synced but before all database pages were
applied. Replaying the same pages is idempotent.

## Failure Semantics

Crash before WAL sync:

The database may not have received any commit pages yet. On reopen, a missing or
incomplete WAL is ignored or removed, so the transaction is not recovered.

Crash after WAL sync, before database sync:

The WAL contains the complete committed page set. On reopen, recovery replays it
and completes the commit.

Crash after database sync, before WAL removal:

The database already contains the committed pages. On reopen, recovery replays
the same pages again and removes the WAL.

Commit error:

`Tx.Commit` returns the error and releases `writeLock`. The in-memory committed
metadata is updated only after WAL application succeeds.

Rollback:

Rollback discards transaction-local state and does not write pages or mutate the
committed freelist.

## Current Limits

- The WAL is a whole-page redo log, not a streaming log.
- There is one WAL per database file.
- Only one writer can be active at a time.
- Collection deletion removes the collection metadata from the root collection,
  but it does not yet walk and release every page in the deleted collection tree.
- The implementation does not yet sync the parent directory after WAL creation or
  deletion, so the durability model depends on the filesystem's handling of
  directory entries.
