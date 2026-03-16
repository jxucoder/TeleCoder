# Sprint 0: Evaluation

## Goal

Validate the new TypeScript/Bun/`acpx` architecture on a real VPS before the
session model hardens around it.

## Why This Sprint Comes First

The reset changed the technical foundation completely. The next risk is no
longer "does our ACP client work?" It is:

- does Bun behave cleanly on the target Ubuntu VPS
- does `acpx` give us the runtime/session semantics we need
- does the new workspace model remain safe and predictable

## Scope

- install Bun on the target Ubuntu VPS
- install Node 22 on the target Ubuntu VPS for agent adapters that need it
- verify `acpx` and at least one real agent runtime on that host
- run TeleCoder against a safe GitHub repo using the new Bun CLI/server
- confirm clone, branch creation, prompt execution, and result capture
- identify what must still be built on top of `acpx`

## Required Environment

- one Ubuntu VPS under user control
- Bun plus Node 22 on that host
- one safe GitHub test repo with clone/push auth
- one working `acpx` installation path
- one or more runtime credential paths behind `acpx`

## Evaluation Scenarios

### Scenario 1: One-shot task run

- start the Bun TeleCoder service or CLI
- clone the repo into a TeleCoder workspace
- create a TeleCoder-owned branch
- call `acpx <agent> exec ...`
- persist the result and event history

Success criteria:

- no custom ACP code is needed for the path to work
- TeleCoder stores enough state to inspect the run afterward
- branch and workspace ownership are clear

### Scenario 2: Server API check

- create a session through the HTTP API
- stream or query session events
- verify status and result retrieval

Success criteria:

- API and CLI reflect the same underlying state
- event streaming or polling is stable enough for operator use

### Scenario 3: Runtime comparison

- run the same prompt with at least two `acpx` agent targets if available
- compare setup friction, output quality, and failure modes

Success criteria:

- one default agent path is chosen for v1
- any adapter-specific quirks are written down early

## Questions This Sprint Must Answer

- is `acpx exec` enough for the first TeleCoder cut, or do we need saved `acpx`
  sessions immediately
- which agent should be the default path on the VPS
- whether Node 22 should be a hard VPS requirement or an optional compatibility layer
- what must TeleCoder own above `acpx` versus what should stay delegated

## Exit Criteria

- one end-to-end Bun/acpx run succeeds on the VPS
- the host runtime baseline is decided and written down
- the default agent path for v1 is chosen
- the new evaluation report reflects the reset, not the removed Go stack
- Sprint 1 is grounded in Bun/acpx realities instead of ACP transport work
