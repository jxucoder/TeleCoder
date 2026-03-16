# Sprint 3 Trust and Policy Report

Date: 2026-03-16

## Outcome

Sprint 3 established the first TeleCoder-owned trust layer on top of `acpx`.

TeleCoder now resolves runs into explicit local policy modes:

- `locked`
- `observe`
- `standard`

Those modes decide:

- the `acpx` permission flag
- the maximum runtime for the task
- whether workspace writes are treated as blocked or contained

## What Landed

- added policy resolution in `src/policy.ts`
- added `TELECODER_POLICY_MODE` with backward compatibility for
  `TELECODER_PERMISSION_MODE`
- persisted the following on every session:
  - `policyMode`
  - `effectivePermissionMode`
  - `workspaceWritePolicy`
  - `maxRuntimeSeconds`
  - `runtimeCommand`
  - `failureKind`
- classified runtime failures into:
  - `policy_denied`
  - `timeout`
  - `runtime_error`
  - `abandoned`
- exposed policy selection and filtering through:
  - `bun src/cli.ts run --policy ...`
  - `bun src/cli.ts list --policy ...`
  - `POST /api/sessions` with `policy`
  - `GET /api/sessions?policy=...`
- updated VPS examples and installer output to default to
  `TELECODER_POLICY_MODE=observe`

## Verification

Automated:

- `bun test ./test`

Manual smoke:

- `TELECODER_DATA_DIR=$PWD/.tmp/sprint3-smoke/data bun src/cli.ts config`
- `TELECODER_DATA_DIR=$PWD/.tmp/sprint3-smoke/data TELECODER_ACPX_COMMAND=$PWD/test/fixtures/fake-acpx bun src/cli.ts run --repo $PWD/.tmp/sprint3-smoke/repo --policy standard 'smoke run'`
- `TELECODER_DATA_DIR=$PWD/.tmp/sprint3-smoke/data bun src/cli.ts list --policy standard`
- `TELECODER_DATA_DIR=$PWD/.tmp/sprint3-smoke/data bun src/cli.ts status 7b65db94`

Observed result:

- the CLI emitted the resolved policy summary before workspace setup
- the session completed successfully
- the stored session record showed `policyMode`, `effectivePermissionMode`,
  `workspaceWritePolicy`, `maxRuntimeSeconds`, `runtimeCommand`, and
  `failureKind`

## Decisions

- unattended VPS installs default to `observe`
- `standard` is the highest built-in trust preset and relies on contained
  TeleCoder-owned branches/workspaces rather than direct edits to the user’s
  main checkout
- the legacy `TELECODER_PERMISSION_MODE` env var remains accepted so older env
  files do not break immediately

## Remaining Gaps

- interactive approvals are still not implemented
- policy does not yet drive webhook/watch-specific escalation rules
- return summaries do not yet explain trust decisions in user-facing language
