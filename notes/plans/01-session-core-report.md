# Sprint 1 Session Core Report

Date: 2026-03-15

## Outcome

Sprint 1 is in progress.

The first two major Sprint 1 milestones are complete: TeleCoder now has
durable session ownership metadata, restart recovery for queued and stale work,
explicit rerun semantics, and filterable lineage-aware inspection exposed
through both the CLI and HTTP API.

## What Landed

### Durable lifecycle ownership

Session records now carry:

- `ownerId`
- `claimedAt`
- `heartbeatAt`
- `startedAt`
- `finishedAt`

This makes task ownership and lifecycle state visible after restart instead of
only during a live process.

### Restart recovery

- pending sessions are requeued on engine startup
- stale running sessions are reconciled to `error`
- task start uses an atomic `pending -> running` claim to prevent duplicate
  starts

This means a process restart no longer leaves queued work sitting until a stale
timeout, and it no longer allows the same session to start twice.

### Explicit rerun semantics

Reruns now create a new linked session instead of mutating the original one.

Each rerun records:

- `parentSessionId`
- `attempt`

Only finished sessions can be rerun. Active `pending` and `running` sessions
are rejected.

### Product surface

The user-facing surface now includes:

- CLI `rerun <session-id>`
- CLI `events <session-id>`
- CLI `lineage <session-id>`
- CLI `list --status ... --agent ... --parent ... --lineage ...`
- API `POST /api/sessions/:id/rerun`
- API `GET /api/sessions` with filter params
- API `GET /api/sessions/:id/lineage`

This moves session control and inspection out of the internal engine and into
the actual product interface.

### Inspection depth

Session inspection is no longer limited to fetching one session at a time.

TeleCoder now supports:

- status filtering, including `active`
- direct child filtering via `parentSessionId`
- lineage views for a whole rerun family
- lineage-aware filtering combined with status or parent filters

## Verification

- `bun test ./test` passed with 18 tests
- store tests cover persistence, single-claim startup, pending recovery, and
  stale-running reconciliation
- store tests also cover status, parent, agent, and lineage listing semantics
- engine tests cover startup requeue, rerun lineage, and active-session rerun
  rejection
- server tests cover rerun, filtered listing, lineage queries, and invalid
  filter rejection
- a local CLI smoke flow using the fake `acpx` fixture verified `run`,
  `status`, `rerun`, `events`, `list`, and `lineage`
- the CLI smoke flow also verified rerun lineage with `parentSessionId` and
  `attempt: 2`

## Decisions From This Milestone

- TeleCoder owns rerun lineage above one-shot `acpx exec`
- rerun means a new linked session, not resetting an old session in place
- active sessions are not rerunnable
- lifecycle inspection belongs in the product surface, not just the store
- filtered listing and lineage views should use the same semantics in the CLI
  and HTTP API

## Remaining Gaps

- there is still no true multi-turn resume model above `acpx exec`
- there is no cancel or stop path for active sessions yet
- server and CLI inspection are now usable, but still minimal
- there are no richer repo, time, or branch-oriented list filters yet
- Sprint 1 still needs a final decision on how much continuity TeleCoder owns
  versus delegating to future named `acpx` sessions
