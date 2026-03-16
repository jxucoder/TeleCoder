# Sprint 0 Evaluation Report

Date: 2026-03-16

## Outcome

Sprint 0 is complete. The Bun/TypeScript/`acpx` reset now works end to end on
the Ubuntu evaluation VPS against the GitHub test repo.

## Environment Verified

- DigitalOcean droplet: `ubuntu-s-1vcpu-2gb-sfo2-01`
- OS: Ubuntu 24.04
- Bun: `1.3.10`
- Node: `22.x` for Claude ACP compatibility, alongside the host default Node 18
- test repo: `jxucoder/telecoder-test-repo`
- repo auth: writable deploy key on the droplet
- `acpx` path under test: local `acpx` snapshot wrapped as `/root/bin/acpx-local`
- runtimes validated on the host: Codex, Opencode, and Claude via the
  `/root/bin/claude-agent-acp-node22` wrapper

## What Was Verified

### CLI path

TeleCoder was run directly on the droplet with:

- Bun CLI entrypoint
- workspace clone into a TeleCoder-owned directory
- branch creation under `telecoder/<session-id>`
- `acpx` execution against Codex

Confirmed result:

- the session completed successfully
- the repo cloned cleanly
- the TeleCoder branch was created
- the final CLI output was only `TELECODER_EVAL_OK`

### API path

TeleCoder was started as a Bun HTTP service on the droplet and exercised
through `POST /api/sessions`, `GET /api/sessions/:id`, and
`GET /api/sessions/:id/events`.

Confirmed result after restarting the server on the patched runtime parser:

- session creation succeeded through the API
- stored session state reached `complete`
- `resultText` was `TELECODER_API_OK`
- the persisted `output` event was `TELECODER_API_OK`

### Runtime comparison

- Codex through `acpx` worked and remains the simplest default runtime path
- Opencode works end to end when TeleCoder targets the installed
  `/usr/local/bin/opencode acp` command explicitly
- Claude works end to end when TeleCoder targets a Node 22 wrapper around
  `@zed-industries/claude-agent-acp` and exports the runtime env to child
  processes
- the generic built-in registry is not enough on every host; TeleCoder needs
  agent-specific command overrides

## Important Fixes Discovered During Evaluation

The first Bun/acpx pass captured transport/progress noise in `resultText`
instead of only the assistant output. That was fixed by:

- switching the `acpx` invocation to `--format json --json-strict`
- parsing only `agent_message_chunk` updates from the JSONL stream

This fix is now confirmed in both the CLI and API paths.

## Decisions From Sprint 0

- keep `acpx exec` as the initial runtime integration path
- make Codex the default agent path for now
- support Opencode and Claude via explicit agent command overrides
- standardize VPS guidance on Bun plus Node 22
- move Sprint 1 focus to durability and session semantics instead of ACP
  transport debugging

## Remaining Gaps

- TeleCoder still owns only one-shot task runs on top of `acpx exec`
- there is no named-session or resume model yet
- there is no approval/audit layer beyond the current `acpx` flags
- there are no watch, webhook, or PR automation flows yet
- agent override configuration still needs first-class product documentation
