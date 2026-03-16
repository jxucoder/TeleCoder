# Sprint 2 VPS Productization Report

Date: 2026-03-16

## Outcome

Sprint 2 is complete.

TeleCoder now has an opinionated Ubuntu VPS install and service path with
pinned runtime versions, env conventions, systemd service wiring, and basic
operator health/log guidance.

## What Landed

### Install scripts

Added:

- `scripts/install/install-ubuntu-vps.sh`
- `scripts/install/check-service.sh`
- `scripts/install/telecoder.env.example`
- `scripts/install/telecoder.service.example`

The install script now handles:

- Bun install or upgrade
- Node 22 install or upgrade
- global `acpx` install
- TeleCoder env file creation
- systemd unit creation
- service enable and start

### Operator docs

Added Ubuntu host guidance in `notes/vps-ubuntu.md`, including:

- pinned host versions
- install flow
- runtime auth conventions
- health and log commands
- upgrade flow

## Versions Pinned

- Bun `1.3.10`
- Node `22.x`
- `acpx` `0.3.0`

## Verification

Local verification:

- `bash -n scripts/install/install-ubuntu-vps.sh`
- `bash -n scripts/install/check-service.sh`

Remote verification on `ubuntu-s-1vcpu-2gb-sfo2-01`:

- copied the current TeleCoder checkout to `/root/telecoder-sprint2`
- ran the install script with a separate smoke service named
  `telecoder-smoke.service`
- used `/etc/telecoder/telecoder-smoke.env` for the env file
- used `/var/lib/telecoder-smoke` for the data dir
- used port `7081`
- confirmed the service reached `active`
- confirmed `curl http://127.0.0.1:7081/health` returned `{"status":"ok"}`
- confirmed `systemctl restart telecoder-smoke.service` worked and health
  recovered after restart
- confirmed the generated env file and systemd unit contents on the host

## Decisions

- standardize Ubuntu VPS guidance on Bun plus Node 22
- install `acpx` globally for the service host
- treat the repo checkout path as the app working directory instead of copying
  app code into a second managed directory
- keep the default service shape simple and root-owned for now

## Remaining Gaps

- there is no package or release artifact yet; install still starts from a repo
  checkout
- the install path does not yet provision runtime credentials automatically
- there is no automated rollback flow yet
