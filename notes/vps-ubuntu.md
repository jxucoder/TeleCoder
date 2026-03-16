# Ubuntu VPS Install

Date: 2026-03-16

## Baseline

TeleCoder VPS hosts currently standardize on:

- Ubuntu 24.04
- Bun `1.3.10`
- Node `22.x`
- `acpx` `0.3.0`

## Install Flow

1. Clone TeleCoder onto the host, preferably into a stable path such as
   `/opt/telecoder/app`.
2. Run the install script as root from that checkout:

```bash
cd /opt/telecoder/app
sudo scripts/install/install-ubuntu-vps.sh
```

This script:

- installs or upgrades Bun
- installs or upgrades Node 22
- installs `acpx@0.3.0`
- writes `/etc/telecoder/telecoder.env.example`
- creates `/etc/telecoder/telecoder.env` if missing
- writes `/etc/systemd/system/telecoder.service`
- enables and starts the service

## Important Paths

- app checkout: chosen by `--app-dir`, commonly `/opt/telecoder/app`
- env file: `/etc/telecoder/telecoder.env`
- env example: `/etc/telecoder/telecoder.env.example`
- systemd unit: `/etc/systemd/system/telecoder.service`
- TeleCoder data: `/var/lib/telecoder`

## Runtime Auth

Add runtime credentials to `/etc/telecoder/telecoder.env`:

- `TELECODER_POLICY_MODE=observe` for the unattended default
- `TELECODER_POLICY_MODE=locked` for deny-all read-only evaluation
- `TELECODER_POLICY_MODE=standard` when contained repo writes should be allowed
- `OPENAI_API_KEY` for Codex
- `ANTHROPIC_API_KEY` for Claude
- `OPENROUTER_API_KEY` if needed
- `TELECODER_AGENT_CLAUDE_COMMAND` or `TELECODER_AGENT_OPENCODE_COMMAND` when
  host-specific overrides are required

After env changes:

```bash
sudo systemctl restart telecoder.service
```

## Health And Logs

Health:

```bash
curl -sS http://127.0.0.1:7080/health
```

Status and logs:

```bash
sudo systemctl status telecoder.service --no-pager
sudo journalctl -u telecoder.service -n 100 --no-pager
```

Or use the helper:

```bash
sudo scripts/install/check-service.sh
```

## Upgrade Flow

1. Update the checkout.
2. Re-run the install script.
3. Restart the service.

```bash
cd /opt/telecoder/app
git pull
sudo scripts/install/install-ubuntu-vps.sh --no-start
sudo systemctl restart telecoder.service
```

The script is intended to be idempotent for this flow.
