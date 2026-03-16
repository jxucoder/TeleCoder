# Sprint 4 Watch Engine and CI Watch Report

Date: 2026-03-16

## Outcome

Sprint 4 added the first proactive TeleCoder path on top of the Bun session
core.

The system now supports persistent CI watches, CI-event ingestion, and
watch-triggered session history.

## What Landed

- persistent `watches` and `watch_runs` storage
- a first watch kind: `ci_failure`
- matching on:
  - repo
  - workflow name
  - branch name
- duplicate suppression by watch plus external CI run ID
- watch-triggered sessions that reuse:
  - the existing Bun session core
  - workspace isolation
  - TeleCoder policy modes
- user-facing surfaces for:
  - creating CI watches
  - listing watches
  - triggering CI events
  - listing watch runs

## Product Surface

HTTP API:

- `GET /api/watches`
- `POST /api/watches`
- `GET /api/watches/:id`
- `GET /api/watches/:id/runs`
- `POST /api/watch-events/ci`

CLI:

- `bun src/cli.ts watch-add-ci`
- `bun src/cli.ts watch-list`
- `bun src/cli.ts watch-runs`
- `bun src/cli.ts trigger-ci`

## Verification

Automated:

- `bun test ./test`

Manual CLI smoke:

- created a temp repo
- created a CI watch with `watch-add-ci`
- triggered a failed CI event with `trigger-ci`
- confirmed the triggered session completed
- confirmed `watch-runs` returned both:
  - the trigger summary
  - the session-derived result summary

Observed result:

- the triggered session used the existing TeleCoder policy summary and workspace
  setup flow
- `trigger-ci` now keeps the engine alive until triggered sessions finish
- `watch-runs` returned `FAKE_ACPX_OK` as the result summary in the smoke run

## Decisions

- Sprint 4 uses a webhook-style internal CI ingestion endpoint instead of a
  GitHub-specific webhook parser
- the first proactive watch stays investigation-only and defaults to contained
  or blocked behavior through the existing session policy layer
- watch result summaries are derived from persisted session outcomes rather than
  stored as a second independent summary record

## Remaining Gaps

- there is no authenticated webhook verification yet
- there is no scheduler or cron-based watch source yet
- there is no PR-context or authorship flow yet
- remote VPS verification was not rerun for this sprint
