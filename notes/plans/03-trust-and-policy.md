# Sprint 3: Trust and Policy

## Goal

Build the first TeleCoder-specific trust layer on top of `acpx`.

## Scope

- map TeleCoder policy modes onto `acpx` permission modes
- add TeleCoder-owned limits for runtime duration and workspace writes
- record approval mode, runtime, and failure metadata per session
- document credential boundaries for Git, Bun, and agent runtimes

## Key Decisions

- what can stay delegated to `acpx`
- what must be enforced by TeleCoder before or after `acpx` runs
- how strict the default mode should be on unattended VPS installs

## Exit Criteria

- TeleCoder has explicit local trust defaults
- session records show how a run was authorized
- unattended execution has bounded failure behavior

## Implemented In This Sprint

- introduced TeleCoder policy modes: `locked`, `observe`, and `standard`
- mapped those modes onto `acpx` permission flags plus TeleCoder-owned runtime
  limits
- persisted policy mode, effective permission mode, workspace write policy,
  runtime command, runtime limit, and failure kind per session
- exposed policy selection in the CLI and HTTP API
- updated VPS defaults to use `TELECODER_POLICY_MODE=observe`

## Current Result

- unattended installs now default to `observe`
- `standard` allows contained workspace writes on TeleCoder-owned branches
- failed runs are classified as `policy_denied`, `timeout`, `runtime_error`, or
  `abandoned`
- session records and filters now make policy decisions inspectable after the
  fact
