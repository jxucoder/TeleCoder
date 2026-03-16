# Sprint 2: VPS Productization

## Goal

Make the Bun/`acpx` stack easy to install and operate on an Ubuntu VPS with a
clear host runtime baseline.

## Scope

- Bun install and upgrade flow
- Node 22 install and upgrade flow
- `acpx` install and runtime prerequisites
- systemd unit for the Bun server
- `.env` conventions and credential paths
- health checks, logs, and restart guidance

## Key Changes

- opinionated Ubuntu setup script
- Node 22 as the documented adapter/tooling runtime
- systemd service for `bun src/cli.ts serve`
- documented data paths and workspace paths
- documented GitHub auth and `acpx` runtime auth

## Exit Criteria

- a fresh Ubuntu host can run TeleCoder without bespoke steps
- Bun, Node 22, and `acpx` versions are pinned in docs
- service restart and upgrade are predictable
