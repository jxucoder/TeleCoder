# Sprint 5: PR Watch and Return Experience

## Goal

Complete the first compelling async loop: TeleCoder notices repo work, runs it,
and returns a reviewable summary.

## Scope

- PR-triggered watch definitions
- branch and diff context loading
- explicit push and PR creation boundaries
- compact result summaries for async review

## Return Summary Requirements

- what changed
- what `acpx`/agent path ran
- what was verified
- what failed or remains uncertain
- what the user should do next

## Exit Criteria

- one PR watch works end to end
- the returned summary is useful without opening raw logs first
- push and PR creation remain explicit, not implicit

## Implemented In This Sprint

- added persistent `pr_review` watch definitions
- added PR event ingestion with dedupe by PR number and head SHA
- loaded base/head diff context from the local git repo when available
- added compact return summaries for watch runs
- kept PR watches review-only with explicit no-push and no-PR-creation
  boundaries

## Current Result

- a PR review watch can trigger a bounded TeleCoder session end to end
- the review prompt includes PR metadata, branch context, and loaded diff context
- `watch-runs` now returns a compact async review summary without requiring raw
  log inspection first
