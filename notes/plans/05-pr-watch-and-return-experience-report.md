# Sprint 5 PR Watch and Return Experience Report

Date: 2026-03-16

## Outcome

Sprint 5 completed the first useful async review loop for pull requests.

TeleCoder can now notice PR updates, load base/head diff context, run a bounded
review task, and return a compact summary suitable for asynchronous review.

## What Landed

- a new watch kind: `pr_review`
- PR event ingestion through:
  - `POST /api/watch-events/pr`
  - `bun src/cli.ts trigger-pr`
- PR watch creation through:
  - `POST /api/watches` with `kind: pr_review`
  - `bun src/cli.ts watch-add-pr`
- base/head branch matching for PR watches
- duplicate suppression by PR number plus head SHA
- best-effort diff loading from the local git checkout using `git diff`
- compact return summaries on watch runs with:
  - trigger
  - runtime path
  - changed
  - verified
  - uncertain
  - next

## Product Surface

CLI:

- `bun src/cli.ts watch-add-pr`
- `bun src/cli.ts trigger-pr`
- `bun src/cli.ts watch-runs`

HTTP API:

- `POST /api/watches` with `kind: pr_review`
- `POST /api/watch-events/pr`
- `GET /api/watches/:id/runs`

## Verification

Automated:

- `bun test ./test`

Manual CLI smoke:

- created a temp git repo with a feature branch
- created a `pr_review` watch with `watch-add-pr`
- triggered a PR event with `trigger-pr`
- confirmed the prompt included:
  - base branch
  - head branch
  - diff stat
  - diff excerpt
  - explicit no-push / no-PR-creation boundaries
- confirmed `watch-runs` returned a compact return summary

Observed result:

- the triggered PR session reused the existing TeleCoder policy and workspace
  path
- `watch-runs` returned a compact multi-line return summary without needing raw
  event logs first
- explicit push and PR creation actions remain outside the automated flow

## Decisions

- the first PR watch is review-only, not authorship
- diff context is loaded locally from git when possible, with fallback to
  provided diff text
- return summaries are derived from session output plus runtime metadata rather
  than stored as a separate authored artifact

## Remaining Gaps

- there is still no authenticated webhook verification
- there is still no explicit push action or PR creation action surface
- remote VPS verification was not rerun for this sprint
