# Sprint 4: Watch Engine and CI Watch

## Goal

Add the first proactive path on top of the Bun session core.

## Scope

- watch definitions and persistence
- scheduler or webhook trigger ingestion
- one CI-focused watch that launches a bounded TeleCoder task
- run history and result summaries for watch-triggered tasks

## Why CI Comes First

- it exercises proactive execution without full PR context
- it keeps the first watch tightly scoped to investigation, not authorship

## Exit Criteria

- one CI watch works end to end
- watch-triggered sessions use the same Bun store and workspace rules
- return summaries explain why the watch fired and what happened

## Implemented In This Sprint

- added persistent watch definitions and watch run history
- added a webhook-style CI event ingestion path
- introduced the first watch kind: `ci_failure`
- linked watch-triggered runs into the existing session/workspace/policy model
- exposed watch creation, listing, triggering, and run history in both the CLI
  and HTTP API

## Current Result

- a CI failure event can create a bounded TeleCoder session end to end
- duplicate CI run IDs are ignored per watch
- watch runs show both the trigger reason and the session-derived result summary
