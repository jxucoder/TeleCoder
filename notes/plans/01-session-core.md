# Sprint 1: Session Core

## Goal

Turn the new Bun skeleton into a durable local core that can survive being the
base product.

## Scope

- harden the Bun SQLite schema and query layer
- define durable session and event semantics
- decide whether TeleCoder owns session continuity or delegates it to named
  `acpx` sessions
- tighten workspace isolation and branch ownership rules
- add tests around store, workspace setup, engine lifecycle, and server APIs

## Key Questions

- should v1 remain one-shot `acpx exec`, or should Sprint 1 add explicit
  session resume behavior
- what session states does TeleCoder need above `acpx`
- how much of the event stream should be stored versus recomputed

## Deliverables

- stable Bun SQLite migrations
- restart-safe session records
- persistent event history
- clear task lifecycle: `pending`, `running`, `complete`, `error`
- explicit workspace and branch metadata on every task session

## Current Progress

- session ownership, heartbeat, and stale-running recovery are implemented
- pending sessions are requeued on engine startup instead of waiting to time out
- task start is now an atomic `pending -> running` claim, which prevents the
  same session from being started twice
- explicit rerun semantics are implemented for finished sessions, with visible
  lineage via parent session id and attempt number
- the API and CLI now expose rerun and event inspection paths, not just raw
  session creation
- session listing now supports status, agent, parent, and lineage filters
- the API and CLI now expose lineage-aware inspection, not just single-session
  reads
- store and engine tests cover restart recovery for both pending and running
  sessions

## Exit Criteria

- state survives process restart
- sessions and events are inspectable after restart
- every task run records repo, branch, workdir, result, and failure state
- the Bun API and CLI reflect the same stored state
