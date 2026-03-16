# Sprint 1 Session Inspection Report

Date: 2026-03-15

## Outcome

Step 2 is complete.

TeleCoder now has filterable, lineage-aware session inspection across the
store, engine, CLI, and HTTP API.

## What Landed

### Query model

Session inspection now supports:

- status filtering, including `active`
- agent filtering
- direct child filtering via `parentSessionId`
- lineage-family views rooted at any session
- lineage queries combined with status or parent filters

### Product surface

The user-facing surface now includes:

- CLI `list --status ... --agent ... --parent ... --lineage ...`
- CLI `lineage <session-id>`
- API `GET /api/sessions` with filter params
- API `GET /api/sessions/:id/lineage`

This makes session inspection useful for real operators instead of only for
single-session debugging.

## Verification

- `bun test ./test` passed with 18 tests
- store tests cover status, parent, agent, and lineage listing semantics
- server tests cover filtered listing, lineage queries, and invalid filter
  rejection
- a local CLI smoke flow using the fake `acpx` fixture verified `list` and
  `lineage` on a real rerun chain

## Decisions

- filtered listing and lineage views use the same semantics in the CLI and API
- lineage is modeled as the full rerun family, not just direct children
- `active` means `pending` plus `running`

## Remaining Gaps

- there are no richer repo, time, or branch-oriented list filters yet
- there is still no cancel or stop path for active sessions
- Sprint 1 still needs a final decision on true resume semantics above
  one-shot `acpx exec`
